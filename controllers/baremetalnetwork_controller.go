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

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	baremetalapi "github.com/rmb938/kube-baremetal/api"
	baremetalv1alpha1 "github.com/rmb938/kube-baremetal/api/v1alpha1"
)

// BareMetalNetworkReconciler reconciles a BareMetalNetwork object
type BareMetalNetworkReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=baremetal.com.rmb938,resources=baremetalnetworks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=baremetal.com.rmb938,resources=baremetalnetworks/status,verbs=get;update;patch

func (r *BareMetalNetworkReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("baremetalnetwork", req.NamespacedName)

	bmn := &baremetalv1alpha1.BareMetalNetwork{}
	if err := r.Client.Get(ctx, req.NamespacedName, bmn); err != nil {
		err = client.IgnoreNotFound(err)
		if err != nil {
			log.Error(err, "failed to retrieve BareMetalNetwork resource")
		}
		return ctrl.Result{}, err
	}

	if bmn.DeletionTimestamp.IsZero() == false {
		// TODO: any deletion protection logic?
		//  should we wait until all "our" endpoints are deleted?

		// Done deleting so remove bme finalizer
		baremetalapi.RemoveFinalizer(bmn, baremetalv1alpha1.BareMetalNetworkFinalizer)
		err := r.Update(ctx, bmn)
		if err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

func (r *BareMetalNetworkReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&baremetalv1alpha1.BareMetalNetwork{}).
		Complete(r)
}
