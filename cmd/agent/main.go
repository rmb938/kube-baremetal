package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
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

type discoveryInput struct {
	SystemUUID types.UID                                    `json:"systemUUID"`
	Hardware   baremetalv1alpha1.BareMetalDiscoveryHardware `json:"hardware"`
}

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

func main() {
	var discoveryURL string
	flag.StringVar(&discoveryURL, "discovery-url", "", "The URL to the discovery server")
	flag.Parse()

	if len(discoveryURL) == 0 {
		cmdLineFile, err := os.Open("/proc/cmdline")
		if err != nil {
			log.Fatalf("Error openning /proc/cmdline %s", err)
		}
		cmdLineBytes, err := ioutil.ReadAll(cmdLineFile)
		if err != nil {
			log.Fatalf("Error reading /proc/cmdline %s", err)
		}

		cmdLineString := string(cmdLineBytes)
		cmdLineArgs := strings.Split(cmdLineString, " ")
		discoveryArg := cmdLineArgs[len(cmdLineArgs)-1]
		discoveryURL = strings.TrimSpace(strings.Split(discoveryArg, "=")[1])

		log.Printf("Using discovery URL %s", discoveryURL)
	}

	log.Printf("Starting Bare Metal Agent")

	hostInfo, err := host.Info()
	if err != nil {
		log.Fatalf("Error gathering host info %s", err)
	}

	virtualMemoryStat, err := mem.VirtualMemory()
	if err != nil {
		log.Fatalf("Error gathering virtual memory %s", err)
	}

	memQty := resource.MustParse(strconv.FormatInt(int64(virtualMemoryStat.Total), 10))

	cpuInfo, err := cpu.Info()
	if err != nil {
		log.Fatalf("Error gathering cpu info %s", err)
	}

	cpuQty := resource.MustParse(strconv.FormatInt(int64(len(cpuInfo)), 10))

	storage := make([]baremetalv1alpha1.BareMetalDiscoveryHardwareStorage, 0)

	lsblkCmd := exec.Command("lsblk", "--json", "-d", "-b", "-e1,7,11", "-o", "name,size,rota,serial,disc-max")
	output, err := lsblkCmd.CombinedOutput()
	if err != nil {
		log.Fatalf("Error gathering storage info: %s: %v", err, string(output))
	}

	lsblk := &lsblk{}
	err = json.Unmarshal(output, lsblk)
	if err != nil {
		log.Fatalf("Error parsing storage info output: %s: %v", err, string(output))
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
		log.Fatalf("Error gathering interface info %s", err)
	}

	for _, interf := range interfaceStat {
		if interf.Name != "lo" && interf.Name != "tunl0" && interf.Name != "ip6tnl0" {
			i := baremetalv1alpha1.BareMetalDiscoveryHardwareNIC{
				Name: interf.Name,
				MAC:  interf.HardwareAddr,
			}

			f, err := os.Open(fmt.Sprintf("/sys/class/net/%s/speed", interf.Name))
			if err != nil {
				log.Fatalf("Error opening file to check speed for interface %s: %s", interf.Name, err)
			}
			b, err := ioutil.ReadAll(f)
			if err != nil {
				if strings.HasSuffix(err.Error(), "invalid argument") == false {
					log.Fatalf("Error reading file to check speed for interface %s: %s", interf.Name, err)
				}
			} else {
				i.Speed = resource.MustParse(strings.Trim(string(b), "\n") + "M")
			}
			nics = append(nics, i)
		}
	}

	input := &discoveryInput{
		SystemUUID: types.UID(hostInfo.HostID),
		Hardware: baremetalv1alpha1.BareMetalDiscoveryHardware{
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

	data, err := json.Marshal(input)
	if err != nil {
		log.Fatalf("Error marshaling discovery input %s", err)
	}

	log.Printf("Making discover request")

	req, err := http.NewRequest(http.MethodPost, discoveryURL+"/discover", bytes.NewBuffer(data))
	if err != nil {
		log.Fatalf("Error creating discovery request %s", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("Error doing discovery request %s", err)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Error reading discover response body %s", err)
	}

	if resp.StatusCode != http.StatusNoContent {
		log.Fatalf("Discovery response returned an error %s", string(body))
	}

}
