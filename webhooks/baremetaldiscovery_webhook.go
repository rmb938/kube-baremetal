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

package webhooks

import (
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	baremetalv1alpha1 "github.com/rmb938/kube-baremetal/api/v1alpha1"
	"github.com/rmb938/kube-baremetal/webhook"
	"github.com/rmb938/kube-baremetal/webhook/admission"
)

// log is for logging in this package.
var baremetaldiscoverylog = logf.Log.WithName("baremetaldiscovery-resource")

type BareMetalDiscoveryWebhook struct {
	client client.Client
}

func (w *BareMetalDiscoveryWebhook) SetupWebhookWithManager(mgr ctrl.Manager) {
	w.client = mgr.GetClient()
	hookServer := mgr.GetWebhookServer()

	hookServer.Register("/mutate-baremetal-com-rmb938-v1alpha1-baremetaldiscovery", admission.DefaultingWebhookFor(w, &baremetalv1alpha1.BareMetalDiscovery{}))
	hookServer.Register("/validate-baremetal-com-rmb938-v1alpha1-baremetaldiscovery", admission.ValidatingWebhookFor(w, &baremetalv1alpha1.BareMetalDiscovery{}))
}

var _ webhook.Defaulter = &BareMetalDiscoveryWebhook{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (w *BareMetalDiscoveryWebhook) Default(obj runtime.Object) {
	r := obj.(*baremetalv1alpha1.BareMetalDiscovery)

	baremetaldiscoverylog.Info("default", "name", r.Name)

	// TODO(user): fill in your defaulting logic.
}

var _ webhook.Validator = &BareMetalDiscoveryWebhook{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (w *BareMetalDiscoveryWebhook) ValidateCreate(obj runtime.Object) error {
	r := obj.(*baremetalv1alpha1.BareMetalDiscovery)

	baremetaldiscoverylog.Info("validate create", "name", r.Name)

	// TODO(user): fill in your validation logic upon object creation.
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (w *BareMetalDiscoveryWebhook) ValidateUpdate(obj runtime.Object, old runtime.Object) error {
	r := obj.(*baremetalv1alpha1.BareMetalDiscovery)

	baremetaldiscoverylog.Info("validate update", "name", r.Name)
	oldBMD := old.(*baremetalv1alpha1.BareMetalDiscovery)

	var allErrs field.ErrorList

	// Never allow changing system uuid
	if r.Spec.SystemUUID != oldBMD.Spec.SystemUUID {
		allErrs = append(allErrs, field.Forbidden(
			field.NewPath("spec").Child("systemUUID"),
			"Cannot change the discovery system uuid",
		))
	}

	// never allow changing the hardware is already set
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
		schema.GroupKind{Group: baremetalv1alpha1.GroupVersion.Group, Kind: baremetalv1alpha1.BareMetalDiscoveryKind},
		r.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (w *BareMetalDiscoveryWebhook) ValidateDelete(obj runtime.Object) error {
	r := obj.(*baremetalv1alpha1.BareMetalDiscovery)

	baremetaldiscoverylog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
