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

// AgentMode defines the agent lifecycle mode.
// +kubebuilder:validation:Enum=daemon;task
type AgentMode string

const (
	AgentModeDaemon AgentMode = "daemon"
	AgentModeTask   AgentMode = "task"
)

// AgentPhase describes the current phase of an Agent.
type AgentPhase string

const (
	AgentPhasePending AgentPhase = "Pending"
	AgentPhaseRunning AgentPhase = "Running" // daemon
	AgentPhaseReady   AgentPhase = "Ready"   // task
	AgentPhaseFailed  AgentPhase = "Failed"
)

// AgentSpec defines the desired state of Agent.
type AgentSpec struct {

	// ====================================================================
	// MODE
	// ====================================================================

	// Mode: daemon (Deployment+PVC+Service) or task (Job template).
	// +kubebuilder:validation:Required
	Mode AgentMode `json:"mode"`

	// ====================================================================
	// MODEL
	// ====================================================================

	// Primary model in provider/model format (e.g. anthropic/claude-sonnet-4-20250514).
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Model string `json:"model"`

	// Thinking level for the model.
	// +optional
	// +kubebuilder:validation:Enum=off;minimal;low;medium;high;xhigh
	ThinkingLevel string `json:"thinkingLevel,omitempty"`

	// LLM providers with API key references. At least one required.
	// +kubebuilder:validation:MinItems=1
	Providers []ProviderRef `json:"providers"`

	// Fallback models tried in order if the primary model fails.
	// Each must reference a provider listed in providers.
	// +optional
	FallbackModels []string `json:"fallbackModels,omitempty"`

	// ====================================================================
	// IDENTITY
	// ====================================================================

	// System prompt injected at the start of every session.
	// +optional
	SystemPrompt string `json:"systemPrompt,omitempty"`

	// Context files loaded from ConfigMaps (e.g. AGENTS.md).
	// +optional
	ContextFiles []ContextFileRef `json:"contextFiles,omitempty"`

	// ====================================================================
	// TOOLS
	// ====================================================================

	// Pi built-in tools to enable. Omit or [] for OCI-only agents.
	// Valid: read, bash, edit, write, grep, find, ls.
	// +optional
	BuiltinTools []string `json:"builtinTools,omitempty"`

	// OCI packages, ConfigMaps, or inline JS modules providing tools.
	// +optional
	ToolRefs []ResourceRef `json:"toolRefs,omitempty"`

	// ====================================================================
	// ENVIRONMENT
	// ====================================================================

	// Plain-text environment variables.
	// +optional
	Env map[string]string `json:"env,omitempty"`

	// Secret-backed environment variables.
	// +optional
	Secrets []SecretEnvVar `json:"secrets,omitempty"`

	// ====================================================================
	// STORAGE (daemon only, ignored for task)
	// ====================================================================

	// Persistent storage for daemon agents (PVC, RWO).
	// +optional
	Storage *StorageSpec `json:"storage,omitempty"`

	// ====================================================================
	// COMPACTION (daemon only)
	// ====================================================================

	// Session compaction settings for daemon agents.
	// +optional
	Compaction *CompactionSpec `json:"compaction,omitempty"`

	// ====================================================================
	// MCP SERVERS
	// ====================================================================

	// Shared MCPServer bindings with per-agent permissions.
	// +optional
	MCPServers []MCPServerBinding `json:"mcpServers,omitempty"`

	// ====================================================================
	// SKILLS
	// ====================================================================

	// Skills loaded from OCI, ConfigMap, or inline content.
	// +optional
	Skills []ResourceRef `json:"skills,omitempty"`

	// ====================================================================
	// EXTENSIONS
	// ====================================================================

	// Additional Pi extensions loaded from OCI.
	// +optional
	Extensions []ExtensionRef `json:"extensions,omitempty"`

	// ====================================================================
	// SCHEDULE
	// ====================================================================

	// Cron schedule for creating periodic AgentRuns.
	// +optional
	Schedule string `json:"schedule,omitempty"`

	// Prompt used when schedule triggers an AgentRun.
	// +optional
	SchedulePrompt string `json:"schedulePrompt,omitempty"`

	// ====================================================================
	// CONCURRENCY
	// ====================================================================

	// Concurrency control for parallel AgentRun execution.
	// +optional
	Concurrency *ConcurrencySpec `json:"concurrency,omitempty"`

	// ====================================================================
	// TOOL HOOKS (defense-in-depth)
	// ====================================================================

	// Runtime security hooks for tool calls.
	// +optional
	ToolHooks *ToolHooksSpec `json:"toolHooks,omitempty"`

	// ====================================================================
	// INFRASTRUCTURE
	// ====================================================================

	// Compute resources for the agent container.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// Container image for the agent runtime.
	// +optional
	// +kubebuilder:default="ghcr.io/samyn92/agent-runtime:latest"
	Image string `json:"image,omitempty"`

	// Image pull policy.
	// +optional
	// +kubebuilder:validation:Enum=Always;IfNotPresent;Never
	// +kubebuilder:default=IfNotPresent
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// ServiceAccount for the agent pod.
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// Timeout for task jobs or per-prompt timeout for daemons (e.g. "10m").
	// +optional
	// +kubebuilder:default="10m"
	Timeout string `json:"timeout,omitempty"`

	// Network policy configuration.
	// +optional
	NetworkPolicy *NetworkPolicySpec `json:"networkPolicy,omitempty"`
}

// AgentStatus defines the observed state of Agent.
type AgentStatus struct {
	// Current phase: Pending, Running (daemon), Ready (task), Failed.
	// +optional
	Phase AgentPhase `json:"phase,omitempty"`

	// Service URL for daemon agents (e.g. http://agent.ns.svc:4096).
	// +optional
	ServiceURL string `json:"serviceURL,omitempty"`

	// Number of ready replicas (daemon only).
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// Name of the PVC created for daemon agents.
	// +optional
	StoragePVC string `json:"storagePVC,omitempty"`

	// Currently active model (may differ from spec.model if fallback triggered).
	// +optional
	ActiveModel string `json:"activeModel,omitempty"`

	// Standard conditions.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// Condition types for Agent.
const (
	// AgentConditionReady indicates the agent is fully operational.
	AgentConditionReady = "Ready"
	// AgentConditionToolsLoaded indicates all tool packages are loaded.
	AgentConditionToolsLoaded = "ToolsLoaded"
	// AgentConditionMCPServersReady indicates all MCP server bindings are ready.
	AgentConditionMCPServersReady = "MCPServersReady"
	// AgentConditionProvidersReady indicates all LLM providers are configured.
	AgentConditionProvidersReady = "ProvidersReady"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=ag
// +kubebuilder:printcolumn:name="Mode",type=string,JSONPath=`.spec.mode`
// +kubebuilder:printcolumn:name="Model",type=string,JSONPath=`.spec.model`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Agent is the Schema for the agents API.
// One CRD for all agents. mode controls lifecycle:
// daemon = Deployment + PVC + Service (always running).
// task = Job template (one prompt, exits).
type Agent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AgentSpec   `json:"spec,omitempty"`
	Status AgentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AgentList contains a list of Agent.
type AgentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Agent `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Agent{}, &AgentList{})
}
