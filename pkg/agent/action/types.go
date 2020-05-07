package action

import baremetalv1alpha1 "github.com/rmb938/kube-baremetal/api/v1alpha1"

type Type string

const (
	ImagingActionType  Type = "Imaging"
	CleaningActionType Type = "Cleaning"
)

type Status struct {
	Type  Type   `json:"type"`
	Done  bool   `json:"done"`
	Error string `json:"error,omitempty"`
}

type Action interface {
	Do(hardware *baremetalv1alpha1.BareMetalDiscoveryHardware)
	Status() (*Status, error)
}
