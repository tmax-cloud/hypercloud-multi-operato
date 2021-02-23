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
	"encoding/json"

	"github.com/go-logr/logr"
	servicecatalogv1beta1 "github.com/kubernetes-sigs/service-catalog/pkg/apis/servicecatalog/v1beta1"
	"github.com/prometheus/common/log"
	clusterv1alpha1 "github.com/tmax-cloud/hypercloud-multi-operator/apis/cluster/v1alpha1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	CAPI_SYSTEM_NAMESPACE       = "capi-system"
	CLAIM_API_GROUP             = "claim.tmax.io"
	CLUSTER_API_GROUP           = "cluster.tmax.io"
	CLAIM_API_Kind              = "clusterclaims"
	CLAIM_API_GROUP_VERSION     = "claim.tmax.io/v1alpha1"
	HYPERCLOUD_SYSTEM_NAMESPACE = ""
)

type ClusterParameter struct {
	ClusterName       string
	AWSRegion         string
	SshKey            string
	MasterNum         int
	MasterType        string
	WorkerNum         int
	WorkerType        string
	Owner             string
	KubernetesVersion string
}

// ClusterManagerReconciler reconciles a ClusterManager object
type ClusterManagerReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=cluster.tmax.io,resources=clustermanagers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cluster.tmax.io,resources=clustermanagers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machinedeployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machinedeployments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=kubeadmcontrolplanes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=kubeadmcontrolplanes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=servicecatalog.k8s.io,resources=serviceinstances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=servicecatalog.k8s.io,resources=serviceinstances/status,verbs=get;update;patch

func (r *ClusterManagerReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	log := r.Log.WithValues("clustermanager", req.NamespacedName)

	//get ClusterManager
	clusterManager := &clusterv1alpha1.ClusterManager{}
	if err := r.Get(context.TODO(), req.NamespacedName, clusterManager); err != nil {
		if errors.IsNotFound(err) {
			log.Info("ClusterManager resource not found. Ignoring since object must be deleted.")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get ClusterManager")
		return ctrl.Result{}, err
	}

	if err := r.CreateServiceInstance(clusterManager); err != nil {
		log.Error(err, "Failed to create ServiceInstance")
		return ctrl.Result{}, err
	}
	if err := r.CreateClusterMnagerOwnerRole(clusterManager); err != nil {
		log.Error(err, "Failed to create ServiceInstance")
		return ctrl.Result{}, err
	}
	// r.CreateMember(clusterManager)

	r.kubeadmControlPlaneUpdate(clusterManager)
	r.machineDeploymentUpdate(clusterManager)

	return ctrl.Result{}, nil
}

///

func (r *ClusterManagerReconciler) CreateServiceInstance(clusterManager *clusterv1alpha1.ClusterManager) error {

	serviceInstance := &servicecatalogv1beta1.ServiceInstance{}
	serviceInstanceKey := types.NamespacedName{Name: clusterManager.Name, Namespace: CAPI_SYSTEM_NAMESPACE}
	if err := r.Get(context.TODO(), serviceInstanceKey, serviceInstance); err != nil {
		if errors.IsNotFound(err) {
			clusterParameter := ClusterParameter{
				ClusterName:       clusterManager.Name,
				AWSRegion:         clusterManager.Spec.Region,
				SshKey:            clusterManager.Spec.SshKey,
				MasterNum:         clusterManager.Spec.MasterNum,
				MasterType:        clusterManager.Spec.MasterType,
				WorkerNum:         clusterManager.Spec.WorkerNum,
				WorkerType:        clusterManager.Spec.WorkerType,
				Owner:             clusterManager.Annotations["owner"],
				KubernetesVersion: clusterManager.Spec.Version,
			}

			byte, err := json.Marshal(&clusterParameter)
			if err != nil {
				log.Error(err, "Failed to marshal cluster parameters")
				return err
			}

			newServiceInstance := &servicecatalogv1beta1.ServiceInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterManager.Name,
					Namespace: CAPI_SYSTEM_NAMESPACE,
				},
				Spec: servicecatalogv1beta1.ServiceInstanceSpec{
					PlanReference: servicecatalogv1beta1.PlanReference{
						ClusterServiceClassExternalName: "capi-aws-template",
						ClusterServicePlanExternalName:  "capi-aws-template-plan-default",
					},
					Parameters: &runtime.RawExtension{
						Raw: byte,
					},
				},
			}

			ctrl.SetControllerReference(clusterManager, newServiceInstance, r.Scheme)
			err = r.Create(context.TODO(), newServiceInstance)
			if err != nil {
				log.Error(err, "Failed to create "+clusterManager.Name+" serviceInstance")
				return err
			}
		} else {
			log.Error(err, "Failed to get serviceInstance")
			return err
		}
	}
	return nil
}

