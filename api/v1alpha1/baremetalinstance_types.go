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

	conditionv1 "github.com/rmb938/kube-baremetal/apis/condition/v1"
)

var (
	BareMetalInstanceFinalizer = "bmi." + FinalizerPrefix
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// BareMetalInstanceSpec defines the desired state of BareMetalInstance
type BareMetalInstanceSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +kubebuilder:validation:Optional
	Selector map[string]string `json:"hardwareSelector,omitempty"`

	// +kubebuilder:validation:Optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
}

// +kubebuilder:validation:Enum=Pending;Provisioning;Imaging;Running;Cleaning;Terminating;Terminated
type BareMetalInstanceStatusPhase string

const (
	BareMetalInstanceStatusPhasePending      BareMetalInstanceStatusPhase = "Pending"
	BareMetalInstanceStatusPhaseProvisioning BareMetalInstanceStatusPhase = "Provisioning"
	BareMetalInstanceStatusPhaseRunning      BareMetalInstanceStatusPhase = "Running"
	BareMetalInstanceStatusPhaseCleaning     BareMetalInstanceStatusPhase = "Cleaning"
	BareMetalInstanceStatusPhaseTerminating  BareMetalInstanceStatusPhase = "Terminating"
	BareMetalInstanceStatusPhaseTerminated   BareMetalInstanceStatusPhase = "Terminated"
)

type BareMetalInstanceStatusAgentInfo struct {
	// +kubebuilder:validation:Required
	IP string `json:"ip"`

	// TODO: do we want to put anything else here?
}

// BareMetalInstanceStatus defines the observed state of BareMetalInstance
type BareMetalInstanceStatus struct {
	conditionv1.StatusConditions `json:",inline"`
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +kubebuilder:validation:Optional
	AgentInfo *BareMetalInstanceStatusAgentInfo `json:"agentInfo,omitempty"`

	// +kubebuilder:validation:Optional
	HardwareName string `json:"hardwareName,omitempty"`

	// +kubebuilder:validation:Optional
	Phase BareMetalInstanceStatusPhase `json:"phase,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=bmi
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="STATUS",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="HARDWARE",type=string,JSONPath=`.status.hardwareName`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// BareMetalInstance is the Schema for the baremetalinstances API
type BareMetalInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Required
	Spec BareMetalInstanceSpec `json:"spec"`

	// +kubebuilder:validation:Optional
	Status BareMetalInstanceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BareMetalInstanceList contains a list of BareMetalInstance
type BareMetalInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BareMetalInstance `json:"items"`
}

const (
	// Condition Types
	BareMetalHardwareConditionTypeInstanceScheduled conditionv1.ConditionType = "InstanceScheduled"
	BareMetalHardwareConditionTypeInstanceNetworked conditionv1.ConditionType = "InstanceNetworkConfigured"
	BareMetalHardwareConditionTypeInstanceImaged    conditionv1.ConditionType = "InstanceImaged"
	BareMetalHardwareConditionTypeInstanceCleaned   conditionv1.ConditionType = "InstanceCleaned"

	// Condition Reasons
	BareMetalInstanceImagingFailedConditionReason string = "ImagingFailed"

	// Event Reasons
	BareMetalInstanceScheduleEventReason   string = "InstanceScheduled"
	BareMetalInstanceUnscheduleEventReason string = "InstanceUnscheduled"

	BareMetalInstanceProvisioningEventReason string = "InstanceProvisioning"

	BareMetalInstanceNetworkingEventReason string = "InstanceNetworking"
	BareMetalInstanceNetworkedEventReason  string = "InstanceNetworked"

	BareMetalInstanceNoAgentEventReason string = "InstanceNoAgent"

	BareMetalInstanceNotCleanedEventReason string = "InstanceNotCleaned"
	BareMetalInstanceCleaningEventReason   string = "InstanceCleaning"
	BareMetalInstanceCleanedEventReason    string = "InstanceCleaned"
)

func init() {
	SchemeBuilder.Register(&BareMetalInstance{}, &BareMetalInstanceList{})
}
