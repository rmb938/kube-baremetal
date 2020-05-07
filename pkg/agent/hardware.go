package agent

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/host"
	"github.com/shirou/gopsutil/mem"
	"github.com/shirou/gopsutil/net"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"

	baremetalv1alpha1 "github.com/rmb938/kube-baremetal/api/v1alpha1"
)

type blockDevices struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	Rota    bool   `json:"rota"`
	Serial  string `json:"serial"`
	DiscMax int64  `json:"disc-max"`
}

type lsblk struct {
	BlockDevices []blockDevices `json:"blockdevices"`
}

type discoveryHardware struct {
	SystemUUID types.UID                                     `json:"systemUUID"`
	Hardware   *baremetalv1alpha1.BareMetalDiscoveryHardware `json:"hardware,omitempty"`
}

func DiscoverHardware() (*discoveryHardware, error) {
	hostInfo, err := host.Info()
	if err != nil {
		return nil, fmt.Errorf("error gathering host info: %v", err)
	}

	virtualMemoryStat, err := mem.VirtualMemory()
	if err != nil {
		return nil, fmt.Errorf("error gathering virtual memory: %v", err)
	}

	memQty := resource.MustParse(strconv.FormatInt(int64(virtualMemoryStat.Total), 10))

	cpuInfo, err := cpu.Info()
	if err != nil {
		return nil, fmt.Errorf("error gathering cpu info: %v", err)
	}

	cpuQty := resource.MustParse(strconv.FormatInt(int64(len(cpuInfo)), 10))

	storage := make([]baremetalv1alpha1.BareMetalDiscoveryHardwareStorage, 0)

	lsblkCmd := exec.Command("lsblk", "--json", "-d", "-b", "-e1,7,11", "-o", "name,size,rota,serial,disc-max")
	output, err := lsblkCmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("error gathering storage info: %v, lsblk_output %s", err, string(output))
	}

	lsblk := &lsblk{}
	err = json.Unmarshal(output, lsblk)
	if err != nil {
		return nil, fmt.Errorf("error parsing storage info: %v, lsblk_output %s", err, string(output))
	}

	for _, blockDevice := range lsblk.BlockDevices {
		s := baremetalv1alpha1.BareMetalDiscoveryHardwareStorage{
			Name:       blockDevice.Name,
			Size:       resource.MustParse(strconv.FormatInt(blockDevice.Size, 10)),
			Serial:     strings.TrimSpace(blockDevice.Serial),
			Rotational: blockDevice.Rota,
			Trim:       false,
		}

		if blockDevice.DiscMax > 0 {
			s.Trim = true
		}

		storage = append(storage, s)
	}

	nics := make([]baremetalv1alpha1.BareMetalDiscoveryHardwareNIC, 0)
	interfaceStat, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("error gathering interface memory: %v", err)
	}

	for _, interf := range interfaceStat {
		if interf.Name != "lo" && interf.Name != "tunl0" && interf.Name != "ip6tnl0" {
			i := baremetalv1alpha1.BareMetalDiscoveryHardwareNIC{
				Name: interf.Name,
				MAC:  interf.HardwareAddr,
			}

			f, err := os.Open(fmt.Sprintf("/sys/class/net/%s/tx_queue_len", interf.Name))
			if err != nil {
				return nil, fmt.Errorf("error opening file to check speed for interface %s: %v", interf.Name, err)
			}
			b, err := ioutil.ReadAll(f)
			if err != nil {
				return nil, fmt.Errorf("error reading file to check speed for interface %s: %v", interf.Name, err)
			} else {
				i.Speed = resource.MustParse(strings.Trim(string(b), "\n") + "M")
			}
			nics = append(nics, i)
		}
	}

	discovery := &discoveryHardware{
		SystemUUID: types.UID(hostInfo.HostID),
		Hardware: &baremetalv1alpha1.BareMetalDiscoveryHardware{
			Ram: memQty,
			CPU: baremetalv1alpha1.BareMetalDiscoveryHardwareCPU{
				ModelName:    cpuInfo[0].ModelName,
				Architecture: runtime.GOARCH,
				CPUS:         cpuQty,
			},
			Storage: storage,
			NICS:    nics,
		},
	}

	return discovery, nil
}
