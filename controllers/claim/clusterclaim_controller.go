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

	"github.com/go-logr/logr"
	"github.com/prometheus/common/log"
	clusterv1alpha1 "github.com/tmax-cloud/hypercloud-multi-operator/apis/cluster/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"

	claimv1alpha1 "github.com/tmax-cloud/hypercloud-multi-operator/apis/claim/v1alpha1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	CLAIM_API_GROUP             = "claim.tmax.io"
	CLAIM_API_Kind              = "clusterclaims"
	CLAIM_API_GROUP_VERSION     = "claim.tmax.io/v1alpha1"
	HYPERCLOUD_SYSTEM_NAMESPACE = ""
)

var AutoAdmit bool

// ClusterClaimReconciler reconciles a ClusterClaim object
type ClusterClaimReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=claim.tmax.io,resources=clusterclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=claim.tmax.io,resources=clusterclaims/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cluster.tmax.io,resources=clustermanagers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cluster.tmax.io,resources=clustermanagers/status,verbs=get;update;patch

func (r *ClusterClaimReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	log := r.Log.WithValues("ClusterClaim", req.NamespacedName)

	clusterClaim := &claimv1alpha1.ClusterClaim{}

	if err := r.Get(context.TODO(), req.NamespacedName, clusterClaim); err != nil {
		if errors.IsNotFound(err) {
			log.Info("ClusterClaim resource not found. Ignoring since object must be deleted.")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get ClusterClaim")
		return ctrl.Result{}, err
	} else if clusterClaim.Status.Phase == "" {
		if err := r.CreateClaimRole(clusterClaim); err != nil {
			return ctrl.Result{}, err
		}
	}

	if AutoAdmit == false {
		if clusterClaim.Status.Phase == "" {
			clusterClaim.Status.Phase = "Awaiting"
			clusterClaim.Status.Reason = "Waiting for admin approval"
			err := r.Status().Update(context.TODO(), clusterClaim)
			if err != nil {
				log.Error(err, "Failed to update ClusterClaim status")
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		} else if clusterClaim.Status.Phase == "Awaiting" {
			return ctrl.Result{}, nil
		}
	}

	if clusterClaim.Status.Phase == "Rejected" {
		return ctrl.Result{}, nil
	} else if clusterClaim.Status.Phase == "Admitted" {
		if err := r.CreateClusterManager(clusterClaim); err != nil {
			return ctrl.Result{}, err
		}

		clusterClaim.Status.Phase = "Success"
		if err := r.Status().Update(context.TODO(), clusterClaim); err != nil {
			log.Error(err, "Failed to update ClusterClaim status")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *ClusterClaimReconciler) CreateClaimRole(clusterClaim *claimv1alpha1.ClusterClaim) error {
	clusterRole := &rbacv1.ClusterRole{}
	clusterRoleName := clusterClaim.Annotations["creator"] + "-" + clusterClaim.Name + "-cc-role"
	clusterRoleNameKey := types.NamespacedName{Name: clusterRoleName, Namespace: HYPERCLOUD_SYSTEM_NAMESPACE}
	if err := r.Get(context.TODO(), clusterRoleNameKey, clusterRole); err != nil {
		if errors.IsNotFound(err) {
			newClusterRole := &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterRoleName,
				},
				Rules: []rbacv1.PolicyRule{
					{APIGroups: []string{CLAIM_API_GROUP}, Resources: []string{"clusterclaims"},
						ResourceNames: []string{clusterClaim.Name}, Verbs: []string{"get"}},
					{APIGroups: []string{CLAIM_API_GROUP}, Resources: []string{"clusterclaims/status"},
						ResourceNames: []string{clusterClaim.Name}, Verbs: []string{"get"}},
				},
			}
			ctrl.SetControllerReference(clusterClaim, newClusterRole, r.Scheme)
			err = r.Create(context.TODO(), newClusterRole)
			if err != nil {
				log.Error(err, "Failed to create "+clusterRoleName+" role.")
				return err
			}
		} else {
			log.Error(err, "Failed to get role")
			return err
		}
	}

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{}
	clusterRoleBindingName := clusterClaim.Annotations["creator"] + "-" + clusterClaim.Name + "-cc-rolebinding"
	clusterRoleBindingKey := types.NamespacedName{Name: clusterClaim.Name, Namespace: HYPERCLOUD_SYSTEM_NAMESPACE}
	if err := r.Get(context.TODO(), clusterRoleBindingKey, clusterRoleBinding); err != nil {
		if errors.IsNotFound(err) {
			newClusterRoleBinding := &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterRoleBindingName,
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "ClusterRole",
					Name:     clusterRoleName,
				},
				Subjects: []rbacv1.Subject{
					{
						APIGroup: "rbac.authorization.k8s.io",
						Kind:     "User",
						Name:     clusterClaim.Annotations["creator"],
					},
				},
			}
			ctrl.SetControllerReference(clusterClaim, newClusterRoleBinding, r.Scheme)
			err = r.Create(context.TODO(), newClusterRoleBinding)
			if err != nil {
				log.Error(err, "Failed to create "+clusterRoleBindingName+" role.")
				return err
			}
		} else {
			log.Error(err, "Failed to get rolebinding")
			return err
		}
	}
	return nil
}

