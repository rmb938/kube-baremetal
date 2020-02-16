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

package discovery

import (
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	baremetalv1alpha1 "github.com/rmb938/kube-baremetal/api/v1alpha1"
)

type DummyReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

func (r *DummyReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	// We don't care about reconciling
	return ctrl.Result{}, nil
}

func (r *DummyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(&baremetalv1alpha1.BareMetalDiscovery{}, "spec.systemUUID", func(rawObj runtime.Object) []string {
		bmd := rawObj.(*baremetalv1alpha1.BareMetalDiscovery)
		return []string{string(bmd.Spec.SystemUUID)}
	}); err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(&baremetalv1alpha1.BareMetalHardware{}, "spec.systemUUID", func(rawObj runtime.Object) []string {
		bmh := rawObj.(*baremetalv1alpha1.BareMetalHardware)
		return []string{string(bmh.Spec.SystemUUID)}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&baremetalv1alpha1.BareMetalDiscovery{}).
		Watches(&source.Kind{Type: &baremetalv1alpha1.BareMetalHardware{}}, handler.Funcs{}).
		Watches(&source.Kind{Type: &baremetalv1alpha1.BareMetalInstance{}}, handler.Funcs{}).
		Complete(r)
}
