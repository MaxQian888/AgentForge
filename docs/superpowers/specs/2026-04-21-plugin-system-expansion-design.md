# Plugin System Expansion Design

**Date:** 2026-04-21  
**Scope:** Integration Plugin expansion (CI/CD + notifications) + Workflow mode completion (Hierarchical / Event-Driven) + ToolChain primitive  
**Approach:** Method B — Common Adapter Framework, strengthening existing capabilities without infrastructure refactor

---

## 1. Background & Goals

AgentForge's plugin system currently supports 5 plugin types with a solid foundation (Go orchestrator as state authority, TS Bridge as stateless executor, MCP for tools, wazero sandbox for WASM). This spec addresses three identified gaps:

1. **Integration Plugin gap** — No CI/CD event receivers or notification senders exist; IM platform support is already covered by the dedicated IM Bridge service and must not be duplicated
2. **Workflow mode gap** — Only Sequential mode is live; Hierarchical and Event-Driven are deferred
3. **Tool composition gap** — Tools are invoked in isolation; no declarative pipeline/chain mechanism

**Non-goals:** Event bus infrastructure upgrade (Redis Streams), multi-language WASM (Extism), plugin marketplace UI, security model changes.

---

## 2. Design Overview

Three coordinated additions, all within existing packages:

```
src-go/internal/model/plugin.go        ← add hierarchical/event-driven fields to WorkflowPluginSpec
src-go/internal/plugin/
  workflow_executor.go                 ← new: WorkflowExecutor interface + mode routing
  toolchain_executor.go                ← new: ToolChainExecutor
  toolchain_resolver.go                ← new: template variable resolver
src-go/internal/service/
  hierarchical_executor.go             ← new: HierarchicalExecutor
  event_driven_executor.go             ← new: EventDrivenExecutor (background subscriber)
src-go/cmd/
  github-actions-adapter/             ← new: GitHub webhook → EventBus WASM plugin
  generic-webhook-adapter/            ← new: configurable webhook → EventBus WASM plugin
  email-adapter/                      ← new: SMTP outbound WASM plugin
```

No changes to external APIs, TS Bridge protocol, DB schema, or IM Bridge.

**Key existing facts that constrain this design:**
- `WorkflowProcessMode` and `WorkflowPluginSpec.Process` already exist — do not re-define them
- **IM Bridge (`src-im-bridge/`) already handles all 8 IM platforms** (Feishu, Slack, DingTalk, Discord, Telegram, WeChat, QQ, QQ Bot) — IM adapters must NOT be duplicated as WASM plugins
- MCP tool invocation available as `PluginService.CallMCPTool()` Go function — use directly, not via HTTP
- Trigger Engine = one-time dispatch; EventDrivenExecutor = persistent subscriber — these are distinct, non-overlapping

---

## 3. Integration Plugin Expansion

### 3.1 Scope Boundary (Critical)

The **IM Bridge** (`src-im-bridge/`, port 7779) is a production Go service that already owns the full inbound/outbound lifecycle for all 8 IM platforms (Feishu, Slack, DingTalk, Discord, Telegram, WeChat, QQ, QQ Bot). It handles: transport (webhook/polling/WebSocket), signature validation, credential management, message normalisation, per-platform rendering, durable delivery guarantees, and multi-tenant isolation.

**Integration Plugins must NOT duplicate IM Bridge responsibilities.** Their correct scope is:

| Use case | Correct component |
|----------|-------------------|
| Receive chat messages from users via IM | IM Bridge — already done |
| Send IM replies/notifications to users | IM Bridge — already done |
| Receive CI/CD events from GitHub/GitLab | **Integration Plugin** |
| Accept arbitrary HTTP webhooks | **Integration Plugin** |
| Send email notifications (SMTP) | **Integration Plugin** |
| Fan-out one notification to multiple channels | **Integration Plugin** |

### 3.2 New Integration Plugins

**Phase 1 — CI/CD event receivers**

| Plugin | Runtime | Inbound event | EventBus output |
|--------|---------|--------------|----------------|
| `github-actions-adapter` | Go WASM | GitHub webhook (push, PR, release, workflow_run) | `vcs.push`, `vcs.pull_request.opened`, `vcs.release.published`, `vcs.workflow_run.completed` |
| `generic-webhook-adapter` | Go WASM | Any HTTP POST | `integration.webhook.received` with configurable payload mapping |

**Phase 2 — Notification senders**

| Plugin | Runtime | Purpose |
|--------|---------|---------|
| `email-adapter` | Go WASM | SMTP outbound: send email notifications triggered by EventBus events |
| `notification-fanout` | firstparty-inproc | Route one notification to multiple delivery channels (IM Bridge + email) by rule |

### 3.3 Standard Integration Plugin Manifest