func (r *ClusterManagerReconciler) CreateClusterMnagerOwnerRole(clusterManager *clusterv1alpha1.ClusterManager) error {
	clusterRole := &rbacv1.ClusterRole{}
	clusterRoleName := clusterManager.Annotations["owner"] + "-" + clusterManager.Name + "-clm-role"
	clusterRoleKey := types.NamespacedName{Name: clusterRoleName, Namespace: ""}
	if err := r.Get(context.TODO(), clusterRoleKey, clusterRole); err != nil {
		if errors.IsNotFound(err) {
			newClusterRole := &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterRoleName,
				},
				Rules: []rbacv1.PolicyRule{
					{APIGroups: []string{CLUSTER_API_GROUP}, Resources: []string{"clustermanagers"},
						ResourceNames: []string{clusterManager.Name}, Verbs: []string{"get", "update", "delete"}},
					{APIGroups: []string{CLUSTER_API_GROUP}, Resources: []string{"clustermanagers/status"},
						ResourceNames: []string{clusterManager.Name}, Verbs: []string{"get"}},
				},
			}
			ctrl.SetControllerReference(clusterManager, newClusterRole, r.Scheme)
			err := r.Create(context.TODO(), newClusterRole)
			if err != nil {
				log.Error(err, "Failed to create "+clusterRoleName+" clusterRole.")
				return err
			}
		} else {
			log.Error(err, "Failed to get clusterRole")
			return err
		}
	}
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{}
	clusterRoleBindingName := clusterManager.Annotations["owner"] + "-" + clusterManager.Name + "-clm-rolebinding"
	clusterRoleBindingKey := types.NamespacedName{Name: clusterRoleBindingName, Namespace: ""}
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
						Name:     clusterManager.Annotations["owner"],
					},
				},
			}
			ctrl.SetControllerReference(clusterManager, newClusterRoleBinding, r.Scheme)
			err = r.Create(context.TODO(), newClusterRoleBinding)
			if err != nil {
				log.Error(err, "Failed to create "+clusterRoleBindingName+" clusterRole.")
				return err
			}
		} else {
			log.Error(err, "Failed to get rolebinding")
			return err
		}
	}
	return nil
}

func (r *ClusterManagerReconciler) kubeadmControlPlaneUpdate(clusterManager *clusterv1alpha1.ClusterManager) {
	kcp := &controlplanev1.KubeadmControlPlane{}
	key := types.NamespacedName{Name: clusterManager.Name + "-control-plane", Namespace: CAPI_SYSTEM_NAMESPACE}

	if err := r.Get(context.TODO(), key, kcp); err != nil {
		return
	}

	//create helper for patch
	helper, _ := patch.NewHelper(kcp, r.Client)
	defer func() {
		if err := helper.Patch(context.TODO(), kcp); err != nil {
			r.Log.Error(err, "kubeadmcontrolplane patch error")
		}
	}()

	if *kcp.Spec.Replicas != int32(clusterManager.Spec.MasterNum) {
		*kcp.Spec.Replicas = int32(clusterManager.Spec.MasterNum)
	}
	if kcp.Spec.Version != clusterManager.Spec.Version {
		kcp.Spec.Version = clusterManager.Spec.Version
	}
}

