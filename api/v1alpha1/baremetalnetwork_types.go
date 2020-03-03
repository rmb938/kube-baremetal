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
)

var (
	BareMetalNetworkFinalizer = "bmn." + FinalizerPrefix
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// BareMetalNetworkSpec defines the desired state of BareMetalNetwork
type BareMetalNetworkSpec struct {
	// +kubebuilder:validation:Required
	CIDR string `json:"cidr"`

	// +kubebuilder:validation:Required
	Gateway string `json:"gateway"`

	// +kubebuilder:validation:Required
	Start string `json:"start"`

	// +kubebuilder:validation:Required
	End string `json:"end"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Nameservers []string `json:"nameservers"`

	// +kubebuilder:validation:Optional
	Search []string `json:"search,omitempty"`
}

// BareMetalNetworkStatus defines the observed state of BareMetalNetwork
type BareMetalNetworkStatus struct {
	// TODO: do we need any status?
	//  a network object is immutable and all validation is in the webhook
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=bmn

// BareMetalNetwork is the Schema for the baremetalnetworks API
type BareMetalNetwork struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Required
	Spec BareMetalNetworkSpec `json:"spec"`

	// +kubebuilder:validation:Optional
	Status BareMetalNetworkStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BareMetalNetworkList contains a list of BareMetalNetwork
type BareMetalNetworkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BareMetalNetwork `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BareMetalNetwork{}, &BareMetalNetworkList{})
}
