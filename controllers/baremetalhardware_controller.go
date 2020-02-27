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

package controllers

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/clock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	baremetalapi "github.com/rmb938/kube-baremetal/api"
	baremetalv1alpha1 "github.com/rmb938/kube-baremetal/api/v1alpha1"
	conditionv1 "github.com/rmb938/kube-baremetal/apis/condition/v1"
)

// BareMetalHardwareReconciler reconciles a BareMetalHardware object
type BareMetalHardwareReconciler struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Clock    clock.Clock
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=baremetal.com.rmb938,resources=baremetalhardwares,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=baremetal.com.rmb938,resources=baremetalhardwares/status,verbs=get;update;patch

func (r *BareMetalHardwareReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("baremetalhardware", req.NamespacedName)

	bmh := &baremetalv1alpha1.BareMetalHardware{}
	if err := r.Client.Get(ctx, req.NamespacedName, bmh); err != nil {
		err = client.IgnoreNotFound(err)
		if err != nil {
			log.Error(err, "failed to retrieve BareMetalHardware resource")
		}
		return ctrl.Result{}, err
	}

	// BMH is deleting
	if bmh.DeletionTimestamp.IsZero() == false {
		hasTaint := false

		for _, t := range bmh.Spec.Taints {
			if t.Key == baremetalv1alpha1.BareMetalHardwareTaintKeyNoSchedule {
				hasTaint = true
				break
			}
		}

		if hasTaint == false {
			nowTime := metav1.NewTime(r.Clock.Now())
			bmh.Spec.Taints = append(bmh.Spec.Taints, corev1.Taint{
				Key:       baremetalv1alpha1.BareMetalHardwareTaintKeyNoSchedule,
				Effect:    corev1.TaintEffectNoSchedule,
				TimeAdded: &nowTime,
			})
			err := r.Update(ctx, bmh)
			if err != nil {
				return ctrl.Result{}, err
			}

			r.Recorder.Eventf(bmh, corev1.EventTypeNormal, baremetalv1alpha1.BareMetalHardwareNotSchedulableEventReason, "Hardware %s status is now HardwareNotSchedulable", bmh.Name)
			return ctrl.Result{}, nil
		}

		if bmh.Status.InstanceRef != nil {
			bmi := &baremetalv1alpha1.BareMetalInstance{}
			err := r.Get(ctx, types.NamespacedName{Namespace: bmh.Status.InstanceRef.Namespace, Name: bmh.Status.InstanceRef.Name}, bmi)
			if err != nil {
				if apierrors.IsNotFound(err) {
					// bmi wasn't found so set instance ref to nil
					bmh.Status.InstanceRef = nil
					err := r.Status().Update(ctx, bmh)
					if err != nil {
						return ctrl.Result{}, err
					}
					return ctrl.Result{}, nil
				} else {
					return ctrl.Result{}, err
				}
			}

			if bmi.UID != bmh.Status.InstanceRef.UID {
				// bmi was found but doesn't match UID
				// this means it's not our instance
				// so lets remove the instance ref
				bmh.Status.InstanceRef = nil
				err := r.Status().Update(ctx, bmh)
				if err != nil {
					return ctrl.Result{}, err
				}
				return ctrl.Result{}, nil
			}

			r.Recorder.Eventf(bmh, corev1.EventTypeNormal, "FailedDelete", "Cannot delete hardware while an instance is scheduled.")

			err = r.Delete(ctx, bmi)
			if err != nil {
				return ctrl.Result{}, err
			}

			return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
		}

		// Done deleting so remove bmh finalizer
		baremetalapi.RemoveFinalizer(bmh, baremetalv1alpha1.BareMetalHardwareFinalizer)
		err := r.Update(ctx, bmh)
		if err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// If canProvision is true remove no schedule taint
	//  if false add no schedule taint
	if bmh.Spec.CanProvision {
		taintIndex := -1
		for idx, t := range bmh.Spec.Taints {
			if t.Key == baremetalv1alpha1.BareMetalHardwareTaintKeyNoSchedule {
				taintIndex = idx
				break
			}
		}

		if taintIndex >= 0 {
			bmh.Spec.Taints = append(bmh.Spec.Taints[:taintIndex], bmh.Spec.Taints[taintIndex+1:]...)
			err := r.Update(ctx, bmh)
			if err != nil {
				return ctrl.Result{}, err
			}

			r.Recorder.Eventf(bmh, corev1.EventTypeNormal, baremetalv1alpha1.BareMetalHardwareSchedulableEventReason, "Hardware %s status is now HardwareSchedulable", bmh.Name)
			return ctrl.Result{}, nil
		}
	} else {
		hasTaint := false

		for _, t := range bmh.Spec.Taints {
			if t.Key == baremetalv1alpha1.BareMetalHardwareTaintKeyNoSchedule {
				hasTaint = true
				break
			}
		}

		if hasTaint == false {
			nowTime := metav1.NewTime(r.Clock.Now())
			bmh.Spec.Taints = append(bmh.Spec.Taints, corev1.Taint{
				Key:       baremetalv1alpha1.BareMetalHardwareTaintKeyNoSchedule,
				Effect:    corev1.TaintEffectNoSchedule,
				TimeAdded: &nowTime,
			})
			err := r.Update(ctx, bmh)
			if err != nil {
				return ctrl.Result{}, err
			}

			r.Recorder.Eventf(bmh, corev1.EventTypeNormal, baremetalv1alpha1.BareMetalHardwareNotSchedulableEventReason, "Hardware %s status is now HardwareNotSchedulable", bmh.Name)
			return ctrl.Result{}, nil
		}
	}

	imageDriveValidCond := bmh.Status.GetCondition(baremetalv1alpha1.BareMetalHardwareConditionTypeImageDriveValid)
	nicsValidCond := bmh.Status.GetCondition(baremetalv1alpha1.BareMetalHardwareConditionTypeNicsValid)
	hardwareSetCond := bmh.Status.GetCondition(baremetalv1alpha1.BareMetalHardwareConditionTypeHardwareSet)

	// If conditions are met remove BareMetalHardwareTaintKeyNotReady taint
	//  if they are not met add the taint
	if (imageDriveValidCond != nil && imageDriveValidCond.Status == conditionv1.ConditionStatusTrue) &&
		(nicsValidCond != nil && nicsValidCond.Status == conditionv1.ConditionStatusTrue) &&
		(hardwareSetCond != nil && hardwareSetCond.Status == conditionv1.ConditionStatusTrue) {
		taintIndex := -1
		for idx, t := range bmh.Spec.Taints {
			if t.Key == baremetalv1alpha1.BareMetalHardwareTaintKeyNotReady {
				taintIndex = idx
				break
			}
		}

		if taintIndex >= 0 {
			bmh.Spec.Taints = append(bmh.Spec.Taints[:taintIndex], bmh.Spec.Taints[taintIndex+1:]...)
			err := r.Update(ctx, bmh)
			if err != nil {
				return ctrl.Result{}, err
			}

			r.Recorder.Eventf(bmh, corev1.EventTypeNormal, baremetalv1alpha1.BareMetalHardwareReadyEventReason, "Hardware %s status is now HardwareReady", bmh.Name)
			return ctrl.Result{}, nil
		}
	} else {
		hasTaint := false

		for _, t := range bmh.Spec.Taints {
			if t.Key == baremetalv1alpha1.BareMetalHardwareTaintKeyNotReady {
				hasTaint = true
				break
			}
		}

		if hasTaint == false {
			nowTime := metav1.NewTime(r.Clock.Now())
			bmh.Spec.Taints = append(bmh.Spec.Taints, corev1.Taint{
				Key:       baremetalv1alpha1.BareMetalHardwareTaintKeyNotReady,
				Effect:    corev1.TaintEffectNoSchedule,
				TimeAdded: &nowTime,
			})
			err := r.Update(ctx, bmh)
			if err != nil {
				return ctrl.Result{}, err
			}

			r.Recorder.Eventf(bmh, corev1.EventTypeWarning, baremetalv1alpha1.BareMetalHardwareNotReadyEventReason, "Hardware %s status is now HardwareNotReady", bmh.Name)
			return ctrl.Result{}, nil
		}
	}

	if bmh.Status.Hardware != nil {
		if hardwareSetCond == nil || hardwareSetCond.Reason != baremetalv1alpha1.BareMetalHardwareHardwareIsSetConditionReason {
			nowTime := metav1.NewTime(r.Clock.Now())
			err := bmh.Status.SetCondition(&conditionv1.StatusCondition{
				Type:               baremetalv1alpha1.BareMetalHardwareConditionTypeHardwareSet,
				Status:             conditionv1.ConditionStatusTrue,
				LastTransitionTime: &nowTime,
				Reason:             baremetalv1alpha1.BareMetalHardwareHardwareIsSetConditionReason,
				Message:            "hardware information is set",
			})
			if err != nil {
				return ctrl.Result{}, err
			}
			err = r.Status().Update(ctx, bmh)
			if err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}

		if len(bmh.Spec.NICS) > 0 {
			foundAllNics := true

			for _, nic := range bmh.Spec.NICS {
				if nic.Bond != nil {
					foundAllBondNics := true

					for _, bondNicName := range nic.Bond.Interfaces {
						foundNic := false

						for _, hardwareNic := range bmh.Status.Hardware.NICS {
							if hardwareNic.Name == bondNicName {
								foundNic = true
								break
							}
						}

						if foundNic == false {
							foundAllBondNics = false
							break
						}
					}

					if foundAllBondNics == false {
						foundAllNics = false
						break
					}
				} else {
					foundNic := false

					for _, hardwareNic := range bmh.Status.Hardware.NICS {
						if hardwareNic.Name == nic.Name {
							foundNic = true
							break
						}
					}

					if foundNic == false {
						foundAllNics = false
						break
					}
				}
			}
			if foundAllNics {
				if nicsValidCond == nil || nicsValidCond.Reason != baremetalv1alpha1.BareMetalHardwareValidNicsConditionReason {
					nowTime := metav1.NewTime(r.Clock.Now())
					err := bmh.Status.SetCondition(&conditionv1.StatusCondition{
						Type:               baremetalv1alpha1.BareMetalHardwareConditionTypeNicsValid,
						Status:             conditionv1.ConditionStatusTrue,
						LastTransitionTime: &nowTime,
						Reason:             baremetalv1alpha1.BareMetalHardwareValidNicsConditionReason,
						Message:            "nics are found",
					})
					if err != nil {
						return ctrl.Result{}, err
					}
					err = r.Status().Update(ctx, bmh)
					if err != nil {
						return ctrl.Result{}, err
					}
					r.Recorder.Event(bmh, corev1.EventTypeNormal, baremetalv1alpha1.BareMetalHardwareValidNicsConditionReason, "Nics are found in hardware nics list")
					return ctrl.Result{}, nil
				}
			} else {
				if nicsValidCond == nil || nicsValidCond.Reason != baremetalv1alpha1.BareMetalHardwareInvalidNicsConditionReason {
					nowTime := metav1.NewTime(r.Clock.Now())
					err := bmh.Status.SetCondition(&conditionv1.StatusCondition{
						Type:               baremetalv1alpha1.BareMetalHardwareConditionTypeNicsValid,
						Status:             conditionv1.ConditionStatusFalse,
						LastTransitionTime: &nowTime,
						Reason:             baremetalv1alpha1.BareMetalHardwareInvalidNicsConditionReason,
						Message:            "invalid nics",
					})
					if err != nil {
						return ctrl.Result{}, err
					}
					err = r.Status().Update(ctx, bmh)
					if err != nil {
						return ctrl.Result{}, err
					}
					r.Recorder.Event(bmh, corev1.EventTypeWarning, baremetalv1alpha1.BareMetalHardwareInvalidNicsConditionReason, "Could not find nics in hardware nics list")
					return ctrl.Result{}, nil
				}
			}
		} else {
			if nicsValidCond == nil || nicsValidCond.Reason != baremetalv1alpha1.BareMetalHardwareNicsAreNotSetConditionReason {
				nowTime := metav1.NewTime(r.Clock.Now())
				err := bmh.Status.SetCondition(&conditionv1.StatusCondition{
					Type:               baremetalv1alpha1.BareMetalHardwareConditionTypeNicsValid,
					Status:             conditionv1.ConditionStatusFalse,
					LastTransitionTime: &nowTime,
					Reason:             baremetalv1alpha1.BareMetalHardwareNicsAreNotSetConditionReason,
					Message:            "nics are not set",
				})
				if err != nil {
					return ctrl.Result{}, err
				}
				err = r.Status().Update(ctx, bmh)
				if err != nil {
					return ctrl.Result{}, err
				}
				r.Recorder.Event(bmh, corev1.EventTypeWarning, baremetalv1alpha1.BareMetalHardwareNicsAreNotSetConditionReason, "NICS are not set")
				return ctrl.Result{}, nil
			}
		}

		if len(bmh.Spec.ImageDrive) > 0 {
			foundImageDrive := false

			for _, storage := range bmh.Status.Hardware.Storage {
				for storage.Name == bmh.Spec.ImageDrive {
					foundImageDrive = true
					break
				}
			}

			if foundImageDrive {
				if imageDriveValidCond == nil || imageDriveValidCond.Reason != baremetalv1alpha1.BareMetalHardwareValidImageDriveConditionReason {
					nowTime := metav1.NewTime(r.Clock.Now())
					err := bmh.Status.SetCondition(&conditionv1.StatusCondition{
						Type:               baremetalv1alpha1.BareMetalHardwareConditionTypeImageDriveValid,
						Status:             conditionv1.ConditionStatusTrue,
						LastTransitionTime: &nowTime,
						Reason:             baremetalv1alpha1.BareMetalHardwareValidImageDriveConditionReason,
						Message:            "found image drive",
					})
					if err != nil {
						return ctrl.Result{}, err
					}
					err = r.Status().Update(ctx, bmh)
					if err != nil {
						return ctrl.Result{}, err
					}
					r.Recorder.Event(bmh, corev1.EventTypeNormal, baremetalv1alpha1.BareMetalHardwareValidImageDriveConditionReason, "Found image drive in hardware storage list")
					return ctrl.Result{}, nil
				}
			} else {
				if imageDriveValidCond == nil || imageDriveValidCond.Reason != baremetalv1alpha1.BareMetalHardwareInvalidImageDriveConditionReason {
					nowTime := metav1.NewTime(r.Clock.Now())
					err := bmh.Status.SetCondition(&conditionv1.StatusCondition{
						Type:               baremetalv1alpha1.BareMetalHardwareConditionTypeImageDriveValid,
						Status:             conditionv1.ConditionStatusFalse,
						LastTransitionTime: &nowTime,
						Reason:             baremetalv1alpha1.BareMetalHardwareInvalidImageDriveConditionReason,
						Message:            "invalid image drive",
					})
					if err != nil {
						return ctrl.Result{}, err
					}
					err = r.Status().Update(ctx, bmh)
					if err != nil {
						return ctrl.Result{}, err
					}
					r.Recorder.Event(bmh, corev1.EventTypeWarning, baremetalv1alpha1.BareMetalHardwareInvalidImageDriveConditionReason, "Could not find image drive in hardware storage list")
					return ctrl.Result{}, nil
				}
			}
		} else {
			if imageDriveValidCond == nil || imageDriveValidCond.Reason != baremetalv1alpha1.BareMetalHardwareImageDriveIsNotSetConditionReason {
				nowTime := metav1.NewTime(r.Clock.Now())
				err := bmh.Status.SetCondition(&conditionv1.StatusCondition{
					Type:               baremetalv1alpha1.BareMetalHardwareConditionTypeImageDriveValid,
					Status:             conditionv1.ConditionStatusFalse,
					LastTransitionTime: &nowTime,
					Reason:             baremetalv1alpha1.BareMetalHardwareImageDriveIsNotSetConditionReason,
					Message:            "image drive is not set",
				})
				if err != nil {
					return ctrl.Result{}, err
				}
				err = r.Status().Update(ctx, bmh)
				if err != nil {
					return ctrl.Result{}, err
				}
				r.Recorder.Event(bmh, corev1.EventTypeWarning, baremetalv1alpha1.BareMetalHardwareImageDriveIsNotSetConditionReason, "Image Drive is not set in the spec")
				return ctrl.Result{}, nil
			}
		}
	} else {
		if hardwareSetCond == nil || hardwareSetCond.Reason != baremetalv1alpha1.BareMetalHardwareHardwareIsNotSetConditionReason {
			nowTime := metav1.NewTime(r.Clock.Now())
			err := bmh.Status.SetCondition(&conditionv1.StatusCondition{
				Type:               baremetalv1alpha1.BareMetalHardwareConditionTypeHardwareSet,
				Status:             conditionv1.ConditionStatusFalse,
				LastTransitionTime: &nowTime,
				Reason:             baremetalv1alpha1.BareMetalHardwareHardwareIsNotSetConditionReason,
				Message:            "hardware information is not set",
			})
			if err != nil {
				return ctrl.Result{}, err
			}
			err = r.Status().Update(ctx, bmh)
			if err != nil {
				return ctrl.Result{}, err
			}

			r.Recorder.Event(bmh, corev1.EventTypeWarning, baremetalv1alpha1.BareMetalHardwareHardwareIsNotSetConditionReason, "Hardware information is not set")
			return ctrl.Result{}, nil
		}

		var bmd *baremetalv1alpha1.BareMetalDiscovery

		discoveryList := &baremetalv1alpha1.BareMetalDiscoveryList{}
		err := r.List(context.Background(), discoveryList, client.MatchingFields{"spec.systemUUID": string(bmh.Spec.SystemUUID)})
		if err != nil {
			return ctrl.Result{}, err
		}

		switch len(discoveryList.Items) {
		case 0:
			r.Recorder.Eventf(bmh, corev1.EventTypeWarning, baremetalv1alpha1.BareMetalHardwareDiscoveryNotFoundEventReason, "Could not find the discovery resource for the systemUUID of %s", bmh.Spec.SystemUUID)
			return ctrl.Result{Requeue: true}, nil
		case 1:
			bmd = &discoveryList.Items[0]
			break
		default:
			// we found multiple discoveries something messed up
			r.Recorder.Eventf(bmh, corev1.EventTypeWarning, baremetalv1alpha1.BareMetalHardwareManyDiscoveryFoundEventReason, "Found multiple discovery resources for the systemUUID of %s", bmh.Spec.SystemUUID)
			return ctrl.Result{Requeue: true}, nil
		}

		// TODO: discovery hardware may be nil due to "secure" discovery
		//  check if it's nil and event and requeue if it is

		bmh.Status.Hardware = bmd.Spec.Hardware.DeepCopy()
		err = r.Status().Update(ctx, bmh)
		if err != nil {
			return ctrl.Result{}, err
		}

		r.Recorder.Eventf(bmh, corev1.EventTypeNormal, baremetalv1alpha1.BareMetalHardwareDiscoveryFoundEventReason, "Found discovery resource for the systemUUID of %s", bmh.Spec.SystemUUID)
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

func (r *BareMetalHardwareReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(&baremetalv1alpha1.BareMetalHardware{}, "spec.systemUUID", func(rawObj runtime.Object) []string {
		bmh := rawObj.(*baremetalv1alpha1.BareMetalHardware)
		return []string{string(bmh.Spec.SystemUUID)}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&baremetalv1alpha1.BareMetalHardware{}).
		Complete(r)
}
