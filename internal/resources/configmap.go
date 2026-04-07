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

package resources

import (
	"encoding/json"
	"fmt"
	"strings"

	agentsv1alpha1 "github.com/samyn92/agentops-core/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ====================================================================
// Config types
// ====================================================================

// ToolEntry describes a tool package path.
type ToolEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// MCPEntry describes an MCP server binding.
type MCPEntry struct {
	Name        string   `json:"name"`
	Port        int      `json:"port"`
	DirectTools []string `json:"directTools,omitempty"`
}

// ProviderEntry describes a configured provider.
type ProviderEntry struct {
	Name string `json:"name"`
}

// ToolHooksEntry holds runtime hook config.
type ToolHooksEntry struct {
	BlockedCommands []string `json:"blockedCommands,omitempty"`
	AllowedPaths    []string `json:"allowedPaths,omitempty"`
	AuditTools      []string `json:"auditTools,omitempty"`
}

// ContextEntry describes a context file.
type ContextEntry struct {
	Path string `json:"path"`
}

// AgentConfig is the JSON structure mounted at /etc/operator/config.json for the Fantasy runtime.
type AgentConfig struct {
	Runtime         string          `json:"runtime"`
	Providers       []ProviderEntry `json:"providers"`
	PrimaryModel    string          `json:"primaryModel"`
	FallbackModels  []string        `json:"fallbackModels,omitempty"`
	SystemPrompt    string          `json:"systemPrompt,omitempty"`
	BuiltinTools    []string        `json:"builtinTools,omitempty"`
	Tools           []ToolEntry     `json:"tools"`
	MCPServers      []MCPEntry      `json:"mcpServers,omitempty"`
	ToolHooks       *ToolHooksEntry `json:"toolHooks,omitempty"`
	ContextFiles    []ContextEntry  `json:"contextFiles,omitempty"`
	Temperature     *float64        `json:"temperature,omitempty"`
	MaxOutputTokens *int64          `json:"maxOutputTokens,omitempty"`
	MaxSteps        *int            `json:"maxSteps,omitempty"`
}

// ====================================================================
// ConfigMap builder
// ====================================================================

// BuildAgentConfigMap generates the operator extension ConfigMap from an Agent spec.
func BuildAgentConfigMap(agent *agentsv1alpha1.Agent) (*corev1.ConfigMap, error) {
	config := AgentConfig{
		Runtime:         "fantasy",
		PrimaryModel:    agent.Spec.Model,
		SystemPrompt:    agent.Spec.SystemPrompt,
		BuiltinTools:    agent.Spec.BuiltinTools,
		Temperature:     agent.Spec.Temperature,
		MaxOutputTokens: agent.Spec.MaxOutputTokens,
		MaxSteps:        agent.Spec.MaxSteps,
	}

	// Tools (toolRefs — loaded as MCP servers by Fantasy runtime)
	for _, tr := range agent.Spec.ToolRefs {
		path := fmt.Sprintf("%s/%s", MountTools, tr.Name)
		config.Tools = append(config.Tools, ToolEntry{Name: tr.Name, Path: path})
	}

	// Providers
	for _, p := range agent.Spec.Providers {
		config.Providers = append(config.Providers, ProviderEntry{Name: p.Name})
	}

	// Fallback models
	config.FallbackModels = agent.Spec.FallbackModels

	// MCP servers
	for i, ms := range agent.Spec.MCPServers {
		port := GatewayBasePort + i
		config.MCPServers = append(config.MCPServers, MCPEntry{
			Name:        ms.Name,
			Port:        port,
			DirectTools: ms.DirectTools,
		})
	}

	// Tool hooks
	if agent.Spec.ToolHooks != nil {
		config.ToolHooks = &ToolHooksEntry{
			BlockedCommands: agent.Spec.ToolHooks.BlockedCommands,
			AllowedPaths:    agent.Spec.ToolHooks.AllowedPaths,
			AuditTools:      agent.Spec.ToolHooks.AuditTools,
		}
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal operator config: %w", err)
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ObjectName(agent.Name, "config"),
			Namespace: agent.Namespace,
			Labels:    CommonLabels(agent.Name, "config"),
		},
		Data: map[string]string{
			"config.json": string(data),
		},
	}, nil
}

