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

package baremetalendpoint

import (
	"bytes"
	"context"
	"net"
	"sync"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/clock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	baremetalv1alpha1 "github.com/rmb938/kube-baremetal/api/v1alpha1"
)

type Network struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Clock    clock.Clock
	Recorder record.EventRecorder

	addressLock sync.Mutex
}

func (r *Network) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("baremetalendpoint", req.NamespacedName)

	bme := &baremetalv1alpha1.BareMetalEndpoint{}
	if err := r.Client.Get(ctx, req.NamespacedName, bme); err != nil {
		err = client.IgnoreNotFound(err)
		if err != nil {
			log.Error(err, "failed to retrieve BareMetalEndpoint resource")
		}
		return ctrl.Result{}, err
	}

	// we only care about stuff that has a phase
	if len(bme.Status.Phase) == 0 {
		return ctrl.Result{}, nil
	}

	bmn := &baremetalv1alpha1.BareMetalNetwork{}

	// we only care if the bme belongs to our group
	if bme.Spec.NetworkRef.Group != baremetalv1alpha1.GroupVersion.Group {
		return ctrl.Result{}, nil
	}

	// we only care if the bme belongs to our kind
	if bme.Spec.NetworkRef.Kind != "BareMetalNetwork" {
		return ctrl.Result{}, nil
	}

	// find bmn, if we can't find it event and set it to nil
	if err := r.Client.Get(ctx, types.NamespacedName{Namespace: bme.Namespace, Name: bme.Spec.NetworkRef.Name}, bmn); err != nil {
		if apierrors.IsNotFound(err) {
			bmn = nil
			r.Recorder.Eventf(bme, corev1.EventTypeWarning, "NetworkNotFound", "Could not find a BareMetalNetwork with the name of %s", bme.Spec.NetworkRef.Name)
		} else {
			if err != nil {
				log.Error(err, "failed to retrieve BareMetalNetwork resource")
			}
			return ctrl.Result{}, err
		}
	}

	if bme.DeletionTimestamp.IsZero() == false {
		// bme is already deleted so we don't care about it
		if bme.Status.Phase == baremetalv1alpha1.BareMetalEndpointStatusPhaseDeleted {
			return ctrl.Result{}, nil
		}

		// address is nil so deleted it
		if bme.Status.Address == nil {
			bme.Status.Phase = baremetalv1alpha1.BareMetalEndpointStatusPhaseDeleted
			err := r.Status().Update(ctx, bme)
			if err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}

		// bmn exists do cleanup stuffs
		if bmn != nil {
			if bme.Status.Phase != baremetalv1alpha1.BareMetalEndpointStatusPhaseDeleting {
				bme.Status.Phase = baremetalv1alpha1.BareMetalEndpointStatusPhaseDeleting
				err := r.Status().Update(ctx, bme)
				if err != nil {
					return ctrl.Result{}, err
				}
				return ctrl.Result{}, nil
			}

			// TODO: do we need to do any deletion things?
		}

		// we are done so set address to nil
		bme.Status.Address = nil
		err := r.Status().Update(ctx, bme)
		if err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// bme is already addressed so we don't care about it
	if bme.Status.Phase == baremetalv1alpha1.BareMetalEndpointStatusPhaseAddressed {
		return ctrl.Result{}, nil
	}

	// bme has an address so set it to addressed
	if bme.Status.Address != nil {
		bme.Status.Phase = baremetalv1alpha1.BareMetalEndpointStatusPhaseAddressed
		err := r.Status().Update(ctx, bme)
		if err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// if we can't find the bmn retry back-off, it may eventually be found
	if bmn == nil {
		return ctrl.Result{Requeue: true}, nil
	}

	// bme is pending so set it to addressing
	if bme.Status.Phase == baremetalv1alpha1.BareMetalEndpointStatusPhasePending {
		bme.Status.Phase = baremetalv1alpha1.BareMetalEndpointStatusPhaseAddressing
		err := r.Status().Update(ctx, bme)
		if err != nil {
			return ctrl.Result{}, err
		}
		r.Recorder.Eventf(bme, corev1.EventTypeNormal, "Addressing", "Endpoint is being addressed")
		return ctrl.Result{}, nil
	}

	// lock so we don't accidentally hand out duplicate ips
	// this doesn't need to be fast, it needs to be accurate
	r.addressLock.Lock()
	defer r.addressLock.Unlock()

	bmeList := &baremetalv1alpha1.BareMetalEndpointList{}
	err := r.List(ctx, bmeList, client.MatchingFields{"spec.networkRef.group,kind,name": baremetalv1alpha1.GroupVersion.Group + "." + bmn.Kind + "." + bmn.Name})
	if err != nil {
		return ctrl.Result{}, err
	}

	cidrIP, network, err := net.ParseCIDR(bmn.Spec.CIDR)
	if err != nil {
		return ctrl.Result{}, err
	}
	networkIP := cidrIP.Mask(network.Mask)

	start := net.ParseIP(bmn.Spec.Start)
	end := net.ParseIP(bmn.Spec.End)

	gateway := net.ParseIP(bmn.Spec.Gateway)

	// find the broadcast address
	broadcast := net.IP(make([]byte, len(network.Mask)))
	for i := range network.Mask {
		broadcast[i] = networkIP[i] | ^network.Mask[i]
	}

	var nextIP *net.IP
	for ip := start; bytes.Compare(end, ip) >= 0; r.inc(ip) {
		// ignore network, gateway and broadcast IPs
		if ip.Equal(networkIP) || ip.Equal(gateway) || ip.Equal(broadcast) {
			continue
		}

		found := false
		// ignore nameservers (they might be in the range)
		for _, ns := range bmn.Spec.Nameservers {
			nsIP := net.ParseIP(ns)
			if ip.Equal(nsIP) {
				found = true
				break
			}
		}

		// if nameserver wasn't found check allocated
		if found == false {
			// ignore ips already allocated
			for _, bme := range bmeList.Items {
				if bme.Status.Address == nil {
					continue
				}

				bmeIP := net.ParseIP(bme.Status.Address.IP)
				if ip.Equal(bmeIP) {
					found = true
					break
				}
			}
		}

		// if ip is not allocated set it as the nextIP
		if found == false {
			ipCopy := make(net.IP, len(ip))
			copy(ipCopy, ip)
			nextIP = &ipCopy
			break
		}
	}

	// if a next ip couldn't be found event and try again with back-off
	if nextIP == nil {
		r.Recorder.Eventf(bme, corev1.EventTypeWarning, "NoIPAvailable", "Could find an available IP address on the BareMetalNetwork %s", bme.Spec.NetworkRef.Name)
		return ctrl.Result{Requeue: true}, err
	}

	// a next ip was found so set the address
	bme.Status.Address = &baremetalv1alpha1.BareMetalEndpointStatusAddress{
		IP:          nextIP.String(),
		CIDR:        bmn.Spec.CIDR,
		Gateway:     bmn.Spec.Gateway,
		Nameservers: bmn.Spec.Nameservers,
		Search:      bmn.Spec.Search,
	}
	err = r.Status().Update(ctx, bme)
	if err != nil {
		return ctrl.Result{}, err
	}
	r.Recorder.Eventf(bme, corev1.EventTypeNormal, "Addressed", "Endpoint has been addressed")
	return ctrl.Result{}, nil
}

// helper method for incrementing IP addresses
// I wish Go had a good ipaddress library like python https://docs.python.org/3/library/ipaddress.html
func (r *Network) inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

func (r *Network) SetupWithManager(mgr ctrl.Manager) error {
	// custom field index so we can index based off of the network ref settings
	if err := mgr.GetFieldIndexer().IndexField(&baremetalv1alpha1.BareMetalEndpoint{}, "spec.networkRef.group,kind,name", func(rawObj runtime.Object) []string {
		bme := rawObj.(*baremetalv1alpha1.BareMetalEndpoint)
		return []string{bme.Spec.NetworkRef.Group + "." + bme.Spec.NetworkRef.Kind + "." + bme.Spec.NetworkRef.Name}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named("BareMetalEndpointNetwork").
		For(&baremetalv1alpha1.BareMetalEndpoint{}).
		Complete(r)
}
