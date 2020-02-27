package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	net2 "net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/host"
	"github.com/shirou/gopsutil/mem"
	"github.com/shirou/gopsutil/net"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

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

var (
	setupLog = ctrl.Log.WithName("setup")
)

func main() {
	var discoveryURL string
	flag.StringVar(&discoveryURL, "discovery-url", "", "The URL to the discovery server")
	flag.Parse()

	ctrl.SetLogger(zap.New(func(o *zap.Options) {
		o.Development = true
	}))

	if len(discoveryURL) == 0 {
		cmdLineFile, err := os.Open("/proc/cmdline")
		if err != nil {
			setupLog.Error(err, "Error opening /proc/cmdline")
			os.Exit(1)
		}
		cmdLineBytes, err := ioutil.ReadAll(cmdLineFile)
		if err != nil {
			setupLog.Error(err, "Error reading /proc/cmdline")
			os.Exit(1)
		}

		cmdLineString := string(cmdLineBytes)
		cmdLineArgs := strings.Split(cmdLineString, " ")
		discoveryArg := cmdLineArgs[len(cmdLineArgs)-1]
		discoveryURL = strings.TrimSpace(strings.Split(discoveryArg, "=")[1])

		setupLog.Info("Using discovery URL", "url", discoveryURL)
	}

	setupLog.Info("Starting Bare Metal Agent")

	hostInfo, err := host.Info()
	if err != nil {
		setupLog.Error(err, "Error gathering host info")
		os.Exit(1)
	}

	virtualMemoryStat, err := mem.VirtualMemory()
	if err != nil {
		setupLog.Error(err, "Error gathering virtual memory")
		os.Exit(1)
	}

	memQty := resource.MustParse(strconv.FormatInt(int64(virtualMemoryStat.Total), 10))

	cpuInfo, err := cpu.Info()
	if err != nil {
		setupLog.Error(err, "Error gathering cpu info")
		os.Exit(1)
	}

	cpuQty := resource.MustParse(strconv.FormatInt(int64(len(cpuInfo)), 10))

	storage := make([]baremetalv1alpha1.BareMetalDiscoveryHardwareStorage, 0)

	lsblkCmd := exec.Command("lsblk", "--json", "-d", "-b", "-e1,7,11", "-o", "name,size,rota,serial,disc-max")
	output, err := lsblkCmd.CombinedOutput()
	if err != nil {
		setupLog.Error(err, "Error gathering storage info", "lsblk_output", string(output))
		os.Exit(1)
	}

	lsblk := &lsblk{}
	err = json.Unmarshal(output, lsblk)
	if err != nil {
		setupLog.Error(err, "Error parsing storage info output", "lsblk_output", string(output))
		os.Exit(1)
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
		setupLog.Error(err, "Error gathering interface info")
		os.Exit(1)
	}

	for _, interf := range interfaceStat {
		if interf.Name != "lo" && interf.Name != "tunl0" && interf.Name != "ip6tnl0" {
			i := baremetalv1alpha1.BareMetalDiscoveryHardwareNIC{
				Name: interf.Name,
				MAC:  interf.HardwareAddr,
			}

			f, err := os.Open(fmt.Sprintf("/sys/class/net/%s/speed", interf.Name))
			if err != nil {
				setupLog.Error(err, "Error opening file to check speed for interface", "interface_name", interf.Name)
				os.Exit(1)
			}
			b, err := ioutil.ReadAll(f)
			if err != nil {
				if strings.HasSuffix(err.Error(), "invalid argument") == false {
					setupLog.Error(err, "Error reading file to check speed for interface", "interface_name", interf.Name)
					os.Exit(1)
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
		setupLog.Error(err, "Error marshaling discovery input")
		os.Exit(1)
	}

	setupLog.Info("Making discover request")

	req, err := http.NewRequest(http.MethodPost, discoveryURL+"/discover", bytes.NewBuffer(data))
	if err != nil {
		setupLog.Error(err, "Error creating discovery request")
		os.Exit(1)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		setupLog.Error(err, "Error doing discovery request")
		os.Exit(1)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		setupLog.Error(err, "Error reading discover response body")
		os.Exit(1)
	}

	if resp.StatusCode != http.StatusNoContent {
		setupLog.Error(nil, "Discovery response returned an error", "body", string(body))
		os.Exit(1)
	}

	router := gin.Default()

	signalChan := ctrl.SetupSignalHandler()

	srv := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	// TODO: send ready (uuid and ip)

	httpListener, err := net2.Listen("tcp", srv.Addr)
	if err != nil {
		setupLog.Error(err, "Error listening on "+srv.Addr)
		os.Exit(1)
	}

	go func() {
		wait.Until(func() {
			// TODO: heartbeat here (uuid)
		}, 30*time.Second, signalChan)
	}()

	go func() {
		setupLog.Info("Starting agent http server")

		if err := srv.Serve(httpListener); err != nil && err != http.ErrServerClosed {
			setupLog.Error(err, "Error when serving agent server")
		}
	}()

	<-signalChan
	setupLog.Info("Stopping agent http server")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		setupLog.Error(err, "Error while stopping agent server")
		os.Exit(1)
	}

}
