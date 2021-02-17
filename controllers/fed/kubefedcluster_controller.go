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
	"strconv"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	_ "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	typesv1beta1 "multi.tmax.io/apis/external/v1beta1"

	constant "multi.tmax.io/controllers/util"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
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
// +kubebuilder:rbac:groups=types.kubefed.io,resources=federatedconfigmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete

func (r *KubeFedClusterReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	// log := r.Log.WithValues("KubeFedClusters", req.NamespacedName)
	// // your logic here

	// //get kubefedcluster
	// kfc := &fedcore.KubeFedCluster{}
	// if err := r.Get(context.TODO(), req.NamespacedName, kfc); err != nil {
	// 	if errors.IsNotFound(err) {
	// 		log.Info("KubeFedClusters resource not found. Ignoring since object must be deleted.")
	// 		return ctrl.Result{}, nil
	// 	}

	// 	log.Error(err, "Failed to get KubeFedClusters")
	// 	return ctrl.Result{}, err
	// }

	// helper, _ := patch.NewHelper(kfc, r.Client)
	// defer func() {
	// 	if err := helper.Patch(context.TODO(), kfc); err != nil {
	// 		r.Log.Error(err, "kfc patch error")
	// 	}
	// }()

	// if kfc.DeletionTimestamp != nil {
	// 	if err := r.deleteSecret(kfc); err != nil {
	// 		log.Error(err, "Related secret cann't be deleted")
	// 	}

	// 	controllerutil.RemoveFinalizer(kfc, constant.KubefedclusterFinalizer)

	// 	return ctrl.Result{}, nil
	// }

	// // Add finalizer first if not exist to avoid the race condition between init and delete
	// if !controllerutil.ContainsFinalizer(kfc, constant.KubefedclusterFinalizer) {
	// 	controllerutil.AddFinalizer(kfc, constant.KubefedclusterFinalizer)
	// }

	// // set cluster owner
	// if len(kfc.OwnerReferences) == 0 {
	// 	r.patchKubeFedCluster(kfc)
	// } else {
	// 	for index, ref := range kfc.OwnerReferences {
	// 		if ref.Kind == "Cluster" {
	// 			break
	// 		}

	// 		if index == len(kfc.OwnerReferences)-1 {
	// 			r.patchKubeFedCluster(kfc)
	// 		}
	// 	}
	// }

	// //create hyperclusterresources if doesn't exist
	// if err := r.isHcr(req.NamespacedName); err != nil {
	// 	if err := r.createHcr(req.NamespacedName, kfc); err != nil {
	// 		log.Error(err, "HyperClusterResources cannot created")
	// 	}
	// }

	// //modify federatedconfigmap
	// if err := r.handleFcm(req.NamespacedName.Name); err != nil {
	// 	log.Error(err, "handling federatedconfigmap error")
	// }

	return ctrl.Result{}, nil
}

func (r *KubeFedClusterReconciler) patchKubeFedCluster(kfc *fedcore.KubeFedCluster) {
	c := &clusterv1.Cluster{}

	key := types.NamespacedName{
		Name:      kfc.Name,
		Namespace: "default",
	}

	if err := r.Get(context.TODO(), key, c); err != nil {
		r.Log.Error(err, "get cluster error")
	}

	kfc.SetOwnerReferences(append(kfc.OwnerReferences, metav1.OwnerReference{
		APIVersion: clusterv1.GroupVersion.String(),
		Kind:       "Cluster",
		Name:       c.Name,
		UID:        c.UID,
	}))
}

func (r *KubeFedClusterReconciler) deleteSecret(kfc *fedcore.KubeFedCluster) error {
	secret := &corev1.Secret{}
	secretKey := types.NamespacedName{Name: kfc.Spec.SecretRef.Name, Namespace: constant.KubeFedNamespace}

	r.Get(context.TODO(), secretKey, secret)

	if err := r.Delete(context.TODO(), secret); err != nil {
		return err
	}

	return nil
}

func (r *KubeFedClusterReconciler) handleFcm(clusterName string) error {
	fcm := &typesv1beta1.FederatedConfigMap{}
	key := types.NamespacedName{Name: constant.FederatedConfigMapName, Namespace: constant.FederatedConfigMapNamespace}

	if err := r.Get(context.TODO(), key, fcm); err != nil && errors.IsNotFound(err) {
		r.createFcm(clusterName)
		return nil
	}

	helper, _ := patch.NewHelper(fcm, r.Client)
	defer func() {
		if err := helper.Patch(context.TODO(), fcm); err != nil {
			r.Log.Error(err, "fcm patch error")
		}
	}()

	if _, found := Find(fcm.Spec.Placement.Clusters, clusterName); !found {
		fcm.Spec.Placement.Clusters = append(fcm.Spec.Placement.Clusters, typesv1beta1.Clusters{Name: clusterName})
		override := &typesv1beta1.Overrides{
			ClusterName: clusterName,
			ClusterOverrides: []typesv1beta1.ClusterOverrides{
				{
					Path:  "/data/cluster-name",
					Value: clusterName,
				},
			},
		}
		fcm.Spec.Overrides = append(fcm.Spec.Overrides, *override)
	}

	return nil
}

func (r *KubeFedClusterReconciler) createFcm(clusterName string) error {
	svcKey := types.NamespacedName{Name: constant.MultiApiServerServiceName, Namespace: constant.MultiApiServerNamespace}
	svc := &corev1.Service{}

	if err := r.Get(context.TODO(), svcKey, svc); err != nil {
		if errors.IsNotFound(err) {
			return err
		}
	}

	fcm := &typesv1beta1.FederatedConfigMap{}
	fcm.Name = constant.FederatedConfigMapName
	fcm.Namespace = constant.FederatedConfigMapNamespace
	fcm.Spec = typesv1beta1.FederatedConfigMapSpec{
		Placement: typesv1beta1.Placement{
			Clusters: []typesv1beta1.Clusters{
				{
					Name: clusterName,
				},
			},
		},
		Template: typesv1beta1.Template{
			Data: map[string]string{
				"mgnt-ip":      svc.Status.LoadBalancer.Ingress[0].IP,
				"mgnt-port":    strconv.Itoa(int(svc.Spec.Ports[0].Port)),
				"cluster-name": "cluster-name",
			},
		},
		Overrides: []typesv1beta1.Overrides{
			{
				ClusterName: clusterName,
				ClusterOverrides: []typesv1beta1.ClusterOverrides{
					{
						Path:  "/data/cluster-name",
						Value: clusterName,
					},
				},
			},
		},
	}

	if err := r.Create(context.TODO(), fcm); err != nil {
		return err
	}

	return nil
}

func Find(slice []typesv1beta1.Clusters, val string) (int, bool) {
	for i, item := range slice {
		if item.Name == val {
			return i, true
		}
	}
	return -1, false
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
