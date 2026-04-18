# PROPOSAL: First-class Outcome on AgentRun

Status: **Draft v3** — split intent (why the run happened) from artifacts
(what durable thing it produced). Caller hints intent, executing agent
finalizes it.
Target CRD: `agentruns.agents.agentops.io`
Related: `agentops-console` Runs sidebar UX, planned DelegationGraph view.

## Motivation

An `AgentRun` today captures *execution* (phase, tokens, tool calls, trace)
but only bolts on one outcome field — `status.pullRequestURL` — populated
ad-hoc by task agents with a git spec.

In practice every meaningful run has two orthogonal dimensions:

1. **Intent** — *why* the run happened (a code change, a planning session,
   an incident triage, a knowledge capture, or chit-chat).
2. **Artifacts** — *what* durable references it produced (a PR, an Issue,
   a memory note, a commit).

These are independent: a planning session and an incident triage both
typically produce an Issue, but they're rendered differently and queried
differently. Mixing them into a single `kind` field forces lossy
classification. Keeping them separate gives the console a meaningful
Run card, lets operators reason about *what a run did*, and makes
PR-creation a special case of a general pattern instead of a bolt-on.

| Intent       | Typical artifacts                          | Example |
|--------------|--------------------------------------------|---------|
| `change`     | `pr` (+ `commit`)                          | "Add memory dedup" |
| `plan`       | `issue` (+ optional `memory`)              | "Spec out OCI Skills feature" |
| `incident`   | `issue` (RCA) + `memory` (lesson learned)  | "Tempo 502s triaged" |
| `discovery`  | `memory`                                   | "How FEP propagation works" |
| `noop`       | —                                          | Chit-chat, aborted run, daemon idle |

## Who writes what?

**Caller hints intent. Executing agent finalizes intent and writes artifacts.**
No external observer, no separate Scribe process.

- The caller (a daemon delegating, or the operator creating an ad-hoc run)
  can set `spec.outcome.intent` to declare expectation. Optional.
- The executing agent writes `status.outcome.intent` at end-of-run. This
  is authoritative. If the run started as `plan` but became `change`,
  the agent records the actual outcome.
- The executing agent appends to `status.outcome.artifacts`. It is the
  only process with full tool-call context *and* the credentials
  provisioned for its AgentResources. It already calls
  `github.create_pull_request`, `gitlab.create_issue`, `mem_save` today —
  this proposal just formalizes where the result lands.
- Daemon chat agents (the one you talk to) typically produce `noop` /
  `discovery` outcomes for their own chat turns. When a daemon delegates
  to a task agent, the *child* Run carries the artifacts and the daemon's
  delegation-result card links to it.

## Design

### Spec additions (optional)

```go
// AgentRunOutcomeSpec lets the caller hint the expected intent
// when the run is created. The executing agent overrides this in
// status.outcome.intent if reality differs.
type AgentRunOutcomeSpec struct {
    // +kubebuilder:validation:Enum=change;plan;incident;discovery;noop
    // +optional
    Intent AgentRunIntent `json:"intent,omitempty"`
}
```

Added to `AgentRunSpec`:
```go
Outcome *AgentRunOutcomeSpec `json:"outcome,omitempty"`
```

### Status additions

```go
type AgentRunIntent string
const (
    IntentChange    AgentRunIntent = "change"
    IntentPlan      AgentRunIntent = "plan"
    IntentIncident  AgentRunIntent = "incident"
    IntentDiscovery AgentRunIntent = "discovery"
    IntentNoop      AgentRunIntent = "noop"
)

// AgentRunArtifact describes a durable external reference produced by the run.
type AgentRunArtifact struct {
    // Kind of artifact. "pr" subsumes the legacy PullRequestURL field.
    // +kubebuilder:validation:Enum=pr;mr;issue;memory;commit
    Kind string `json:"kind"`

    // Forge provider (github, gitlab, memory, ...).
    Provider string `json:"provider,omitempty"`

    // Fully-qualified URL — where a human clicks.
    URL string `json:"url,omitempty"`

    // Identifier within the provider (PR number, issue number, memory id, sha).
    Ref string `json:"ref,omitempty"`

    // Short human title rendered in the console Run card.
    Title string `json:"title,omitempty"`
}

type AgentRunOutcomeStatus struct {
    // Finalized intent. Written once at end-of-run by the executing
    // agent. Authoritative — overrides spec.outcome.intent. Console
    // falls back to spec value when this is empty (run still in flight).
    // Immutable after that — re-classification requires a new run.
    // +optional
    Intent AgentRunIntent `json:"intent,omitempty"`

    // Artifacts produced by the run. A "change" typically has one `pr`
    // plus a `commit`; a "plan" has one `issue`; an "incident" has one
    // `issue` plus a `memory` RCA entry; a "discovery" has one `memory`.
    // The executing agent appends here.
    // +optional
    Artifacts []AgentRunArtifact `json:"artifacts,omitempty"`

    // Short summary (1-3 sentences) written by the executing agent at
    // end-of-run, for rendering in the Run sidebar card.
    // +optional
    Summary string `json:"summary,omitempty"`
}
```

