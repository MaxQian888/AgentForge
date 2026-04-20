# Plugin System Expansion Design

**Date:** 2026-04-21  
**Scope:** Integration Plugin expansion (ChannelAdapter framework) + Workflow mode completion (Hierarchical / Event-Driven) + ToolChain primitive  
**Approach:** Method B — Common Adapter Framework, strengthening existing capabilities without infrastructure refactor

---

## 1. Background & Goals

AgentForge's plugin system currently supports 5 plugin types with a solid foundation (Go orchestrator as state authority, TS Bridge as stateless executor, MCP for tools, wazero sandbox for WASM). This spec addresses three identified gaps:

1. **Integration Plugin gap** — Only Feishu is implemented; no abstraction makes adding new channels repetitive
2. **Workflow mode gap** — Only Sequential mode is live; Hierarchical and Event-Driven are deferred
3. **Tool composition gap** — Tools are invoked in isolation; no declarative pipeline/chain mechanism

**Non-goals:** Event bus infrastructure upgrade (Redis Streams), multi-language WASM (Extism), plugin marketplace UI, security model changes.

---

## 2. Design Overview

Three coordinated additions, all within existing packages:

```
src-go/internal/model/plugin.go        ← add ChannelPluginSpec, hierarchical/event-driven fields
src-go/internal/plugin/
  channel_adapter.go                   ← new: Go-side ChannelAdapter wrapper interface
  workflow_executor.go                 ← new: WorkflowExecutor interface + mode routing
  toolchain_executor.go                ← new: ToolChainExecutor
  toolchain_resolver.go                ← new: template variable resolver
src-go/internal/service/
  hierarchical_executor.go             ← new: HierarchicalExecutor
  event_driven_executor.go             ← new: EventDrivenExecutor (background subscriber)
```

No changes to external APIs, TS Bridge protocol, or DB schema.

**Key existing facts that constrain this design:**
- `WorkflowProcessMode` and `WorkflowPluginSpec.Process` already exist — do not re-define them
- `IMMessageRequest.Platform` enum already includes all target IM platforms — new adapters must use matching platform identifiers
- MCP tool invocation available as `PluginService.CallMCPTool()` Go function — use directly, not via HTTP
- Trigger Engine = one-time dispatch; EventDrivenExecutor = persistent subscriber — these are distinct, non-overlapping

---

## 3. ChannelAdapter Framework

### 3.1 Interface

`ChannelAdapter` is a **Go-side wrapper interface** over the existing WASM operation-based invocation model. WASM plugins do not implement a Go interface directly; they declare capability names and handle them inside `Invoke()`. The Go runtime wraps each WASM plugin in a `ChannelAdapter` adapter that translates interface method calls to `Invoke` operations.

```go
// src-go/internal/plugin/channel_adapter.go
// Go-side abstraction over WASM operation routing
type ChannelAdapter interface {
    Capabilities() ChannelCapabilities
    HandleInbound(ctx context.Context, payload map[string]any) (*InboundResult, error)
    SendOutbound(ctx context.Context, msg OutboundMessage) (*OutboundResult, error)
    HealthCheck(ctx context.Context) error
}

type ChannelCapabilities struct {
    Inbound    bool
    Outbound   bool
    Threading  bool
    Reactions  bool
    FileAttach bool
    RichCards  bool
}
```

**WASM plugin side contract** — the plugin declares these operation names in its manifest `capabilities` list and routes them in `Invoke()`:

| Go interface method | WASM `Invocation.Operation` value |
|---------------------|------------------------------------|
| `HealthCheck()`     | `health`                           |
| `HandleInbound()`   | `handle_inbound`                   |
| `SendOutbound()`    | `send_outbound`                    |
| `Capabilities()`    | `capabilities` (optional, for self-description) |

The Go-side `WASMChannelAdapter` struct implements `ChannelAdapter` by calling `runtime.Invoke()` with the corresponding operation name.

### 3.2 Manifest Extension

`PluginSpec` in `src-go/internal/model/plugin.go` gains a formal `Channel *ChannelPluginSpec` field (not via the `Extra` catch-all):

```go
// src-go/internal/model/plugin.go
type ChannelPluginSpec struct {
    Capabilities  []string `yaml:"capabilities,omitempty" json:"capabilities,omitempty"`
    InboundEvents []string `yaml:"inboundEvents,omitempty" json:"inboundEvents,omitempty"`
    OutboundFmts  []string `yaml:"outboundFormats,omitempty" json:"outboundFormats,omitempty"`
    Platform      string   `yaml:"platform,omitempty" json:"platform,omitempty"`
}

// Added to PluginSpec:
Channel *ChannelPluginSpec `yaml:"channel,omitempty" json:"channel,omitempty"`
```

