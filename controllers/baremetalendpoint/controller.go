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
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	baremetalapi "github.com/rmb938/kube-baremetal/api"
	baremetalv1alpha1 "github.com/rmb938/kube-baremetal/api/v1alpha1"
)

type Controller struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

func (r *Controller) Reconcile(req ctrl.Request) (ctrl.Result, error) {
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

	if bme.DeletionTimestamp.IsZero() == false {
		// we only care about stuff that is deleted
		if bme.Status.Phase != baremetalv1alpha1.BareMetalEndpointStatusPhaseDeleted {
			return ctrl.Result{}, nil
		}

		// TODO: any deletion protection logic?
		//  don't delete until the owning instance is gone?

		// Done deleting so remove bme finalizer
		baremetalapi.RemoveFinalizer(bme, baremetalv1alpha1.BareMetalEndpointFinalizer)
		err := r.Update(ctx, bme)
		if err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if len(bme.Status.Phase) == 0 {
		bme.Status.Phase = baremetalv1alpha1.BareMetalEndpointStatusPhasePending
		err := r.Status().Update(ctx, bme)
		if err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

func (r *Controller) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&baremetalv1alpha1.BareMetalEndpoint{}).
		Complete(r)
}
