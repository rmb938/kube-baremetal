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
	"fmt"
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
	conditionv1 "github.com/rmb938/kube-baremetal/apis/condition/v1"
	"github.com/rmb938/kube-baremetal/webhook"
	"github.com/rmb938/kube-baremetal/webhook/admission"
)

// log is for logging in this package.
var baremetalendpointlog = logf.Log.WithName("baremetalendpoint-resource")

type BareMetalEndpointWebhook struct {
	client client.Client
}

func (w *BareMetalEndpointWebhook) SetupWebhookWithManager(mgr ctrl.Manager) {
	w.client = mgr.GetClient()
	hookServer := mgr.GetWebhookServer()

	hookServer.Register("/mutate-baremetal-com-rmb938-v1alpha1-baremetalendpoint", admission.DefaultingWebhookFor(w, &baremetalv1alpha1.BareMetalEndpoint{}))
	hookServer.Register("/validate-baremetal-com-rmb938-v1alpha1-baremetalendpoint", admission.ValidatingWebhookFor(w, &baremetalv1alpha1.BareMetalEndpoint{}))
}

var _ webhook.Defaulter = &BareMetalEndpointWebhook{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (w *BareMetalEndpointWebhook) Default(obj runtime.Object) {
	r := obj.(*baremetalv1alpha1.BareMetalEndpoint)

	baremetalendpointlog.Info("default", "name", r.Name)

	if r.DeletionTimestamp.IsZero() {
		// add the finalizer
		if baremetalapi.HasFinalizer(r, baremetalv1alpha1.BareMetalEndpointFinalizer) == false {
			r.Finalizers = append(r.Finalizers, baremetalv1alpha1.BareMetalEndpointFinalizer)
		}
	}
}

var _ webhook.Validator = &BareMetalEndpointWebhook{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (w *BareMetalEndpointWebhook) ValidateCreate(obj runtime.Object) error {
	r := obj.(*baremetalv1alpha1.BareMetalEndpoint)

	baremetalendpointlog.Info("validate create", "name", r.Name)

	var allErrs field.ErrorList

	// Block creation when address is set
	if r.Status.Address != nil {
		allErrs = append(allErrs, field.Forbidden(field.NewPath("status").Child("address"), "Cannot have address set when creating"))
	}

	if r.Spec.Bond != nil {

		// validate macs
		for i, mac := range r.Spec.Bond.MACS {
			_, err := net.ParseMAC(mac)
			if err != nil {
				allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("bond").Child("macs").Index(i), mac, err.Error()))
			}

			// check to make sure macs are unique
			for j, m := range r.Spec.Bond.MACS {
				if i == j {
					continue
				}
				if mac == m {
					allErrs = append(allErrs, field.Duplicate(field.NewPath("spec").Child("bond").Child("macs").Index(i), "duplicate bond mac"))
				}
			}
		}

	}

	// validate mac
	_, err := net.ParseMAC(r.Spec.MAC)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("mac"), r.Spec.MAC, err.Error()))
	}

	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(
		schema.GroupKind{Group: baremetalv1alpha1.GroupVersion.Group, Kind: r.Kind},
		r.Name, allErrs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (w *BareMetalEndpointWebhook) ValidateUpdate(obj runtime.Object, old runtime.Object) error {
	r := obj.(*baremetalv1alpha1.BareMetalEndpoint)

	baremetalendpointlog.Info("validate update", "name", r.Name)
	oldBME := old.(*baremetalv1alpha1.BareMetalEndpoint)

	var allErrs field.ErrorList

	// never allow setting the phase to empty
	if len(oldBME.Status.Phase) > 0 && len(r.Status.Phase) == 0 {
		allErrs = append(allErrs, field.Forbidden(
			field.NewPath("status").Child("phase"),
			"Cannot set phase to empty string",
		))
	}

	// never allow removing conditions
	var existingConditions []conditionv1.ConditionType
	for _, cond := range oldBME.Status.GetConditions() {
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

	// never allow changing mac
	if reflect.DeepEqual(r.Spec.MAC, oldBME.Spec.MAC) == false {
		allErrs = append(allErrs, field.Forbidden(
			field.NewPath("spec").Child("mac"),
			"Cannot change the mac",
		))
	}

	// never allow changing bond
	if reflect.DeepEqual(r.Spec.Bond, oldBME.Spec.Bond) == false {
		allErrs = append(allErrs, field.Forbidden(
			field.NewPath("spec").Child("bond"),
			"Cannot change the bond",
		))
	}

	// never allow changing network ref
	if reflect.DeepEqual(r.Spec.NetworkRef, oldBME.Spec.NetworkRef) == false {
		allErrs = append(allErrs, field.Forbidden(
			field.NewPath("spec").Child("networkRef"),
			"Cannot change the networkRef",
		))
	}

	if r.Status.Address != nil {
		// never allow changing address if it is already set
		if reflect.DeepEqual(r.Status.Address, oldBME.Status.Address) == false {
			allErrs = append(allErrs, field.Forbidden(
				field.NewPath("status").Child("address"),
				"Cannot change the address",
			))
		} else {
			// validate network
			var networkIP net.IP
			cidrIP, network, err := net.ParseCIDR(r.Status.Address.CIDR)
			if err != nil {
				allErrs = append(allErrs, field.Invalid(field.NewPath("status").Child("address").Child("cidr"), r.Status.Address.CIDR, fmt.Sprintf("invalid cidr address %v", err)))
			}
			if network != nil && cidrIP != nil {
				networkIP = cidrIP.Mask(network.Mask)
			}

			// Validate ip
			ip := net.ParseIP(r.Status.Address.IP)
			if ip == nil {
				allErrs = append(allErrs, field.Invalid(field.NewPath("status").Child("address").Child("ip"), r.Status.Address.IP, "invalid ip address"))
			} else {
				if network != nil {
					if network.Contains(ip) == false {
						allErrs = append(allErrs, field.Invalid(field.NewPath("status").Child("address").Child("ip"), r.Status.Address.IP, "ip is not in cidr"))
					}
				}
			}

			// Validate gateway
			gateway := net.ParseIP(r.Status.Address.Gateway)
			if gateway == nil {
				allErrs = append(allErrs, field.Invalid(field.NewPath("status").Child("address").Child("gateway"), r.Status.Address.Gateway, "invalid gateway address"))
			} else {
				if network != nil {
					if network.Contains(gateway) == false {
						allErrs = append(allErrs, field.Invalid(field.NewPath("status").Child("address").Child("gateway"), r.Status.Address.Gateway, "gateway is not in cidr"))
					}
				}
			}

			// Validate nameservers
			for i, ns := range r.Status.Address.Nameservers {
				nsIP := net.ParseIP(ns)
				if nsIP == nil {
					allErrs = append(allErrs, field.Invalid(field.NewPath("status").Child("address").Child("nameservers").Index(i), ns, "invalid nameserver address"))
				} else {
					if networkIP != nil {
						if len(networkIP) != len(nsIP) {
							allErrs = append(allErrs, field.Invalid(field.NewPath("status").Child("address").Child("nameservers").Index(i), ns, "nameserver ip version is different then cidr ip version"))
						}
					}
				}

				for j, n := range r.Status.Address.Nameservers {
					if i == j {
						continue
					}

					if ns == n {
						allErrs = append(allErrs, field.Duplicate(field.NewPath("status").Child("address").Child("nameservers").Index(i), "duplicate nameserver"))
					}
				}
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

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (w *BareMetalEndpointWebhook) ValidateDelete(obj runtime.Object) error {
	r := obj.(*baremetalv1alpha1.BareMetalEndpoint)

	baremetalendpointlog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
