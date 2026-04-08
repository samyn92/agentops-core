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

// -------------------------------------------------------------------
// Secret references
// -------------------------------------------------------------------

// SecretKeyRef references a key in a Kubernetes Secret (same namespace).
type SecretKeyRef struct {
	// Name of the Secret.
	Name string `json:"name"`
	// Key within the Secret.
	Key string `json:"key"`
}

// SecretEnvVar injects a Secret key as an environment variable.
type SecretEnvVar struct {
	// Environment variable name.
	Name string `json:"name"`
	// Reference to the secret key.
	SecretRef SecretKeyRef `json:"secretRef"`
}

// -------------------------------------------------------------------
// OCI artifacts
// -------------------------------------------------------------------

// OCIRef references an OCI artifact.
type OCIRef struct {
	// Full OCI reference (e.g. ghcr.io/org/tool:1.0.0).
	Ref string `json:"ref"`
	// Optional digest for pinning (e.g. sha256:abc...).
	// +optional
	Digest string `json:"digest,omitempty"`
	// Pull policy: Always, IfNotPresent, Never. Defaults based on tag.
	// +optional
	// +kubebuilder:validation:Enum=Always;IfNotPresent;Never
	PullPolicy string `json:"pullPolicy,omitempty"`
	// Optional pull secret for private registries.
	// +optional
	PullSecret *SecretKeyRef `json:"pullSecret,omitempty"`
}

// -------------------------------------------------------------------
// Provider & model
// -------------------------------------------------------------------

// ProviderRef configures an LLM provider.
type ProviderRef struct {
	// Provider name (e.g. anthropic, openai, google).
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// Secret containing the API key.
	ApiKeySecret SecretKeyRef `json:"apiKeySecret"`
}

// -------------------------------------------------------------------
// Context files
// -------------------------------------------------------------------

// ContextFileRef loads an AGENTS.md or similar context file from a ConfigMap.
type ContextFileRef struct {
	// ConfigMap reference (name + key).
	ConfigMapRef SecretKeyRef `json:"configMapRef"`
}

// -------------------------------------------------------------------
// AgentTool bindings (per-agent)
// -------------------------------------------------------------------

// AgentToolBinding references an AgentTool CR with optional per-agent overrides.
type AgentToolBinding struct {
	// Name of the AgentTool CR.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// Override permissions from AgentTool defaults.
	// +optional
	Permissions *MCPPermissions `json:"permissions,omitempty"`
	// MCP tools to promote to first-class (mcpServer/mcpEndpoint sources only).
	// +optional
	DirectTools []string `json:"directTools,omitempty"`
	// Auto-inject skill content into every prompt (skill sources only).
	// +optional
	AutoContext bool `json:"autoContext,omitempty"`
}

// -------------------------------------------------------------------
// MCP permissions (used by AgentToolBinding)
// -------------------------------------------------------------------

// MCPPermissions configures deny/allow rules for MCP tool calls on the proxy gateway.
type MCPPermissions struct {
	// Mode: "deny" blocks matching rules, "allow" only permits matching rules.
	// +kubebuilder:validation:Enum=deny;allow
	Mode string `json:"mode"`
	// Rules in the form "tool_name", "tool_name:arg=value", or "tool_name:arg=pattern*".
	Rules []string `json:"rules"`
}

// -------------------------------------------------------------------
// Tool hooks (defense-in-depth)
// -------------------------------------------------------------------

// ToolHooksSpec configures defense-in-depth runtime constraints on tool calls.
type ToolHooksSpec struct {
	// Patterns blocked in bash commands (substring match).
	// +optional
	BlockedCommands []string `json:"blockedCommands,omitempty"`
	// Restrict file tool paths to these prefixes (absolute paths only).
	// +optional
	AllowedPaths []string `json:"allowedPaths,omitempty"`
	// Tools to audit-log via afterToolCall hook.
	// +optional
	AuditTools []string `json:"auditTools,omitempty"`
}

// -------------------------------------------------------------------
// Concurrency
// -------------------------------------------------------------------

// ConcurrencySpec controls parallel AgentRun execution.
type ConcurrencySpec struct {
	// Maximum concurrent runs. Default: 1.
	// +optional
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	MaxRuns int `json:"maxRuns,omitempty"`
	// Policy when at max: queue, reject, or replace.
	// +optional
	// +kubebuilder:default=queue
	// +kubebuilder:validation:Enum=queue;reject;replace
	Policy string `json:"policy,omitempty"`
}

// -------------------------------------------------------------------
// Storage (daemon agents)
// -------------------------------------------------------------------

// StorageSpec defines PVC configuration for daemon agents.
type StorageSpec struct {
	// PVC size (e.g. "10Gi").
	Size string `json:"size"`
	// Storage class name.
	// +optional
	StorageClass string `json:"storageClass,omitempty"`
}

// -------------------------------------------------------------------
// Network policy
// -------------------------------------------------------------------

// NetworkPolicySpec controls agent network isolation.
type NetworkPolicySpec struct {
	// Whether to create a NetworkPolicy.
	// +optional
	Enabled bool `json:"enabled,omitempty"`
}

// -------------------------------------------------------------------
// Channel platform configs
// -------------------------------------------------------------------

// TelegramChannelConfig configures a Telegram bot channel.
type TelegramChannelConfig struct {
	// Secret containing the bot token.
	BotTokenSecret SecretKeyRef `json:"botTokenSecret"`
	// Allowed Telegram user IDs.
	// +optional
	AllowedUsers []string `json:"allowedUsers,omitempty"`
	// Allowed Telegram chat IDs.
	// +optional
	AllowedChats []string `json:"allowedChats,omitempty"`
}

