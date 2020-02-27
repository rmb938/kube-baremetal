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
	conditionv1 "github.com/rmb938/kube-baremetal/apis/condition/v1"
	"github.com/rmb938/kube-baremetal/webhook"
	"github.com/rmb938/kube-baremetal/webhook/admission"
)

// log is for logging in this package.
var baremetalinstancelog = logf.Log.WithName("baremetalinstance-resource")

type BareMetalInstanceWebhook struct {
	client client.Client
}

func (w *BareMetalInstanceWebhook) SetupWebhookWithManager(mgr ctrl.Manager) {
	w.client = mgr.GetClient()
	hookServer := mgr.GetWebhookServer()

	hookServer.Register("/mutate-baremetal-com-rmb938-v1alpha1-baremetalinstance", admission.DefaultingWebhookFor(w, &baremetalv1alpha1.BareMetalInstance{}))
	hookServer.Register("/validate-baremetal-com-rmb938-v1alpha1-baremetalinstance", admission.ValidatingWebhookFor(w, &baremetalv1alpha1.BareMetalInstance{}))
}

var _ webhook.Defaulter = &BareMetalInstanceWebhook{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (w *BareMetalInstanceWebhook) Default(obj runtime.Object) {
	r := obj.(*baremetalv1alpha1.BareMetalInstance)

	baremetalinstancelog.Info("default", "name", r.Name)

	if r.DeletionTimestamp.IsZero() {
		// add the finalizer
		if baremetalapi.HasFinalizer(r, baremetalv1alpha1.BareMetalInstanceFinalizer) == false {
			r.Finalizers = append(r.Finalizers, baremetalv1alpha1.BareMetalInstanceFinalizer)
		}
	}

}

var _ webhook.Validator = &BareMetalInstanceWebhook{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (w *BareMetalInstanceWebhook) ValidateCreate(obj runtime.Object) error {
	r := obj.(*baremetalv1alpha1.BareMetalInstance)

	baremetalinstancelog.Info("validate create", "name", r.Name)

	var allErrs field.ErrorList

	if len(r.Status.HardwareName) > 0 {
		allErrs = append(allErrs, field.Forbidden(
			field.NewPath("status").Child("hardwareName"),
			"Cannot set hardware name during creation",
		))
	}

	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(
		schema.GroupKind{Group: baremetalv1alpha1.GroupVersion.Group, Kind: baremetalv1alpha1.BareMetalDiscoveryKind},
		r.Name, allErrs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (w *BareMetalInstanceWebhook) ValidateUpdate(obj runtime.Object, old runtime.Object) error {
	r := obj.(*baremetalv1alpha1.BareMetalInstance)

	baremetalinstancelog.Info("validate update", "name", r.Name)
	oldBMI := old.(*baremetalv1alpha1.BareMetalInstance)

	var allErrs field.ErrorList

	// never allow setting the phase to empty
	if len(oldBMI.Status.Phase) > 0 && len(r.Status.Phase) == 0 {
		allErrs = append(allErrs, field.Forbidden(
			field.NewPath("status").Child("phase"),
			"Cannot set phase to empty string",
		))
	}

	// never allow removing conditions
	var existingConditions []conditionv1.ConditionType
	for _, cond := range oldBMI.Status.GetConditions() {
		existingConditions = append(existingConditions, cond.Type)
	}
	for _, condType := range existingConditions {
		cond := r.Status.GetCondition(condType)
		if cond == nil {
			allErrs = append(allErrs, field.Forbidden(
				field.NewPath("status").Child("conditions"),
				"Cannot remove conditions",
			))
		}
	}

	// never allow changing hardware selector
	if reflect.DeepEqual(r.Spec.Selector, oldBMI.Spec.Selector) == false {
		allErrs = append(allErrs, field.Forbidden(
			field.NewPath("spec").Child("selector"),
			"Cannot change the selector",
		))
	}

	// never allow changing hardware tolerations
	if reflect.DeepEqual(r.Spec.Tolerations, oldBMI.Spec.Tolerations) == false {
		allErrs = append(allErrs, field.Forbidden(
			field.NewPath("spec").Child("tolerations"),
			"Cannot change the tolerations",
		))
	}

	// when not deleting
	if r.DeletionTimestamp.IsZero() == true {
		// Don't allow changing scheduled hardware name if set
		if len(oldBMI.Status.HardwareName) > 0 && r.Status.HardwareName != oldBMI.Status.HardwareName {
			allErrs = append(allErrs, field.Forbidden(
				field.NewPath("status").Child("hardwareName"),
				"Cannot change the scheduled hardware name",
			))
		}
	}

	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(
		schema.GroupKind{Group: baremetalv1alpha1.GroupVersion.Group, Kind: baremetalv1alpha1.BareMetalDiscoveryKind},
		r.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (w *BareMetalInstanceWebhook) ValidateDelete(obj runtime.Object) error {
	r := obj.(*baremetalv1alpha1.BareMetalInstance)

	baremetalinstancelog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
