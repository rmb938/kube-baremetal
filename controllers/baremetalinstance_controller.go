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
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/clock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	baremetalapi "github.com/rmb938/kube-baremetal/api"
	baremetalv1alpha1 "github.com/rmb938/kube-baremetal/api/v1alpha1"
	conditionv1 "github.com/rmb938/kube-baremetal/apis/condition/v1"
	v1 "github.com/rmb938/kube-baremetal/apis/condition/v1"
)

// BareMetalInstanceReconciler reconciles a BareMetalInstance object
type BareMetalInstanceReconciler struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Clock    clock.Clock
	Recorder record.EventRecorder

	scheduleLock sync.Mutex
}

// +kubebuilder:rbac:groups=baremetal.com.rmb938,resources=baremetalinstances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=baremetal.com.rmb938,resources=baremetalinstances/status,verbs=get;update;patch

func (r *BareMetalInstanceReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("baremetalinstance", req.NamespacedName)

	bmi := &baremetalv1alpha1.BareMetalInstance{}
	if err := r.Client.Get(ctx, req.NamespacedName, bmi); err != nil {
		err = client.IgnoreNotFound(err)
		if err != nil {
			log.Error(err, "failed to retrieve BareMetalInstance resource")
		}
		return ctrl.Result{}, err
	}

	if bmi.DeletionTimestamp.IsZero() == false {
		if len(bmi.Spec.HardwareName) > 0 {
			scheduledCond := bmi.Status.GetCondition(baremetalv1alpha1.BareMetalHardwareConditionTypeInstanceScheduled)

			// We only care to do proper deleting when scheduled
			if scheduledCond == nil || scheduledCond.Status != v1.ConditionStatusTrue {
				bmi.Spec.HardwareName = ""
				err := r.Update(ctx, bmi)
				if err != nil {
					return ctrl.Result{}, err
				}
				return ctrl.Result{}, nil
			}

			bmh := &baremetalv1alpha1.BareMetalHardware{}
			err := r.Client.Get(ctx, types.NamespacedName{Namespace: bmi.Namespace, Name: bmi.Spec.HardwareName}, bmh)
			if err != nil {
				if apierrors.IsNotFound(err) {
					// bmh wasn't found so set the name to empty
					bmi.Spec.HardwareName = ""
					err = r.Update(ctx, bmi)
					if err != nil {
						return ctrl.Result{}, err
					}
					return ctrl.Result{}, nil
				}
				return ctrl.Result{}, err
			}

			if bmi.Status.Phase == baremetalv1alpha1.BareMetalInstanceStatusPhaseCleaning {
				// TODO: check if done cleaning

				// remove owner ref when done cleaning
				ownerRefs := make([]metav1.OwnerReference, 0)
				for _, ref := range bmh.OwnerReferences {
					if ref.UID != bmi.UID {
						ownerRefs = append(ownerRefs, ref)
					}
				}

				bmh.OwnerReferences = ownerRefs
				err = r.Update(ctx, bmh) // TODO: this will cause an extra reconcile due to the old object
				if err != nil {
					return ctrl.Result{}, err
				}
				r.Recorder.Eventf(bmh, corev1.EventTypeNormal, baremetalv1alpha1.BareMetalInstanceUnscheduleEventReason, "Instance %s/%s has been unscheduled", bmi.Namespace, bmi.Name)

				// Since we are done cleaning set hardware name to empty
				bmi.Spec.HardwareName = ""
				err = r.Update(ctx, bmi)
				if err != nil {
					return ctrl.Result{}, err
				}
				return ctrl.Result{}, nil
			} else {
				var existingRef *metav1.OwnerReference
				for _, ref := range bmh.OwnerReferences {
					if ref.UID == bmi.UID {
						existingRef = &ref
					}
				}
				// we only care to do proper deleting when bmi is the owner
				if existingRef == nil {
					bmi.Spec.HardwareName = ""
					err = r.Update(ctx, bmi)
					if err != nil {
						return ctrl.Result{}, err
					}
					return ctrl.Result{}, nil
				}

				bmi.Status.Phase = baremetalv1alpha1.BareMetalInstanceStatusPhaseCleaning
				err = r.Status().Update(ctx, bmi)
				if err != nil {
					return ctrl.Result{}, err
				}
				return ctrl.Result{}, nil
			}
		}

		// Done deleting so remove finalizer
		baremetalapi.RemoveFinalizer(bmi, baremetalv1alpha1.BareMetalInstanceFinalizer)
		err := r.Update(ctx, bmi)
		if err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if len(bmi.Status.Phase) == 0 {
		bmi.Status.Phase = baremetalv1alpha1.BareMetalInstanceStatusPhasePending
		err := r.Status().Update(ctx, bmi)
		if err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	scheduledCond := bmi.Status.GetCondition(baremetalv1alpha1.BareMetalHardwareConditionTypeInstanceScheduled)

	switch bmi.Status.Phase {
	case baremetalv1alpha1.BareMetalInstanceStatusPhasePending:
		if scheduledCond == nil || scheduledCond.Status != v1.ConditionStatusTrue {
			return r.scheduleInstance(ctx, bmi)
		}

		if scheduledCond.Status == v1.ConditionStatusTrue {
			bmi.Status.Phase = baremetalv1alpha1.BareMetalInstanceStatusPhaseImaging
			err := r.Status().Update(ctx, bmi)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	case baremetalv1alpha1.BareMetalInstanceStatusPhaseImaging:
		break
	case baremetalv1alpha1.BareMetalInstanceStatusPhaseRunning:
		break
	default:
		// TODO: do something when the phase is wrong
		break
	}

	return ctrl.Result{}, nil
}

func (r *BareMetalInstanceReconciler) scheduleInstance(ctx context.Context, bmi *baremetalv1alpha1.BareMetalInstance) (ctrl.Result, error) {
	r.scheduleLock.Lock()
	defer r.scheduleLock.Unlock()

	scheduledCond := bmi.Status.GetCondition(baremetalv1alpha1.BareMetalHardwareConditionTypeInstanceScheduled)
	if scheduledCond == nil {
		nowTime := metav1.NewTime(r.Clock.Now())
		err := bmi.Status.SetCondition(&conditionv1.StatusCondition{
			Type:               baremetalv1alpha1.BareMetalHardwareConditionTypeInstanceScheduled,
			Status:             conditionv1.ConditionStatusFalse,
			LastTransitionTime: &nowTime,
		})
		if err != nil {
			return ctrl.Result{}, err
		}
		err = r.Status().Update(ctx, bmi)
		if err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// If BMI has a BMH name it is "forced" onto the BMH
	// when forcing we don't check tolerations
	// we only check to make sure the bmh is valid and there isn't an existing bmi on the node
	if len(bmi.Spec.HardwareName) > 0 {
		bmh := &baremetalv1alpha1.BareMetalHardware{}
		err := r.Get(ctx, types.NamespacedName{Namespace: bmi.Namespace, Name: bmi.Spec.HardwareName}, bmh)
		if err != nil {
			if apierrors.IsNotFound(err) {
				bmh = nil
			} else {
				return ctrl.Result{}, err
			}
		}

		// if bmh is nil, delete us
		if bmh == nil {
			r.Recorder.Eventf(bmi, corev1.EventTypeWarning, baremetalv1alpha1.BareMetalInstanceHardwareNotFoundEventReason, "Could not find a BareMetalHardware named %s to schedule onto", bmi.Spec.HardwareName)
			err = r.Delete(ctx, bmi)
			if err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}

		// If bmh is deleting, delete us
		if bmh.DeletionTimestamp.IsZero() == false {
			r.Recorder.Eventf(bmi, corev1.EventTypeWarning, baremetalv1alpha1.BareMetalInstanceHardwareDeletingEventReason, "Could not schedule onto a deleting BareMetalHardware named %s", bmi.Spec.HardwareName)
			err = r.Delete(ctx, bmi)
			if err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}

		var existingRef *metav1.OwnerReference

		for _, ref := range bmh.OwnerReferences {
			if ref.APIVersion == bmi.APIVersion && ref.Kind == bmi.Kind {
				existingRef = &ref
				break
			}
		}

		// If there is an existing owner ref
		// event with warning if it's not bmi
		// else add owner ref
		if existingRef != nil {
			if bmi.UID != existingRef.UID {
				r.Recorder.Eventf(bmi, corev1.EventTypeWarning, baremetalv1alpha1.BareMetalInstanceHardwareHasInstanceEventReason, "Could not schedule onto a BareMetalHardware named %s due to an existing instance scheduled", bmi.Spec.HardwareName)
				return ctrl.Result{Requeue: true}, nil
			}
		} else {
			controller := false
			blockOwnerDeletion := true
			bmh.OwnerReferences = append(bmh.OwnerReferences, metav1.OwnerReference{
				APIVersion:         bmi.APIVersion,
				Kind:               bmi.Kind,
				Name:               bmi.Name,
				UID:                bmi.UID,
				Controller:         &controller,
				BlockOwnerDeletion: &blockOwnerDeletion,
			})
			// This will cause the BMH to be owned by the bmi
			// So this update will cause a BMI reconcile
			err = r.Update(ctx, bmh)
			if err != nil {
				return ctrl.Result{}, err
			}
			r.Recorder.Eventf(bmh, corev1.EventTypeNormal, baremetalv1alpha1.BareMetalInstanceScheduleEventReason, "Instance %s/%s has been scheduled", bmi.Namespace, bmi.Name)
			return ctrl.Result{}, nil
		}

		nowTime := metav1.NewTime(r.Clock.Now())
		err = bmi.Status.SetCondition(&conditionv1.StatusCondition{
			Type:               baremetalv1alpha1.BareMetalHardwareConditionTypeInstanceScheduled,
			Status:             conditionv1.ConditionStatusTrue,
			LastTransitionTime: &nowTime,
		})
		if err != nil {
			return ctrl.Result{}, err
		}
		err = r.Status().Update(ctx, bmi)
		if err != nil {
			return ctrl.Result{}, err
		}
		r.Recorder.Eventf(bmi, corev1.EventTypeNormal, baremetalv1alpha1.BareMetalInstanceScheduleEventReason, "Instance scheduled onto %s", bmh.Name)
		return ctrl.Result{}, nil
	}

	// normal scheduling where bmh name isn't set
	bmhList := &baremetalv1alpha1.BareMetalHardwareList{}
	err := r.List(ctx, bmhList)
	if err != nil {
		return ctrl.Result{}, err
	}

	totalBMH := bmhList.Items
	scheduledBMH := make([]*baremetalv1alpha1.BareMetalHardware, 0)
	unscheduableBMH := make([]*baremetalv1alpha1.BareMetalHardware, 0)
	notMatchSelectorBMH := make([]*baremetalv1alpha1.BareMetalHardware, 0)
	notTolerateTaint := make([]*baremetalv1alpha1.BareMetalHardware, 0)
	acceptableBMH := make([]*baremetalv1alpha1.BareMetalHardware, 0)

	var labelSelector labels.Selector

	if len(bmi.Spec.Selector) > 0 {
		labelSelector = labels.SelectorFromSet(bmi.Spec.Selector)
	}

	for _, bmh := range totalBMH {
		if bmh.DeletionTimestamp.IsZero() == false {
			continue
		}

		var existingRef *metav1.OwnerReference
		for _, ref := range bmh.OwnerReferences {
			if ref.APIVersion == bmi.APIVersion && ref.Kind == bmi.Kind {
				existingRef = &ref
				break
			}
		}

		if existingRef != nil {
			scheduledBMH = append(scheduledBMH, &bmh)
			continue
		}

		bmiList := &baremetalv1alpha1.BareMetalInstanceList{}
		err = r.List(ctx, bmiList, client.MatchingFields{"spec.hardwareName": bmh.Name})
		if err != nil {
			return ctrl.Result{}, err
		}

		if len(bmiList.Items) > 0 {
			scheduledBMH = append(scheduledBMH, &bmh)
			continue
		}

		if labelSelector != nil {
			if labelSelector.Matches(labels.Set(bmh.Labels)) == false {
				notMatchSelectorBMH = append(notMatchSelectorBMH, &bmh)
				continue
			}
		}

		if bmh.Spec.CanProvision == false {
			unscheduableBMH = append(unscheduableBMH, &bmh)
			continue
		}

		toleratesAll := true

		for _, taint := range bmh.Spec.Taints {
			if taint.Effect != corev1.TaintEffectNoSchedule && taint.Effect != corev1.TaintEffectNoExecute {
				continue
			}

			tolerates := false

			for _, toleration := range bmi.Spec.Tolerations {
				if toleration.ToleratesTaint(&taint) {
					tolerates = true
					break
				}
			}

			if tolerates == false {
				toleratesAll = false
				break
			}

		}

		if toleratesAll == false {
			notTolerateTaint = append(notTolerateTaint, &bmh)
			continue
		}

		acceptableBMH = append(acceptableBMH, &bmh)
	}

	if len(acceptableBMH) == 0 {
		noHardwareDefined := "0 hardware resources are defined"
		allScheduled := "0 hardware is avilable to be scheduled"
		notMatchSelectorMessage := fmt.Sprintf("%v hardware(s) didn't match hardware selector", len(notMatchSelectorBMH))
		unschedulableMessage := fmt.Sprintf("%v hardware(s) were unschedulable", len(unscheduableBMH))
		notnotTolerateTaintMessage := fmt.Sprintf("%v hardware(s) had taints that the instance didn't tolerate", len(notTolerateTaint))

		message := fmt.Sprintf("0/%v hardwares are available: ", len(totalBMH))

		reasons := make([]string, 0)

		if len(totalBMH) == 0 {
			reasons = append(reasons, noHardwareDefined)
		}

		if len(scheduledBMH) == len(totalBMH) {
			reasons = append(reasons, allScheduled)
		} else {
			if len(notMatchSelectorBMH) > 0 {
				reasons = append(reasons, notMatchSelectorMessage)
			}

			if len(unscheduableBMH) > 0 {
				reasons = append(reasons, unschedulableMessage)
			}

			if len(notTolerateTaint) > 0 {
				reasons = append(reasons, notnotTolerateTaintMessage)
			}
		}

		message += strings.Join(reasons, ", ")
		message += "."

		r.Recorder.Event(bmi, corev1.EventTypeNormal, "FailedScheduling", message)

		return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
	}

	bmi.Spec.HardwareName = acceptableBMH[rand.Intn(len(acceptableBMH))].Name
	err = r.Update(ctx, bmi)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *BareMetalInstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.scheduleLock = sync.Mutex{}

	if err := mgr.GetFieldIndexer().IndexField(&baremetalv1alpha1.BareMetalInstance{}, "spec.hardwareName", func(rawObj runtime.Object) []string {
		bmh := rawObj.(*baremetalv1alpha1.BareMetalInstance)
		return []string{bmh.Spec.HardwareName}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&baremetalv1alpha1.BareMetalInstance{}).
		// trigger instance events when BMHs change
		// when instances get scheduled they add a owner-ref to the hardware
		// when instances get unscheduled/deleted they remove the owner-ref
		// We don't control the BMH so use a custom Watches with IsController false
		Watches(&source.Kind{Type: &baremetalv1alpha1.BareMetalHardware{}}, &handler.EnqueueRequestForOwner{OwnerType: &baremetalv1alpha1.BareMetalInstance{}, IsController: false}).
		Complete(r)
}
