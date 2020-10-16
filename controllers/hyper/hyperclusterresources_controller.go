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
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	hyperv1 "multi.tmax.io/apis/hyper/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1alpha3"
)

// HyperClusterResourcesReconciler reconciles a HyperClusterResources object
type HyperClusterResourcesReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=hyper.multi.tmax.io,resources=hyperclusterresources,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=hyper.multi.tmax.io,resources=hyperclusterresources/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machinedeployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machinedeployments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=kubeadmcontrolplanes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=kubeadmcontrolplanes/status,verbs=get;update;patch

func (r *HyperClusterResourcesReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	log := r.Log.WithValues("hyperclusterresources", req.NamespacedName)

	// your logic here
	//get HyperClusterResource
	hcr := &hyperv1.HyperClusterResources{}
	if err := r.Get(context.TODO(), req.NamespacedName, hcr); err != nil {
		if errors.IsNotFound(err) {
			log.Info("HyperClusterResource resource not found. Ignoring since object must be deleted.")
			return ctrl.Result{}, nil
		}

		log.Error(err, "Failed to get HyperClusterResource")
		return ctrl.Result{}, err
	}

	//create helper for patch
	helper, _ := patch.NewHelper(hcr, r.Client)
	defer func() {
		if err := helper.Patch(context.TODO(), hcr); err != nil {
			log.Error(err, "HyperClusterResource patch error")
		}
	}()

	r.kubeadmControlPlaneReconcile(hcr)
	r.machineDeploymentReconcile(hcr)

	return ctrl.Result{}, nil
}

func (r *HyperClusterResourcesReconciler) kubeadmControlPlaneReconcile(hcr *hyperv1.HyperClusterResources) {
	kcp := &controlplanev1.KubeadmControlPlane{}
	key := types.NamespacedName{Name: hcr.Name + "-control-plane", Namespace: "default"}

	if err := r.Get(context.TODO(), key, kcp); err != nil {
		return
	}

	hcr.Status.MasterNum = int(*kcp.Spec.Replicas)
	hcr.Status.MasterRun = int(kcp.Status.Replicas)
	hcr.Spec.Version = kcp.Spec.Version
}

func (r *HyperClusterResourcesReconciler) machineDeploymentReconcile(hcr *hyperv1.HyperClusterResources) {
	md := &clusterv1.MachineDeployment{}
	key := types.NamespacedName{Name: hcr.Name + "-md-0", Namespace: "default"}

	if err := r.Get(context.TODO(), key, md); err != nil {
		return
	}

	hcr.Status.WorkerNum = int(*md.Spec.Replicas)
	hcr.Status.WorkerRun = int(md.Status.Replicas)
}

func (r *HyperClusterResourcesReconciler) requeueHyperClusterResourcesForKubeadmControlPlane(o handler.MapObject) []ctrl.Request {
	cp := o.Object.(*controlplanev1.KubeadmControlPlane)
	log := r.Log.WithValues("objectMapper", "kubeadmControlPlaneToHyperClusterResources", "namespace", cp.Namespace, "kubeadmcontrolplane", cp.Name)

	// Don't handle deleted kubeadmcontrolplane
	if !cp.ObjectMeta.DeletionTimestamp.IsZero() {
		log.V(4).Info("kubeadmcontrolplane has a deletion timestamp, skipping mapping.")
		return nil
	}

	return []ctrl.Request{
		{
			NamespacedName: client.ObjectKey{Namespace: "kube-federation-system", Name: cp.Name[0 : len(cp.Name)-len("-control-plane")]},
		},
	}
}

func (r *HyperClusterResourcesReconciler) requeueHyperClusterResourcesForMachineDeployment(o handler.MapObject) []ctrl.Request {
	md := o.Object.(*clusterv1.MachineDeployment)
	log := r.Log.WithValues("objectMapper", "kubeadmControlPlaneToHyperClusterResources", "namespace", md.Namespace, "machinedeployment", md.Name)

	// Don't handle deleted machinedeployment
	if !md.ObjectMeta.DeletionTimestamp.IsZero() {
		log.V(4).Info("machinedeployment has a deletion timestamp, skipping mapping.")
		return nil
	}

	return []ctrl.Request{
		{
			NamespacedName: client.ObjectKey{Namespace: "kube-federation-system", Name: md.Name[0 : len(md.Name)-len("-md-0")]},
		},
	}
}

func (r *HyperClusterResourcesReconciler) SetupWithManager(mgr ctrl.Manager) error {
	controller, err := ctrl.NewControllerManagedBy(mgr).
		For(&hyperv1.HyperClusterResources{}).
		WithEventFilter(
			predicate.Funcs{
				// Avoid reconciling if the event triggering the reconciliation is related to incremental status updates
				// for kubefedcluster resources only
				CreateFunc: func(e event.CreateEvent) bool {
					return false
				},
				UpdateFunc: func(e event.UpdateEvent) bool {
					oldhcr := e.ObjectOld.(*hyperv1.HyperClusterResources).DeepCopy()
					newhcr := e.ObjectNew.(*hyperv1.HyperClusterResources).DeepCopy()

					oldhcr.Status = hyperv1.HyperClusterResourcesStatus{}
					newhcr.Status = hyperv1.HyperClusterResourcesStatus{}

					oldhcr.ObjectMeta.ResourceVersion = ""
					newhcr.ObjectMeta.ResourceVersion = ""

					return !reflect.DeepEqual(oldhcr, newhcr)
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
		&source.Kind{Type: &controlplanev1.KubeadmControlPlane{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: handler.ToRequestsFunc(r.requeueHyperClusterResourcesForKubeadmControlPlane),
		},
		predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				oldKcp := e.ObjectOld.(*controlplanev1.KubeadmControlPlane)
				newKcp := e.ObjectNew.(*controlplanev1.KubeadmControlPlane)

				if *oldKcp.Spec.Replicas != *newKcp.Spec.Replicas || oldKcp.Status.Replicas != newKcp.Status.Replicas || oldKcp.Spec.Version != newKcp.Spec.Version {
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
			ToRequests: handler.ToRequestsFunc(r.requeueHyperClusterResourcesForMachineDeployment),
		},
		predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				oldMd := e.ObjectOld.(*clusterv1.MachineDeployment)
				newMd := e.ObjectNew.(*clusterv1.MachineDeployment)

				if *oldMd.Spec.Replicas != *newMd.Spec.Replicas || oldMd.Status.Replicas != newMd.Status.Replicas {
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
