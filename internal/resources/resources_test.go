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
	"strings"
	"testing"

	agentsv1alpha1 "github.com/samyn92/agenticops-core/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func testAgent() *agentsv1alpha1.Agent {
	temp := 0.5
	maxTokens := int64(4096)
	maxSteps := 50
	return &agentsv1alpha1.Agent{
		ObjectMeta: metav1.ObjectMeta{Name: "test-agent", Namespace: "agents"},
		Spec: agentsv1alpha1.AgentSpec{
			Mode:  agentsv1alpha1.AgentModeDaemon,
			Model: "anthropic/claude-sonnet-4-20250514",
			Providers: []agentsv1alpha1.ProviderRef{
				{Name: "anthropic", ApiKeySecret: agentsv1alpha1.SecretKeyRef{Name: "keys", Key: "key"}},
			},
			BuiltinTools:    []string{"bash", "read", "edit", "write"},
			Temperature:     &temp,
			MaxOutputTokens: &maxTokens,
			MaxSteps:        &maxSteps,
			SystemPrompt:    "You are helpful.",
		},
	}
}

// ── ConfigMap tests ──

func TestBuildAgentConfigMap(t *testing.T) {
	agent := testAgent()
	cm, err := BuildAgentConfigMap(agent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data := cm.Data["config.json"]
	var cfg AgentConfig
	if err := json.Unmarshal([]byte(data), &cfg); err != nil {
		t.Fatalf("failed to parse config: %v", err)
	}

	if cfg.Runtime != "fantasy" {
		t.Errorf("expected runtime=fantasy, got %q", cfg.Runtime)
	}
	if cfg.PrimaryModel != "anthropic/claude-sonnet-4-20250514" {
		t.Errorf("unexpected model: %s", cfg.PrimaryModel)
	}
	if len(cfg.BuiltinTools) != 4 {
		t.Errorf("expected 4 builtinTools, got %d", len(cfg.BuiltinTools))
	}
	if cfg.Temperature == nil || *cfg.Temperature != 0.5 {
		t.Errorf("unexpected temperature: %v", cfg.Temperature)
	}
	if cfg.MaxOutputTokens == nil || *cfg.MaxOutputTokens != 4096 {
		t.Errorf("unexpected maxOutputTokens: %v", cfg.MaxOutputTokens)
	}
	if cfg.MaxSteps == nil || *cfg.MaxSteps != 50 {
		t.Errorf("unexpected maxSteps: %v", cfg.MaxSteps)
	}
	if cfg.SystemPrompt != "You are helpful." {
		t.Errorf("unexpected systemPrompt: %q", cfg.SystemPrompt)
	}
}

// ── Deployment tests ──

func TestBuildAgentDeployment(t *testing.T) {
	agent := testAgent()
	deploy := BuildAgentDeployment(agent, nil)

	containers := deploy.Spec.Template.Spec.Containers
	if len(containers) < 1 {
		t.Fatal("expected at least 1 container")
	}

	main := containers[0]
	if main.Name != "agent-runtime" {
		t.Errorf("expected container name 'agent-runtime', got %q", main.Name)
	}
	if main.Image != agentsv1alpha1.DefaultFantasyImage {
		t.Errorf("expected image %q, got %q", agentsv1alpha1.DefaultFantasyImage, main.Image)
	}
	if main.Command[0] != "/app/agent-runtime" {
		t.Errorf("expected '/app/agent-runtime' command, got %q", main.Command[0])
	}
	if len(main.Command) < 2 || main.Command[1] != "daemon" {
		t.Errorf("expected 'daemon' arg, got %v", main.Command)
	}
}

func TestBuildAgentDeployment_EnvVars(t *testing.T) {
	agent := testAgent()
	deploy := BuildAgentDeployment(agent, nil)

	main := deploy.Spec.Template.Spec.Containers[0]
	envMap := make(map[string]string)
	for _, e := range main.Env {
		if e.Value != "" {
			envMap[e.Name] = e.Value
		}
	}

	if envMap["AGENT_RUNTIME"] != "fantasy" {
		t.Errorf("expected AGENT_RUNTIME=fantasy, got %q", envMap["AGENT_RUNTIME"])
	}
	if envMap["AGENT_NAME"] != "test-agent" {
		t.Errorf("expected AGENT_NAME=test-agent, got %q", envMap["AGENT_NAME"])
	}
	if envMap["AGENT_MODE"] != "daemon" {
		t.Errorf("expected AGENT_MODE=daemon, got %q", envMap["AGENT_MODE"])
	}
}

func TestBuildAgentDeployment_CustomImage(t *testing.T) {
	agent := testAgent()
	agent.Spec.Image = "custom-registry.io/my-agent:v2"
	deploy := BuildAgentDeployment(agent, nil)

	if deploy.Spec.Template.Spec.Containers[0].Image != "custom-registry.io/my-agent:v2" {
		t.Errorf("expected custom image, got %q", deploy.Spec.Template.Spec.Containers[0].Image)
	}
}

func TestBuildAgentDeployment_HealthCheck(t *testing.T) {
	agent := testAgent()
	deploy := BuildAgentDeployment(agent, nil)

	main := deploy.Spec.Template.Spec.Containers[0]
	if main.LivenessProbe == nil {
		t.Fatal("expected liveness probe for daemon mode")
	}
	if main.ReadinessProbe == nil {
		t.Fatal("expected readiness probe for daemon mode")
	}
	if main.LivenessProbe.HTTPGet.Path != "/healthz" {
		t.Errorf("expected /healthz path, got %q", main.LivenessProbe.HTTPGet.Path)
	}
}

// ── Job tests ──

func TestBuildAgentRunJob(t *testing.T) {
	agent := testAgent()
	agent.Spec.Mode = agentsv1alpha1.AgentModeTask

	run := &agentsv1alpha1.AgentRun{
		ObjectMeta: metav1.ObjectMeta{Name: "test-run", Namespace: "agents"},
		Spec: agentsv1alpha1.AgentRunSpec{
			AgentRef: "test-agent",
			Prompt:   "Do something",
		},
	}

	job := BuildAgentRunJob(run, agent, nil)

	main := job.Spec.Template.Spec.Containers[0]
	if main.Command[0] != "/app/agent-runtime" || main.Command[1] != "task" {
		t.Errorf("expected task command, got %v", main.Command)
	}

	// Check AGENT_PROMPT is injected
	hasPrompt := false
	for _, e := range main.Env {
		if e.Name == "AGENT_PROMPT" && e.Value == "Do something" {
			hasPrompt = true
		}
	}
	if !hasPrompt {
		t.Error("expected AGENT_PROMPT env var in job")
	}

	// RestartPolicy should be Never
	if job.Spec.Template.Spec.RestartPolicy != "Never" {
		t.Errorf("expected RestartPolicy=Never, got %q", job.Spec.Template.Spec.RestartPolicy)
	}
}

// ── MCP ConfigMap tests ──

func TestBuildMCPConfigMap_NoServers(t *testing.T) {
	agent := testAgent()
	cm, err := BuildMCPConfigMap(agent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cm != nil {
		t.Error("expected nil ConfigMap when no MCP servers")
	}
}

func TestBuildMCPConfigMap_WithServers(t *testing.T) {
	agent := testAgent()
	agent.Spec.MCPServers = []agentsv1alpha1.MCPServerBinding{
		{Name: "github-mcp"},
	}

	cm, err := BuildMCPConfigMap(agent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cm == nil {
		t.Fatal("expected ConfigMap")
	}

	data := cm.Data["mcp.json"]
	var mcpCfg MCPJsonConfig
	if err := json.Unmarshal([]byte(data), &mcpCfg); err != nil {
		t.Fatalf("failed to parse mcp.json: %v", err)
	}

	entry, ok := mcpCfg.MCPServers["github-mcp"]
	if !ok {
		t.Fatal("expected github-mcp in mcp.json")
	}
	if entry.Type != "sse" {
		t.Errorf("expected type=sse, got %q", entry.Type)
	}
	if !strings.Contains(entry.URL, "9001") {
		t.Errorf("expected port 9001 in URL, got %q", entry.URL)
	}
}
