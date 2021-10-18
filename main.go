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
	"fmt"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	resourcebaloisechv1alpha1 "github.com/baloise/os3-copier/api/v1alpha1"
	"github.com/baloise/os3-copier/controllers"
	// +kubebuilder:scaffold:imports
)

const (
	WatchNamespaceEnvName = "WATCH_NAMESPACE"
	SyncPeriodEnvName     = "SYNC_PERIOD"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(resourcebaloisechv1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	watchNamespace, err := getEnvVar(WatchNamespaceEnvName)
	if err != nil {
		setupLog.Error(err, "unable to get WatchNamespace, "+
			"please set environment variable "+WatchNamespaceEnvName)
		os.Exit(1)
	}

	syncPeriodS, err := getEnvVar(SyncPeriodEnvName)
	if err != nil {
		setupLog.Error(err, "unable to get SyncPeriod for Reconciler, "+
			"please set environment variable "+SyncPeriodEnvName)
		os.Exit(1)
	}

	syncDuration, err := time.ParseDuration(syncPeriodS)
	if err != nil {
		setupLog.Error(err, "error parsing SyncPeriod from "+
			syncPeriodS+"please fix value for environment variable "+SyncPeriodEnvName+
			"A duration string is a possibly signed sequence of decimal numbers, each with optional "+
			"fraction and a unit suffix,such as \"300ms\", \"-1.5h\" or \"2h45m\". Valid time units are "+
			"\"ns\", \"us\" (or \"µs\"), \"ms\", \"s\", \"m\", \"h\".")
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		Port:               9443,
		LeaderElection:     enableLeaderElection,
		LeaderElectionID:   "3dacd622.baloise.ch",
		SyncPeriod:         &syncDuration,
		Namespace:          watchNamespace,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controllers.CopyResourceReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("CopyResource"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "CopyResource")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("health", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("check", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager for os3-copier. watched namespace is: " + watchNamespace)
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func getEnvVar(name string) (string, error) {
	ns, found := os.LookupEnv(name)
	if !found {
		return "", fmt.Errorf("%s must be set", name)
	}
	return ns, nil
}

func getSyncPeriod() (time.Duration, error) {
	syncPeriodS, err := getEnvVar(SyncPeriodEnvName)
	if err != nil {
		setupLog.Error(err, "unable to get SyncPeriod for Reconciler, "+
			"please set environment variable "+SyncPeriodEnvName)
	}
	duration, err := time.ParseDuration(syncPeriodS)

	return duration, nil
}
