/*
Copyright 2025 KubeAssist Authors.

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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Phase constants for KubeAssistStatus.Phase.
const (
	PhasePending = "Pending"
	PhaseRunning = "Running"
	PhaseFailed  = "Failed"
)

// SecretKeyRef identifies a specific key within a Kubernetes Secret.
type SecretKeyRef struct {
	// Name is the name of the Secret in the same namespace as the KubeAssist CR.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Key is the data key inside the Secret whose value is injected into the
	// agent container as its LLM API key environment variable.
	// Defaults to "API_KEY" — compatible with OpenAI, watsonx, Anthropic, etc.
	// +kubebuilder:default="API_KEY"
	// +optional
	Key string `json:"key,omitempty"`
}

// IngressSpec configures a standard Kubernetes Ingress for the agent endpoint.
// On OpenShift a Route is always created; set Enabled=true here to also create
// a standard Ingress. On vanilla Kubernetes, set Enabled=true to expose the agent.
type IngressSpec struct {
	// Enabled controls whether an Ingress resource is created.
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Host is the fully-qualified hostname for the Ingress rule.
	// Example: "my-agent.example.com"
	// +optional
	Host string `json:"host,omitempty"`

	// IngressClassName is the IngressClass to use (e.g. "nginx", "traefik", "alb").
	// +optional
	IngressClassName *string `json:"ingressClassName,omitempty"`

	// Annotations are added verbatim to the Ingress resource.
	// Use this for provider-specific configuration (timeouts, TLS, etc.).
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// TLSSecretName is the name of a TLS Secret for HTTPS termination.
	// +optional
	TLSSecretName string `json:"tlsSecretName,omitempty"`
}

// KubeAssistSpec defines the desired state of a KubeAssist agent instance.
type KubeAssistSpec struct {
	// ApiKeySecretRef points to the Secret that holds the LLM / AI provider API key.
	// The operator injects the referenced key into the agent container as an
	// environment variable named after SecretKeyRef.Key (default: API_KEY).
	// Works with any OpenAI-compatible provider: OpenAI, watsonx, Anthropic, etc.
	// +kubebuilder:validation:Required
	ApiKeySecretRef SecretKeyRef `json:"apiKeySecretRef"`

	// Image is the fully-qualified container image for the agent runtime.
	// Swap this for your own LangChain, AutoGen, or custom agent image.
	// +kubebuilder:default="ghcr.io/kubeassist-operator/agent:latest"
	// +optional
	Image string `json:"image,omitempty"`

	// ImagePullSecretName is the name of a pull Secret in the same namespace
	// used to fetch the agent image from a private registry.
	// +optional
	ImagePullSecretName string `json:"imagePullSecretName,omitempty"`

	// Mode is an arbitrary string passed as AGENT_MODE to the agent container.
	// Its meaning is defined by the agent runtime image.
	// +kubebuilder:default="default"
	// +optional
	Mode string `json:"mode,omitempty"`

	// Replicas is the desired number of agent Pods.
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// WorkspaceSize is the size of the ephemeral workspace volume mounted at
	// /workspace inside each agent Pod.
	// +kubebuilder:default="5Gi"
	// +optional
	WorkspaceSize string `json:"workspaceSize,omitempty"`

	// Resources defines CPU/memory requests and limits for the agent container.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// TargetNamespaces lists namespaces where the agent ServiceAccount is granted
	// edit RBAC so the agent can deploy and manage workloads there.
	// +optional
	TargetNamespaces []string `json:"targetNamespaces,omitempty"`

	// Ingress configures a standard Kubernetes Ingress for the agent endpoint.
	// Required on vanilla Kubernetes. Optional on OpenShift (Route is created by default).
	// +optional
	Ingress IngressSpec `json:"ingress,omitempty"`

	// RouteTimeout is the haproxy.router.openshift.io/timeout annotation on the
	// OpenShift Route (e.g. "600s"). Ignored on non-OCP clusters.
	// +kubebuilder:default="600s"
	// +optional
	RouteTimeout string `json:"routeTimeout,omitempty"`
}

// KubeAssistStatus defines the observed state of a KubeAssist instance.
type KubeAssistStatus struct {
	// Conditions is the list of standard Kubernetes status conditions.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Phase is the high-level lifecycle state: Pending, Running, or Failed.
	// +kubebuilder:validation:Enum=Pending;Running;Failed
	// +optional
	Phase string `json:"phase,omitempty"`

	// URL is the public HTTPS endpoint of the agent, populated once the
	// Route or Ingress has been admitted.
	// +optional
	URL string `json:"url,omitempty"`

	// ActiveJobs is the number of agent sessions currently running.
	// +optional
	ActiveJobs int32 `json:"activeJobs,omitempty"`

	// TotalJobsRun is the cumulative count of completed agent sessions.
	// +optional
	TotalJobsRun int64 `json:"totalJobsRun,omitempty"`

	// ObservedGeneration is the metadata.generation last successfully reconciled.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// AgentVersion is the version string reported by the running agent image.
	// +optional
	AgentVersion string `json:"agentVersion,omitempty"`
}

// KubeAssist is the Schema for the kubeassists API.
//
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=ka,categories=kubeassist
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`,description="Lifecycle phase"
// +kubebuilder:printcolumn:name="URL",type=string,JSONPath=`.status.url`,description="Public endpoint"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type KubeAssist struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KubeAssistSpec   `json:"spec,omitempty"`
	Status KubeAssistStatus `json:"status,omitempty"`
}

// KubeAssistList contains a list of KubeAssist objects.
//
// +kubebuilder:object:root=true
type KubeAssistList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []KubeAssist `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KubeAssist{}, &KubeAssistList{})
}
