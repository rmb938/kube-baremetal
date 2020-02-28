package baremetalinstance

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
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	baremetalv1alpha1 "github.com/rmb938/kube-baremetal/api/v1alpha1"
	conditionv1 "github.com/rmb938/kube-baremetal/apis/condition/v1"
)

type Provisioner struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Clock    clock.Clock
	Recorder record.EventRecorder
}

func (r *Provisioner) Reconcile(req ctrl.Request) (ctrl.Result, error) {
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

	scheduledCond := bmi.Status.GetCondition(baremetalv1alpha1.BareMetalHardwareConditionTypeInstanceScheduled)
	// if we are not scheduled we don't care about it
	if scheduledCond == nil || scheduledCond.Status != conditionv1.ConditionStatusTrue {
		return ctrl.Result{}, nil
	}

	if bmi.DeletionTimestamp.IsZero() == false {
		// bmi is already terminating or terminated so ignore it
		if bmi.Status.Phase == baremetalv1alpha1.BareMetalInstanceStatusPhaseTerminating || bmi.Status.Phase == baremetalv1alpha1.BareMetalInstanceStatusPhaseTerminated {
			return ctrl.Result{}, nil
		}

		cleanedCond := bmi.Status.GetCondition(baremetalv1alpha1.BareMetalHardwareConditionTypeInstanceCleaned)

		// if no cleaned condition set to false and set phase to cleaning
		if cleanedCond == nil {
			nowTime := metav1.NewTime(r.Clock.Now())
			err := bmi.Status.SetCondition(&conditionv1.StatusCondition{
				Type:               baremetalv1alpha1.BareMetalHardwareConditionTypeInstanceCleaned,
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

		// if cleaned cond is true set to terminating
		if cleanedCond.Status == conditionv1.ConditionStatusTrue {
			bmi.Status.Phase = baremetalv1alpha1.BareMetalInstanceStatusPhaseTerminating
			bmi.Status.AgentInfo = nil
			err := r.Status().Update(ctx, bmi)
			if err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}

		// phase is not cleaning but we should be cleaning
		if bmi.Status.Phase != baremetalv1alpha1.BareMetalInstanceStatusPhaseCleaning {
			bmi.Status.Phase = baremetalv1alpha1.BareMetalInstanceStatusPhaseCleaning
			err := r.Status().Update(ctx, bmi)
			if err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}

		// if for some reason you get here with an empty hardware name
		if len(bmi.Status.HardwareName) == 0 {
			bmi.Status.Phase = baremetalv1alpha1.BareMetalInstanceStatusPhaseTerminating
			err := r.Status().Update(ctx, bmi)
			if err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}

		bmh := &baremetalv1alpha1.BareMetalHardware{}
		err := r.Get(ctx, types.NamespacedName{Namespace: bmi.Namespace, Name: bmi.Status.HardwareName}, bmh)
		if err != nil {
			if apierrors.IsNotFound(err) {
				bmh = nil
			} else {
				return ctrl.Result{}, err
			}
		}

		// bmh is gone
		// or
		// bmh instanceRef is not bmi (how did this happen?)
		// so we can't (or shouldn't) actually clean so go straight to terminating
		if bmh == nil || bmh.Status.InstanceRef != nil && bmh.Status.InstanceRef.UID != bmi.UID {
			if bmh == nil {
				r.Recorder.Eventf(bmi, corev1.EventTypeWarning, baremetalv1alpha1.BareMetalInstanceNotCleanedEventReason, "Instance cannot be cleaned because the BareMetalHardware %s does not exist anymore", bmi.Status.HardwareName)
			} else if bmh.Status.InstanceRef.UID != bmi.UID {
				r.Recorder.Eventf(bmi, corev1.EventTypeWarning, baremetalv1alpha1.BareMetalInstanceNotCleanedEventReason, "Instance cannot be cleaned because the BareMetalHardware %s thinks another instance is provisioned", bmi.Status.HardwareName)
			}

			bmi.Status.Phase = baremetalv1alpha1.BareMetalInstanceStatusPhaseTerminating
			err := r.Status().Update(ctx, bmi)
			if err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}

		// bmh instance ref is nil so we are done cleaning
		if bmh.Status.InstanceRef == nil {
			nowTime := metav1.NewTime(r.Clock.Now())
			err := bmi.Status.SetCondition(&conditionv1.StatusCondition{
				Type:               baremetalv1alpha1.BareMetalHardwareConditionTypeInstanceCleaned,
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
			r.Recorder.Eventf(bmi, corev1.EventTypeNormal, baremetalv1alpha1.BareMetalInstanceCleanedEventReason, "Instance was cleaned off of BareMetalHardware %s", bmh.Name)
			return ctrl.Result{}, nil
		}

		r.Recorder.Eventf(bmi, corev1.EventTypeNormal, baremetalv1alpha1.BareMetalInstanceCleaningEventReason, "Cleaning the instance off of BareMetalHardware %s", bmh.Name)
		r.Recorder.Eventf(bmh, corev1.EventTypeNormal, baremetalv1alpha1.BareMetalHardwareCleaningEventReason, "Cleaning the BareMetalInstance %s off of the hardware", bmi.Name)

		// TODO: do cleaning stuffs

		// we are done cleaning so set instanceRef to nil
		// this will cause a reconcile due to the old object not being nil
		bmh.Status.InstanceRef = nil
		err = r.Status().Update(ctx, bmh)
		if err != nil {
			return ctrl.Result{}, err
		}
		r.Recorder.Eventf(bmh, corev1.EventTypeNormal, baremetalv1alpha1.BareMetalHardwareCleanedEventReason, "Cleaned the BareMetalInstance %s off of the hardware", bmi.Name)
		return ctrl.Result{}, nil
	}

	// if bmi is not phase provisioning we don't care about it
	if bmi.Status.Phase != baremetalv1alpha1.BareMetalInstanceStatusPhaseProvisioning {
		return ctrl.Result{}, nil
	}

	bmh := &baremetalv1alpha1.BareMetalHardware{}
	err := r.Get(ctx, types.NamespacedName{Namespace: bmi.Namespace, Name: bmi.Status.HardwareName}, bmh)
	if err != nil {
		if apierrors.IsNotFound(err) {
			bmh = nil
		} else {
			return ctrl.Result{}, err
		}
	}

	if bmh == nil {
		r.Recorder.Eventf(bmi, corev1.EventTypeWarning, baremetalv1alpha1.BareMetalInstanceProvisioningEventReason, "Cannot find BareMetalHardware %s to provision onto", bmi.Status.HardwareName)
		return ctrl.Result{}, nil
	}

	if bmh.DeletionTimestamp.IsZero() == false {
		r.Recorder.Eventf(bmi, corev1.EventTypeWarning, baremetalv1alpha1.BareMetalInstanceProvisioningEventReason, "Cannot provision onto BareMetalHardware %s when it is deleting", bmi.Status.HardwareName)
		return ctrl.Result{}, nil
	}

	if bmh.Status.InstanceRef != nil && bmh.Status.InstanceRef.UID != bmi.UID {
		r.Recorder.Eventf(bmi, corev1.EventTypeWarning, baremetalv1alpha1.BareMetalInstanceProvisioningEventReason, "BareMetalHardware %s thinks another instance is provisioned on it", bmi.Status.HardwareName)
		return ctrl.Result{}, nil
	}

	for _, t := range bmh.Spec.Taints {
		if t.Key == baremetalv1alpha1.BareMetalHardwareTaintKeyNotReady {
			r.Recorder.Eventf(bmi, corev1.EventTypeWarning, baremetalv1alpha1.BareMetalInstanceProvisioningEventReason, "Cannot provision onto BareMetalHardware %s when it is not ready", bmi.Status.HardwareName)
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
	}

	// if bmh instance ref is nil set it to bmi
	if bmh.Status.InstanceRef == nil {
		bmh.Status.InstanceRef = &baremetalv1alpha1.BareMetalHardwareStatusInstanceRef{
			Name:      bmi.Name,
			Namespace: bmi.Namespace,
			UID:       bmi.UID,
		}
		err := r.Status().Update(ctx, bmh)
		if err != nil {
			return ctrl.Result{}, err
		}
		r.Recorder.Eventf(bmh, corev1.EventTypeNormal, baremetalv1alpha1.BareMetalInstanceScheduleEventReason, "Instance %s/%s has been scheduled", bmi.Namespace, bmi.Name)
		return ctrl.Result{}, nil
	}

	networkedCond := bmi.Status.GetCondition(baremetalv1alpha1.BareMetalHardwareConditionTypeInstanceNetworked)
	if networkedCond == nil {
		nowTime := metav1.NewTime(r.Clock.Now())
		err := bmi.Status.SetCondition(&conditionv1.StatusCondition{
			Type:               baremetalv1alpha1.BareMetalHardwareConditionTypeInstanceNetworked,
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

	if networkedCond.Status == conditionv1.ConditionStatusFalse {
		allAddressed := true
	nicLoop:
		for _, nic := range bmh.Spec.NICS {
			bmeList := &baremetalv1alpha1.BareMetalEndpointList{}
			err = r.List(ctx, bmeList, client.MatchingLabels{baremetalv1alpha1.BareMetalEndpointInstanceLabel: bmi.Name, baremetalv1alpha1.BareMetalEndpointNICLabel: nic.Name})
			if err != nil {
				return ctrl.Result{}, err
			}

			// we only care about bme's owned by us
			ourBMEs := make([]baremetalv1alpha1.BareMetalEndpoint, 0)
			for _, bme := range bmeList.Items {
				ownedByUs := false

				for _, ownerRef := range bme.OwnerReferences {
					if ownerRef.UID == bmi.UID {
						ownedByUs = true
					}
				}

				if ownedByUs == true {
					ourBMEs = append(ourBMEs, bme)
				}
			}

			switch len(ourBMEs) {
			case 0:
				allAddressed = false
				bme := &baremetalv1alpha1.BareMetalEndpoint{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: bmi.Name,
						Namespace:    bmi.Namespace,
						Labels: map[string]string{
							baremetalv1alpha1.BareMetalEndpointInstanceLabel: bmi.Name,
							baremetalv1alpha1.BareMetalEndpointNICLabel:      nic.Name,
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         bmi.APIVersion,
								Kind:               bmi.Kind,
								Name:               bmi.Name,
								UID:                bmi.UID,
								Controller:         func(b bool) *bool { return &b }(true),
								BlockOwnerDeletion: func(b bool) *bool { return &b }(false),
							},
						},
					},
					Spec: baremetalv1alpha1.BareMetalEndpointSpec{
						Primary:    nic.Primary,
						NetworkRef: nic.NetworkRef,
					},
				}

				// Compile all mac addresses
				macs := make([]string, 0)
				for _, interf := range bmh.Status.Hardware.NICS {
					if nic.Bond == nil {
						if interf.Name == nic.Name {
							macs = append(macs, interf.MAC)
						}
					} else {
						for _, bondInterf := range nic.Bond.Interfaces {
							if interf.Name == bondInterf {
								macs = append(macs, interf.MAC)
							}
						}
					}
				}
				// Set mac addresses
				bme.Spec.MACS = macs

				err := r.Create(ctx, bme)
				if err != nil {
					return ctrl.Result{}, err
				}
				break nicLoop
			case 1:
				bme := ourBMEs[0]
				if bme.Status.Phase != baremetalv1alpha1.BareMetalEndpointStatusPhaseAddressed {
					allAddressed = false
					break nicLoop
				}
				break
			default:
				allAddressed = false
				r.Recorder.Eventf(bmi, corev1.EventTypeWarning, baremetalv1alpha1.BareMetalInstanceNetworkingEventReason, "Found multiple BareMetalEndpoints for nic %s, this shouldn't happen", nic.Name)
				return ctrl.Result{}, nil
			}
		}

		if allAddressed == false {
			r.Recorder.Eventf(bmi, corev1.EventTypeNormal, baremetalv1alpha1.BareMetalInstanceNetworkingEventReason, "Waiting for all BareMetalEndpoints to be addressed")

			// I know we will automatically reconcile when the endpoints update, but the events will eventually disappear
			return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
		} else {
			nowTime := metav1.NewTime(r.Clock.Now())
			err := bmi.Status.SetCondition(&conditionv1.StatusCondition{
				Type:               baremetalv1alpha1.BareMetalHardwareConditionTypeInstanceNetworked,
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
			r.Recorder.Eventf(bmi, corev1.EventTypeNormal, baremetalv1alpha1.BareMetalInstanceNetworkedEventReason, "All BareMetalEndpoints have been addressed")
			return ctrl.Result{}, nil
		}
	}

	// network cond is true so lets image
	if networkedCond.Status == conditionv1.ConditionStatusTrue {
		imagedCond := bmi.Status.GetCondition(baremetalv1alpha1.BareMetalHardwareConditionTypeInstanceImaged)
		if imagedCond == nil {
			nowTime := metav1.NewTime(r.Clock.Now())
			err := bmi.Status.SetCondition(&conditionv1.StatusCondition{
				Type:               baremetalv1alpha1.BareMetalHardwareConditionTypeInstanceImaged,
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

		// once we are done imaging set phase to running
		if imagedCond.Status == conditionv1.ConditionStatusTrue {
			bmi.Status.Phase = baremetalv1alpha1.BareMetalInstanceStatusPhaseRunning
			bmi.Status.AgentInfo = nil
			err = r.Status().Update(ctx, bmi)
			if err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}

		// TODO: bmc (re)boot instance

		// wait for agent info to be set
		if bmi.Status.AgentInfo == nil {
			r.Recorder.Eventf(bmi, corev1.EventTypeWarning, baremetalv1alpha1.BareMetalInstanceNoAgentEventReason, "Agent has not reported in yet")

			// I know we will automatically reconcile when the agent reports in, but the events will eventually disappear
			return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
		}

		// TODO: imaging stuffs

		// we are done imaging so set image cond to true
		nowTime := metav1.NewTime(r.Clock.Now())
		err := bmi.Status.SetCondition(&conditionv1.StatusCondition{
			Type:               baremetalv1alpha1.BareMetalHardwareConditionTypeInstanceImaged,
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

	return ctrl.Result{}, nil
}

func (r *Provisioner) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("BareMetalInstanceProvisioner").
		For(&baremetalv1alpha1.BareMetalInstance{}).
		Owns(&baremetalv1alpha1.BareMetalEndpoint{}).
		// This will cause BMH changes to cause a BMI reconcile if instance ref is set
		Watches(&source.Kind{Type: &baremetalv1alpha1.BareMetalHardware{}}, &handler.EnqueueRequestsFromMapFunc{ToRequests: handler.ToRequestsFunc(func(a handler.MapObject) []reconcile.Request {
			bmh := a.Object.(*baremetalv1alpha1.BareMetalHardware)
			var req []reconcile.Request

			if bmh.Status.InstanceRef != nil {
				req = append(req, reconcile.Request{NamespacedName: types.NamespacedName{
					Namespace: bmh.Status.InstanceRef.Namespace,
					Name:      bmh.Status.InstanceRef.Name,
				}})
			}

			return req
		})}).
		Complete(r)
}
