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
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	typesv1beta1 "github.com/tmax-cloud/hypercloud-multi-operator/apis/external/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	fedmultiv1a1 "sigs.k8s.io/kubefed/pkg/apis/multiclusterdns/v1alpha1"
)

// ClusterReconciler reconciles a Memcached object
type FederatedServiceReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups="types.kubefed.io",resources=federatedservices;,verbs=get;watch;list;
// +kubebuilder:rbac:groups="multiclusterdns.kubefed.io",resources=domains;servicednsrecords;,verbs=list;get;watch;create;post;delete;

func (f *FederatedServiceReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	reqLogger := f.Log.WithValues("FederatedService", req.NamespacedName)
	// your logic here

	//get secret
	fs := &typesv1beta1.FederatedService{}
	if err := f.Get(context.TODO(), req.NamespacedName, fs); err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("FederatedService resource not found. Ignoring since object must be deleted.")
			return ctrl.Result{}, nil
		}

		reqLogger.Error(err, "Failed to get FederatedService")
		return ctrl.Result{}, err
	}

	//check federatedservice will be deleted
	willDel := fs.DeletionTimestamp
	switch willDel {
	case nil:
		if fdrCreated, err := f.isServiceDNSRecord(fs.GetName(), fs.GetNamespace()); err != nil {
			reqLogger.Error(err, "FederatedService handling error")
			return ctrl.Result{}, err
		} else if fdrCreated {
			reqLogger.Info("servicednsrecord is created")
		}

	default:
		reqLogger.Info("federatedServide will be deleted")
		if err := f.delServiceRecord(fs.GetName(), fs.GetNamespace()); err != nil {
			reqLogger.Error(err, "Deleting servicednsrecord Error")
			return ctrl.Result{}, err
		}
		reqLogger.Info("servicednsrecord is deleted")
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

func (f *FederatedServiceReconciler) delServiceRecord(name, namespace string) error {
	key := types.NamespacedName{Namespace: namespace, Name: name}
	fdr := &fedmultiv1a1.ServiceDNSRecord{}

	if err := f.Get(context.TODO(), key, fdr); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	if err := f.Delete(context.TODO(), fdr); err != nil {
		return err
	}

	return nil
}

func (f *FederatedServiceReconciler) isServiceDNSRecord(name, namespace string) (bool, error) {
	key := types.NamespacedName{Namespace: namespace, Name: name}
	fdr := &fedmultiv1a1.ServiceDNSRecord{}
	domainName := ""

	if err := f.Get(context.TODO(), key, fdr); err != nil {
		if !errors.IsNotFound(err) {
			return false, nil
		}

		domainList := &fedmultiv1a1.DomainList{}

		if err := f.List(context.TODO(), domainList); err != nil {
			return false, err
		}
		domainName = domainList.Items[0].GetName()

		sr := &fedmultiv1a1.ServiceDNSRecord{}
		sr.SetName(name)
		sr.SetNamespace(namespace)
		sr.Spec.DomainRef = domainName
		sr.Spec.RecordTTL = 300

		if err := f.Create(context.TODO(), sr); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func (f *FederatedServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&typesv1beta1.FederatedService{}).
		Complete(f)
}