func (r *ClusterClaimReconciler) CreateClusterManager(clusterClaim *claimv1alpha1.ClusterClaim) error {
	clm := &clusterv1alpha1.ClusterManager{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterClaim.Name,
			Annotations: map[string]string{
				"owner": clusterClaim.Annotations["creator"],
			},
		},
		FakeObjectMeta: clusterv1alpha1.FakeObjectMeta{
			FakeName: clusterClaim.Spec.ClusterName,
		},
		Spec: clusterv1alpha1.ClusterManagerSpec{
			Provider:   clusterClaim.Spec.Provider,
			Version:    clusterClaim.Spec.Version,
			Region:     clusterClaim.Spec.Region,
			SshKey:     clusterClaim.Spec.SshKey,
			MasterNum:  clusterClaim.Spec.MasterNum,
			MasterType: clusterClaim.Spec.MasterType,
			WorkerNum:  clusterClaim.Spec.WorkerNum,
			WorkerType: clusterClaim.Spec.WorkerType,
		},
		Status: clusterv1alpha1.ClusterManagerStatus{
			// Owner: clusterClaim.Annotations["creator"],
			Owner: map[string]string{
				clusterClaim.Annotations["creator"]: "admin",
			},
		},
	}
	err := r.Create(context.TODO(), clm)
	if err != nil {
		log.Error(err, "Failed to create ClusterManager")
		return err
	}

	err = r.Status().Update(context.TODO(), clm)
	if err != nil {
		log.Error(err, "Failed to update owner in ClusterManager")
		return err
	}

	return nil
}

//

func (r *ClusterClaimReconciler) requeueClusterClaimsForClusterManager(o handler.MapObject) []ctrl.Request {
	clm := o.Object.(*clusterv1alpha1.ClusterManager)
	log := r.Log.WithValues("objectMapper", "clusterManagerToClusterClaim", "clusterManager", clm.Name)

	// Don't handle deleted clusterManager
	// if !clm.ObjectMeta.DeletionTimestamp.IsZero() {
	// 	log.V(4).Info("clusterManager has a deletion timestamp, skipping mapping.")
	// 	return nil
	// }

	//get clusterManager
	cc := &claimv1alpha1.ClusterClaim{}
	key := types.NamespacedName{Namespace: HYPERCLOUD_SYSTEM_NAMESPACE, Name: clm.Name}
	if err := r.Get(context.TODO(), key, cc); err != nil {
		if errors.IsNotFound(err) {
			log.Info("ClusterClaim resource not found. Ignoring since object must be deleted.")
			return nil
		}
		log.Error(err, "Failed to get ClusterClaim")
		return nil
	}

	cc.Status.Phase = "Deleted"
	cc.Status.Reason = "cluster is deleted"
	err := r.Status().Update(context.TODO(), cc)
	if err != nil {
		log.Error(err, "Failed to update ClusterClaim status")
		return nil //??
	}
	return nil
}

func (r *ClusterClaimReconciler) SetupWithManager(mgr ctrl.Manager) error {
	controller, err := ctrl.NewControllerManagedBy(mgr).
		For(&claimv1alpha1.ClusterClaim{}).
		WithEventFilter(
			predicate.Funcs{
				CreateFunc: func(e event.CreateEvent) bool {

					return true
				},
				UpdateFunc: func(e event.UpdateEvent) bool {

					return true
				},
				DeleteFunc: func(e event.DeleteEvent) bool {

					return false
				},
				GenericFunc: func(e event.GenericEvent) bool {

					return false
				},
			},
		).
		Build(r)

	if err != nil {
		return err
	}

	return controller.Watch(
		&source.Kind{Type: &clusterv1alpha1.ClusterManager{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: handler.ToRequestsFunc(r.requeueClusterClaimsForClusterManager),
		},
		predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				return false
			},
			CreateFunc: func(e event.CreateEvent) bool {
				return false
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				return true
			},
			GenericFunc: func(e event.GenericEvent) bool {
				return false
			},
		},
	)
}