Added to `AgentRunStatus`:
```go
Outcome *AgentRunOutcomeStatus `json:"outcome,omitempty"`
```

### Immutability

`status.outcome.intent` is set once by the executing agent at end-of-run.
The operator's AgentRun controller rejects further writes (admission check
on status subresource). `status.outcome.artifacts` is append-only during
the run — once `status.phase` is terminal, no further appends. Users who
want to re-classify create a linked follow-up Run.

### Removal of `status.pullRequestURL`

**Hard drop, no shim.** This is still `v1alpha1`; downstream consumers are
all in-repo (agentops-console). Coordinate the removal with the
corresponding console read-path update in a single migration. `branch` and
`commits` similarly fold into `artifacts` of kind `commit`.

New printcolumns:
```
+kubebuilder:printcolumn:name="Intent",type=string,JSONPath=`.status.outcome.intent`
+kubebuilder:printcolumn:name="Artifact",type=string,JSONPath=`.status.outcome.artifacts[0].url`
```
Drop:
```
PullRequestURL printcolumn (and the underlying field)
```

## How the executing agent writes its outcome

Two surface changes, both in `agentops-runtime`:

1. **Built-in tool `run_finish`** called by the agent at end-of-run.
   Signature:
   ```go
   run_finish(intent, summary, artifacts)
   ```
   The runtime patches `status.outcome` on the owning AgentRun via the
   Kubernetes API. RBAC: the runtime's existing ServiceAccount already
   writes `status.conditions`; add `patch` on `agentruns/status` (narrow
   scope, status subresource only).

2. **System prompt addendum** (opt-in via Agent CRD field, or auto-injected
   for agents with `spec.git`): short instructions telling the agent to
   call `run_finish` before exiting, with a template for `summary` and
   guidance on choosing intent.

### Incident / RCA template

When the agent sets `intent=incident`, its Issue body (via existing
`github.create_issue` / `gitlab.create_issue` MCP tool) uses a standard
structure:

```markdown
## Symptom
<from the triggering prompt or FEP user message>

## Timeline
<from observed tool-call timestamps, bullet list>

## Root Cause
<agent's own diagnosis>

## Fix
<commits produced, or "diagnosis only — no code change">

## Prevention
<lessons learned, also written to mem_save with kind=lesson_learned>

---
_Run: [{ar.metadata.name}]({console-url}) · Trace: [{traceID}]({tempo-url})_
```

The template ships as a context file mounted into the agent's system
prompt (reuses the existing `contextFiles` mechanism on the Agent CRD).

### Planning template

When the agent sets `intent=plan`, the Issue body follows a planning structure
(Goal / Background / Proposed approach / Open questions / Out of scope).
This makes "let's discuss this with the architect later" a first-class
artifact: the Issue is the discussion thread.

## Rollout plan

1. **PR 1 — agentops-core**
   - Add `AgentRunIntent`, `AgentRunOutcomeSpec`, `AgentRunArtifact`,
     `AgentRunOutcomeStatus`
   - Drop `status.pullRequestURL`, `status.branch`, `status.commits` (or
     fold them into an artifact of kind `commit`)
   - `op-reload-full` to regenerate and install CRDs
   - Operator grants `agentruns/status patch` RBAC to the runtime SA

2. **PR 2 — agentops-runtime**
   - Add `run_finish` built-in tool that patches status
   - Add system-prompt addendum for agents with `spec.git`
   - Ship RCA + planning templates as standard context files

3. **PR 3 — agentops-console**
   - Replace `pullRequestURL` reads with `status.outcome.artifacts[0]`
     (filtered by kind=pr/mr) and run-level `intent`
   - Run sidebar card: Intent chip (color by intent) + primary artifact link
   - Delegation-result card in daemon chat: link to child Run's outcome

4. **Phase 2 (separate plan) — DelegationGraph view in console**
   - Tree/DAG of run_agent calls within a parent run
   - Each node shows agent name + intent chip + primary artifact + duration
   - Drill-down to child run detail

## Open Questions

None outstanding. Resolved in v3:
- **Q1** Intent is immutable after run completion; artifacts are
  append-only until phase is terminal.
- **Q2** Caller hints intent via `spec.outcome.intent`; executing agent
  finalizes via `status.outcome.intent`. Status wins.
- **Q3** No Scribe — dropped in v2.
- **Q4** `status.pullRequestURL` is dropped directly, no shim.
- **Q5** Artifact subkind not needed — run-level `intent` disambiguates
  Issue purpose (plan vs incident).
