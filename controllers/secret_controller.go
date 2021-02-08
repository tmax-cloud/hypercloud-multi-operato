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
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	constant "multi.tmax.io/controllers/util"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	fedcore "sigs.k8s.io/kubefed/pkg/apis/core/v1beta1"
	fedv1b1 "sigs.k8s.io/kubefed/pkg/apis/core/v1beta1"
	"sigs.k8s.io/kubefed/pkg/kubefedctl"
)

// ClusterReconciler reconciles a Memcached object
type SecretReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups="",resources=secrets;namespaces;,verbs=get;update;patch;list;watch;create;post;delete;
// +kubebuilder:rbac:groups="core.kubefed.io",resources=kubefedconfigs;kubefedclusters;,verbs=get;update;patch;list;watch;create;post;delete;

func (s *SecretReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	reqLogger := s.Log.WithValues("Secret", req.NamespacedName)

	//get secret
	secret := &corev1.Secret{}
	if err := s.Get(context.TODO(), req.NamespacedName, secret); err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("Secret resource not found. Ignoring since object must be deleted.")
			return ctrl.Result{}, nil
		}

		reqLogger.Error(err, "Failed to get secret")
		return ctrl.Result{}, err
	}

	//set patch helper
	helper, _ := patch.NewHelper(secret, s.Client)
	defer func() {
		if err := helper.Patch(context.TODO(), secret); err != nil {
			s.Log.Error(err, "secret patch error")
		}
	}()

	//check meet condition
	if val, ok := meetCondi(*secret); ok {
		reqLogger.Info(secret.GetName() + " meets condition ")

		clientRestConfig, _ := getKubeConfig(*secret)
		clientName := strings.Split(secret.GetName(), constant.KubeconfigPostfix)[0]

		//delete handling
		if !secret.DeletionTimestamp.IsZero() {
			s.deleteHandling(clientName)
			controllerutil.RemoveFinalizer(secret, constant.SecretFinalizer)
			return ctrl.Result{}, nil
		}

		// Add finalizer first if not exist to avoid the race condition between init and delete
		if !controllerutil.ContainsFinalizer(secret, constant.SecretFinalizer) {
			controllerutil.AddFinalizer(secret, constant.SecretFinalizer)
		}

		val = strings.ToLower(val)
		switch val {
		case constant.WatchAnnotationJoinValue:
			if err := s.joinFed(clientName, clientRestConfig); err != nil {
				secret.GetAnnotations()[constant.WatchAnnotationKey] = "joinError"
				reqLogger.Info(secret.GetName() + " is fail to join")

				return ctrl.Result{}, err
			}
			secret.GetAnnotations()[constant.WatchAnnotationKey] = "joinSuccessful"
			reqLogger.Info(secret.GetName() + " is joined successfully")

		case constant.WatchAnnotationUnJoinValue:
			if err := s.unjoinFed(clientName, clientRestConfig); err != nil {
				secret.GetAnnotations()[constant.WatchAnnotationKey] = "unjoinError"
				reqLogger.Info(secret.GetName() + " is fail to unjoin")

				return ctrl.Result{}, err
			}
			secret.GetAnnotations()[constant.WatchAnnotationKey] = "unjoinSuccessful"
			reqLogger.Info(secret.GetName() + " is unjoined successfully")
		default:
			reqLogger.Info(secret.GetName() + " has unexpected value with " + val)
		}

	}
	return ctrl.Result{}, nil
}

/*
  checkList
   1. has kubeconfig postfix with "-kubeconfig"
   2. has annotation with "key: federation"
*/
func meetCondi(s corev1.Secret) (string, bool) {
	if ok := strings.Contains(s.GetName(), constant.KubeconfigPostfix); ok {
		if val, ok := s.GetAnnotations()[constant.WatchAnnotationKey]; ok {
			return val, ok
		}
	}
	return "", false
}

func (s *SecretReconciler) deleteHandling(clusterName string) {
	kfc := &fedcore.KubeFedCluster{}
	kfcKey := types.NamespacedName{Name: clusterName, Namespace: constant.KubeFedNamespace}

	if err := s.Get(context.TODO(), kfcKey, kfc); err == nil {
		s.Delete(context.TODO(), kfc)
	}
}

func (s *SecretReconciler) unjoinFed(clusterName string, clusterConfig *rest.Config) error {
	hostConfig := ctrl.GetConfigOrDie()
	if err := kubefedctl.UnjoinCluster(hostConfig, clusterConfig,
		constant.KubeFedNamespace, constant.HostClusterName, "", clusterName, false, false); err != nil {
		return err
	}
	return nil
}

func (s *SecretReconciler) joinFed(clusterName string, clusterConfig *rest.Config) error {
	hostConfig := ctrl.GetConfigOrDie()

	fedConfig := &fedv1b1.KubeFedConfig{}
	key := types.NamespacedName{Namespace: constant.KubeFedNamespace, Name: "kubefed"}

	if err := s.Get(context.TODO(), key, fedConfig); err != nil {
		return err
	}

	if _, err := kubefedctl.JoinCluster(hostConfig, clusterConfig,
		constant.KubeFedNamespace, constant.HostClusterName, clusterName, "", fedConfig.Spec.Scope, false, false); err != nil {
		return err
	}
	return nil
}

func getKubeConfig(s corev1.Secret) (*rest.Config, error) {
	if value, ok := s.Data["value"]; ok {
		if clientConfig, err := clientcmd.NewClientConfigFromBytes(value); err == nil {
			if restConfig, err := clientConfig.ClientConfig(); err == nil {
				return restConfig, nil
			}
		}
	}
	return nil, errors.NewBadRequest("getClientConfig Error")
}

func checkValidName(s corev1.Secret) bool {
	ok := strings.Contains(s.GetName(), constant.KubeconfigPostfix)
	return ok
}

func (r *SecretReconciler) SetupWithManager(mgr ctrl.Manager) error {
	_, err := ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Secret{}).
		WithEventFilter(
			predicate.Funcs{
				CreateFunc: func(e event.CreateEvent) bool {
					secret := e.Object.(*corev1.Secret).DeepCopy()
					return checkValidName(*secret)
				},
				UpdateFunc: func(e event.UpdateEvent) bool {
					secret := e.ObjectNew.(*corev1.Secret).DeepCopy()
					return checkValidName(*secret)
				},
				DeleteFunc: func(e event.DeleteEvent) bool {
					secret := e.Object.(*corev1.Secret).DeepCopy()
					return checkValidName(*secret)
				},
				GenericFunc: func(e event.GenericEvent) bool {
					return false
				},
			},
		).
		Build(r)

	return err
}
