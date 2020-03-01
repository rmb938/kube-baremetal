package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"github.com/rmb938/kube-baremetal/pkg/agent"
)

var (
	setupLog = ctrllog.Log.WithName("setup")
)

func main() {
	var discoveryURL string
	flag.StringVar(&discoveryURL, "discovery-url", "", "The URL to the discovery server")
	flag.Parse()

	ctrllog.SetLogger(zap.New(func(o *zap.Options) {
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

	_, err := url.Parse(discoveryURL)
	if err != nil {
		setupLog.Error(err, "Error parsing discovery url")
		os.Exit(1)
	}

	setupLog.Info("Starting Bare Metal Agent")

	hardware, err := agent.DiscoverHardware()
	if err != nil {
		setupLog.Error(err, "Error discovering hardware")
		os.Exit(1)
	}

	data, err := json.Marshal(hardware)
	if err != nil {
		setupLog.Error(err, "Error marshaling discovery hardware")
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

	signalChan := signals.SetupSignalHandler()

	go func() {
		wait.Until(func() {
			// TODO: heartbeat here (uuid)
		}, 30*time.Second, signalChan)
	}()

	manager := agent.NewManager(hardware, discoveryURL, hardware.SystemUUID)

	server := agent.NewServer(":10443", manager)

	err = server.Run(signalChan)
	if err != nil {
		setupLog.Error(err, "Error while running agent server")
		os.Exit(1)
	}

}
