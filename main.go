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

	claimv1alpha1 "github.com/tmax-cloud/hypercloud-multi-operator/apis/claim/v1alpha1"
	clusterv1alpha1 "github.com/tmax-cloud/hypercloud-multi-operator/apis/cluster/v1alpha1"
	claimcontroller "github.com/tmax-cloud/hypercloud-multi-operator/controllers/claim"
	clustercontroller "github.com/tmax-cloud/hypercloud-multi-operator/controllers/cluster"

	// +kubebuilder:scaffold:imports

	clusterv1alpha3 "sigs.k8s.io/cluster-api/api/v1alpha3"
	fedv1a1 "sigs.k8s.io/kubefed/pkg/apis/core/v1alpha1"
	fedv1b1 "sigs.k8s.io/kubefed/pkg/apis/core/v1beta1"
	fedmultiv1a1 "sigs.k8s.io/kubefed/pkg/apis/multiclusterdns/v1alpha1"

	console "github.com/tmax-cloud/console-operator/api/v1"
	typesv1beta1 "github.com/tmax-cloud/hypercloud-multi-operator/apis/external/v1beta1"

	clusterController "github.com/tmax-cloud/hypercloud-multi-operator/controllers/capi"
	federatedServiceController "github.com/tmax-cloud/hypercloud-multi-operator/controllers/fed"
	k8scontroller "github.com/tmax-cloud/hypercloud-multi-operator/controllers/k8s"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1alpha3"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(claimv1alpha1.AddToScheme(scheme))
	utilruntime.Must(clusterv1alpha1.AddToScheme(scheme))

	utilruntime.Must(clusterv1alpha3.AddToScheme(scheme))
	utilruntime.Must(fedv1b1.AddToScheme(scheme))
	utilruntime.Must(fedv1a1.AddToScheme(scheme))
	utilruntime.Must(fedmultiv1a1.AddToScheme(scheme))
	utilruntime.Must(typesv1beta1.AddToScheme(scheme))
	utilruntime.Must(controlplanev1.AddToScheme(scheme))
	utilruntime.Must(console.AddToScheme(scheme))
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
		LeaderElectionID:   "86810e1d.tmax.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&claimcontroller.ClusterClaimReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("ClusterClaim"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ClusterClaim")
		os.Exit(1)
	}
	if err = (&clustercontroller.ClusterManagerReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("ClusterManager"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ClusterManager")
		os.Exit(1)
	}
	if err = (&claimv1alpha1.ClusterClaim{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "ClusterClaim")
		os.Exit(1)
	}
	if err = (&clusterv1alpha1.ClusterManager{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "ClusterManager")
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
	if err = (&k8scontroller.SecretReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controller").WithName("secretController"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "secretController")
		os.Exit(1)
	}
	if err = (&k8scontroller.ServiceReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controller").WithName("serviceController"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "serviceController")
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
	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