`Channel.Platform` must match an existing value from the `IMMessageRequest.Platform` enum (`feishu`, `dingtalk`, `slack`, `discord`, `wecom`, `email`, etc.) so that inbound events and outbound sends route correctly through the existing IM message model.

Example manifest:

```yaml
kind: IntegrationPlugin
spec:
  capabilities: [health, handle_inbound, send_outbound]
  channel:
    platform: slack
    capabilities: [inbound, outbound, threading, rich_cards]
    inboundEvents: [message.received, mention.received]
    outboundFormats: [text, markdown, card]
```

### 3.3 Data Flow

```
External message / webhook
    ↓
Integration Plugin WASM  (operation: handle_inbound)
via WASMChannelAdapter.HandleInbound()
    ↓
Go EventBus  →  integration.im.message_received  (domain.entity.action pattern)
    ↓
Trigger Engine (one-time triggers) OR EventDrivenExecutor (persistent listeners)
    ↓
Workflow / Agent execution
    ↓
WASMChannelAdapter.SendOutbound()  (operation: send_outbound)
→ maps to IMSendRequest using existing IM message model
```

### 3.4 Feishu Refactor

The Go-side runtime wraps the existing Feishu WASM plugin in a `WASMChannelAdapter`. The Feishu WASM itself gains a new `handle_inbound` capability alongside the existing `send_message` (kept for backward compatibility; `send_outbound` is the new canonical name). Manifest and plugin ID stay unchanged.

### 3.5 Integration Plugin Roadmap

**Phase 1 — IM**

| Plugin | Runtime | Inbound | Outbound | Notes |
|--------|---------|---------|----------|-------|
| `dingtalk-adapter` | Go WASM | ✅ | ✅ | Approval cards, group bot |
| `slack-adapter` | Go WASM | ✅ | ✅ | Block Kit, slash commands |
| `discord-adapter` | Go WASM | ✅ | ✅ | Embeds, slash commands, channel routing |

**Phase 2 — CI/CD**

| Plugin | Runtime | Trigger | Output |
|--------|---------|---------|--------|
| `github-actions-adapter` | Go WASM | Webhook (push/PR/release) | EventBus event |
| `generic-webhook-adapter` | Go WASM | HTTP endpoint | Configurable event mapping |

**Phase 3 — Notifications**

| Plugin | Runtime | Purpose |
|--------|---------|---------|
| `email-adapter` | Go WASM | SMTP send, template support |
| `notification-fanout` | firstparty-inproc | Broadcast one notification to multiple channels by rule |

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
│    └─→ EventDrivenExecutor  (new)                       │
│              ↕                                          │
│    ToolChainExecutor (new, used by any executor)        │
│              ↕                                          │
│    MCP Tool Invocation (existing path)                  │
└─────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────┐
│  IntegrationPlugin Runtime                              │
│    ChannelAdapter interface (new)                       │
│    ├─→ FeishuAdapter   (refactored)                     │
│    ├─→ DingtalkAdapter (new)                            │
│    ├─→ SlackAdapter    (new)                            │
│    ├─→ DiscordAdapter  (new)                            │
│    ├─→ GitHubActionsAdapter (new)                       │
│    ├─→ GenericWebhookAdapter (new)                      │
│    ├─→ EmailAdapter    (new)                            │
│    └─→ NotificationFanout (new, firstparty-inproc)      │
└─────────────────────────────────────────────────────────┘
```

---

## 7. Implementation Order

1. **ChannelAdapter interface** + Feishu refactor (validates interface)
2. **SequentialExecutor extraction** (prerequisite for WorkflowExecutor router)
3. **HierarchicalExecutor** 
4. **EventDrivenExecutor**
5. **ToolChainExecutor** + template resolver
6. **Dingtalk adapter** (first new integration, validates ChannelAdapter for IM)
7. **Slack adapter**
8. **Discord adapter**
9. **GitHub Actions adapter** + Generic webhook adapter
10. **Email adapter** + Notification fanout

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
- `IMMessageRequest.Platform` enum values (new adapters declare matching platform identifiers in manifest)

## 9. Audit Notes (2026-04-21)

Corrections applied after codebase audit:

1. **ChannelAdapter** — Go-side wrapper pattern over WASM Invoke, not a Go interface WASM plugins implement directly
2. **`process_mode`** — Field already exists as `spec.workflow.process` (type `WorkflowProcessMode`); spec only adds executor implementations and new config fields
3. **ToolChain invocation** — Uses `PluginService.CallMCPTool()` Go function directly, not the HTTP endpoint
4. **IM platform enum** — New adapters must declare `channel.platform` matching the existing `IMMessageRequest.Platform` enum values
5. **EventDrivenExecutor boundary** — Does not go through Trigger Engine; subscribes to EventBus directly as a persistent goroutine
6. **`channel` section** — Formalized as `Channel *ChannelPluginSpec` in `PluginSpec` struct, not via the `Extra` inline catch-all