// SlackChannelConfig configures a Slack channel.
type SlackChannelConfig struct {
	// Secret containing the bot token.
	BotTokenSecret SecretKeyRef `json:"botTokenSecret"`
	// Allowed Slack channel IDs.
	// +optional
	AllowedChannels []string `json:"allowedChannels,omitempty"`
}

// DiscordChannelConfig configures a Discord channel.
type DiscordChannelConfig struct {
	// Secret containing the bot token.
	BotTokenSecret SecretKeyRef `json:"botTokenSecret"`
	// Allowed Discord channel IDs.
	// +optional
	AllowedChannels []string `json:"allowedChannels,omitempty"`
}

// GitLabChannelConfig configures a GitLab webhook channel.
type GitLabChannelConfig struct {
	// GitLab webhook events to listen for (e.g. "Issue Hook").
	Events []string `json:"events"`
	// Filter by action (e.g. "open").
	// +optional
	Actions []string `json:"actions,omitempty"`
	// Filter by labels on the object.
	// +optional
	Labels []string `json:"labels,omitempty"`
	// Webhook secret for signature verification.
	Secret SecretKeyRef `json:"secret"`
}

// GitHubChannelConfig configures a GitHub webhook channel.
type GitHubChannelConfig struct {
	// GitHub webhook events to listen for (e.g. "pull_request").
	Events []string `json:"events"`
	// Filter by action (e.g. "opened", "synchronize").
	// +optional
	Actions []string `json:"actions,omitempty"`
	// Filter by labels on the object.
	// +optional
	Labels []string `json:"labels,omitempty"`
	// Webhook secret for signature verification.
	Secret SecretKeyRef `json:"secret"`
}

// WebhookChannelConfig configures a generic webhook channel.
type WebhookChannelConfig struct {
	// Optional HMAC secret for signature verification.
	// +optional
	Secret *SecretKeyRef `json:"secret,omitempty"`
}

// WebhookIngressConfig configures ingress for webhook-based channels.
type WebhookIngressConfig struct {
	// Hostname for the ingress.
	Host string `json:"host"`
	// Path (defaults to /).
	// +optional
	Path string `json:"path,omitempty"`
	// Ingress class name.
	// +optional
	IngressClassName string `json:"ingressClassName,omitempty"`
	// TLS configuration.
	// +optional
	TLS *IngressTLS `json:"tls,omitempty"`
}

// IngressTLS configures TLS for webhook ingress.
type IngressTLS struct {
	// Cert-manager cluster issuer name.
	ClusterIssuer string `json:"clusterIssuer"`
}

// -------------------------------------------------------------------
// Agent resource bindings
// -------------------------------------------------------------------

// AgentResourceBinding references an AgentResource CR from an Agent.
type AgentResourceBinding struct {
	// Name of the AgentResource CR to bind.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Mark the resource as read-only for the agent (advisory, enforced by runtime).
	// +optional
	ReadOnly bool `json:"readOnly,omitempty"`

	// Automatically inject this resource context into every prompt
	// without requiring manual selection in the UI.
	// +optional
	AutoContext bool `json:"autoContext,omitempty"`
}

// -------------------------------------------------------------------
// Memory (Engram)
// -------------------------------------------------------------------

// MemorySpec configures the Engram shared memory system for an agent.
type MemorySpec struct {
	// Reference to the memory server. Can be an AgentTool CR name
	// (with mcpServer/mcpEndpoint source) or a plain service name.
	// The runtime connects to Engram's REST API via the resolved
	// service URL.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	ServerRef string `json:"serverRef"`

	// Project name used to scope memories in Engram.
	// Defaults to the Agent CR name if unset.
	// +optional
	Project string `json:"project,omitempty"`

	// Number of recent context entries injected per turn.
	// +optional
	// +kubebuilder:default=5
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=50
	ContextLimit int `json:"contextLimit,omitempty"`

	// Sliding window size for working memory (recent messages kept in-memory).
	// +optional
	// +kubebuilder:default=20
	// +kubebuilder:validation:Minimum=2
	// +kubebuilder:validation:Maximum=200
	WindowSize int `json:"windowSize,omitempty"`

	// Enable auto-summarization of sessions at end.
	// +optional
	// +kubebuilder:default=true
	AutoSummarize *bool `json:"autoSummarize,omitempty"`
}

// -------------------------------------------------------------------
// MCP health check (used by AgentTool mcpServer/mcpEndpoint sources)
// -------------------------------------------------------------------

// MCPHealthCheck configures health probing for MCP tools.
type MCPHealthCheck struct {
	// Health check path (deploy mode) or full URL (external mode).
	// +optional
	Path string `json:"path,omitempty"`
	// Health check URL (external mode only).
	// +optional
	URL string `json:"url,omitempty"`
	// Probe interval in seconds.
	// +optional
	// +kubebuilder:default=30
	IntervalSeconds int `json:"intervalSeconds,omitempty"`
}

// -------------------------------------------------------------------
// MCP OAuth (used by AgentTool mcpEndpoint source)
// -------------------------------------------------------------------

// MCPOAuthConfig configures OAuth for external MCP servers.
type MCPOAuthConfig struct {
	// Secret containing the OAuth client ID.
	ClientIDSecret SecretKeyRef `json:"clientIdSecret"`
	// Secret containing the OAuth client secret.
	ClientSecretSecret SecretKeyRef `json:"clientSecretSecret"`
}
