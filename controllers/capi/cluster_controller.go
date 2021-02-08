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
	"strings"

	"github.com/go-logr/logr"
	"github.com/prometheus/common/log"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/cluster-api/util/patch"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	restclient "k8s.io/client-go/rest"
	constant "multi.tmax.io/controllers/util"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"

	console "github.com/tmax-cloud/console-operator/api/v1"
)

// ClusterReconciler reconciles a Memcached object
type ClusterReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=kubeadmcontrolplanes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=kubeadmcontrolplanes/status,verbs=get;update;patch

func (r *ClusterReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	reqLogger := r.Log.WithValues("Cluster", req.NamespacedName)
	// your logic here

	//catch cluster
	cluster := &clusterv1.Cluster{}
	if err := r.Get(context.TODO(), req.NamespacedName, cluster); err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("cluster resource not found. Ignoring since object must be deleted.")
			return ctrl.Result{}, nil
		}

		reqLogger.Error(err, "Failed to get cluster")
		return ctrl.Result{}, err
	}

	//handling delete
	if cluster.DeletionTimestamp != nil {
		r.deleteConsole(cluster)
	}

	/*
		check cluster's conditions
		  if true: annotate to cluster's secret what contains cluster's kueconfig
		  else: do nothing
	*/
	if ok := meetCondi(*cluster); ok {
		reqLogger.Info(cluster.GetName() + " meets condition ")

		if err := r.patchSecret(types.NamespacedName{Name: cluster.GetName() + constant.KubeconfigPostfix, Namespace: cluster.Namespace}, constant.WatchAnnotationJoinValue); err != nil {
			_ = r.patchCluster(cluster, "error")

			reqLogger.Error(err, "Failed to patch secret")
			return ctrl.Result{}, err
		}

		if err := r.patchCluster(cluster, "success"); err != nil {
			return ctrl.Result{}, err
		}

		reqLogger.Info(cluster.GetName() + " is successful")

		r.deployRB2remote(cluster.Name, cluster.Annotations["owner"])
		r.handleConsole(cluster)
	} else {
		reqLogger.Info(cluster.GetName() + " doesn't meet the condition")
	}
	return ctrl.Result{}, nil
}

func (r *ClusterReconciler) deleteConsole(c *clusterv1.Cluster) {
	cs := &console.Console{}
	key := types.NamespacedName{Name: "hypercloud5-multi-cluster", Namespace: "console-system"}

	r.Get(context.TODO(), key, cs)

	helper, _ := patch.NewHelper(cs, r.Client)
	defer func() {
		if err := helper.Patch(context.TODO(), cs); err != nil {
			r.Log.Error(err, "console patch error")
		}
	}()

	delete(cs.Spec.Configuration.Routers, c.Name)
}

func (r *ClusterReconciler) handleConsole(c *clusterv1.Cluster) {
	cs := &console.Console{}
	key := types.NamespacedName{Name: "hypercloud5-multi-cluster", Namespace: "console-system"}

	router := &console.Router{
		Server: "https://" + c.Spec.ControlPlaneEndpoint.Host + ":6443",
		Rule:   "PathPrefix(`/api/" + c.Name + "/`)",
		Path:   "/api/" + c.Name + "/",
	}

	if err := r.Get(context.TODO(), key, cs); err != nil {
		if errors.IsNotFound(err) {
			r.Log.Info("console resource not found. create console resource.")

			newCs := &console.Console{}
			newCs.Name = "hypercloud5-multi-cluster"
			newCs.Namespace = "console-system"

			masterRouter := &console.Router{
				Server: "https://",
				Rule:   "PathPrefix(`/api/master/`)",
				Path:   "/api/master/",
			}

			newCs.Spec.Configuration.Routers = map[string]*console.Router{
				c.Name:   router,
				"master": masterRouter,
			}

			if err2 := r.Create(context.TODO(), newCs); err2 != nil {
				r.Log.Info(err2.Error())
			}
		}
	} else {
		//set patch helper
		helper, _ := patch.NewHelper(cs, r.Client)
		defer func() {
			if err := helper.Patch(context.TODO(), cs); err != nil {
				r.Log.Error(err, "console patch error")
			}
		}()

		cs.Spec.Configuration.Routers[c.Name] = router
	}
}

/*
  checkList
   1. has annotation with "key: federation, value: join"
   2. ControlPlaneInitialized check to confirm node is ready
*/
func meetCondi(c clusterv1.Cluster) bool {
	if val, ok := c.GetAnnotations()[constant.WatchAnnotationKey]; ok {
		if ok := strings.EqualFold(val, constant.WatchAnnotationJoinValue); ok {
			if &c.Status != nil && &c.Status.ControlPlaneInitialized != nil && c.Status.ControlPlaneInitialized {
				return true
			}
		}
	}
	return false
}

func (r *ClusterReconciler) patchCluster(bcluster *clusterv1.Cluster, result string) error {
	acluster := bcluster.DeepCopy()
	acluster.GetAnnotations()[constant.WatchAnnotationKey] = result

	if err := r.Patch(context.TODO(), acluster, client.MergeFrom(bcluster)); err != nil {
		return err
	}

	return nil
}

func (r *ClusterReconciler) patchSecret(key types.NamespacedName, status string) error {
	bsecret := &corev1.Secret{}

	if err := r.Get(context.TODO(), key, bsecret); err != nil {
		return err
	}

	asecret := bsecret.DeepCopy()
	if asecret.GetAnnotations() == nil {
		asecret.Annotations = map[string]string{}
	}
	asecret.GetAnnotations()[constant.WatchAnnotationKey] = status
	if err := r.Patch(context.TODO(), asecret, client.MergeFrom(bsecret)); err != nil {
		return err
	}

	return nil
}

func (r *ClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&clusterv1.Cluster{}).
		Complete(r)
}

func (r *ClusterReconciler) deployRB2remote(clusterName, owner string) {
	var remoteClient client.Client

	if restConfig, err := getConfigFromSecret(r.Client, clusterName); err != nil {
		log.Error(err)
		return
	} else {
		remoteScheme := runtime.NewScheme()
		utilruntime.Must(rbacv1.AddToScheme(remoteScheme))
		remoteClient, err = client.New(restConfig, client.Options{Scheme: remoteScheme})

		rb := &rbacv1.ClusterRoleBinding{}
		rb.Name = clusterName + "-" + owner
		rb.RoleRef = rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "cluster-admin",
		}
		sub := rbacv1.Subject{
			Kind:     "User",
			APIGroup: "rbac.authorization.k8s.io",
			Name:     owner,
		}
		rb.Subjects = append(rb.Subjects, sub)

		if err := remoteClient.Create(context.TODO(), rb); err != nil {
			log.Error(err)
		}
	}
}

func getConfigFromSecret(c client.Client, clusterName string) (*restclient.Config, error) {
	secret := &corev1.Secret{}

	if err := c.Get(context.TODO(), types.NamespacedName{Name: clusterName + "-kubeconfig", Namespace: "capi-system"}, secret); err != nil {
		log.Errorln(err)
	}

	return getKubeConfig(*secret)
}

func getKubeConfig(s corev1.Secret) (*restclient.Config, error) {
	if value, ok := s.Data["value"]; ok {
		if clientConfig, err := clientcmd.NewClientConfigFromBytes(value); err == nil {
			if restConfig, err := clientConfig.ClientConfig(); err == nil {
				return restConfig, nil
			}
		}
	}
	return nil, errors.NewBadRequest("getClientConfig Error")
}
