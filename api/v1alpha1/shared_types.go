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
)

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
// Resource references (tools, skills)
// -------------------------------------------------------------------

// ResourceRef references a resource from OCI, ConfigMap, or inline content.
// Exactly one of ociRef, configMapRef, or content must be set.
type ResourceRef struct {
	// Logical name for this resource.
	Name string `json:"name"`
	// OCI artifact reference.
	// +optional
	OCIRef *OCIRef `json:"ociRef,omitempty"`
	// ConfigMap reference (name + key).
	// +optional
	ConfigMapRef *SecretKeyRef `json:"configMapRef,omitempty"`
	// Inline content (< 4KB, for prototyping).
	// +optional
	Content string `json:"content,omitempty"`
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
// Extensions (Pi runtime only)
// -------------------------------------------------------------------

// ExtensionRef loads a Pi extension from OCI with optional env/secrets.
// Only used when spec.pi is set.
type ExtensionRef struct {
	// Logical name.
	Name string `json:"name"`
	// OCI artifact containing the extension.
	OCIRef OCIRef `json:"ociRef"`
	// Plain-text environment variables.
	// +optional
	Env map[string]string `json:"env,omitempty"`
	// Secret-backed environment variables.
	// +optional
	Secrets []SecretEnvVar `json:"secrets,omitempty"`
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
// MCP bindings (per-agent)
// -------------------------------------------------------------------

// MCPServerBinding references a shared MCPServer with per-agent permissions.
type MCPServerBinding struct {
	// Name of the MCPServer CR to bind.
	Name string `json:"name"`
	// Per-agent deny/allow rules for MCP tool calls.
	// +optional
	Permissions *MCPPermissions `json:"permissions,omitempty"`
	// MCP tools to promote to first-class Pi tools (registered directly, not via proxy).
	// +optional
	DirectTools []string `json:"directTools,omitempty"`
}

// MCPPermissions configures deny/allow rules for MCP tool calls on the proxy gateway.
type MCPPermissions struct {
	// Mode: "deny" blocks matching rules, "allow" only permits matching rules.
	// +kubebuilder:validation:Enum=deny;allow
	Mode string `json:"mode"`
	// Rules in the form "tool_name", "tool_name:arg=value", or "tool_name:arg=pattern*".
	Rules []string `json:"rules"`
}

// -------------------------------------------------------------------
// Compaction (Pi runtime, daemon only)
// -------------------------------------------------------------------

// CompactionSpec controls Pi session compaction behavior.
// Only used when spec.pi is set with mode=daemon.
type CompactionSpec struct {
	// Whether compaction is enabled. Default: true.
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
	// Strategy: auto (default), manual, or off.
	// +optional
	// +kubebuilder:validation:Enum=auto;manual;off
	// +kubebuilder:default=auto
	Strategy string `json:"strategy,omitempty"`
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
// MCPServer health check
// -------------------------------------------------------------------

// MCPHealthCheck configures health probing for an MCPServer.
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
// MCPServer OAuth (external mode)
// -------------------------------------------------------------------

// MCPOAuthConfig configures OAuth for external MCP servers.
type MCPOAuthConfig struct {
	// Secret containing the OAuth client ID.
	ClientIDSecret SecretKeyRef `json:"clientIdSecret"`
	// Secret containing the OAuth client secret.
	ClientSecretSecret SecretKeyRef `json:"clientSecretSecret"`
}

// -------------------------------------------------------------------
// Common resource requirements (re-export for convenience)
// -------------------------------------------------------------------

// ResourceRequirements is an alias for corev1.ResourceRequirements.
type ResourceRequirements = corev1.ResourceRequirements
