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
	"bytes"
	"net"
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
var baremetalnetworklog = logf.Log.WithName("baremetalnetwork-resource")

type BareMetalNetworkWebhook struct {
	client client.Client
}

func (w *BareMetalNetworkWebhook) SetupWebhookWithManager(mgr ctrl.Manager) {
	w.client = mgr.GetClient()
	hookServer := mgr.GetWebhookServer()

	hookServer.Register("/mutate-baremetal-com-rmb938-v1alpha1-baremetalnetwork", admission.DefaultingWebhookFor(w, &baremetalv1alpha1.BareMetalNetwork{}))
	hookServer.Register("/validate-baremetal-com-rmb938-v1alpha1-baremetalnetwork", admission.ValidatingWebhookFor(w, &baremetalv1alpha1.BareMetalNetwork{}))
}

var _ webhook.Defaulter = &BareMetalNetworkWebhook{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (w *BareMetalNetworkWebhook) Default(obj runtime.Object) {
	r := obj.(*baremetalv1alpha1.BareMetalNetwork)

	baremetalnetworklog.Info("default", "name", r.Name)

	if r.DeletionTimestamp.IsZero() {
		// add the finalizer
		if baremetalapi.HasFinalizer(r, baremetalv1alpha1.BareMetalNetworkFinalizer) == false {
			r.Finalizers = append(r.Finalizers, baremetalv1alpha1.BareMetalNetworkFinalizer)
		}
	}
}

var _ webhook.Validator = &BareMetalNetworkWebhook{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (w *BareMetalNetworkWebhook) ValidateCreate(obj runtime.Object) error {
	r := obj.(*baremetalv1alpha1.BareMetalNetwork)

	baremetalnetworklog.Info("validate create", "name", r.Name)

	var allErrs field.ErrorList

	// validate network
	var networkIP net.IP
	cidrIP, network, err := net.ParseCIDR(r.Spec.CIDR)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("cidr"), r.Spec.CIDR, "invalid cidr"))
	}
	if network != nil && cidrIP != nil {
		networkIP = cidrIP.Mask(network.Mask)
	}

	// validate start
	start := net.ParseIP(r.Spec.Start)
	if start == nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("start"), r.Spec.Start, "invalid start address"))
	} else {
		if network != nil {
			if network.Contains(start) == false {
				allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("start"), r.Spec.Start, "start address is not in cidr"))
			}
		}
	}

	// validate end
	end := net.ParseIP(r.Spec.End)
	if end == nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("start"), r.Spec.End, "invalid end address"))
	} else {
		if network != nil {
			if network.Contains(end) == false {
				allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("end"), r.Spec.End, "end address is not in cidr"))
			}
		}
	}

	// validate ordering
	if start != nil && end != nil {
		if bytes.Compare(start, end) >= 0 {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("start"), r.Spec.Start, "start address must be greater then end address"))
		}
	}

	// validate gateway
	gateway := net.ParseIP(r.Spec.Gateway)
	if gateway == nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("gateway"), r.Spec.End, "invalid gateway address"))
	} else {
		if network != nil {
			if network.Contains(gateway) == false {
				allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("gateway"), r.Spec.End, "gateway address is not in cidr"))
			}
		}
	}

	// Validate nameservers
	for i, ns := range r.Spec.Nameservers {
		nsIP := net.ParseIP(ns)
		if nsIP == nil {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("nameservers").Index(i), ns, "invalid nameserver address"))
		} else {
			nsIP4 := nsIP.To4()
			if nsIP4 != nil {
				nsIP = nsIP4
			}
			if networkIP != nil {
				if len(networkIP) != len(nsIP) {
					allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("nameservers").Index(i), ns, "nameserver ip version is different then cidr ip version"))
				}
			}
		}

		for j, n := range r.Spec.Nameservers {
			if i == j {
				continue
			}

			if ns == n {
				allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("nameservers").Index(i), ns, "duplicate nameserver"))
			}
		}
	}

	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(
		schema.GroupKind{Group: baremetalv1alpha1.GroupVersion.Group, Kind: r.Kind},
		r.Name, allErrs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (w *BareMetalNetworkWebhook) ValidateUpdate(obj runtime.Object, old runtime.Object) error {
	r := obj.(*baremetalv1alpha1.BareMetalNetwork)

	baremetalnetworklog.Info("validate update", "name", r.Name)
	oldBMN := old.(*baremetalv1alpha1.BareMetalNetwork)

	var allErrs field.ErrorList

	if reflect.DeepEqual(r.Spec, oldBMN.Spec) == false {
		allErrs = append(allErrs, field.Forbidden(
			field.NewPath("spec"),
			"Cannot change the spec",
		))
	}

	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(
		schema.GroupKind{Group: baremetalv1alpha1.GroupVersion.Group, Kind: r.Kind},
		r.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (w *BareMetalNetworkWebhook) ValidateDelete(obj runtime.Object) error {
	r := obj.(*baremetalv1alpha1.BareMetalNetwork)

	baremetalnetworklog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
