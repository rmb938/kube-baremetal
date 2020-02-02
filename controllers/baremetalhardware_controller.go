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

	baremetalv1alpha1 "github.com/rmb938/kube-baremetal/api/v1alpha1"
)

// BareMetalHardwareReconciler reconciles a BareMetalHardware object
type BareMetalHardwareReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=baremetal.com.rmb938,resources=baremetalhardwares,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=baremetal.com.rmb938,resources=baremetalhardwares/status,verbs=get;update;patch

func (r *BareMetalHardwareReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("baremetalhardware", req.NamespacedName)

	// TODO: deletion
	//  taint with NoProvision
	//  if instanceRef is not nil prevent deletion and event saying there's an instance still

	// TODO: taint with NoProvision when CanProvision is false
	//   this should be the first thing we do if not deleting

	// TODO: if hardware is nil find discovery any copy it
	//  if can't find discovery event
	//  once copied set HardwareSet condition to true

	// TODO: conditions
	// 	ImageDriveValid - image drive is valid
	//  NicsAreValid - nics are valid
	//  HardwareSet - hardware is set

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
		For(&baremetalv1alpha1.BareMetalDiscovery{}). // explicitly for discovery because this controller needs them in the cache
		Complete(r)
}
