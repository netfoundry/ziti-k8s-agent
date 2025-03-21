/*
Copyright 2025 NetFoundry.

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
	admissionregistration1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ZitiWebhookSpec defines the desired state of ZitiWebhook
type ZitiWebhookSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Webhook Name
	Name string `json:"name,omitempty"`

	// Controller CR Name
	ZitiControllerName string `json:"zitiControllerName,omitempty"`

	// Webhook Certificate
	Cert CertificateSpecs `json:"cert,omitempty"`

	// Deployment Specs
	DeploymentSpec DeploymentSpec `json:"deploymentSpec,omitempty"`

	// Mutating Webhook Specs
	MutatingWebhookSpec MutatingWebhookSpec `json:"mutatingWebhookSpec,omitempty"`

	// Cluster Role Specs
	ClusterRoleSpec ClusterRoleSpec `json:"clusterRoleSpec,omitempty"`

	// Cluster Role Binding Specs
	ClusterRoleBindingSpec ClusterRoleBindingSpec `json:"clusterRoleBindingSpec,omitempty"`
}

type CertificateSpecs struct {
	// Cert Duration
	// +kubebuilder:default:=2160
	Duration int64 `json:"duration,omitempty"`

	// Cert Renew Before
	// +kubebuilder:default:=360
	RenewBefore int64 `json:"renewBefore,omitempty"`
	// Cert Organization
	// +kubebuilder:default:={"netfoundry"}
	Organizations []string `json:"organization,omitempty"`
}

type DeploymentSpec struct {
	// Number of replicas
	// +kubebuilder:default:=1
	Replicas int32 `json:"replicas,omitempty"`

	// Webhook Image
	// +kubebuilder:default:=netfoundry/ziti-k8s-agent
	Image string `json:"image,omitempty"`

	// Webhook Image Version
	// +kubebuilder:default:=latest
	ImageVersion string `json:"imageVersion,omitempty"`

	// Weebhook Image Pull Policy
	// +kubebuilder:enum:=Always;Never;IfNotPresent
	// +kubebuilder:default:=IfNotPresent
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// Secure port that the webhook listens on
	// +kubebuilder:default:=9443
	Port int32 `json:"port,omitempty"`

	// Environment Variables
	Env DeploymentEnvVars `json:"env,omitempty"`

	// Resource Request
	// +kubebuilder:default:={"cpu":"100m","memory":"128Mi"}
	ResourceRequest corev1.ResourceList `json:"resourceRequest,omitempty"`

	// Resource Limit
	// +kubebuilder:default:={"cpu":"500m","memory":"512Mi"}
	ResourceLimit corev1.ResourceList `json:"resourceLimit,omitempty"`

	// Max Unavailable
	// +kubebuilder:validation:Pattern:="^[0-9]+%$"
	// +kubebuilder:default:="25%"
	MaxUnavailable string `json:"maxUnavailable,omitempty"`

	// Max Surge
	// +kubebuilder:validation:Pattern:="^[0-9]+%$"
	// +kubebuilder:default:="25%"
	MaxSurge string `json:"maxSurge,omitempty"`

	// Termination Grace Period
	// +kubebuilder:default:=30
	TerminationGracePeriodSeconds int64 `json:"terminationGracePeriodSeconds,omitempty"`

	// Progress Deadline
	// +kubebuilder:default:=600
	// +kubebuilder:validation:Minimum=0
	ProgressDeadlineSeconds int32 `json:"progressDeadlineSeconds,omitempty"`

	// Revision History Limit
	// +kubebuilder:default:=10
	// +kubebuilder:validation:Minimum=0
	RevisionHistoryLimit int32 `json:"revisionHistoryLimit,omitempty"`

	// Log Verbose Level
	// +kubebuilder:default:=2
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=5
	LogLevel int32 `json:"logLevel,omitempty"`
}

type DeploymentEnvVars struct {

	// Sidecar container image
	// +kubebuilder:default:=openziti/ziti-tunnel
	SidecarImage string `json:"sidecarImage,omitempty"`

	// Sidecar container image version
	// +kubebuilder:default:=latest
	SidecarImageVersion string `json:"sidecarImageVersion,omitempty"`

	// Image pull policy for sidecar container
	// +kubebuilder:enum:=Always;Never;IfNotPresent
	// +kubebuilder:default:=IfNotPresent
	SidecarImagePullPolicy corev1.PullPolicy `json:"sidecarImagePullPolicy,omitempty"`

	// Used in creation of ContainerName to be used as injected sidecar
	// +kubebuilder:default:=zt
	SidecarPrefix string `json:"sidecarPrefix,omitempty"`

	// Directory where sidecar container will store identity files
	// +kubebuilder:default:=/ziti-tunnel
	SidecarIdentityDir string `json:"sidecarIdentityDir,omitempty"`

	// Ziti Controller Management URL, i.e. https://{FQDN}:{PORT}/edge/management/v1
	// +kubebuilder:default:="https://ziti-controller:1280/edge/management/v1"
	ZitiCtrlMgmtApi string `json:"zitiCtrlMgmtApi,omitempty"`

	// Ziti Controller Client Certificate
	ZitiCtrlClientCertFile string `json:"zitiCtrlClientCertFile,omitempty"`

	// Ziti Controller Client Key
	ZitiCtrlClientKeyFile string `json:"zitiCtrlClientKeyFile,omitempty"`

	// Ziti Controller CA Bundle
	ZitiCtrlCaBundleFile string `json:"zitiCtrlCaBundleFile,omitempty"`

	// Override the security context at pod level, i.e. runAsNonRoot: false
	// +kubebuilder:default:=false
	PodSecurityOverride bool `json:"podSecurityOverride,omitempty"`

	// Cluster DNS Service IP
	ClusterDnsServiceIP string `json:"clusterDnsServiceIP,omitempty"`

	// A list of DNS search domains as space seperated string i.e. 'value1 value2'
	SearchDomainList string `json:"searchDomainList,omitempty"`

	// Ziti Identity Role Key used in pod annotation
	// +kubebuilder:default:=identity.openziti.io/role-attributes
	ZitiRoleKey string `json:"zitiRoleKey,omitempty"`
}

type MutatingWebhookSpec struct {

	// Object Selector
	ObjectSelector *metav1.LabelSelector `json:"objectSelector,omitempty"`

	// Namespace Selector
	// +kubebuilder:default:={matchExpressions: {{key: "kubernetes.io/metadata.name", operator: NotIn, values: {"kube-system"}}, {key: "tunnel.openziti.io/enabled", operator: In, values: {"true", "false"}}}}
	NamespaceSelector *metav1.LabelSelector `json:"namespaceSelector,omitempty"`

	// Webhook Side EfFect
	// +kubebuilder:validation:Enum:=None;Unknown;Some;NoneOnDryRun
	// +kubebuilder:default:=None
	SideEffectType *admissionregistration1.SideEffectClass `json:"sideEffectType,omitempty"`

	// Webhook Failure Policy
	// +kubebuilder:default:=Fail
	// +kubebuilder:validation:Enum:=Ignore;Fail
	FailurePolicy *admissionregistration1.FailurePolicyType `json:"failurePolicy,omitempty"`

	// Webhook Timeout
	// +kubebuilder:default:=30
	TimeoutSeconds *int32 `json:"timeoutSeconds,omitempty"`

	// Webhook Match Policy
	// +kubebuilder:default:=Equivalent
	// +kubebuilder:validation:Enum:=Exact;Equivalent
	MatchPolicy *admissionregistration1.MatchPolicyType `json:"matchPolicy,omitempty"`

	// Webhook Reinvocation Policy
	// +kubebuilder:default:=Never
	// +kubebuilder:validation:Enum:=Never;IfNeeded
	ReinvocationPolicy *admissionregistration1.ReinvocationPolicyType `json:"reinvocationPolicy,omitempty"`

	// Wenhoo Admission Review Versions
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:default:={"v1"}
	AdmissionReviewVersions []string `json:"admissionReviewVersions,omitempty"`

	// Webhook Rules
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:default:={{"operations":{"CREATE","UPDATE","DELETE"},"apiGroups":{"*"},"apiVersions":{"v1","v1beta1"},"resources":{"pods"},"scope":"*"}}
	Rules []admissionregistrationv1.RuleWithOperations `json:"rules,omitempty"`

	// Webhook Client Config
	ClientConfig ClientConfigSpec `json:"clientConfig,omitempty"`
}

type ClientConfigSpec struct {
	// Webhook Service Name
	ServiceName string `json:"serviceName,omitempty"`

	// Webhook Service Namespace
	Namespace string `json:"namespace,omitempty"`

	// Webhook Service Path
	// +kubebuilder:default:=/ziti-tunnel
	Path string `json:"path,omitempty"`

	// Webhook Service Port
	// +kubebuilder:default:=443
	Port int32 `json:"port,omitempty"`

	// Webhook Service Ca Bundle
	// +kubebuilder:default:=""
	CaBundle string `json:"caBundle,omitempty"`
}

type ClusterRoleSpec struct {
	// Cluster Role Rules
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:default:={{"apiGroups":{"*"},"resources":{"services","namespaces"},"verbs":{"get","list","watch"}}}
	Rules []rbacv1.PolicyRule `json:"rules,omitempty"`
}

type ClusterRoleBindingSpec struct {
	// Cluster Role Binding Subjects
	// +kubebuilder:validation:MinItems=1
	Subjects []rbacv1.Subject `json:"subjects,omitempty"`
}

// ZitiWebhookStatus defines the observed state of ZitiWebhook
type ZitiWebhookStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// ZitiWebhook is the Schema for the zitiwebhooks API
type ZitiWebhook struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ZitiWebhookSpec   `json:"spec,omitempty"`
	Status ZitiWebhookStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ZitiWebhookList contains a list of ZitiWebhook
type ZitiWebhookList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ZitiWebhook `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ZitiWebhook{}, &ZitiWebhookList{})
}