func (r *ClusterManagerReconciler) machineDeploymentUpdate(clusterManager *clusterv1alpha1.ClusterManager) {
	md := &clusterv1.MachineDeployment{}
	key := types.NamespacedName{Name: clusterManager.Name + "-md-0", Namespace: CAPI_SYSTEM_NAMESPACE}

	if err := r.Get(context.TODO(), key, md); err != nil {
		return
	}

	//create helper for patch
	helper, _ := patch.NewHelper(md, r.Client)
	defer func() {
		if err := helper.Patch(context.TODO(), md); err != nil {
			r.Log.Error(err, "kubeadmcontrolplane patch error")
		}
	}()

	if *md.Spec.Replicas != int32(clusterManager.Spec.WorkerNum) {
		*md.Spec.Replicas = int32(clusterManager.Spec.WorkerNum)
	}

	if *md.Spec.Template.Spec.Version != clusterManager.Spec.Version {
		*md.Spec.Template.Spec.Version = clusterManager.Spec.Version
	}
}

func (r *ClusterManagerReconciler) requeueClusterManagersForCluster(o handler.MapObject) []ctrl.Request {
	c := o.Object.(*clusterv1.Cluster)

	//get ClusterManager
	clm := &clusterv1alpha1.ClusterManager{}
	key := types.NamespacedName{Namespace: "", Name: c.Name}

	if err := r.Get(context.TODO(), key, clm); err != nil {
		if errors.IsNotFound(err) {
			log.Info("ClusterManager resource not found. Ignoring since object must be deleted.")
			return nil
		}

		log.Error(err, "Failed to get ClusterManager")
		return nil
	}

	//create helper for patch
	helper, _ := patch.NewHelper(clm, r.Client)
	defer func() {
		if err := helper.Patch(context.TODO(), clm); err != nil {
			log.Error(err, "ClusterManager patch error")
		}
	}()

	clm.Status.Ready = c.Status.ControlPlaneInitialized

	return nil
}

func (r *ClusterManagerReconciler) requeueClusterManagersForKubeadmControlPlane(o handler.MapObject) []ctrl.Request {
	cp := o.Object.(*controlplanev1.KubeadmControlPlane)
	log := r.Log.WithValues("objectMapper", "kubeadmControlPlaneToClusterManagers", "namespace", cp.Namespace, "kubeadmcontrolplane", cp.Name)

	// Don't handle deleted kubeadmcontrolplane
	if !cp.ObjectMeta.DeletionTimestamp.IsZero() {
		log.V(4).Info("kubeadmcontrolplane has a deletion timestamp, skipping mapping.")
		return nil
	}

	//get ClusterManager
	clm := &clusterv1alpha1.ClusterManager{}
	key := types.NamespacedName{Namespace: "", Name: cp.Name[0 : len(cp.Name)-len("-control-plane")]}
	if err := r.Get(context.TODO(), key, clm); err != nil {
		if errors.IsNotFound(err) {
			log.Info("ClusterManager resource not found. Ignoring since object must be deleted.")
			return nil
		}

		log.Error(err, "Failed to get ClusterManager")
		return nil
	}

	//create helper for patch
	helper, _ := patch.NewHelper(clm, r.Client)
	defer func() {
		if err := helper.Patch(context.TODO(), clm); err != nil {
			log.Error(err, "ClusterManager patch error")
		}
	}()

	clm.Status.MasterRun = int(cp.Status.Replicas)

	return nil
}

