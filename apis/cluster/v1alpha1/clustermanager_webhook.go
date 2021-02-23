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

package v1alpha1

import (
	"errors"
	"reflect"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var clustermanagerlog = logf.Log.WithName("clustermanager-resource")

func (r *ClusterManager) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/mutate-cluster-tmax-io-v1alpha1-clustermanager,mutating=true,failurePolicy=fail,groups=cluster.tmax.io,resources=clustermanagers,verbs=create;update,versions=v1alpha1,name=mclustermanager.kb.io

var _ webhook.Defaulter = &ClusterManager{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *ClusterManager) Default() {
	clustermanagerlog.Info("default", "name", r.Name)

	// TODO(user): fill in your defaulting logic.
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// +kubebuilder:webhook:verbs=update,path=/validate-cluster-tmax-io-v1alpha1-clustermanager,mutating=false,failurePolicy=fail,groups=cluster.tmax.io,resources=clustermanagers,versions=v1alpha1,name=vclustermanager.kb.io

var _ webhook.Validator = &ClusterManager{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *ClusterManager) ValidateCreate() error {
	clustermanagerlog.Info("validate create", "name", r.Name)

	// TODO(user): fill in your validation logic upon object creation.
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *ClusterManager) ValidateUpdate(old runtime.Object) error {
	clustermanagerlog.Info("validate update", "name", r.Name)
	oldClusterClaim := old.(*ClusterManager).DeepCopy()

	if r.Annotations["owner"] != oldClusterClaim.Annotations["owner"] {
		return errors.New("Cannot modify clusterManager.Annotations.owner")
	}

	if r.Status.Ready == false {
		if !reflect.DeepEqual(r.Status.Members, oldClusterClaim.Status.Members) {
			return errors.New("Cannot modify members when cluster status is not ready")
		}
		if !reflect.DeepEqual(r.Status.Groups, oldClusterClaim.Status.Groups) {
			return errors.New("Cannot modify groups when cluster status is not ready")
		}
	}

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *ClusterManager) ValidateDelete() error {
	clustermanagerlog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
