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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// FederatedConfigMapSpec defines the desired state of FederatedConfigMap
type FederatedConfigMapSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of FederatedConfigMap. Edit FederatedConfigMap_types.go to remove/update
	Placement Placement   `json:"placement,omitempty"`
	Template  Template    `json:"template,omitempty"`
	Overrides []Overrides `json:"overrides,omitempty"`
}

type Overrides struct {
	ClusterName      string             `json:"clusterName,omitempty"`
	ClusterOverrides []ClusterOverrides `json:"clusterOverrides,omitempty"`
}
type ClusterOverrides struct {
	Path  string `json:"path,omitempty"`
	Value string `json:"value,omitempty"`
}

type Template struct {
	Data map[string]string `json:"data,omitempty"`
}
type Placement struct {
	Clusters []Clusters `json:"clusters,omitempty"`
}
type Clusters struct {
	Name string `json:"name,omitempty"`
}

// +kubebuilder:object:root=true

// FederatedConfigMap is the Schema for the FederatedConfigMaps API
type FederatedConfigMap struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec FederatedConfigMapSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// FederatedConfigMapList contains a list of FederatedConfigMap
type FederatedConfigMapList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FederatedConfigMap `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FederatedConfigMap{}, &FederatedConfigMapList{})
}
