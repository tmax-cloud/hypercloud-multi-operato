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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// HyperClusterResourcesSpec defines the desired state of HyperClusterResources
type HyperClusterResourcesSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	Provider  string `json:"provider,omitempty"`
	Version   string `json:"version,omitempty"`
	MasterNum int    `json:"masterNum,omitempty"`
	WorkerNum int    `json:"workerNum,omitempty"`
}

// HyperClusterResourcesStatus defines the observed state of HyperClusterResources
type HyperClusterResourcesStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	Ready     bool           `json:"ready,omitempty"`
	MasterRun int            `json:"masterRun,omitempty"`
	WorkerRun int            `json:"workerRun,omitempty"`
	Resources []ResourceType `json:"resources,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=hyperclusterresources,scope=Namespaced,categories=hypercloud4-multi,shortName=hcr
// +kubebuilder:printcolumn:name="Provider",type="string",JSONPath=".spec.provider",description="provider"
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".spec.version",description="k8s version"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="is running"
// +kubebuilder:printcolumn:name="MasterNum",type="string",JSONPath=".spec.masterNum",description="replica number of master"
// +kubebuilder:printcolumn:name="MasterRun",type="string",JSONPath=".status.masterRun",description="running of master"
// +kubebuilder:printcolumn:name="WorkerNum",type="string",JSONPath=".spec.workerNum",description="replica number of worker"
// +kubebuilder:printcolumn:name="WorkerRun",type="string",JSONPath=".status.workerRun",description="running of worker"

// HyperClusterResources is the Schema for the hyperclusterresources API
type HyperClusterResources struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HyperClusterResourcesSpec   `json:"spec,omitempty"`
	Status HyperClusterResourcesStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// HyperClusterResourcesList contains a list of HyperClusterResources
type HyperClusterResourcesList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HyperClusterResources `json:"items"`
}

type ResourceType struct {
	Name    string `json:"name,omitempty"`
	Total   int    `json:"total,omitempty"`
	Running int    `json:"running,omitempty"`
}

func init() {
	SchemeBuilder.Register(&HyperClusterResources{}, &HyperClusterResourcesList{})
}
