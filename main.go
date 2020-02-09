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

package main

import (
	"flag"
	"math/rand"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/utils/clock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	baremetalv1alpha1 "github.com/rmb938/kube-baremetal/api/v1alpha1"
	"github.com/rmb938/kube-baremetal/controllers"
	"github.com/rmb938/kube-baremetal/webhooks"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = baremetalv1alpha1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	flag.Parse()

	ctrl.SetLogger(zap.New(func(o *zap.Options) {
		o.Development = true
	}))

	rand.Seed(time.Now().Unix())

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		LeaderElection:     enableLeaderElection,
		Port:               9443,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controllers.BareMetalDiscoveryReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("BareMetalDiscovery"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "BareMetalDiscovery")
		os.Exit(1)
	}
	(&webhooks.BareMetalDiscoveryWebhook{}).SetupWebhookWithManager(mgr)
	if err = (&controllers.BareMetalHardwareReconciler{
		Client:   mgr.GetClient(),
		Log:      ctrl.Log.WithName("controllers").WithName("BareMetalHardware"),
		Scheme:   mgr.GetScheme(),
		Clock:    clock.RealClock{},
		Recorder: mgr.GetEventRecorderFor("BareMetalHardware"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "BareMetalHardware")
		os.Exit(1)
	}
	(&webhooks.BareMetalHardwareWebhook{}).SetupWebhookWithManager(mgr)
	if err = (&controllers.BareMetalInstanceReconciler{
		Client:   mgr.GetClient(),
		Log:      ctrl.Log.WithName("controllers").WithName("BareMetalInstance"),
		Scheme:   mgr.GetScheme(),
		Clock:    clock.RealClock{},
		Recorder: mgr.GetEventRecorderFor("BareMetalInstance"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "BareMetalInstance")
		os.Exit(1)
	}
	(&webhooks.BareMetalInstanceWebhook{}).SetupWebhookWithManager(mgr)
	// +kubebuilder:scaffold:builder

	signalHandler := ctrl.SetupSignalHandler()

	setupLog.Info("starting manager")
	if err := mgr.Start(signalHandler); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
