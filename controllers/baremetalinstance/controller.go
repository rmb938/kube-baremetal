package baremetalinstance

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
		// if bmi is not phase terminated we don't care about it
		if bmi.Status.Phase != baremetalv1alpha1.BareMetalInstanceStatusPhaseTerminated {
			return ctrl.Result{}, nil
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

	return ctrl.Result{}, nil
}

func (r *Controller) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&baremetalv1alpha1.BareMetalInstance{}).
		Complete(r)
}