No new model fields are needed. Existing `PluginSpec.Capabilities` and `PluginSpec.ConfigSchema` are sufficient. The `feishu-adapter` WASM stub is **not modified** — its existing `send_message` capability is already a thin notification sender, and its inbound path belongs to the IM Bridge, not this plugin.

```yaml
kind: IntegrationPlugin
spec:
  runtime: wasm
  module: ./dist/github-actions.wasm
  abiVersion: v1
  capabilities:
    - health
    - handle_webhook    # receive + validate a GitHub webhook HTTP delivery
  configSchema:
    type: object
    required: [webhook_secret]
    properties:
      webhook_secret:
        type: string
        description: HMAC-SHA256 secret for GitHub webhook signature verification
permissions:
  network:
    required: false   # webhook receiver does not call external APIs
```

### 3.4 Inbound Webhook Data Flow

```
GitHub POST /api/v1/integrations/github-actions-adapter/webhook
    ↓
Go Orchestrator: reads raw body + X-Hub-Signature-256 header
    ↓
WASM plugin Invoke("handle_webhook", {headers, body})
    → validates HMAC-SHA256 signature
    → maps GitHub event type to AgentForge event type
    → returns {event_type: "vcs.pull_request.opened", payload: {...}}
    ↓
Go Orchestrator publishes to EventBus
    ↓
Trigger Engine / EventDrivenExecutor → Workflow / Agent execution
```

### 3.5 Notification Fanout Data Flow

`notification-fanout` is a firstparty-inproc plugin that reads project-level `notification_rules` config. When triggered, it:

1. Evaluates which channels are configured for the event (e.g., `on: vcs.pull_request.opened → [slack:#dev, email:team@co]`)
2. For IM channels: calls `POST /api/v1/im/notify` on the Go Orchestrator, which routes through the IM Bridge
3. For email: calls `email-adapter.Invoke("send_email", ...)` directly

This keeps the IM Bridge as the single owner of IM delivery; `notification-fanout` is an orchestrator, not a transport layer.

---

## 4. WorkflowExecutor Framework

### 4.1 Existing Model (Important — Do Not Duplicate)

`WorkflowProcessMode` and `WorkflowPluginSpec.Process` **already exist** in `src-go/internal/model/plugin.go`:

```go
type WorkflowProcessMode string
const (
    WorkflowProcessSequential   WorkflowProcessMode = "sequential"
    WorkflowProcessHierarchical WorkflowProcessMode = "hierarchical"
    WorkflowProcessEventDriven  WorkflowProcessMode = "event-driven"
    WorkflowProcessWave         WorkflowProcessMode = "wave"
)

type WorkflowPluginSpec struct {
    Process  WorkflowProcessMode      `yaml:"process" json:"process"`
    Roles    []WorkflowRoleBinding    `yaml:"roles,omitempty" json:"roles,omitempty"`
    Steps    []WorkflowStepDefinition `yaml:"steps,omitempty" json:"steps,omitempty"`
    Triggers []PluginWorkflowTrigger  `yaml:"triggers,omitempty" json:"triggers,omitempty"`
    Limits   *WorkflowExecutionLimits `yaml:"limits,omitempty" json:"limits,omitempty"`
}
```

What does **not** exist: the executor interface and routing logic. The current `WorkflowExecutionService` has sequential logic inlined with no mode dispatch.

### 4.2 Interface

```go
// src-go/internal/plugin/workflow_executor.go
type WorkflowExecutor interface {
    Mode() WorkflowProcessMode
    Execute(ctx context.Context, plan WorkflowPlan, input WorkflowInput) (<-chan WorkflowEvent, error)
    Cancel(ctx context.Context, instanceID string) error
}
```

`WorkflowExecutionService` is extended to route by `spec.Workflow.Process` to the registered executor. `SequentialExecutor` is extracted from the existing inline logic (no behavior change).

### 4.3 New Manifest Fields for Hierarchical Mode

The following optional fields are added to `WorkflowPluginSpec` (backward compatible — existing `sequential` manifests omit them):

```go
type WorkflowPluginSpec struct {
    // existing fields ...
    ManagerRole         string              `yaml:"managerRole,omitempty" json:"managerRole,omitempty"`
    WorkerRoles         []string            `yaml:"workerRoles,omitempty" json:"workerRoles,omitempty"`
    MaxParallelWorkers  int                 `yaml:"maxParallelWorkers,omitempty" json:"maxParallelWorkers,omitempty"`
    WorkerFailurePolicy string              `yaml:"workerFailurePolicy,omitempty" json:"workerFailurePolicy,omitempty"`
    Aggregation         string              `yaml:"aggregation,omitempty" json:"aggregation,omitempty"`
}
```

