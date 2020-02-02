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

	baremetalapi "github.com/rmb938/kube-baremetal/api"
	baremetalv1alpha1 "github.com/rmb938/kube-baremetal/api/v1alpha1"
	"github.com/rmb938/kube-baremetal/webhook"
	"github.com/rmb938/kube-baremetal/webhook/admission"
)

// log is for logging in this package.
var baremetalhardwarelog = logf.Log.WithName("baremetalhardware-resource")

type BareMetalHardwareWebhook struct {
	client client.Client
}

func (w *BareMetalHardwareWebhook) SetupWebhookWithManager(mgr ctrl.Manager) {
	w.client = mgr.GetClient()
	hookServer := mgr.GetWebhookServer()

	hookServer.Register("/mutate-baremetal-com-rmb938-v1alpha1-baremetalhardware", admission.DefaultingWebhookFor(w, &baremetalv1alpha1.BareMetalHardware{}))
	hookServer.Register("/validate-baremetal-com-rmb938-v1alpha1-baremetalhardware", admission.ValidatingWebhookFor(w, &baremetalv1alpha1.BareMetalHardware{}))
}

var _ webhook.Defaulter = &BareMetalHardwareWebhook{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (w *BareMetalHardwareWebhook) Default(obj runtime.Object) {
	r := obj.(*baremetalv1alpha1.BareMetalHardware)

	baremetalhardwarelog.Info("default", "name", r.Name)

	if r.DeletionTimestamp.IsZero() {
		// add the finalizer
		if baremetalapi.HasFinalizer(r, baremetalv1alpha1.BareMetalHardwareFinalizer) == false {
			r.Finalizers = append(r.Finalizers, baremetalv1alpha1.BareMetalHardwareFinalizer)
		}

		// set the default nic bond mode
		for _, nic := range r.Spec.NICS {
			if nic.Bond != nil {
				if len(nic.Bond.Mode) == 0 {
					nic.Bond.Mode = baremetalv1alpha1.BondModeBalanceRR
				}
			}
		}
	}
}

var _ webhook.Validator = &BareMetalHardwareWebhook{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (w *BareMetalHardwareWebhook) ValidateCreate(obj runtime.Object) error {
	r := obj.(*baremetalv1alpha1.BareMetalHardware)

	baremetalhardwarelog.Info("validate create", "name", r.Name)

	// TODO(user): fill in your validation logic upon object creation.
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (w *BareMetalHardwareWebhook) ValidateUpdate(obj runtime.Object, old runtime.Object) error {
	r := obj.(*baremetalv1alpha1.BareMetalHardware)

	baremetalhardwarelog.Info("validate update", "name", r.Name)
	oldBMH := old.(*baremetalv1alpha1.BareMetalHardware)

	var allErrs field.ErrorList

	if r.DeletionTimestamp.IsZero() == false {

		// don't allow changing spec when deleting
		if reflect.DeepEqual(r.Spec, oldBMH.Spec) == false {
			allErrs = append(allErrs, field.Forbidden(
				field.NewPath("spec"),
				"Cannot change spec when resource is deleting",
			))
		}
	}

	// Never allow changing system uuid
	if r.Spec.SystemUUID != oldBMH.Spec.SystemUUID {
		allErrs = append(allErrs, field.Forbidden(
			field.NewPath("spec").Child("systemUUID"),
			"Cannot change the system uuid",
		))
	}

	// never allow changing hardware is already set
	if oldBMH.Status.Hardware != nil && reflect.DeepEqual(r.Status.Hardware, oldBMH.Status.Hardware) == false {
		allErrs = append(allErrs, field.Forbidden(
			field.NewPath("status").Child("hardware"),
			"Cannot change the hardware",
		))
	}

	// TODO: prevent changing things when instanceRef is not null (only allow changing CanProvision)

	// TODO: prevent changing things when CanProvision is true (only allow changing CanProvision)

	// TODO: don't allow removing instanceRef when the instance still exists

	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(
		schema.GroupKind{Group: baremetalv1alpha1.GroupVersion.Group, Kind: baremetalv1alpha1.BareMetalDiscoveryKind},
		r.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (w *BareMetalHardwareWebhook) ValidateDelete(obj runtime.Object) error {
	r := obj.(*baremetalv1alpha1.BareMetalHardware)
	baremetalhardwarelog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
