package baremetalinstance

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
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
	"github.com/rmb938/kube-baremetal/pkg/agent/action"
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

		// cleanedCond had an error so don't do anything else
		if cleanedCond.Status == conditionv1.ConditionStatusError {
			// TODO: when we get stuck here the user needs to know to manually clean and force delete the object
			//  I'm not sure how else to do this as we shouldn't keep trying over and over
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

		// TODO: bmc (re)boot instance

		// wait for agent info to be set
		if bmi.Status.AgentInfo == nil {
			r.Recorder.Eventf(bmi, corev1.EventTypeWarning, baremetalv1alpha1.BareMetalInstanceNoAgentEventReason, "Agent has not reported in yet")

			// I know we will automatically reconcile when the agent reports in, but the events will eventually disappear
			return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
		}

		// check agent status
		agentStatus, err := r.getAgentStatus(ctx, bmi.Status.AgentInfo.IP)
		if err != nil {
			r.Recorder.Eventf(bmi, corev1.EventTypeWarning, "AgentError", "Could not check agent status: %v", err)
			return ctrl.Result{}, err
		}

		if agentStatus == nil {
			r.Recorder.Eventf(bmi, corev1.EventTypeNormal, baremetalv1alpha1.BareMetalInstanceCleaningEventReason, "Cleaning the instance off of BareMetalHardware %s", bmh.Name)
			r.Recorder.Eventf(bmh, corev1.EventTypeNormal, baremetalv1alpha1.BareMetalHardwareCleaningEventReason, "Cleaning the BareMetalInstance %s off of the hardware", bmi.Name)

			// tell agent to clean
			req, err := http.NewRequestWithContext(ctx, "POST", "http://"+bmi.Status.AgentInfo.IP+":10443/clean", nil)
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("error while creating request for agent clean: %v", err)
			}
			cleanResp, err := http.DefaultClient.Do(req)
			if err != nil {
				r.Recorder.Eventf(bmi, corev1.EventTypeWarning, "AgentError", "Could not tell agent to clean: %v", err)
				return ctrl.Result{}, err
			}
			defer cleanResp.Body.Close()
			cleanBody, err := ioutil.ReadAll(cleanResp.Body)
			if err != nil {
				r.Recorder.Eventf(bmi, corev1.EventTypeWarning, "AgentError", "Error reading agent clean body: %v", err)
				return ctrl.Result{}, err
			}

			if cleanResp.StatusCode == http.StatusConflict {
				r.Recorder.Eventf(bmi, corev1.EventTypeWarning, "AgentWrongAction", "Agent is already performing an action")
				return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
			} else if cleanResp.StatusCode == http.StatusAccepted {
				r.Recorder.Eventf(bmi, corev1.EventTypeNormal, "AgentWorking", "Agent cleaning started")
				return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
			} else {
				// some other error happened
				r.Recorder.Eventf(bmi, corev1.EventTypeWarning, "AgentError", "Agent clean request returned an error: %v", string(cleanBody))
				return ctrl.Result{Requeue: true}, nil
			}
		} else {
			// agent is doing something
			if agentStatus.Type != action.CleaningActionType {
				r.Recorder.Eventf(bmi, corev1.EventTypeWarning, "AgentWrongAction", "Agent is performing a different action")
				if len(agentStatus.Error) > 0 {
					r.Recorder.Eventf(bmi, corev1.EventTypeWarning, "AgentLastActionFailed", "Agent last action %s failed: %v", agentStatus.Type, agentStatus.Error)
				}
				return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
			}

			if agentStatus.Done == false {
				r.Recorder.Eventf(bmi, corev1.EventTypeNormal, "AgentWorking", "Agent is still cleaning")
				return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
			} else {
				// agent errored so set the condition
				if len(agentStatus.Error) > 0 {
					r.Recorder.Eventf(bmi, corev1.EventTypeWarning, "AgentFailed", "Agent cleaning failed: %v", agentStatus.Error)
					nowTime := metav1.NewTime(r.Clock.Now())
					err = bmi.Status.SetCondition(&conditionv1.StatusCondition{
						Type:               baremetalv1alpha1.BareMetalHardwareConditionTypeInstanceCleaned,
						Status:             conditionv1.ConditionStatusError,
						Reason:             baremetalv1alpha1.BareMetalInstanceCleaningFailedConditionReason,
						Message:            agentStatus.Error,
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

				r.Recorder.Eventf(bmi, corev1.EventTypeNormal, "AgentFinished", "Agent has finished cleaning")

				// we are done cleaning so set instanceRef to nil
				// this will cause a reconcile due to the old object not being nil
				bmh.Status.InstanceRef = nil
				err = r.Status().Update(ctx, bmh)
				if err != nil {
					return ctrl.Result{}, err
				}
				r.Recorder.Eventf(bmi, corev1.EventTypeNormal, baremetalv1alpha1.BareMetalInstanceCleanedEventReason, "Cleaned the instance off of BareMetalHardware %s", bmh.Name)
				r.Recorder.Eventf(bmh, corev1.EventTypeNormal, baremetalv1alpha1.BareMetalHardwareCleanedEventReason, "Cleaned the BareMetalInstance %s off of the hardware", bmi.Name)
				return ctrl.Result{}, nil
			}
		}
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

				// get mac address
				var macs []string
				for _, interf := range bmh.Status.Hardware.NICS {
					if nic.Bond == nil {
						if interf.Name == nic.Name {
							macs = append(macs, interf.MAC)
							break
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
				bme.Spec.MAC = macs[0]
				if nic.Bond != nil {
					bme.Spec.Bond = &baremetalv1alpha1.BareMetalEndpointBond{
						Mode: nic.Bond.Mode,
						MACS: macs,
					}
				}

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
			// TODO: call reboot

			bmi.Status.Phase = baremetalv1alpha1.BareMetalInstanceStatusPhaseRunning
			bmi.Status.AgentInfo = nil
			err = r.Status().Update(ctx, bmi)
			if err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}

		// imagedCond had an error so don't do anything else
		if imagedCond.Status == conditionv1.ConditionStatusError {
			// TODO: when we get stuck here the user needs to know to manually force delete the object
			//  I'm not sure how else to do this as we shouldn't keep trying over and over
			return ctrl.Result{}, nil
		}

		// TODO: bmc (re)boot instance

		// wait for agent info to be set
		if bmi.Status.AgentInfo == nil {
			r.Recorder.Eventf(bmi, corev1.EventTypeWarning, baremetalv1alpha1.BareMetalInstanceNoAgentEventReason, "Agent has not reported in yet")

			// I know we will automatically reconcile when the agent reports in, but the events will eventually disappear
			return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
		}

		// check agent status
		agentStatus, err := r.getAgentStatus(ctx, bmi.Status.AgentInfo.IP)
		if err != nil {
			r.Recorder.Eventf(bmi, corev1.EventTypeWarning, "AgentError", "Could not check agent status: %v", err)
			return ctrl.Result{}, err
		}

		bmeList := &baremetalv1alpha1.BareMetalEndpointList{}
		err = r.List(ctx, bmeList, client.MatchingLabels{baremetalv1alpha1.BareMetalEndpointInstanceLabel: bmi.Name})
		if err != nil {
			return ctrl.Result{}, err
		}

		networkLinks := make([]NetworkDataLink, 0)
		networks := make([]NetworkDataNetwork, 0)

		for _, bme := range bmeList.Items {
			ownedByUs := false

			for _, ownerRef := range bme.OwnerReferences {
				if ownerRef.UID == bmi.UID {
					ownedByUs = true
				}
			}

			if ownedByUs == true {
				linkName, ok := bme.Labels[baremetalv1alpha1.BareMetalEndpointNICLabel]
				if !ok {
					continue
				}

				link := NetworkDataLink{
					ID:  linkName,
					MAC: bme.Spec.MAC,
				}

				if bme.Spec.Bond != nil {
					link.Type = "bond"
					link.BondMode = string(bme.Spec.Bond.Mode)
					link.BondMiiMon = 100
					link.BondLinks = []string{}

					for i, bondMAC := range bme.Spec.Bond.MACS {
						bondLinkName := fmt.Sprintf("%s-bond-%d", linkName, i)
						link.BondLinks = append(link.BondLinks, bondLinkName)

						networkLinks = append(networkLinks, NetworkDataLink{
							ID:   bondLinkName,
							MAC:  bondMAC,
							Type: "phy",
						})
					}
				} else {
					link.Type = "phy"
				}

				networkLinks = append(networkLinks, link)

				_, cidrNetwork, err := net.ParseCIDR(bme.Status.Address.CIDR)
				if err != nil {
					return ctrl.Result{}, err
				}

				networkType := "ipv4"
				if cidrNetwork.IP.To4() == nil {
					networkType = "ipv6"
				}

				network := NetworkDataNetwork{
					Link:      linkName,
					Type:      networkType,
					IPAddress: bme.Status.Address.IP,
					Netmask:   net.IP(cidrNetwork.Mask).String(),
				}

				if bme.Spec.Primary {
					network.Gateway = bme.Status.Address.Gateway
					network.Nameservers = bme.Status.Address.Nameservers
					network.Search = bme.Status.Address.Search
				}

				networks = append(networks, network)
			}
		}

		metadata := fmt.Sprintf(`
{
  "uuid": "%s",
  "public_keys": {
    "rmb938": "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQDGh42fHGzThG+7pA8O6DgSXQYeMHsHMkGVJdZNLvKc43lL2+Ovv8q+fYr2h1TQkHvGb4loRPLfGU6QgF5aJ8gRzwsCyDB58xeakF7otrShhZkLsjws4wKRJuv6svP5zVADSDz4TEEHXNONvoF/KU+PUY4NbS40G6qjlcfSKyp/aHDxtfNY/Q4erh80hYRJjdopA6DHx0UIDR4rSN9mymrVbvoRRnNhHynaUrBPhJ+C7ty7lzg6PhiHr6CK0iCFZHaDobXa9aa7ML1AliXzyB1tOIHU/4mJPXZzvFhmHfM4L+mau32BrxU/W8TwibfenFnaRY1E6ylRdmFDK0U2MdQTDCZDb2F+oGmbcvp8g3BixHL8p7L7yh+D7PuhPsoXKY8W586MZmNlp4MOpPQ8dFd0cCt7X9rc4nmHYtpwRZKyS6+zQRJj9tjAj4/6MzXPrH+AUKdL0pyQkeqUU+32y/LnBskhWjDoq8tg4JsoJtDe2t6XR9KQOWVn/EdNwD+Y9P0="
  },
  "hostname": "%s"
}
`, bmi.UID, bmi.Name)

		networkData := &NetworkData{
			Links:    networkLinks,
			Networks: networks,
		}
		networkDataBytes, err := json.Marshal(networkData)
		if err != nil {
			return ctrl.Result{}, err
		}

		userdata := `#cloud-config\n{}`

		// agent is not doing anything
		if agentStatus == nil {
			r.Recorder.Eventf(bmi, corev1.EventTypeNormal, baremetalv1alpha1.BareMetalInstanceImagingEventReason, "Imaging the instance onto BareMetalHardware %s", bmh.Name)
			r.Recorder.Eventf(bmh, corev1.EventTypeNormal, baremetalv1alpha1.BareMetalInstanceImagingEventReason, "Imaging the BareMetalInstance %s onto the hardware", bmi.Name)

			imageRequest := action.ImageRequest{
				ImageURL: "https://cloud.centos.org/centos/7/images/CentOS-7-x86_64-GenericCloud-2003.raw.tar.gz",
				// ImageURL:            "https://download.fedoraproject.org/pub/fedora/linux/releases/32/Cloud/x86_64/images/Fedora-Cloud-Base-32-1.6.x86_64.raw.xz",
				// ImageURL:            "https://cdimage.debian.org/cdimage/openstack/current-10/debian-10-openstack-amd64.raw",
				// ImageURL:            "https://download.opensuse.org/repositories/Cloud:/Images:/Leap_15.1/images/openSUSE-Leap-15.1-EC2-HVM.x86_64.raw.xz",
				DiskPath:            fmt.Sprintf("/dev/%s", bmh.Spec.ImageDrive),
				MetadataContents:    base64.StdEncoding.EncodeToString([]byte(strings.TrimSpace(metadata))),
				NetworkDataContents: base64.StdEncoding.EncodeToString(networkDataBytes),
				UserDataContents:    base64.StdEncoding.EncodeToString([]byte(strings.TrimSpace(userdata))),
			}
			imageRequestBytes, err := json.Marshal(imageRequest)
			if err != nil {
				r.Recorder.Eventf(bmi, corev1.EventTypeWarning, "AgentError", "Error marshalling agent image request: %v", err)
				return ctrl.Result{}, err
			}

			req, err := http.NewRequestWithContext(ctx, "POST", "http://"+bmi.Status.AgentInfo.IP+":10443/image", bytes.NewBuffer(imageRequestBytes))
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("error while creating request for agent image: %v", err)
			}

			imageResp, err := http.DefaultClient.Do(req)
			if err != nil {
				r.Recorder.Eventf(bmi, corev1.EventTypeWarning, "AgentError", "Could not tell agent to image: %v", err)
				return ctrl.Result{}, err
			}
			defer imageResp.Body.Close()
			imageBody, err := ioutil.ReadAll(imageResp.Body)
			if err != nil {
				r.Recorder.Eventf(bmi, corev1.EventTypeWarning, "AgentError", "Error reading agent image body: %v", err)
				return ctrl.Result{}, err
			}

			if imageResp.StatusCode == http.StatusConflict {
				r.Recorder.Eventf(bmi, corev1.EventTypeWarning, "AgentWrongAction", "Agent is already performing an action")
				return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
			} else if imageResp.StatusCode == http.StatusAccepted {
				r.Recorder.Eventf(bmi, corev1.EventTypeNormal, "AgentWorking", "Agent imaging started")
				return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
			} else {
				// some other error happened
				r.Recorder.Eventf(bmi, corev1.EventTypeWarning, "AgentError", "Agent image request returned an error: %v", string(imageBody))
				return ctrl.Result{Requeue: true}, nil
			}
		} else {
			// agent is doing something

			if agentStatus.Type != action.ImagingActionType {
				r.Recorder.Eventf(bmi, corev1.EventTypeWarning, "AgentWrongAction", "Agent is performing a different action")
				if len(agentStatus.Error) > 0 {
					r.Recorder.Eventf(bmi, corev1.EventTypeWarning, "AgentLastActionFailed", "Agent last action %s failed: %v", agentStatus.Type, agentStatus.Error)
				}
				return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
			}

			if agentStatus.Done == false {
				r.Recorder.Eventf(bmi, corev1.EventTypeNormal, "AgentWorking", "Agent is still imaging")
				return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
			} else {
				// agent errored so set the condition
				if len(agentStatus.Error) > 0 {
					r.Recorder.Eventf(bmi, corev1.EventTypeWarning, "AgentFailed", "Agent imaging failed: %v", agentStatus.Error)
					nowTime := metav1.NewTime(r.Clock.Now())
					err = bmi.Status.SetCondition(&conditionv1.StatusCondition{
						Type:               baremetalv1alpha1.BareMetalHardwareConditionTypeInstanceImaged,
						Status:             conditionv1.ConditionStatusError,
						Reason:             baremetalv1alpha1.BareMetalInstanceImagingFailedConditionReason,
						Message:            agentStatus.Error,
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

				r.Recorder.Eventf(bmi, corev1.EventTypeNormal, "AgentFinished", "Agent has finished imaging")
				r.Recorder.Eventf(bmi, corev1.EventTypeNormal, baremetalv1alpha1.BareMetalInstanceImagedEventReason, "Imaged the instance onto BareMetalHardware %s", bmh.Name)
				r.Recorder.Eventf(bmh, corev1.EventTypeNormal, baremetalv1alpha1.BareMetalInstanceImagedEventReason, "Imaged the BareMetalInstance %s onto the hardware", bmi.Name)

				// we are done imaging so set image cond to true
				nowTime := metav1.NewTime(r.Clock.Now())
				err = bmi.Status.SetCondition(&conditionv1.StatusCondition{
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

				// TODO: bmc (re)boot instance

				return ctrl.Result{}, nil
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *Provisioner) getAgentStatus(ctx context.Context, ip string) (*action.Status, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "http://"+ip+":10443/status", nil)
	if err != nil {
		return nil, fmt.Errorf("error while creating request for agent status: %v", err)
	}

	statusResp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error while requesting agent status: %v", err)
	}
	defer statusResp.Body.Close()
	statusBody, err := ioutil.ReadAll(statusResp.Body)
	if err != nil {
		return nil, fmt.Errorf("error while reading agent status body: %v", err)
	}

	if statusResp.StatusCode == http.StatusNoContent {
		// there is no status which means the agent isn't doing anything
		return nil, nil
	} else if statusResp.StatusCode == http.StatusOK {
		// there is a status so the agent is doing something
		actionStatus := &action.Status{}
		err = json.Unmarshal(statusBody, actionStatus)
		if err != nil {
			return nil, fmt.Errorf("error while unmarshalling agent status: %v", err)
		}
		return actionStatus, nil
	} else {
		return nil, fmt.Errorf("agent status request returned an error: %v", string(statusBody))
	}
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
