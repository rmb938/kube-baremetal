/*

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type BareMetalDiscoveryHardwareCPU struct {
	// The model name of the CPU
	// +kubebuilder:validation:Required
	ModelName string `json:"modelName"`

	// The architecture of the CPU
	// +kubebuilder:validation:Required
	Architecture string `json:"architecture"`

	// The number of CPUs
	// +kubebuilder:validation:Required
	CPUS resource.Quantity `json:"cpus"`
}

// is disc-max >0 the device supports trim
type BareMetalDiscoveryHardwareStorage struct {
	// The name of the storage device
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// The size of the storage device
	// +kubebuilder:validation:Required
	Size resource.Quantity `json:"size"`

	// If the device is a rotational device
	// +kubebuilder:validation:Optional
	Rotational bool `json:"rotational"`

	// If the device supports trim
	// +kubebuilder:validation:Optional
	Trim bool `json:"trim"`

	// The device's serial number
	// +kubebuilder:validation:Optional
	Serial string `json:"serial,omitempty"`
}

type BareMetalDiscoveryHardwareNIC struct {
	// The name of the NIC
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// The mac address of the NIC
	// +kubebuilder:validation:Required
	MAC string `json:"mac"`

	// sometimes this can be -1 like in vms so don't multiply
	// The speed of the NIC
	// +kubebuilder:validation:Required
	Speed resource.Quantity `json:"speed"`
}

type BareMetalDiscoveryHardware struct {
	// The system's cpu information
	// +kubebuilder:validation:Required
	CPU BareMetalDiscoveryHardwareCPU `json:"cpu"`

	// The amount of memory in the system
	// +kubebuilder:validation:Required
	Ram resource.Quantity `json:"ram"`

	// A list of the system's storage devices
	Storage []BareMetalDiscoveryHardwareStorage `json:"storage"`

	// A list of she system's nics
	NICS []BareMetalDiscoveryHardwareNIC `json:"nics"`
}

// BareMetalDiscoverySpec defines the desired state of BareMetalDiscovery
type BareMetalDiscoverySpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +kubebuilder:validation:Required
	SystemUUID types.UID `json:"systemUUID"`

	// The hardware that the discovered system contains
	// +kubebuilder:validation:Optional
	Hardware *BareMetalDiscoveryHardware `json:"hardware,omitempty"`
}

// BareMetalDiscoveryStatus defines the observed state of BareMetalDiscovery
type BareMetalDiscoveryStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,shortName=bmd
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="CPU Model",type=string,JSONPath=`.spec.hardware.cpu.modelName`
// +kubebuilder:printcolumn:name="CPU Count",type=string,JSONPath=`.spec.hardware.cpu.cpus`
// +kubebuilder:printcolumn:name="Ram",type=string,JSONPath=`.spec.hardware.ram`

// BareMetalDiscovery is the Schema for the baremetaldiscoveries API
type BareMetalDiscovery struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BareMetalDiscoverySpec   `json:"spec,omitempty"`
	Status BareMetalDiscoveryStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BareMetalDiscoveryList contains a list of BareMetalDiscovery
type BareMetalDiscoveryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BareMetalDiscovery `json:"items"`
}

const BareMetalDiscoveryKind = "BareMetalDiscovery"

func init() {
	SchemeBuilder.Register(&BareMetalDiscovery{}, &BareMetalDiscoveryList{})
}
