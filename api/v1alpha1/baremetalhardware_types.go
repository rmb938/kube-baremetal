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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	conditionv1 "github.com/rmb938/kube-baremetal/apis/condition/v1"
	kbmeta "github.com/rmb938/kube-baremetal/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// +kubebuilder:validation:Enum=balance-rr;active-backup;balance-xor;lacp;broadcast;balance-tlb;balance-alb
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

	BareMetalHardwareTaintKeyNotReady   = "hardware." + GroupVersion.Group + "/not-ready"
	BareMetalHardwareTaintKeyNoSchedule = "hardware." + GroupVersion.Group + "/unschedulable"
)

type BareMetalHardwareNICBond struct {
	// The nic names to bond together
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:Required
	Interfaces []string `json:"interfaces"`

	// The bonding mode
	// +kubebuilder:validation:Optional
	Mode BondMode `json:"mode,omitempty"`
}

type BareMetalHardwareNIC struct {
	// The name of the nic
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// If the nic is the primary nic
	// +kubebuilder:validation:Required
	Primary bool `json:"primary"`

	// Bond information for the nic
	// +kubebuilder:validation:Optional
	Bond *BareMetalHardwareNICBond `json:"bond,omitempty"`

	// The reference to the network object
	// +kubebuilder:validation:Required
	NetworkRef kbmeta.ObjectReference `json:"networkRef"`
}

// BareMetalHardwareSpec defines the desired state of BareMetalHardware
type BareMetalHardwareSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +kubebuilder:validation:Required
	SystemUUID types.UID `json:"systemUUID"`

	// Can the hardware be provisioned into an instance
	// +kubebuilder:validation:Optional
	CanProvision bool `json:"canProvision"`

	// The drive to install the image onto
	// +kubebuilder:validation:Optional
	ImageDrive string `json:"imageDrive,omitempty"`

	// The nics that should be configured
	// +kubebuilder:validation:Optional
	NICS []BareMetalHardwareNIC `json:"nics,omitempty"`

	// Taints on the hardware
	// +kubebuilder:validation:Optional
	Taints []corev1.Taint `json:"taints,omitempty"`
}

type BareMetalHardwareStatusInstanceRef struct {
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// +kubebuilder:validation:Required
	Namespace string `json:"namespace"`

	// +kubebuilder:validation:Required
	UID types.UID `json:"uid"`
}

// BareMetalHardwareStatus defines the observed state of BareMetalHardware
type BareMetalHardwareStatus struct {
	conditionv1.StatusConditions `json:",inline"`
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// The reference to the instance running on the hardware
	// +kubebuilder:validation:Optional
	InstanceRef *BareMetalHardwareStatusInstanceRef `json:"instanceRef,omitempty"`

	// The hardware that the discovered system contains
	// +kubebuilder:validation:Optional
	Hardware *BareMetalDiscoveryHardware `json:"hardware,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=bmh
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="CPU Model",type=string,JSONPath=`.status.hardware.cpu.modelName`
// +kubebuilder:printcolumn:name="CPU Count",type=string,JSONPath=`.status.hardware.cpu.cpus`
// +kubebuilder:printcolumn:name="Ram",type=string,JSONPath=`.status.hardware.ram`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// BareMetalHardware is the Schema for the baremetalhardwares API
type BareMetalHardware struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Required
	Spec BareMetalHardwareSpec `json:"spec"`

	// +kubebuilder:validation:Optional
	Status BareMetalHardwareStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BareMetalHardwareList contains a list of BareMetalHardware
type BareMetalHardwareList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BareMetalHardware `json:"items"`
}

const (
	// Condition Types
	BareMetalHardwareConditionTypeHardwareSet     conditionv1.ConditionType = "HardwareSet"
	BareMetalHardwareConditionTypeImageDriveValid conditionv1.ConditionType = "ImageDriveValid"
	BareMetalHardwareConditionTypeNicsValid       conditionv1.ConditionType = "NicsValid"

	// Condition Reasons
	BareMetalHardwareHardwareIsSetConditionReason    string = "HardwareIsSet"
	BareMetalHardwareHardwareIsNotSetConditionReason string = "HardwareIsNotSet"

	BareMetalHardwareValidImageDriveConditionReason    string = "ValidImageDrive"
	BareMetalHardwareInvalidImageDriveConditionReason  string = "InvalidImageDrive"
	BareMetalHardwareImageDriveIsNotSetConditionReason string = "ImageDriveIsNotSet"

	BareMetalHardwareValidNicsConditionReason     string = "ValidNics"
	BareMetalHardwareInvalidNicsConditionReason   string = "InvalidNics"
	BareMetalHardwareNicsAreNotSetConditionReason string = "NicsAreNotSet"

	// Event Reasons
	BareMetalHardwareNotSchedulableEventReason string = "HardwareNotSchedulable"
	BareMetalHardwareSchedulableEventReason    string = "HardwareSchedulable"

	BareMetalHardwareReadyEventReason    string = "HardwareReady"
	BareMetalHardwareNotReadyEventReason string = "HardwareNotReady"

	BareMetalHardwareDiscoveryNotFoundEventReason   string = "DiscoveryNotFound"
	BareMetalHardwareDiscoveryNoHardwareEventReason string = "DiscoveryNoHardware"
	BareMetalHardwareManyDiscoveryFoundEventReason  string = "ManyDiscoveryFound"
	BareMetalHardwareDiscoveryFoundEventReason      string = "DiscoveryFound"

	BareMetalHardwareCleaningEventReason string = "HardwareCleaning"
	BareMetalHardwareCleanedEventReason  string = "HardwareCleaned"
)

func init() {
	SchemeBuilder.Register(&BareMetalHardware{}, &BareMetalHardwareList{})
}
