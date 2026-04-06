/*
Copyright 2026.

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

// MCPServerPhase describes the current phase of an MCPServer.
type MCPServerPhase string

const (
	MCPServerPhasePending   MCPServerPhase = "Pending"
	MCPServerPhaseDeploying MCPServerPhase = "Deploying"
	MCPServerPhaseReady     MCPServerPhase = "Ready"
	MCPServerPhaseFailed    MCPServerPhase = "Failed"
)

// MCPServerSpec defines the desired state of MCPServer.
// Exactly one of image or url must be set.
type MCPServerSpec struct {

	// ====================================================================
	// DEPLOY MODE (image set)
	// ====================================================================

	// Container image for the MCP server (deploy mode).
	// Mutually exclusive with url.
	// +optional
	Image string `json:"image,omitempty"`

	// Port the MCP server listens on.
	// +optional
	// +kubebuilder:default=8080
	Port int32 `json:"port,omitempty"`

	// Override command for the container.
	// +optional
	Command []string `json:"command,omitempty"`

	// Plain-text environment variables.
	// +optional
	Env map[string]string `json:"env,omitempty"`

	// Secret-backed environment variables.
	// +optional
	Secrets []SecretEnvVar `json:"secrets,omitempty"`

	// ServiceAccount for the MCP server pod.
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// Compute resources for the MCP server.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// ====================================================================
	// EXTERNAL MODE (url set)
	// ====================================================================

	// URL of an external MCP server (external mode).
	// Mutually exclusive with image.
	// +optional
	URL string `json:"url,omitempty"`

	// Static headers to send to external MCP server.
	// +optional
	Headers map[string]string `json:"headers,omitempty"`

	// OAuth config for external MCP server authentication.
	// +optional
	OAuth *MCPOAuthConfig `json:"oauth,omitempty"`

	// ====================================================================
	// HEALTH CHECK (both modes)
	// ====================================================================

	// Health check configuration.
	// +optional
	HealthCheck *MCPHealthCheck `json:"healthCheck,omitempty"`
}

// MCPServerStatus defines the observed state of MCPServer.
type MCPServerStatus struct {
	// Current phase: Pending, Deploying, Ready, Failed.
	// +optional
	Phase MCPServerPhase `json:"phase,omitempty"`

	// Internal service URL (e.g. http://mcp-k8s-mcp.ns.svc:8080).
	// +optional
	ServiceURL string `json:"serviceURL,omitempty"`

	// Standard conditions.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// Condition types for MCPServer.
const (
	MCPServerConditionReady = "Ready"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=mcp
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="URL",type=string,JSONPath=`.status.serviceURL`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// MCPServer is the Schema for the mcpservers API.
// Shared or external MCP infrastructure service. Multiple agents connect
// to the same server via proxy sidecars.
type MCPServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MCPServerSpec   `json:"spec,omitempty"`
	Status MCPServerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MCPServerList contains a list of MCPServer.
type MCPServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MCPServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MCPServer{}, &MCPServerList{})
}
