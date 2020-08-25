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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	constant "multi.tmax.io/controllers/util"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
)

// ClusterReconciler reconciles a Memcached object
type ClusterReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters/status,verbs=get;update;patch

func (r *ClusterReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	reqLogger := r.Log.WithValues("Cluster", req.NamespacedName)
	reqLogger.Info("Reconciling capi/Cluster")
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

	/*
		check cluster's conditions
		  if true: annotate to cluster's secret what contains cluster's kueconfig
		  else: do nothing
	*/
	if ok := meetCondi(*cluster); ok {
		reqLogger.Info(cluster.GetName() + " meets condition ")

		if err := r.patchSecret(cluster.GetName()+constant.KubeconfigPostfix, constant.WatchAnnotationJoinValue); err != nil {
			_ = r.patchCluster(cluster, "error")

			reqLogger.Error(err, "Failed to patch secret")
			return ctrl.Result{}, err
		}

		if err := r.patchCluster(cluster, "success"); err != nil {
			return ctrl.Result{}, err
		}

		reqLogger.Info(cluster.GetName() + " is successful")
	} else {
		reqLogger.Info(cluster.GetName() + " doesn't meet the condition")
	}
	return ctrl.Result{}, nil
}

/*
  checkList
   1. has annotation with "key: federation, value: join"
   2. ControlPlaneInitialized check to confirm node is ready
*/
func meetCondi(c clusterv1.Cluster) bool {
	if val, ok := c.GetAnnotations()[constant.WatchAnnotationKey]; ok {
		if ok := strings.EqualFold(val, constant.WatchAnnotationJoinValue); ok {
			if &c.Status != nil && &c.Status.ControlPlaneInitialized != nil {
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

func (r *ClusterReconciler) patchSecret(name string, status string) error {
	key := types.NamespacedName{Namespace: constant.ClusterNamespace, Name: name}
	bsecret := &corev1.Secret{}

	if err := r.Get(context.TODO(), key, bsecret); err != nil {
		return err
	}

	asecret := bsecret.DeepCopy()
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
