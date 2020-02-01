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
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var baremetaldiscoverylog = logf.Log.WithName("baremetaldiscovery-resource")

func (r *BareMetalDiscovery) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/mutate-baremetal-com-rmb938-v1alpha1-baremetaldiscovery,mutating=true,failurePolicy=fail,groups=baremetal.com.rmb938,resources=baremetaldiscoveries,verbs=create;update,versions=v1alpha1,name=mbaremetaldiscovery.kb.io

var _ webhook.Defaulter = &BareMetalDiscovery{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *BareMetalDiscovery) Default() {
	baremetaldiscoverylog.Info("default", "name", r.Name)

	// TODO(user): fill in your defaulting logic.
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// +kubebuilder:webhook:verbs=create;update,path=/validate-baremetal-com-rmb938-v1alpha1-baremetaldiscovery,mutating=false,failurePolicy=fail,groups=baremetal.com.rmb938,resources=baremetaldiscoveries,versions=v1alpha1,name=vbaremetaldiscovery.kb.io

var _ webhook.Validator = &BareMetalDiscovery{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *BareMetalDiscovery) ValidateCreate() error {
	baremetaldiscoverylog.Info("validate create", "name", r.Name)

	// TODO(user): fill in your validation logic upon object creation.
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *BareMetalDiscovery) ValidateUpdate(old runtime.Object) error {
	baremetaldiscoverylog.Info("validate update", "name", r.Name)
	oldBMD := old.(*BareMetalDiscovery)

	var allErrs field.ErrorList

	if r.Spec.SystemUUID != oldBMD.Spec.SystemUUID {
		allErrs = append(allErrs, field.Forbidden(
			field.NewPath("spec").Child("systemUUID"),
			"Cannot change the discovery system uuid",
		))
	}

	if oldBMD.Spec.Hardware != nil && reflect.DeepEqual(r.Spec.Hardware, oldBMD.Spec.Hardware) == false {
		allErrs = append(allErrs, field.Forbidden(
			field.NewPath("spec").Child("hardware"),
			"Cannot change the discovery hardware",
		))
	}

	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(
		schema.GroupKind{Group: GroupVersion.Group, Kind: BareMetalDiscoveryKind},
		r.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *BareMetalDiscovery) ValidateDelete() error {
	baremetaldiscoverylog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