**Manifest example:**
```yaml
kind: WorkflowPlugin
spec:
  workflow:
    process: hierarchical
    managerRole: project-assistant
    workerRoles: [coding-agent, test-engineer, doc-writer]
    maxParallelWorkers: 3
    workerFailurePolicy: best_effort   # fail_fast | best_effort
    aggregation: manager_summarize
```

**Execution flow:**
```
WorkflowInput
    ↓
Manager Role  →  decomposes into N SubTasks via task-control MCP tool
    ↓ dispatch (uses existing TaskDispatchService.Spawn(), AdmissionController)
Worker Roles  (parallel agent runs, capped by maxParallelWorkers)
    ↓ results (poll WorkflowRunStore until all complete)
Manager Role  →  aggregates, produces final WorkflowOutput
```

**Constraints:**
- Manager uses existing `task-control` MCP tool to create subtasks — no new dispatch channel
- Manager dispatches at most once (no recursive manager delegation)
- `fail_fast`: abort all workers on first failure; `best_effort`: collect all completed results
- Worker dispatch reuses existing `TaskDispatchService.Spawn()` and `AdmissionController` — no new queue primitives

### 4.4 Event-Driven Mode

**Boundary: Trigger Engine vs EventDrivenExecutor**

The existing `Trigger Engine` fires **one-time, on-demand runs** (manual, schedule, or external push). `EventDrivenExecutor` is a **persistent subscriber** that lives as a background goroutine for the duration the plugin is enabled. They share the same EventBus but serve different lifecycles and are not duplicates.

**New manifest fields added to `PluginWorkflowTrigger`:**

```go
type PluginWorkflowTrigger struct {
    // existing fields ...
    Filter       map[string]any `yaml:"filter,omitempty" json:"filter,omitempty"`
    Role         string         `yaml:"role,omitempty" json:"role,omitempty"`
    Action       string         `yaml:"action,omitempty" json:"action,omitempty"`
    MaxConcurrent int           `yaml:"maxConcurrent,omitempty" json:"maxConcurrent,omitempty"`
}
```

**Manifest example:**
```yaml
kind: WorkflowPlugin
spec:
  workflow:
    process: event-driven
    triggers:
      - event: integration.im.message_received
        filter:
          channel: "general"
          contains_mention: true
        role: project-assistant
        action: reply
        maxConcurrent: 2

      - event: vcs.pull_request.opened
        filter:
          base_branch: "main"
        role: code-reviewer
        action: review
        maxConcurrent: 1
```

**Execution flow:**
```
Plugin enabled  →  EventDrivenExecutor subscribes to EventBus (as Mod or goroutine)
    ↓
EventBus receives event
    ↓
EventDrivenExecutor matches against this workflow's trigger list + filter
    ↓
If matched and under maxConcurrent cap:
    Creates agent run (role + action + event payload as input)
    ↓
Plugin disabled  →  EventDrivenExecutor unsubscribes / goroutine cancelled
```

**Constraints:**
- EventDrivenExecutor does NOT go through the Trigger Engine — it subscribes to EventBus directly
- `maxConcurrent` per trigger enforced via a per-trigger semaphore
- Persistent lifecycle: plugin enable/disable controls subscription; no DB schema change required (no new workflow run row needed per subscription)

---

## 5. ToolChain Primitive

### 5.1 Concept

ToolChain is a new Workflow Step `action` type that declaratively chains MCP tool calls within a single step. Each tool's output is available as a template variable for subsequent tools.

### 5.2 Manifest Syntax

```yaml
steps:
  - id: "research_and_store"
    role: "coding-agent"
    action: "tool_chain"
    tool_chain:
      steps:
        - tool: "web-search"
          input:
            query: "{{workflow.input.topic}}"
          output_as: "search_results"

        - tool: "github-tool"
          input:
            query: "{{steps.search_results.top_result}}"
          output_as: "github_data"

        - tool: "db-query"
          input:
            sql: "INSERT INTO findings VALUES ('{{steps.github_data.repo}}')"

      on_error: stop     # stop | skip | retry(n)
    next: ["summarize"]
```

### 5.3 Template Variable Resolution

Resolved in Go before each tool call. Variable prefix determines source:

| Prefix | Source |
|--------|--------|
| `workflow.input.*` | Workflow's initial input |
| `steps.<id>.*` | Output of a prior ToolChain step |
| `env.*` | Project Secrets (read-only injection) |

The resolver (`toolchain_resolver.go`) runs server-side only and does not expose template evaluation to WASM plugins, preventing template injection.

### 5.4 Execution Flow

```
WorkflowEngine encounters action: tool_chain
    ↓
ToolChainExecutor.Execute(steps)
    ├─→ Step 1: invoke web-search  →  store as "search_results"
    ├─→ Step 2: resolve template, invoke github-tool  →  store as "github_data"
    └─→ Step 3: resolve template, invoke db-query
    ↓
Return: final step output + all intermediate step summaries
    ↓
WorkflowEngine continues to next Workflow Step
```

