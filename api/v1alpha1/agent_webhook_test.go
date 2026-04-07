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
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func float64Ptr(f float64) *float64 { return &f }
func intPtr(i int) *int             { return &i }

func validAgent() *Agent {
	return &Agent{
		ObjectMeta: metav1.ObjectMeta{Name: "test-agent", Namespace: "default"},
		Spec: AgentSpec{
			Mode:  AgentModeDaemon,
			Model: "anthropic/claude-sonnet-4-20250514",
			Providers: []ProviderRef{
				{Name: "anthropic", ApiKeySecret: SecretKeyRef{Name: "keys", Key: "key"}},
			},
			BuiltinTools: []string{"bash", "read", "edit", "write"},
		},
	}
}

// ── Builtin tools ──

func TestValidate_InvalidTool(t *testing.T) {
	agent := validAgent()
	agent.Spec.BuiltinTools = []string{"bash", "find"} // find is not valid

	_, err := agent.validate()
	if err == nil {
		t.Fatal("expected error for invalid tool 'find'")
	}
}

func TestValidate_ValidTools(t *testing.T) {
	agent := validAgent()
	agent.Spec.BuiltinTools = []string{"bash", "read", "edit", "write", "grep", "ls", "glob", "fetch"}

	_, err := agent.validate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ── Temperature ──

func TestValidate_TemperatureOutOfRange(t *testing.T) {
	agent := validAgent()
	agent.Spec.Temperature = float64Ptr(3.0)

	_, err := agent.validate()
	if err == nil {
		t.Fatal("expected error for temperature > 2.0")
	}
}

func TestValidate_NegativeTemperature(t *testing.T) {
	agent := validAgent()
	agent.Spec.Temperature = float64Ptr(-0.5)

	_, err := agent.validate()
	if err == nil {
		t.Fatal("expected error for negative temperature")
	}
}

func TestValidate_ValidTemperature(t *testing.T) {
	agent := validAgent()
	agent.Spec.Temperature = float64Ptr(0.7)

	_, err := agent.validate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ── MaxSteps ──

func TestValidate_ZeroMaxSteps(t *testing.T) {
	agent := validAgent()
	agent.Spec.MaxSteps = intPtr(0)

	_, err := agent.validate()
	if err == nil {
		t.Fatal("expected error for maxSteps = 0")
	}
}

// ── Mode validation ──

func TestValidate_TaskMode_NoStorage(t *testing.T) {
	agent := validAgent()
	agent.Spec.Mode = AgentModeTask
	agent.Spec.Storage = &StorageSpec{Size: "10Gi"}

	_, err := agent.validate()
	if err == nil {
		t.Fatal("expected error for storage in task mode")
	}
}

// ── Providers ──

func TestValidate_NoProviders(t *testing.T) {
	agent := validAgent()
	agent.Spec.Providers = nil

	_, err := agent.validate()
	if err == nil {
		t.Fatal("expected error when no providers configured")
	}
}

// ── Tools: at least one required ──

func TestValidate_NoTools(t *testing.T) {
	agent := validAgent()
	agent.Spec.BuiltinTools = nil
	agent.Spec.ToolRefs = nil

	_, err := agent.validate()
	if err == nil {
		t.Fatal("expected error when no tools configured")
	}
}

func TestValidate_ToolRefsOnly(t *testing.T) {
	agent := validAgent()
	agent.Spec.BuiltinTools = nil
	agent.Spec.ToolRefs = []ResourceRef{
		{Name: "my-tool", OCIRef: &OCIRef{Ref: "ghcr.io/test/tool:1.0"}},
	}

	_, err := agent.validate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ── FallbackModels ──

func TestValidate_FallbackModel_UnknownProvider(t *testing.T) {
	agent := validAgent()
	agent.Spec.FallbackModels = []string{"openai/gpt-4o"}

	_, err := agent.validate()
	if err == nil {
		t.Fatal("expected error for fallback model with unconfigured provider")
	}
}

func TestValidate_FallbackModel_ValidProvider(t *testing.T) {
	agent := validAgent()
	agent.Spec.Providers = append(agent.Spec.Providers,
		ProviderRef{Name: "openai", ApiKeySecret: SecretKeyRef{Name: "keys", Key: "key"}},
	)
	agent.Spec.FallbackModels = []string{"openai/gpt-4o"}

	_, err := agent.validate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ── Schedule ──

func TestValidate_Schedule_MissingPrompt(t *testing.T) {
	agent := validAgent()
	agent.Spec.Schedule = "0 * * * *"
	agent.Spec.SchedulePrompt = ""

	_, err := agent.validate()
	if err == nil {
		t.Fatal("expected error when schedule set without schedulePrompt")
	}
}

// ── ToolHooks ──

func TestValidate_ToolHooks_RelativePath(t *testing.T) {
	agent := validAgent()
	agent.Spec.ToolHooks = &ToolHooksSpec{
		AllowedPaths: []string{"relative/path"},
	}

	_, err := agent.validate()
	if err == nil {
		t.Fatal("expected error for relative path in allowedPaths")
	}
}

func TestValidate_ToolHooks_AbsolutePath(t *testing.T) {
	agent := validAgent()
	agent.Spec.ToolHooks = &ToolHooksSpec{
		AllowedPaths: []string{"/data/workspace"},
	}

	_, err := agent.validate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ── ResourceRefs ──

func TestValidate_ToolRef_MultipleSourcesInvalid(t *testing.T) {
	agent := validAgent()
	agent.Spec.ToolRefs = []ResourceRef{
		{
			Name:         "bad-tool",
			OCIRef:       &OCIRef{Ref: "ghcr.io/test:1.0"},
			ConfigMapRef: &SecretKeyRef{Name: "cm", Key: "k"},
		},
	}

	_, err := agent.validate()
	if err == nil {
		t.Fatal("expected error for tool with multiple sources")
	}
}

// ── Helper methods ──

func TestRuntimeImage_Default(t *testing.T) {
	agent := validAgent()
	if agent.RuntimeImage() != DefaultFantasyImage {
		t.Fatalf("expected %q, got %q", DefaultFantasyImage, agent.RuntimeImage())
	}
}

func TestRuntimeImage_Custom(t *testing.T) {
	agent := validAgent()
	agent.Spec.Image = "custom:1.0"
	if agent.RuntimeImage() != "custom:1.0" {
		t.Fatalf("expected 'custom:1.0', got %q", agent.RuntimeImage())
	}
}

func TestBuiltinToolCount(t *testing.T) {
	agent := validAgent()
	if agent.BuiltinToolCount() != 4 {
		t.Fatalf("expected 4, got %d", agent.BuiltinToolCount())
	}
}
