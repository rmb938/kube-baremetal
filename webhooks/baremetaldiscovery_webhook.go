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
	"context"
	"net"
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

func validateHardware(hardware *baremetalv1alpha1.BareMetalDiscoveryHardware, startPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if hardware.CPU.CPUS.IsZero() {
		allErrs = append(allErrs, field.Invalid(startPath.Child("hardware").Child("cpu").Child("cpus"), hardware.CPU.CPUS.String(), "CPU quantity cannot be zero"))
	}

	if hardware.Ram.IsZero() {
		allErrs = append(allErrs, field.Invalid(startPath.Child("hardware").Child("ram"), hardware.Ram.String(), "Memory quantity cannot be zero"))
	}

	var storageNames []string
	for i, storage := range hardware.Storage {
		for _, existingName := range storageNames {
			if existingName == storage.Name {
				allErrs = append(allErrs, field.Duplicate(startPath.Child("hardware").Child("storage").Index(i).Child("mac"), storage.Name))
			}
		}

		storageNames = append(storageNames, storage.Name)
	}

	var nicNames []string
	for i, nic := range hardware.NICS {
		_, err := net.ParseMAC(nic.MAC)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(startPath.Child("hardware").Child("nics").Index(i).Child("mac"), nic.MAC, err.Error()))
		}

		for _, existingName := range nicNames {
			if existingName == nic.Name {
				allErrs = append(allErrs, field.Duplicate(startPath.Child("hardware").Child("nics").Index(i).Child("mac"), nic.Name))
			}
		}

		nicNames = append(nicNames, nic.Name)
	}

	return allErrs
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
	ctx := context.Background()
	r := obj.(*baremetalv1alpha1.BareMetalDiscovery)

	baremetaldiscoverylog.Info("validate create", "name", r.Name)

	var allErrs field.ErrorList

	// Block creation if existing BMD
	existingBMD := &baremetalv1alpha1.BareMetalDiscoveryList{}
	err := w.client.List(ctx, existingBMD, client.MatchingFields{"spec.systemUUID": string(r.Spec.SystemUUID)})
	if err != nil {
		allErrs = append(allErrs, field.InternalError(
			field.NewPath("spec").Child("systemUUID"),
			err,
		))
	}

	if len(existingBMD.Items) > 0 {
		allErrs = append(allErrs, field.Duplicate(
			field.NewPath("spec").Child("systemUUID"),
			string(r.Spec.SystemUUID),
		))
	}

	// Block creation if existing BMH
	existingBMH := &baremetalv1alpha1.BareMetalDiscoveryList{}
	err = w.client.List(ctx, existingBMH, client.MatchingFields{"spec.systemUUID": string(r.Spec.SystemUUID)})
	if err != nil {
		allErrs = append(allErrs, field.InternalError(
			field.NewPath("spec").Child("systemUUID"),
			err,
		))
	}

	if len(existingBMH.Items) > 0 {
		allErrs = append(allErrs, field.Duplicate(
			field.NewPath("spec").Child("systemUUID"),
			string(r.Spec.SystemUUID),
		))
	}

	// TODO: block creation if existing CBMH

	// Validate Hardware
	allErrs = append(allErrs, validateHardware(r.Spec.Hardware, field.NewPath("spec"))...)

	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(
		schema.GroupKind{Group: baremetalv1alpha1.GroupVersion.Group, Kind: baremetalv1alpha1.BareMetalDiscoveryKind},
		r.Name, allErrs)
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

	// never allow changing the hardware if it is already set
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
