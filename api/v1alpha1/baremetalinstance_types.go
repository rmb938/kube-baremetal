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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// BareMetalInstanceSpec defines the desired state of BareMetalInstance
type BareMetalInstanceSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of BareMetalInstance. Edit BareMetalInstance_types.go to remove/update
	Foo string `json:"foo,omitempty"`
}

// BareMetalInstanceStatus defines the observed state of BareMetalInstance
type BareMetalInstanceStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=bmi
// +kubebuilder:subresource:status

// BareMetalInstance is the Schema for the baremetalinstances API
type BareMetalInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BareMetalInstanceSpec   `json:"spec,omitempty"`
	Status BareMetalInstanceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BareMetalInstanceList contains a list of BareMetalInstance
type BareMetalInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BareMetalInstance `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BareMetalInstance{}, &BareMetalInstanceList{})
}
