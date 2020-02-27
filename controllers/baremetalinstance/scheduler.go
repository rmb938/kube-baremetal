package baremetalinstance

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/clock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	baremetalv1alpha1 "github.com/rmb938/kube-baremetal/api/v1alpha1"
	conditionv1 "github.com/rmb938/kube-baremetal/apis/condition/v1"
)

type Scheduler struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Clock    clock.Clock
	Recorder record.EventRecorder

	scheduleLock sync.Mutex
}

func (r *Scheduler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
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

	// if bmi is deleting
	if bmi.DeletionTimestamp.IsZero() == false {
		// if we are already terminated ignore
		if bmi.Status.Phase == baremetalv1alpha1.BareMetalInstanceStatusPhaseTerminated {
			return ctrl.Result{}, nil
		}

		scheduledCond := bmi.Status.GetCondition(baremetalv1alpha1.BareMetalHardwareConditionTypeInstanceScheduled)

		// if we are not scheduled just set terminated
		if scheduledCond == nil || scheduledCond.Status != conditionv1.ConditionStatusTrue {
			bmi.Status.Phase = baremetalv1alpha1.BareMetalInstanceStatusPhaseTerminated
			err := r.Status().Update(ctx, bmi)
			if err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}

		// if bmi is not phase terminating we don't care about it
		if bmi.Status.Phase != baremetalv1alpha1.BareMetalInstanceStatusPhaseTerminating {
			return ctrl.Result{}, nil
		}

		// we want to lock to remove the hardware name
		r.scheduleLock.Lock()
		defer r.scheduleLock.Unlock()

		// TODO: do we need to do anything else to unschedule?

		nowTime := metav1.NewTime(r.Clock.Now())
		err := bmi.Status.SetCondition(&conditionv1.StatusCondition{
			Type:               baremetalv1alpha1.BareMetalHardwareConditionTypeInstanceScheduled,
			Status:             conditionv1.ConditionStatusFalse,
			LastTransitionTime: &nowTime,
		})
		if err != nil {
			return ctrl.Result{}, err
		}

		existing := bmi.Status.HardwareName
		bmi.Status.HardwareName = "" // I don't like assigning "" maybe make this a pointer or something?
		err = r.Status().Update(ctx, bmi)
		if err != nil {
			return ctrl.Result{}, err
		}
		r.Recorder.Eventf(bmi, corev1.EventTypeNormal, baremetalv1alpha1.BareMetalInstanceUnscheduleEventReason, "Successfully unassigned %s/%s from %s", bmi.Namespace, bmi.Name, existing)
		return ctrl.Result{}, nil
	}

	// if bmi is not phase pending we don't care about it
	if bmi.Status.Phase != baremetalv1alpha1.BareMetalInstanceStatusPhasePending {
		return ctrl.Result{}, nil
	}

	scheduledCond := bmi.Status.GetCondition(baremetalv1alpha1.BareMetalHardwareConditionTypeInstanceScheduled)

	// if no scheduling condition add it
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

	// if scheduling condition is true change the phase
	if scheduledCond.Status == conditionv1.ConditionStatusTrue {
		bmi.Status.Phase = baremetalv1alpha1.BareMetalInstanceStatusPhaseProvisioning
		err := r.Status().Update(ctx, bmi)
		if err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	// if hardware name is set change the scheduling condition to true
	if len(bmi.Status.HardwareName) > 0 {
		nowTime := metav1.NewTime(r.Clock.Now())
		err := bmi.Status.SetCondition(&conditionv1.StatusCondition{
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
		return ctrl.Result{}, nil
	}

	// we want to lock while finding and setting a hardware name
	r.scheduleLock.Lock()
	defer r.scheduleLock.Unlock()

	// TODO: scheduling stuff here

	bmhList := &baremetalv1alpha1.BareMetalHardwareList{}
	err := r.List(ctx, bmhList, client.InNamespace(bmi.Namespace))
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

		if bmh.Status.InstanceRef != nil {
			scheduledBMH = append(scheduledBMH, &bmh)
			continue
		}

		bmiList := &baremetalv1alpha1.BareMetalInstanceList{}
		err = r.List(ctx, bmiList, client.MatchingFields{"status.hardwareName": bmh.Name})
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
		allScheduled := "0 hardware is available to be scheduled"
		notMatchSelectorMessage := fmt.Sprintf("%v hardware(s) didn't match hardware selector", len(notMatchSelectorBMH))
		unschedulableMessage := fmt.Sprintf("%v hardware(s) were unschedulable", len(unscheduableBMH))
		notnotTolerateTaintMessage := fmt.Sprintf("%v hardware(s) had taints that the instance didn't tolerate", len(notTolerateTaint))

		message := fmt.Sprintf("0/%v hardwares are available: ", len(totalBMH))

		reasons := make([]string, 0)

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

	bmi.Status.HardwareName = acceptableBMH[rand.Intn(len(acceptableBMH))].Name
	err = r.Update(ctx, bmi)
	if err != nil {
		return ctrl.Result{}, err
	}
	r.Recorder.Eventf(bmi, corev1.EventTypeNormal, baremetalv1alpha1.BareMetalInstanceScheduleEventReason, "Successfully assigned %s/%s to %s", bmi.Namespace, bmi.Name, bmi.Status.HardwareName)

	return ctrl.Result{}, nil
}

func (r *Scheduler) SetupWithManager(mgr ctrl.Manager) error {
	r.scheduleLock = sync.Mutex{}

	if err := mgr.GetFieldIndexer().IndexField(&baremetalv1alpha1.BareMetalInstance{}, "status.hardwareName", func(rawObj runtime.Object) []string {
		bmi := rawObj.(*baremetalv1alpha1.BareMetalInstance)
		return []string{bmi.Status.HardwareName}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&baremetalv1alpha1.BareMetalInstance{}).
		Complete(r)
}
