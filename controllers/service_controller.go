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
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"multi.tmax.io/controllers/util"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	typesv1beta1 "multi.tmax.io/apis/external/v1beta1"
	constant "multi.tmax.io/controllers/util"
)

// ServiceReconciler reconciles a Memcached object
type ServiceReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups="",resources=service;,verbs=get;update;patch;list;watch;create;post;delete;

func (r *ServiceReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()

	//get service
	svc := &corev1.Service{}
	if err := r.Get(context.TODO(), req.NamespacedName, svc); err != nil {
		if errors.IsNotFound(err) {
			r.Log.Info("Service resource not found. Ignoring since object must be deleted.")
			return ctrl.Result{}, nil
		}

		r.Log.Error(err, "Failed to get service")
		return ctrl.Result{}, err
	}

	//get ip & port
	var ip, port string
	switch svc.Spec.Type {
	case "LoadBalancer":
		ip = svc.Status.LoadBalancer.Ingress[0].IP
		port = strconv.Itoa(int(svc.Spec.Ports[0].Port))
	}

	//handle fcm
	r.handleFcm(ip, port)

	return ctrl.Result{}, nil
}

func (r *ServiceReconciler) handleFcm(ip string, port string) {
	//get fcm
	fcm := &typesv1beta1.FederatedConfigMap{}
	key := types.NamespacedName{Name: constant.FederatedConfigMapName, Namespace: constant.FederatedConfigMapNamespace}

	if err := r.Get(context.TODO(), key, fcm); err != nil && errors.IsNotFound(err) {
		r.Log.Info("FederatedConfigMap doesn't exist yet")
		return
	}

	//update fcm
	helper, _ := patch.NewHelper(fcm, r.Client)
	defer func() {
		if err := helper.Patch(context.TODO(), fcm); err != nil {
			r.Log.Error(err, "kfc patch error")
		}
	}()
	fcm.Spec.Template.Data["mgnt-ip"] = ip
	fcm.Spec.Template.Data["mgnt-port"] = port
}

func (r *ServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	_, err := ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Service{}).
		WithEventFilter(
			predicate.Funcs{
				CreateFunc: func(e event.CreateEvent) bool {
					return false
				},
				UpdateFunc: func(e event.UpdateEvent) bool {
					svc := e.ObjectNew.(*corev1.Service).DeepCopy()
					return checkValidSelector(*svc) && checkValidUpdate(e)
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

	return err
}

func checkValidUpdate(e event.UpdateEvent) bool {
	oldsvc := e.ObjectOld.(*corev1.Service).DeepCopy()
	newsvc := e.ObjectNew.(*corev1.Service).DeepCopy()

	if oldsvc.Spec.Ports[0].NodePort != newsvc.Spec.Ports[0].NodePort ||
		oldsvc.Spec.Ports[0].Port != newsvc.Spec.Ports[0].Port ||
		strings.Compare(string(oldsvc.Spec.Type), string(newsvc.Spec.Type)) != 0 ||
		strings.Compare(oldsvc.Status.LoadBalancer.Ingress[0].IP, newsvc.Status.LoadBalancer.Ingress[0].IP) != 0 {
		return true
	}

	return false
}

func checkValidSelector(svc corev1.Service) bool {
	selector := svc.Spec.Selector
	if selector != nil {
		if val, ok := selector[util.MultiApiServerServiceSelectorKey]; ok && strings.Compare(val, util.MultiApiServerServiceSelectorValue) == 0 {
			return true
		}
	}

	return false
}
