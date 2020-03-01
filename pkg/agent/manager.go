package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"sync"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	baremetalv1alpha1 "github.com/rmb938/kube-baremetal/api/v1alpha1"
	"github.com/rmb938/kube-baremetal/pkg/agent/action"
)

type Manager struct {
	hardware *baremetalv1alpha1.BareMetalDiscoverySpec

	discoveryURL string
	systemUUID   types.UID

	logger     logr.Logger
	actionLock sync.Mutex

	currentAction action.Action
}

func NewManager(hardware *baremetalv1alpha1.BareMetalDiscoverySpec, discoveryURL string, systemUUID types.UID) *Manager {
	return &Manager{
		hardware: hardware,

		discoveryURL: discoveryURL,
		systemUUID:   systemUUID,

		logger: ctrllog.Log.WithName("manager"),
	}
}

type readyInput struct {
	SystemUUID types.UID `json:"systemUUID"`
	IP         string    `json:"ip"`
}

func (m *Manager) SendReady() error {
	// this doesn't actually form a connection
	// we just use this to find the default ip address
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return fmt.Errorf("error creating connection to find IP address: %v", err)
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)

	input := &readyInput{
		SystemUUID: m.systemUUID,
		IP:         localAddr.IP.String(),
	}
	inputBytes, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("error marshaling ready input: %v", err)
	}

	resp, err := http.Post(m.discoveryURL+"/ready", "application/json", bytes.NewBuffer(inputBytes))
	if err != nil {
		return fmt.Errorf("error sending ready request")
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading ready response body")
	}

	if resp.StatusCode != http.StatusNoContent {
		if resp.StatusCode == http.StatusNotFound {
			return fmt.Errorf("there is no instance waiting for an agent with the system uuid of %s", input.SystemUUID)
		}

		return fmt.Errorf("received an error from ready request %v", string(body))
	}

	return nil
}

func (m *Manager) DoAction(action action.Action) bool {
	m.actionLock.Lock()
	defer m.actionLock.Unlock()

	if m.currentAction == nil {
		m.currentAction = action
		go m.currentAction.Do(m.hardware)
		return true
	}

	return false
}

func (m *Manager) CurrentStatus() (*action.Status, error) {
	m.actionLock.Lock()
	defer m.actionLock.Unlock()

	if m.currentAction == nil {
		return nil, nil
	}

	return m.currentAction.Status()
}
