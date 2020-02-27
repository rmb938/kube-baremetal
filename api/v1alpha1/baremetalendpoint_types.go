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

	conditionv1 "github.com/rmb938/kube-baremetal/apis/condition/v1"
	kbmeta "github.com/rmb938/kube-baremetal/apis/meta/v1"
)

var (
	BareMetalEndpointFinalizer = "bme." + FinalizerPrefix
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// BareMetalEndpointSpec defines the desired state of BareMetalEndpoint
type BareMetalEndpointSpec struct {
	// If this endpoint is the primary nic
	// +kubebuilder:validation:Required
	Primary bool `json:"primary"`

	// The mac addresses for this endpoint, a bonded nic will have more than 1
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	MACS []string `json:"macs"`

	// The reference to the network object
	// +kubebuilder:validation:Required
	NetworkRef kbmeta.ObjectReference `json:"networkRef"`
}

// We will not enum this
// network controllers may want to set this to different things
// as long as it ends at "Addressed" is all that matters
type BareMetalEndpointStatusPhase string

const (
	BareMetalEndpointStatusPhasePending   BareMetalEndpointStatusPhase = "Pending"
	BareMetalEndpointStatusPhaseAddressed BareMetalEndpointStatusPhase = "Addressed"
)

type BareMetalEndpointStatusAddress struct {
	// +kubebuilder:validation:Required
	IP string `json:"ip"`
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=128
	CIDR int `json:"cidr"`
	// +kubebuilder:validation:Required
	Gateway string `json:"gateway"`
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Nameservers []string `json:"nameservers"`
	// +kubebuilder:validation:Optional
	Search []string `json:"search,omitempty"`
}

// BareMetalEndpointStatus defines the observed state of BareMetalEndpoint
type BareMetalEndpointStatus struct {
	// network controllers may want to set conditions
	conditionv1.StatusConditions `json:",inline"`

	// +kubebuilder:validation:Optional
	Address *BareMetalEndpointStatusAddress `json:"address,omitempty"`

	// +kubebuilder:validation:Optional
	Phase BareMetalEndpointStatusPhase `json:"phase,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=bme
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="STATUS",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="IP",type=string,JSONPath=`.status.address.ip`

// BareMetalEndpoint is the Schema for the baremetalendpoints API
type BareMetalEndpoint struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Required
	Spec BareMetalEndpointSpec `json:"spec"`

	// +kubebuilder:validation:Optional
	Status BareMetalEndpointStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BareMetalEndpointList contains a list of BareMetalEndpoint
type BareMetalEndpointList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BareMetalEndpoint `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BareMetalEndpoint{}, &BareMetalEndpointList{})
}