// ====================================================================
// Gateway & MCP ConfigMaps
// ====================================================================

// BuildGatewayConfigMap generates the MCP gateway permission rules ConfigMap.
// Only created when the agent has mcpServers bindings.
func BuildGatewayConfigMap(agent *agentsv1alpha1.Agent) (*corev1.ConfigMap, error) {
	if len(agent.Spec.MCPServers) == 0 {
		return nil, nil
	}

	// Build per-server permission rules
	rules := make(map[string]interface{})
	for _, ms := range agent.Spec.MCPServers {
		if ms.Permissions != nil {
			rules[ms.Name] = map[string]interface{}{
				"mode":  ms.Permissions.Mode,
				"rules": ms.Permissions.Rules,
			}
		}
	}

	data, err := json.MarshalIndent(rules, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal gateway config: %w", err)
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ObjectName(agent.Name, "gateway"),
			Namespace: agent.Namespace,
			Labels:    CommonLabels(agent.Name, "gateway"),
		},
		Data: map[string]string{
			"permissions.json": string(data),
		},
	}, nil
}

// MCPJsonConfig is the native MCP config format consumed by the runtime's MCP adapter.
type MCPJsonConfig struct {
	MCPServers map[string]MCPJsonServerEntry `json:"mcpServers"`
}

// MCPJsonServerEntry describes one MCP server in mcp.json.
type MCPJsonServerEntry struct {
	Type        string `json:"type"`
	URL         string `json:"url"`
	Lifecycle   string `json:"lifecycle"`
	IdleTimeout int    `json:"idleTimeout"`
}

// BuildMCPConfigMap generates the mcp.json ConfigMap for the runtime's MCP adapter.
// Only created when the agent has mcpServers bindings.
func BuildMCPConfigMap(agent *agentsv1alpha1.Agent) (*corev1.ConfigMap, error) {
	if len(agent.Spec.MCPServers) == 0 {
		return nil, nil
	}

	mcpConfig := MCPJsonConfig{
		MCPServers: make(map[string]MCPJsonServerEntry),
	}

	for i, ms := range agent.Spec.MCPServers {
		port := GatewayBasePort + i
		mcpConfig.MCPServers[ms.Name] = MCPJsonServerEntry{
			Type:        "sse",
			URL:         fmt.Sprintf("http://localhost:%d/sse", port),
			Lifecycle:   "keep-alive",
			IdleTimeout: 300,
		}
	}

	data, err := json.MarshalIndent(mcpConfig, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal mcp.json: %w", err)
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ObjectName(agent.Name, "mcp"),
			Namespace: agent.Namespace,
			Labels:    CommonLabels(agent.Name, "mcp"),
		},
		Data: map[string]string{
			"mcp.json": string(data),
		},
	}, nil
}

// BuildMCPServerConfigMap generates a ConfigMap for an MCPServer deployment.
func BuildMCPServerConfigMap(mcp *agentsv1alpha1.MCPServer) *corev1.ConfigMap {
	name := MCPServerObjectName(mcp.Name)

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-config", name),
			Namespace: mcp.Namespace,
			Labels: map[string]string{
				LabelComponent: "mcp-server",
				LabelManagedBy: ManagedByValue,
			},
		},
		Data: map[string]string{
			"port": fmt.Sprintf("%d", mcp.Spec.Port),
		},
	}
}

// ProviderEnvVarName returns the standard env var name for a provider API key.
// e.g. "anthropic" -> "ANTHROPIC_API_KEY"
func ProviderEnvVarName(providerName string) string {
	return fmt.Sprintf("%s_API_KEY", strings.ToUpper(providerName))
}