func (r *ClusterManagerReconciler) requeueClusterManagersForMachineDeployment(o handler.MapObject) []ctrl.Request {
	md := o.Object.(*clusterv1.MachineDeployment)
	log := r.Log.WithValues("objectMapper", "kubeadmControlPlaneToClusterManagers", "namespace", md.Namespace, "machinedeployment", md.Name)

	// Don't handle deleted machinedeployment
	if !md.ObjectMeta.DeletionTimestamp.IsZero() {
		log.V(4).Info("machinedeployment has a deletion timestamp, skipping mapping.")
		return nil
	}

	//get ClusterManager
	clm := &clusterv1alpha1.ClusterManager{}
	key := types.NamespacedName{Namespace: "", Name: md.Name[0 : len(md.Name)-len("-md-0")]}
	if err := r.Get(context.TODO(), key, clm); err != nil {
		if errors.IsNotFound(err) {
			log.Info("ClusterManager resource not found. Ignoring since object must be deleted.")
			return nil
		}

		log.Error(err, "Failed to get ClusterManager")
		return nil
	}

	//create helper for patch
	helper, _ := patch.NewHelper(clm, r.Client)
	defer func() {
		if err := helper.Patch(context.TODO(), clm); err != nil {
			log.Error(err, "ClusterManager patch error")
		}
	}()

	clm.Status.WorkerRun = int(md.Status.Replicas)

	return nil
}

func (r *ClusterManagerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	controller, err := ctrl.NewControllerManagedBy(mgr).
		For(&clusterv1alpha1.ClusterManager{}).
		WithEventFilter(
			predicate.Funcs{
				// Avoid reconciling if the event triggering the reconciliation is related to incremental status updates
				// for kubefedcluster resources only
				CreateFunc: func(e event.CreateEvent) bool {
					return true
				},
				UpdateFunc: func(e event.UpdateEvent) bool {
					oldCLM := e.ObjectOld.(*clusterv1alpha1.ClusterManager).DeepCopy()
					newCLM := e.ObjectNew.(*clusterv1alpha1.ClusterManager).DeepCopy()

					if oldCLM.Spec.MasterNum != newCLM.Spec.MasterNum || oldCLM.Spec.WorkerNum != newCLM.Spec.WorkerNum || oldCLM.Spec.Version != newCLM.Spec.Version {
						return true
					}
					return false
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

	controller.Watch(
		&source.Kind{Type: &clusterv1.Cluster{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: handler.ToRequestsFunc(r.requeueClusterManagersForCluster),
		},
		predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				oldc := e.ObjectOld.(*clusterv1.Cluster)
				newc := e.ObjectNew.(*clusterv1.Cluster)

				if &newc.Status != nil && oldc.Status.ControlPlaneInitialized == false && newc.Status.ControlPlaneInitialized == true {
					return true
				}
				return false
			},
			CreateFunc: func(e event.CreateEvent) bool {
				return false
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				return false
			},
			GenericFunc: func(e event.GenericEvent) bool {
				return false
			},
		},
	)

	controller.Watch(
		&source.Kind{Type: &controlplanev1.KubeadmControlPlane{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: handler.ToRequestsFunc(r.requeueClusterManagersForKubeadmControlPlane),
		},
		predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				oldKcp := e.ObjectOld.(*controlplanev1.KubeadmControlPlane)
				newKcp := e.ObjectNew.(*controlplanev1.KubeadmControlPlane)

				if oldKcp.Status.Replicas != newKcp.Status.Replicas {
					return true
				}
				return false
			},
			CreateFunc: func(e event.CreateEvent) bool {
				return true
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				return false
			},
			GenericFunc: func(e event.GenericEvent) bool {
				return false
			},
		},
	)

	return controller.Watch(
		&source.Kind{Type: &clusterv1.MachineDeployment{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: handler.ToRequestsFunc(r.requeueClusterManagersForMachineDeployment),
		},
		predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				oldMd := e.ObjectOld.(*clusterv1.MachineDeployment)
				newMd := e.ObjectNew.(*clusterv1.MachineDeployment)

				if oldMd.Status.Replicas != newMd.Status.Replicas {
					return true
				}
				return false
			},
			CreateFunc: func(e event.CreateEvent) bool {
				return true
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				return false
			},
			GenericFunc: func(e event.GenericEvent) bool {
				return false
			},
		},
	)
}
