/*
Agent Runtime — Fantasy (Go)

Main entrypoint. Two subcommands:
  - daemon: HTTP server on :4096 (Deployment mode)
  - task: Read AGENT_PROMPT, run once, JSON to stdout (Job mode)
*/
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"charm.land/fantasy"
)

const (
	configPath = "/etc/operator/config.json"
	port       = 4096
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: agent-runtime <daemon|task>")
		os.Exit(1)
	}

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))

	switch os.Args[1] {
	case "daemon":
		if err := runDaemon(); err != nil {
			slog.Error("daemon failed", "error", err)
			os.Exit(1)
		}
	case "task":
		if err := runTask(); err != nil {
			slog.Error("task failed", "error", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

// ====================================================================
// Shared setup
// ====================================================================

func loadConfig() (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &cfg, nil
}

// agentBundle holds the built agent and resources that need cleanup.
type agentBundle struct {
	agent      fantasy.Agent
	providers  map[string]fantasy.Provider
	mcpConns   []mcpConnection
}

func buildAgentBundle(ctx context.Context, cfg *Config) (*agentBundle, error) {
	// Resolve providers
	providers := make(map[string]fantasy.Provider)
	for _, p := range cfg.Providers {
		provider, err := resolveProvider(p.Name)
		if err != nil {
			slog.Warn("failed to resolve provider", "name", p.Name, "error", err)
			continue
		}
		providers[p.Name] = provider
		slog.Info("registered provider", "name", p.Name)
	}

	// Resolve primary model
	model, err := resolveModel(ctx, cfg.PrimaryModel, providers)
	if err != nil {
		return nil, fmt.Errorf("resolve model %s: %w", cfg.PrimaryModel, err)
	}

	// Build tools
	tools := buildBuiltinTools(cfg.BuiltinTools)

	// Load MCP tools from OCI-packaged servers
	var allMCPConns []mcpConnection
	if len(cfg.Tools) > 0 {
		ociTools, conns, err := loadOCITools(ctx, cfg.Tools)
		if err != nil {
			slog.Warn("failed to load some OCI tools", "error", err)
		}
		tools = append(tools, ociTools...)
		allMCPConns = append(allMCPConns, conns...)
	}

	// Load MCP tools from gateway sidecars
	if len(cfg.MCPServers) > 0 {
		gwTools, conns, err := loadGatewayMCPTools(ctx, cfg.MCPServers)
		if err != nil {
			slog.Warn("failed to load some gateway MCP tools", "error", err)
		}
		tools = append(tools, gwTools...)
		allMCPConns = append(allMCPConns, conns...)
	}

	// Wrap with security hooks
	tools = wrapToolsWithHooks(tools, cfg.ToolHooks)

	// Add orchestration tools (run_agent, get_agent_run)
	k8sClient, err := NewK8sClient()
	if err != nil {
		slog.Warn("K8s client unavailable, orchestration tools disabled", "error", err)
		// Add stub tools that return errors
		tools = append(tools, newRunAgentToolStub(), newGetAgentRunToolStub())
	} else {
		tools = append(tools, newRunAgentTool(k8sClient), newGetAgentRunTool(k8sClient))
	}

	// Build agent options
	opts := []fantasy.AgentOption{
		fantasy.WithTools(tools...),
	}

	if cfg.SystemPrompt != "" {
		opts = append(opts, fantasy.WithSystemPrompt(cfg.SystemPrompt))
	}
	if cfg.Temperature != nil {
		opts = append(opts, fantasy.WithTemperature(*cfg.Temperature))
	}
	if cfg.MaxOutputTokens != nil {
		opts = append(opts, fantasy.WithMaxOutputTokens(*cfg.MaxOutputTokens))
	}
	if cfg.MaxSteps != nil {
		opts = append(opts, fantasy.WithStopConditions(fantasy.StepCountIs(*cfg.MaxSteps)))
	}

	return &agentBundle{
		agent:      fantasy.NewAgent(model, opts...),
		providers:  providers,
		mcpConns:   allMCPConns,
	}, nil
}

// buildFallbackAgent creates a new agent with a fallback model (reusing tools/options from config).
func buildFallbackAgent(ctx context.Context, cfg *Config, providers map[string]fantasy.Provider, modelStr string, originalAgent fantasy.Agent) (fantasy.Agent, error) {
	model, err := resolveModel(ctx, modelStr, providers)
	if err != nil {
		return nil, err
	}

	opts := []fantasy.AgentOption{}
	if cfg.SystemPrompt != "" {
		opts = append(opts, fantasy.WithSystemPrompt(cfg.SystemPrompt))
	}
	if cfg.Temperature != nil {
		opts = append(opts, fantasy.WithTemperature(*cfg.Temperature))
	}
	if cfg.MaxOutputTokens != nil {
		opts = append(opts, fantasy.WithMaxOutputTokens(*cfg.MaxOutputTokens))
	}
	if cfg.MaxSteps != nil {
		opts = append(opts, fantasy.WithStopConditions(fantasy.StepCountIs(*cfg.MaxSteps)))
	}

	return fantasy.NewAgent(model, opts...), nil
}

// generateWithFallback tries the primary model, then fallbacks on retryable errors.
func generateWithFallback(ctx context.Context, cfg *Config, bundle *agentBundle, call fantasy.AgentCall) (*fantasy.AgentResult, string, error) {
	// Try primary model
	result, err := bundle.agent.Generate(ctx, call)
	if err == nil {
		return result, cfg.PrimaryModel, nil
	}

	if !isRetryableError(err) || len(cfg.FallbackModels) == 0 {
		return nil, cfg.PrimaryModel, err
	}

	slog.Warn("primary model failed, trying fallbacks",
		"model", cfg.PrimaryModel, "error", err)

	// Try fallback models
	for _, fbModel := range cfg.FallbackModels {
		fbAgent, fbErr := buildFallbackAgent(ctx, cfg, bundle.providers, fbModel, bundle.agent)
		if fbErr != nil {
			slog.Warn("failed to build fallback agent", "model", fbModel, "error", fbErr)
			continue
		}

		result, err = fbAgent.Generate(ctx, call)
		if err == nil {
			slog.Info("fallback model succeeded", "model", fbModel)
			return result, fbModel, nil
		}

		if !isRetryableError(err) {
			return nil, fbModel, err
		}
		slog.Warn("fallback model failed", "model", fbModel, "error", err)
	}

	return nil, cfg.PrimaryModel, fmt.Errorf("all models failed, last error: %w", err)
}

// streamWithFallback tries the primary model, then fallbacks on retryable errors.
func streamWithFallback(ctx context.Context, cfg *Config, bundle *agentBundle, call fantasy.AgentStreamCall) (*fantasy.AgentResult, string, error) {
	result, err := bundle.agent.Stream(ctx, call)
	if err == nil {
		return result, cfg.PrimaryModel, nil
	}

	if !isRetryableError(err) || len(cfg.FallbackModels) == 0 {
		return nil, cfg.PrimaryModel, err
	}

	slog.Warn("primary model failed on stream, trying fallbacks",
		"model", cfg.PrimaryModel, "error", err)

	for _, fbModel := range cfg.FallbackModels {
		fbAgent, fbErr := buildFallbackAgent(ctx, cfg, bundle.providers, fbModel, bundle.agent)
		if fbErr != nil {
			continue
		}

		result, err = fbAgent.Stream(ctx, call)
		if err == nil {
			return result, fbModel, nil
		}

		if !isRetryableError(err) {
			return nil, fbModel, err
		}
	}

	return nil, cfg.PrimaryModel, fmt.Errorf("all models failed, last error: %w", err)
}

// isRetryableError checks if an error should trigger fallback.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "429") ||
		strings.Contains(s, "500") ||
		strings.Contains(s, "502") ||
		strings.Contains(s, "503") ||
		strings.Contains(s, "rate limit") ||
		strings.Contains(s, "overloaded")
}

// ====================================================================
// Daemon mode: HTTP server
// ====================================================================

func runDaemon() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	slog.Info("starting Fantasy daemon agent",
		"model", cfg.PrimaryModel,
		"providers", len(cfg.Providers),
		"builtinTools", len(cfg.BuiltinTools),
		"ociTools", len(cfg.Tools),
		"mcpServers", len(cfg.MCPServers),
		"fallbackModels", len(cfg.FallbackModels),
	)

	bundle, err := buildAgentBundle(ctx, cfg)
	if err != nil {
		return err
	}
	defer shutdownMCPConnections(bundle.mcpConns)

	srv := &daemonServer{
		bundle:      bundle,
		cfg:         cfg,
		activeModel: cfg.PrimaryModel,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /prompt", srv.handlePrompt)
	mux.HandleFunc("POST /prompt/stream", srv.handlePromptStream)
	mux.HandleFunc("POST /followup", srv.handleFollowup)
	mux.HandleFunc("DELETE /abort", srv.handleAbort)
	mux.HandleFunc("GET /healthz", srv.handleHealthz)
	mux.HandleFunc("GET /status", srv.handleStatus)

	httpSrv := &http.Server{Addr: fmt.Sprintf(":%d", port), Handler: mux}

	go func() {
		<-ctx.Done()
		slog.Info("shutting down...")
		httpSrv.Close()
	}()

	slog.Info("listening", "port", port)
	return httpSrv.ListenAndServe()
}

type daemonServer struct {
	bundle      *agentBundle
	cfg         *Config
	mu          sync.Mutex
	busy        bool
	lastOutput  string
	activeModel string
	totalSteps  int
	cancel      context.CancelFunc
	messages    []fantasy.Message // conversation history
}

type promptRequest struct {
	Prompt string `json:"prompt"`
}

type promptResponse struct {
	Output string `json:"output"`
	Model  string `json:"model"`
}

func (s *daemonServer) handlePrompt(w http.ResponseWriter, r *http.Request) {
	var req promptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Prompt == "" {
		http.Error(w, `{"error":"prompt is required"}`, http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	if s.busy {
		s.mu.Unlock()
		http.Error(w, `{"error":"Agent is busy"}`, http.StatusTooManyRequests)
		return
	}
	s.busy = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.busy = false
		s.mu.Unlock()
	}()

	ctx, cancel := context.WithCancel(r.Context())
	s.mu.Lock()
	s.cancel = cancel
	s.mu.Unlock()
	defer cancel()

	result, usedModel, err := generateWithFallback(ctx, s.cfg, s.bundle, fantasy.AgentCall{
		Prompt:   req.Prompt,
		Messages: s.messages,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	output := result.Response.Content.Text()
	s.mu.Lock()
	s.lastOutput = output
	s.activeModel = usedModel
	s.totalSteps += len(result.Steps)
	s.messages = append(s.messages, fantasy.NewUserMessage(req.Prompt))
	for _, step := range result.Steps {
		s.messages = append(s.messages, step.Messages...)
	}
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(promptResponse{Output: output, Model: usedModel})
}

func (s *daemonServer) handlePromptStream(w http.ResponseWriter, r *http.Request) {
	var req promptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Prompt == "" {
		http.Error(w, `{"error":"prompt is required"}`, http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	if s.busy {
		s.mu.Unlock()
		http.Error(w, `{"error":"Agent is busy"}`, http.StatusTooManyRequests)
		return
	}
	s.busy = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.busy = false
		s.mu.Unlock()
	}()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, _ := w.(http.Flusher)

	ctx, cancel := context.WithCancel(r.Context())
	s.mu.Lock()
	s.cancel = cancel
	s.mu.Unlock()
	defer cancel()

	result, usedModel, err := streamWithFallback(ctx, s.cfg, s.bundle, fantasy.AgentStreamCall{
		Prompt:   req.Prompt,
		Messages: s.messages,
		OnTextDelta: func(id, text string) error {
			data, _ := json.Marshal(map[string]string{"type": "text_delta", "delta": text})
			fmt.Fprintf(w, "data: %s\n\n", data)
			if flusher != nil {
				flusher.Flush()
			}
			return nil
		},
		OnToolCall: func(tc fantasy.ToolCallContent) error {
			data, _ := json.Marshal(map[string]string{"type": "tool_start", "tool": tc.ToolName})
			fmt.Fprintf(w, "data: %s\n\n", data)
			if flusher != nil {
				flusher.Flush()
			}
			return nil
		},
		OnToolResult: func(tr fantasy.ToolResultContent) error {
			data, _ := json.Marshal(map[string]any{"type": "tool_end", "tool": tr.ToolName})
			fmt.Fprintf(w, "data: %s\n\n", data)
			if flusher != nil {
				flusher.Flush()
			}
			return nil
		},
	})

	if err != nil {
		data, _ := json.Marshal(map[string]string{"type": "error", "error": err.Error()})
		fmt.Fprintf(w, "data: %s\n\n", data)
	} else {
		output := result.Response.Content.Text()
		s.mu.Lock()
		s.lastOutput = output
		s.activeModel = usedModel
		s.totalSteps += len(result.Steps)
		s.messages = append(s.messages, fantasy.NewUserMessage(req.Prompt))
		for _, step := range result.Steps {
			s.messages = append(s.messages, step.Messages...)
		}
		s.mu.Unlock()

		data, _ := json.Marshal(map[string]string{"type": "complete", "output": output, "model": usedModel})
		fmt.Fprintf(w, "data: %s\n\n", data)
	}
}

func (s *daemonServer) handleFollowup(w http.ResponseWriter, r *http.Request) {
	s.handlePrompt(w, r)
}

func (s *daemonServer) handleAbort(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
	}
	s.busy = false
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	io.WriteString(w, `{"ok":true}`)
}

func (s *daemonServer) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	io.WriteString(w, `{"status":"ok"}`)
}

func (s *daemonServer) handleStatus(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"busy":   s.busy,
		"output": s.lastOutput,
		"model":  s.activeModel,
		"steps":  s.totalSteps,
	})
}

// ====================================================================
// Task mode: one-shot execution
// ====================================================================

type taskResult struct {
	Output  string `json:"output"`
	Steps   int    `json:"steps"`
	Model   string `json:"model"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

func runTask() error {
	ctx := context.Background()

	prompt := os.Getenv("AGENT_PROMPT")
	if prompt == "" {
		result := taskResult{Success: false, Error: "AGENT_PROMPT environment variable is not set"}
		json.NewEncoder(os.Stdout).Encode(result)
		return fmt.Errorf("AGENT_PROMPT not set")
	}

	cfg, err := loadConfig()
	if err != nil {
		result := taskResult{Success: false, Error: err.Error()}
		json.NewEncoder(os.Stdout).Encode(result)
		return err
	}

	slog.Info("running Fantasy task agent",
		"model", cfg.PrimaryModel,
		"prompt", truncate(prompt, 100),
	)

	bundle, err := buildAgentBundle(ctx, cfg)
	if err != nil {
		result := taskResult{Success: false, Error: err.Error(), Model: cfg.PrimaryModel}
		json.NewEncoder(os.Stdout).Encode(result)
		return err
	}
	defer shutdownMCPConnections(bundle.mcpConns)

	agentResult, usedModel, err := generateWithFallback(ctx, cfg, bundle, fantasy.AgentCall{Prompt: prompt})
	if err != nil {
		result := taskResult{Success: false, Error: err.Error(), Model: cfg.PrimaryModel}
		json.NewEncoder(os.Stdout).Encode(result)
		return err
	}

	output := agentResult.Response.Content.Text()
	result := taskResult{
		Output:  output,
		Steps:   len(agentResult.Steps),
		Model:   usedModel,
		Success: true,
	}
	json.NewEncoder(os.Stdout).Encode(result)
	return nil
}

// ====================================================================
// Orchestration tools (run_agent, get_agent_run)
// ====================================================================

type runAgentInput struct {
	Agent  string `json:"agent" description:"Agent name to run"`
	Prompt string `json:"prompt" description:"Prompt to send to the agent"`
}

func newRunAgentTool(k8s *K8sClient) fantasy.AgentTool {
	return fantasy.NewAgentTool("run_agent",
		"Trigger another agent with a prompt. Creates an AgentRun CR tracked by the operator.",
		func(ctx context.Context, input runAgentInput, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if input.Agent == "" || input.Prompt == "" {
				return fantasy.NewTextErrorResponse("agent and prompt are required"), nil
			}
			agentName := os.Getenv("AGENT_NAME")
			if agentName == "" {
				agentName = "unknown"
			}
			run, err := k8s.CreateAgentRun(ctx, input.Agent, input.Prompt, "agent", agentName)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("Failed to create AgentRun: %s", err)), nil
			}
			return fantasy.NewTextResponse(fmt.Sprintf("AgentRun %s created for agent %q", run.Name, input.Agent)), nil
		})
}

func newRunAgentToolStub() fantasy.AgentTool {
	return fantasy.NewAgentTool("run_agent",
		"Trigger another agent with a prompt. (Unavailable: K8s client not configured)",
		func(_ context.Context, _ runAgentInput, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.NewTextErrorResponse("run_agent unavailable: not running in Kubernetes"), nil
		})
}

type getAgentRunInput struct {
	Name string `json:"name" description:"AgentRun name to check"`
}

func newGetAgentRunTool(k8s *K8sClient) fantasy.AgentTool {
	return fantasy.NewAgentTool("get_agent_run",
		"Check the status and output of an AgentRun.",
		func(ctx context.Context, input getAgentRunInput, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if input.Name == "" {
				return fantasy.NewTextErrorResponse("name is required"), nil
			}
			status, err := k8s.GetAgentRun(ctx, input.Name)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("Failed to get AgentRun: %s", err)), nil
			}
			return fantasy.NewTextResponse(fmt.Sprintf("Phase: %s\nOutput: %s", status.Phase, status.Output)), nil
		})
}

func newGetAgentRunToolStub() fantasy.AgentTool {
	return fantasy.NewAgentTool("get_agent_run",
		"Check the status and output of an AgentRun. (Unavailable: K8s client not configured)",
		func(_ context.Context, _ getAgentRunInput, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.NewTextErrorResponse("get_agent_run unavailable: not running in Kubernetes"), nil
		})
}
