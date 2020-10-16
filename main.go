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
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	clusterv1alpha3 "sigs.k8s.io/cluster-api/api/v1alpha3"
	fedv1a1 "sigs.k8s.io/kubefed/pkg/apis/core/v1alpha1"
	fedv1b1 "sigs.k8s.io/kubefed/pkg/apis/core/v1beta1"
	fedmultiv1a1 "sigs.k8s.io/kubefed/pkg/apis/multiclusterdns/v1alpha1"

	typesv1beta1 "multi.tmax.io/apis/external/v1beta1"
	hyperv1 "multi.tmax.io/apis/hyper/v1"
	secretController "multi.tmax.io/controllers"
	clusterController "multi.tmax.io/controllers/capi"
	federatedServiceController "multi.tmax.io/controllers/fed"
	hypercontroller "multi.tmax.io/controllers/hyper"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1alpha3"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(clusterv1alpha3.AddToScheme(scheme))

	utilruntime.Must(fedv1b1.AddToScheme(scheme))

	utilruntime.Must(fedv1a1.AddToScheme(scheme))

	utilruntime.Must(fedmultiv1a1.AddToScheme(scheme))

	utilruntime.Must(typesv1beta1.AddToScheme(scheme))

	utilruntime.Must(hyperv1.AddToScheme(scheme))

	utilruntime.Must(controlplanev1.AddToScheme(scheme))
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

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		Port:               9443,
		LeaderElection:     enableLeaderElection,
		LeaderElectionID:   "19da4036.tmax.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&clusterController.ClusterReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controller").WithName("capi/clusterController"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "capi/clusterController")
		os.Exit(1)
	}
	if err = (&secretController.SecretReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controller").WithName("secretController"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "secretController")
		os.Exit(1)
	}
	if err = (&federatedServiceController.FederatedServiceReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controller").WithName("fed/federatedserviceController"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "fed/federatedserviceController")
		os.Exit(1)
	}
	if err = (&federatedServiceController.KubeFedClusterReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controller").WithName("fed/kubefedclusterController"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "fed/kubefedclusterController")
		os.Exit(1)
	}
	if err = (&hypercontroller.HyperClusterResourcesReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("HyperClusterResources"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "HyperClusterResources")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
