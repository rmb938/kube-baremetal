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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type BondMode string

const (
	BondModeBalanceRR    BondMode = "balance-rr"
	BondModeActiveBackup BondMode = "active-backup"
	BondModeBalanceXOR   BondMode = "balance-xor"
	BondModeBroadcast    BondMode = "broadcast"
	BondModeLACP         BondMode = "lacp"
	BondModeBalanceTLB   BondMode = "balance-tlb"
	BondModeBalanceALB   BondMode = "balance-alb"
)

var (
	BareMetalHardwareFinalizer = "bmh." + FinalizerPrefix
)

type BareMetalHardwareNICBond struct {
	// The nic names to bond together
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:Required
	Interfaces []string `json:"interfaces"`

	// The bonding mode
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=balance-rr;active-backup;balance-xor;lacp;broadcast;balance-tlb;balance-alb
	Mode BondMode `json:"mode,omitempty"`
}

type BareMetalHardwareNIC struct {
	// The name of the nic
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Bond information for the nic
	// +kubebuilder:validation:Optional
	// +nullable
	Bond *BareMetalHardwareNICBond `json:"bond"`

	// TODO: the network reference the nic is plugged into
}

// BareMetalHardwareSpec defines the desired state of BareMetalHardware
type BareMetalHardwareSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +kubebuilder:validation:Required
	SystemUUID types.UID `json:"systemUUID"`

	// Can the hardware be provisioned into an instance
	// +kubebuilder:validation:Optional
	CanProvision bool `json:"canProvision,omitempty"`

	// The drive to install the image onto
	// +kubebuilder:validation:Optional
	ImageDrive string `json:"imageDrive"`

	// The nics that should be configured
	// +kubebuilder:validation:Optional
	NICS []BareMetalHardwareNIC `json:"nics,omitempty"`
}

// BareMetalHardwareStatus defines the observed state of BareMetalHardware
type BareMetalHardwareStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// The hardware that the discovered system contains
	// +kubebuilder:validation:Optional
	Hardware *BareMetalDiscoveryHardware `json:"hardware,omitempty"`

	// TODO: taints (instances need to tolerate them)

	// TODO: instanceRef

	// TODO: conditions
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=bmh
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="CPU Model",type=string,JSONPath=`.status.hardware.cpu.modelName`
// +kubebuilder:printcolumn:name="CPU Count",type=string,JSONPath=`.status.hardware.cpu.cpus`
// +kubebuilder:printcolumn:name="Ram",type=string,JSONPath=`.status.hardware.ram`

// BareMetalHardware is the Schema for the baremetalhardwares API
type BareMetalHardware struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BareMetalHardwareSpec   `json:"spec,omitempty"`
	Status BareMetalHardwareStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BareMetalHardwareList contains a list of BareMetalHardware
type BareMetalHardwareList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BareMetalHardware `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BareMetalHardware{}, &BareMetalHardwareList{})
}