### 5.5 Error Handling

| `on_error` | Behavior |
|------------|----------|
| `stop` (default) | Terminate chain immediately, return partial results from completed steps |
| `skip` | Skip failed step (output_as set to null), continue subsequent steps |
| `retry(n)` | Retry up to n times, then apply `stop` |

### 5.6 Reuse of Existing Infrastructure

ToolChainExecutor calls `PluginService.CallMCPTool(ctx, pluginID, toolName, args)` **directly as a Go function** — not via the HTTP endpoint. The HTTP handler `POST /api/v1/plugins/:id/mcp/tools/call` is the external API entry point; internally, both the HTTP handler and ToolChainExecutor share the same service method. This avoids an HTTP round-trip and preserves the existing audit trail (the service method already logs interactions).

`env.*` template resolution calls `secrets.Service.GetPlaintext(ctx, projectID, name)` directly. The project ID is available from the workflow execution context.

### 5.7 Intentional Constraints (YAGNI)

- No conditional branching (`if`/`switch`) within a ToolChain — that is Workflow-level responsibility
- No parallel tool steps — tools often have side effects; serial execution is safer; parallelism is a future extension point
- ToolChain outputs are scoped to the current Step execution only; no cross-Workflow output sharing

---

## 6. Component Interaction Map

```
┌─────────────────────────────────────────────────────────┐
│  WorkflowEngine (router)                                │
│    ├─→ SequentialExecutor   (existing, extracted)       │
│    ├─→ HierarchicalExecutor (new)                       │
│    └─→ EventDrivenExecutor  (new, subscribes EventBus)  │
│              ↕                                          │
│    ToolChainExecutor (new, used by any executor)        │
│              ↕                                          │
│    MCP Tool Invocation (existing path)                  │
└─────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────┐
│  Integration Plugin Runtime (WASM)                      │
│    github-actions-adapter  → EventBus vcs.* events      │
│    generic-webhook-adapter → EventBus integration.*     │
│    email-adapter           → SMTP outbound               │
│    notification-fanout     → routes to IM Bridge + email │
│                               (firstparty-inproc)        │
└─────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────┐
│  IM Bridge (src-im-bridge/ — UNCHANGED)                 │
│  Owns all 8 IM platforms: Feishu, Slack, DingTalk,      │
│  Discord, Telegram, WeChat, QQ, QQ Bot                  │
│  notification-fanout routes IM delivery through here    │
└─────────────────────────────────────────────────────────┘
```

---

## 7. Implementation Order

1. **SequentialExecutor extraction** (prerequisite for WorkflowExecutor router)
2. **HierarchicalExecutor**
3. **EventDrivenExecutor**
4. **ToolChainExecutor** + template resolver
5. **github-actions-adapter** WASM plugin
6. **generic-webhook-adapter** WASM plugin
7. **email-adapter** WASM plugin
8. **notification-fanout** firstparty-inproc plugin

---

## 8. What This Does Not Change

- Go ↔ TS Bridge protocol (HTTP + WS heartbeat)
- Plugin lifecycle state machine
- MCP tool invocation path (ToolChainExecutor calls it as a Go function, not a replacement)
- EventBus implementation (stays in-memory; EventDrivenExecutor subscribes as a goroutine)
- DB schema
- Existing Sequential workflow manifests (fully backward compatible; `process: sequential` unchanged)
- Plugin permission/security model
- `WorkflowProcessMode` type (already defined; this spec adds executor implementations, not new enum values)
- **IM Bridge** (`src-im-bridge/`) — no changes; it remains the sole owner of all IM platform transport
- `feishu-adapter` WASM plugin — no changes; its existing `send_message` capability is sufficient

## 9. Audit Notes (2026-04-21)

Corrections applied after codebase audit:

1. **ChannelAdapter removed** — The original design proposed WASM plugins for Dingtalk/Slack/Discord. Audit found the IM Bridge (`src-im-bridge/`) already implements full inbound/outbound for all 8 IM platforms. Adding WASM adapters would duplicate transport, signature validation, credential management, and delivery guarantees. The ChannelAdapter interface and all IM WASM plugins have been removed from scope.
2. **`process_mode`** — Field already exists as `spec.workflow.process` (type `WorkflowProcessMode`); spec only adds executor implementations and new config fields
3. **ToolChain invocation** — Uses `PluginService.CallMCPTool()` Go function directly, not the HTTP endpoint
4. **EventDrivenExecutor boundary** — Does not go through Trigger Engine; subscribes to EventBus directly as a persistent goroutine
5. **Integration Plugin correct scope** — CI/CD event receivers and notification senders; IM platform adapters belong to IM Bridge
