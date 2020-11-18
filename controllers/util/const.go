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

package util

const (
	WatchAnnotationKey          = "federation"
	WatchAnnotationJoinValue    = "join"
	WatchAnnotationUnJoinValue  = "unjoin"
	KubeconfigPostfix           = "-kubeconfig"
	ClusterNamespace            = "default"
	KubeFedNamespace            = "kube-federation-system"
	HostClusterName             = "hostcluster"
	FederatedConfigMapName      = "hypercloud-multi-agent-agentconfig"
	FederatedConfigMapNamespace = "hypercloud-multi-agent-system"
	MultiApiServerNamespace     = "hypercloud4-multi-system"
	MultiApiServerServiceName   = "hypercloud4-multi-api-server-service"
	SecretFinalizer             = "secret/finalizers"
	KubefedclusterFinalizer     = "kubefedcluster/finalizers"
)
