# agentops-core

Kubernetes operator and agent runtime for running AI coding agents as native Kubernetes workloads. Agents run the [Pi SDK](https://github.com/badlogic/pi-mono) inside containers, with the operator managing their full lifecycle: deployments, jobs, storage, MCP server bindings, channel bridges, and concurrency control.

## Architecture

```
EXTERNAL                       OPERATOR (reconciles)                     KUBERNETES
                                                                        
  Telegram ─┐                 ┌──────────────────┐                       
  Slack    ──┤  Channel CRs   │  Channel Bridge   │  HTTP POST /prompt   
  Discord  ──┤──────────────► │  (Deployment)     │─────────────────────► Agent (daemon)
             │  chat types    │                   │                       Deployment + PVC + Service
             │  forward msgs  └──────────────────┘                       Pi SDK + HTTP server (:4096)
             │  directly                                                  ├── /prompt
  GitLab  ───┤                                                            ├── /prompt/stream
  GitHub  ───┤  event types   ┌──────────────────┐                       ├── /steer, /followup, /abort
  Webhook ───┘──────────────► │  Channel Bridge   │─── creates ──► AgentRun CR
                              │  (renders prompt  │                  │
                              │   from template)  │                  │
                              └──────────────────┘                  │
                                                                    │
  run_agent tool ──────────────────── creates ─────────────────► AgentRun CR
  (daemon agent calling another)                                    │
                                                                    │
  Cron Schedule ───────────────────── creates ─────────────────► AgentRun CR
                                                                    │
                                                                    ▼
                                                        ┌──────────────────┐
                                                        │ AgentRunReconciler│
                                                        │                  │
                                                        │ task agent?      │
                                                        │   → create Job   │──► Agent (task)
                                                        │                  │    Job (one-shot)
                                                        │ daemon agent?    │    Pi SDK, exits
                                                        │   → HTTP POST    │──► Agent (daemon)
                                                        │     /prompt      │    (already running)
                                                        └──────────────────┘

  MCPServer CRs                ┌──────────────────┐
  (deploy mode)  ────────────► │  MCP Deployment   │  :8080 (mcp-gateway spawn mode)
                               │  + Service        │◄──── SSE ──── gw sidecar in Agent pod
  (external mode) ─────────────│  health probe     │               (proxy mode, deny/allow rules)
                               └──────────────────┘
```

### Data flow summary

| Source | Target Agent Mode | Path |
|--------|:---:|------|
| Chat channel (Telegram/Slack/Discord) | daemon | Channel bridge → HTTP POST → Agent Service |
| Event channel (GitLab/GitHub/Webhook) | daemon or task | Channel bridge → AgentRun CR → Reconciler |
| `run_agent` tool | daemon or task | Daemon agent → AgentRun CR → Reconciler |
| Cron schedule | daemon or task | Operator → AgentRun CR → Reconciler |

### Two components in this repo

1. **Operator** (Go) — watches 4 CRDs and reconciles Kubernetes resources
2. **Agent Runtime** (TypeScript) — runs inside agent pods, wraps Pi SDK

## Custom Resource Definitions

| CRD | Short Name | Description |
|-----|-----------|-------------|
| `Agent` | `ag` | Defines an AI agent (model, tools, MCP bindings, mode) |
| `Channel` | `ch` | Bridges external platforms (Telegram, Slack, GitLab, GitHub, etc.) to agents |
| `AgentRun` | `ar` | Tracks a single prompt execution against an agent |
| `MCPServer` | `mcp` | Shared MCP infrastructure (deployed or external) |

### Agent modes

- **`daemon`** — long-running Deployment + PVC + Service. Receives prompts via HTTP (`/prompt`, `/prompt/stream`, `/steer`, `/followup`, `/abort`). Supports session compaction.
- **`task`** — one-shot Job per AgentRun. Prompt in, structured result out, container exits.

## Project Structure

```
agentops-core/
  api/v1alpha1/           # CRD types (Agent, Channel, AgentRun, MCPServer)
  internal/
    controller/           # 4 reconcilers
    resources/            # Kubernetes resource builders (Deployments, Jobs, PVCs, etc.)
  images/agent-runtime/   # TypeScript agent runtime (Pi SDK wrapper)
  config/
    crd/bases/            # Generated CRD YAMLs
    rbac/                 # Generated RBAC
    manager/              # Operator Deployment
    webhook/              # Webhook configuration
    samples/              # Example CRs
  cmd/main.go             # Operator entrypoint
  Dockerfile              # Operator image
```

## Prerequisites

- Go 1.25+
- Node.js 22+ (for agent-runtime)
- Docker
- kubectl
- Access to a Kubernetes cluster (v1.28+)

## Quick Start

### Install CRDs

```sh
make install
```

### Run the operator locally (development)

```sh
make run
```

### Deploy to a cluster

```sh
make docker-build docker-push IMG=ghcr.io/samyn92/agentops-core:latest
make deploy IMG=ghcr.io/samyn92/agentops-core:latest
```

### Apply sample resources

```sh
kubectl apply -k config/samples/
```

### Create a minimal agent

```yaml
apiVersion: agents.agentops.io/v1alpha1
kind: Agent
metadata:
  name: my-agent
spec:
  mode: daemon
  model: anthropic/claude-sonnet-4-20250514
  providers:
    - name: anthropic
      apiKeySecret:
        name: llm-api-keys
        key: ANTHROPIC_API_KEY
  builtinTools:
    - read
    - bash
    - edit
    - write
```

### Trigger a run

```yaml
apiVersion: agents.agentops.io/v1alpha1
kind: AgentRun
metadata:
  name: test-run
spec:
  agentRef: my-agent
  source: channel
  sourceRef: manual
  prompt: "List all files in the workspace"
```

## Uninstall

```sh
kubectl delete -k config/samples/
make undeploy
make uninstall
```

## Development

```sh
# Generate deepcopy and CRD manifests
make generate
make manifests

# Build
go build ./...

# Lint
golangci-lint run

# Build agent runtime
cd images/agent-runtime && npm install && npm run build
```

## License

Copyright 2026. Licensed under the Apache License, Version 2.0.
