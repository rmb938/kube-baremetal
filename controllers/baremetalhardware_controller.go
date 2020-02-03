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

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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

// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update
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

			r.Recorder.Event(bmh, corev1.EventTypeNormal, "HardwareNotSchedulable", fmt.Sprintf("Hardware %s status is now HardwareNotSchedulable", bmh.Name))
			return ctrl.Result{}, nil
		}

		// TODO: do actual deletion stuffs
		//  if instanceRef is not nil prevent deletion and event saying there's an instance still

		// Done deleting so remove finalizer
		baremetalapi.RemoveFinalizer(bmh, baremetalv1alpha1.BareMetalHardwareFinalizer)
		err := r.Update(ctx, bmh)
		if err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// add the finalizer
	if baremetalapi.HasFinalizer(bmh, baremetalv1alpha1.BareMetalHardwareFinalizer) == false {
		bmh.Finalizers = append(bmh.Finalizers, baremetalv1alpha1.BareMetalHardwareFinalizer)
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

			r.Recorder.Event(bmh, corev1.EventTypeNormal, "HardwareSchedulable", fmt.Sprintf("Hardware %s status is now HardwareSchedulable", bmh.Name))
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

			r.Recorder.Event(bmh, corev1.EventTypeNormal, "HardwareNotSchedulable", fmt.Sprintf("Hardware %s status is now HardwareNotSchedulable", bmh.Name))
			return ctrl.Result{}, nil
		}
	}

	// TODO: if conditions are not met set BareMetalHardwareTaintKeyNotReady taint

	// TODO: if hardware is nil find discovery any copy it
	//  if can't find discovery event
	//  once copied set HardwareSet condition to true

	// TODO: conditions
	// 	ImageDriveValid - image drive is valid
	//  NicsValid - nics are valid
	//  HardwareSet - hardware is set

	imageDriveValidCond := bmh.Status.GetCondition(conditionv1.ConditionType(baremetalv1alpha1.ConditionTypeImageDriveValid))
	nicsValidCond := bmh.Status.GetCondition(conditionv1.ConditionType(baremetalv1alpha1.ConditionTypeNicsValid))
	hardwareSetCond := bmh.Status.GetCondition(conditionv1.ConditionType(baremetalv1alpha1.ConditionTypeHardwareSet))

	// If conditions are met remove BareMetalHardwareTaintKeyNotReady taint
	//  if they are not met add the taint
	if imageDriveValidCond != nil && imageDriveValidCond.Status == conditionv1.ConditionStatusTrue &&
		nicsValidCond != nil && nicsValidCond.Status == conditionv1.ConditionStatusTrue &&
		hardwareSetCond != nil && hardwareSetCond.Status == conditionv1.ConditionStatusTrue {
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

			r.Recorder.Event(bmh, corev1.EventTypeNormal, "HardwareReady", fmt.Sprintf("Hardware %s status is now HardwareReady", bmh.Name))
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

			r.Recorder.Event(bmh, corev1.EventTypeNormal, "HardwareNotReady", fmt.Sprintf("Hardware %s status is now HardwareNotReady", bmh.Name))
			return ctrl.Result{}, nil
		}
	}

	return ctrl.Result{}, nil
}

func (r *BareMetalHardwareReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// This controller needs this indexer
	if err := mgr.GetFieldIndexer().IndexField(&baremetalv1alpha1.BareMetalDiscovery{}, "spec.systemUUID", func(rawObj runtime.Object) []string {
		bmd := rawObj.(*baremetalv1alpha1.BareMetalDiscovery)
		return []string{string(bmd.Spec.SystemUUID)}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&baremetalv1alpha1.BareMetalHardware{}).
		Complete(r)
}
