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
	"reflect"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	hyperv1 "multi.tmax.io/apis/hyper/v1"
	fedcore "sigs.k8s.io/kubefed/pkg/apis/core/v1beta1"
)

// ClusterReconciler reconciles a Memcached object
type KubeFedClusterReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=core.kubefed.io,resources=kubefedclusters;,verbs=get;watch;list;
// +kubebuilder:rbac:groups=hyper.multi.tmax.io,resources=hyperclusterresources,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=hyper.multi.tmax.io,resources=hyperclusterresources/status,verbs=get;update;patch

func (r *KubeFedClusterReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	log := r.Log.WithValues("KubeFedClusters", req.NamespacedName)
	// your logic here

	//get kubefedcluster
	kfc := &fedcore.KubeFedCluster{}
	if err := r.Get(context.TODO(), req.NamespacedName, kfc); err != nil {
		if errors.IsNotFound(err) {
			log.Info("KubeFedClusters resource not found. Ignoring since object must be deleted.")
			return ctrl.Result{}, nil
		}

		log.Error(err, "Failed to get KubeFedClusters")
		return ctrl.Result{}, err
	}

	if kfc.DeletionTimestamp != nil {
		if err := r.deleteHcr(req.NamespacedName); err != nil {
			log.Error(err, "HyperClusterResources cannot deleted")
		}
	}

	//create hyperclusterresources if doesn't exist
	if err := r.isHcr(req.NamespacedName); err != nil {
		if err := r.createHcr(req.NamespacedName, kfc); err != nil {
			log.Error(err, "HyperClusterResources cannot created")
		}
	}
	return ctrl.Result{}, nil
}

func (r *KubeFedClusterReconciler) deleteHcr(key types.NamespacedName) error {
	hcr := &hyperv1.HyperClusterResource{}
	if err := r.Get(context.TODO(), key, hcr); err != nil {
		return err
	}

	if err := r.Delete(context.TODO(), hcr); err != nil {
		return err
	}

	return nil
}

func (r *KubeFedClusterReconciler) createHcr(key types.NamespacedName, kfc *fedcore.KubeFedCluster) error {
	hcr := &hyperv1.HyperClusterResource{}
	hcr.Name = key.Name
	hcr.Namespace = key.Namespace

	hcr.SetOwnerReferences(append(hcr.OwnerReferences, metav1.OwnerReference{
		APIVersion: fedcore.SchemeGroupVersion.String(),
		Kind:       "KubeFedCluster",
		Name:       kfc.Name,
		UID:        kfc.UID,
	}))

	hcr.Spec.Provider = "none"
	hcr.Spec.Version = "none"
	hcr.Status.Ready = true

	if err := r.Create(context.TODO(), hcr); err != nil {
		return err
	}

	return nil
}

func (r *KubeFedClusterReconciler) isHcr(key types.NamespacedName) error {
	hcr := &hyperv1.HyperClusterResource{}
	if err := r.Get(context.TODO(), key, hcr); err != nil {
		if errors.IsNotFound(err) {
			return err
		}
	}

	return nil
}

func (r *KubeFedClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&fedcore.KubeFedCluster{}).
		WithEventFilter(
			predicate.Funcs{
				// Avoid reconciling if the event triggering the reconciliation is related to incremental status updates
				// for kubefedcluster resources only
				UpdateFunc: func(e event.UpdateEvent) bool {
					oldkfc := e.ObjectOld.(*fedcore.KubeFedCluster).DeepCopy()
					newkfc := e.ObjectNew.(*fedcore.KubeFedCluster).DeepCopy()

					oldkfc.Status = fedcore.KubeFedClusterStatus{}
					newkfc.Status = fedcore.KubeFedClusterStatus{}

					oldkfc.ObjectMeta.ResourceVersion = ""
					newkfc.ObjectMeta.ResourceVersion = ""

					return !reflect.DeepEqual(oldkfc, newkfc)
				},
			},
		).
		Complete(r)
}
