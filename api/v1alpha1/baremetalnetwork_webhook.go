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
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var baremetalnetworklog = logf.Log.WithName("baremetalnetwork-resource")

// THIS IS JUST A DUMMY FILE REAL WEBHOOK IMPLEMENTATION IS IN "github.com/rmb938/kube-baremetal/webhooks"

func (r *BareMetalNetwork) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/mutate-baremetal-com-rmb938-v1alpha1-baremetalnetwork,mutating=true,failurePolicy=fail,groups=baremetal.com.rmb938,resources=baremetalnetworks,verbs=create;update,versions=v1alpha1,name=mbaremetalnetwork.kb.io

var _ webhook.Defaulter = &BareMetalNetwork{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *BareMetalNetwork) Default() {
	baremetalnetworklog.Info("default", "name", r.Name)

	// TODO(user): fill in your defaulting logic.
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// +kubebuilder:webhook:verbs=create;update,path=/validate-baremetal-com-rmb938-v1alpha1-baremetalnetwork,mutating=false,failurePolicy=fail,groups=baremetal.com.rmb938,resources=baremetalnetworks,versions=v1alpha1,name=vbaremetalnetwork.kb.io

var _ webhook.Validator = &BareMetalNetwork{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *BareMetalNetwork) ValidateCreate() error {
	baremetalnetworklog.Info("validate create", "name", r.Name)

	// TODO(user): fill in your validation logic upon object creation.
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *BareMetalNetwork) ValidateUpdate(old runtime.Object) error {
	baremetalnetworklog.Info("validate update", "name", r.Name)

	// TODO(user): fill in your validation logic upon object update.
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *BareMetalNetwork) ValidateDelete() error {
	baremetalnetworklog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
