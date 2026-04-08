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

func validGitHubRepoResource() *AgentResource {
	return &AgentResource{
		ObjectMeta: metav1.ObjectMeta{Name: "test-resource", Namespace: "default"},
		Spec: AgentResourceSpec{
			Kind:        AgentResourceKindGitHubRepo,
			DisplayName: "Test Repo",
			GitHub: &GitHubResourceConfig{
				Owner: "samyn92",
				Repo:  "agentops-core",
			},
		},
	}
}

// ── Kind config matching ──

func TestAgentResourceValidate_Valid_GitHubRepo(t *testing.T) {
	res := validGitHubRepoResource()
	_, err := res.validate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAgentResourceValidate_Valid_GitLabGroup(t *testing.T) {
	res := &AgentResource{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: AgentResourceSpec{
			Kind:        AgentResourceKindGitLabGroup,
			DisplayName: "Test Group",
			GitLabGroup: &GitLabGroupResourceConfig{
				BaseURL: "https://gitlab.com",
				Group:   "mygroup",
			},
		},
	}
	_, err := res.validate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAgentResourceValidate_Valid_Documentation(t *testing.T) {
	res := &AgentResource{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: AgentResourceSpec{
			Kind:        AgentResourceKindDocumentation,
			DisplayName: "Docs",
			Documentation: &DocumentationResourceConfig{
				URLs: []string{"https://example.com/docs"},
			},
		},
	}
	_, err := res.validate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAgentResourceValidate_Valid_MCPEndpoint(t *testing.T) {
	res := &AgentResource{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: AgentResourceSpec{
			Kind:        AgentResourceKindMCPEndpoint,
			DisplayName: "MCP Server",
			MCP: &MCPResourceConfig{
				URL: "http://mcp.example.com/sse",
			},
		},
	}
	_, err := res.validate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ── Missing config block ──

func TestAgentResourceValidate_MissingConfig(t *testing.T) {
	res := &AgentResource{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: AgentResourceSpec{
			Kind:        AgentResourceKindGitHubRepo,
			DisplayName: "Test",
			// Missing github config block
		},
	}
	_, err := res.validate()
	if err == nil {
		t.Fatal("expected error when github config is missing for kind=github-repo")
	}
}

func TestAgentResourceValidate_MissingGitLabConfig(t *testing.T) {
	res := &AgentResource{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: AgentResourceSpec{
			Kind:        AgentResourceKindGitLabProject,
			DisplayName: "Test",
		},
	}
	_, err := res.validate()
	if err == nil {
		t.Fatal("expected error when gitlab config is missing for kind=gitlab-project")
	}
}

// ── Wrong config block ──

func TestAgentResourceValidate_WrongConfig(t *testing.T) {
	res := &AgentResource{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: AgentResourceSpec{
			Kind:        AgentResourceKindGitHubRepo,
			DisplayName: "Test",
			GitHub: &GitHubResourceConfig{
				Owner: "samyn92",
				Repo:  "test",
			},
			// Extra config block that doesn't match kind
			GitLab: &GitLabResourceConfig{
				BaseURL: "https://gitlab.com",
				Project: "test",
			},
		},
	}
	_, err := res.validate()
	if err == nil {
		t.Fatal("expected error when extra config block is present")
	}
}

// ── All kind configs ──

func TestAgentResourceValidate_Valid_GitRepo(t *testing.T) {
	res := &AgentResource{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: AgentResourceSpec{
			Kind:        AgentResourceKindGitRepo,
			DisplayName: "Git Repo",
			Git: &GitResourceConfig{
				URL: "https://github.com/samyn92/test.git",
			},
		},
	}
	_, err := res.validate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAgentResourceValidate_Valid_S3Bucket(t *testing.T) {
	res := &AgentResource{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: AgentResourceSpec{
			Kind:        AgentResourceKindS3Bucket,
			DisplayName: "S3 Bucket",
			S3: &S3ResourceConfig{
				Bucket: "my-bucket",
				Region: "us-east-1",
			},
		},
	}
	_, err := res.validate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAgentResourceValidate_Valid_GitHubOrg(t *testing.T) {
	res := &AgentResource{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: AgentResourceSpec{
			Kind:        AgentResourceKindGitHubOrg,
			DisplayName: "GitHub Org",
			GitHubOrg: &GitHubOrgResourceConfig{
				Org: "samyn92",
			},
		},
	}
	_, err := res.validate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAgentResourceValidate_Valid_GitLabProject(t *testing.T) {
	res := &AgentResource{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: AgentResourceSpec{
			Kind:        AgentResourceKindGitLabProject,
			DisplayName: "GitLab Project",
			GitLab: &GitLabResourceConfig{
				BaseURL: "https://gitlab.com",
				Project: "mygroup/myproject",
			},
		},
	}
	_, err := res.validate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ── Agent resourceBindings webhook tests ──

func TestValidate_ResourceBindings_Valid(t *testing.T) {
	agent := validAgent()
	agent.Spec.ResourceBindings = []AgentResourceBinding{
		{Name: "repo-a"},
		{Name: "repo-b", ReadOnly: true, AutoContext: true},
	}
	_, err := agent.validate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_ResourceBindings_DuplicateName(t *testing.T) {
	agent := validAgent()
	agent.Spec.ResourceBindings = []AgentResourceBinding{
		{Name: "repo-a"},
		{Name: "repo-a"},
	}
	_, err := agent.validate()
	if err == nil {
		t.Fatal("expected error for duplicate resource binding names")
	}
}

func TestValidate_ResourceBindings_EmptyName(t *testing.T) {
	agent := validAgent()
	agent.Spec.ResourceBindings = []AgentResourceBinding{
		{Name: ""},
	}
	_, err := agent.validate()
	if err == nil {
		t.Fatal("expected error for empty resource binding name")
	}
}
