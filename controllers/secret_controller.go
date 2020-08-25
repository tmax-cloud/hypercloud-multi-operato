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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	constant "multi.tmax.io/controllers/util"

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

	//check meet condition
	if val, ok := meetCondi(*secret); ok {
		reqLogger.Info(secret.GetName() + " meets condition ")

		clientRestConfig, _ := getKubeConfig(*secret)
		clientName := strings.Split(secret.GetName(), constant.KubeconfigPostfix)[0]

		val = strings.ToLower(val)
		switch val {
		case constant.WatchAnnotationJoinValue:
			if err := s.joinFed(clientName, clientRestConfig); err != nil {
				s.patchSecret(secret.GetName(), "joinError")
				reqLogger.Info(secret.GetName() + " is fail to join")

				return ctrl.Result{}, err
			}
			s.patchSecret(secret.GetName(), "joinSuccessful")
			reqLogger.Info(secret.GetName() + " is joined successfully")

		case constant.WatchAnnotationUnJoinValue:
			if err := s.unjoinFed(clientName, clientRestConfig); err != nil {
				s.patchSecret(secret.GetName(), "unjoinError")
				reqLogger.Info(secret.GetName() + " is fail to unjoin")

				return ctrl.Result{}, err
			}
			s.patchSecret(secret.GetName(), "unjoinSuccessful")
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

func (s *SecretReconciler) patchSecret(name string, status string) error {
	key := types.NamespacedName{Namespace: constant.ClusterNamespace, Name: name}
	bsecret := &corev1.Secret{}

	if err := s.Get(context.TODO(), key, bsecret); err != nil {
		return err
	}

	asecret := bsecret.DeepCopy()
	asecret.GetAnnotations()[constant.WatchAnnotationKey] = status
	if err := s.Patch(context.TODO(), asecret, client.MergeFrom(bsecret)); err != nil {
		return err
	}

	return nil
}

func (r *SecretReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Secret{}).
		Complete(r)
}
