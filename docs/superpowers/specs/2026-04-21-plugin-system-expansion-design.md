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
src-go/internal/plugin/
  channel_adapter.go       ← new: ChannelAdapter interface
  workflow_executor.go     ← new: WorkflowExecutor interface + mode routing
  toolchain_executor.go    ← new: ToolChainExecutor
  toolchain_resolver.go    ← new: template variable resolver
```

No changes to external APIs, DB schema, or TS Bridge protocol.

---

## 3. ChannelAdapter Framework

### 3.1 Interface

```go
type ChannelAdapter interface {
    Capabilities() ChannelCapabilities
    HandleInbound(ctx context.Context, event Event) (*InboundResult, error)
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

### 3.2 Manifest Extension

`IntegrationPlugin` manifests gain a `channel` section:

```yaml
kind: IntegrationPlugin
spec:
  channel:
    capabilities: [inbound, outbound, threading, rich_cards]
    inbound_events: [message.received, mention.received]
    outbound_formats: [text, markdown, card]
```

### 3.3 Data Flow

```
External message / webhook
    ↓
Integration Plugin WASM (ChannelAdapter.HandleInbound)
    ↓
Go EventBus  →  integration.im.message_received (etc.)
    ↓
Trigger Engine (rule matching)
    ↓
Workflow / Agent execution
    ↓
ChannelAdapter.SendOutbound
```

### 3.4 Feishu Refactor

Feishu's existing WASM module is refactored to implement `ChannelAdapter`. The public manifest and plugin ID stay unchanged; the Go-side runtime wraps it via the new interface. This validates the interface before new adapters are built.

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

### 4.1 Interface

```go
type WorkflowExecutor interface {
    Mode() WorkflowMode
    Execute(ctx context.Context, plan WorkflowPlan, input WorkflowInput) (<-chan WorkflowEvent, error)
    Cancel(ctx context.Context, instanceID string) error
}

type WorkflowMode string
const (
    ModeSequential   WorkflowMode = "sequential"   // existing
    ModeHierarchical WorkflowMode = "hierarchical" // new
    ModeEventDriven  WorkflowMode = "event_driven" // new
)
```

WorkflowEngine becomes a router: it reads `process_mode` from the manifest and delegates to the registered executor. SequentialExecutor is extracted from existing inline logic.

### 4.2 Hierarchical Mode

Manager Role decomposes the task, dispatches to Worker Roles in parallel or serial, then aggregates results.

**Manifest:**
```yaml
process_mode: hierarchical
spec:
  manager_role: project-assistant
  worker_roles: [coding-agent, test-engineer, doc-writer]
  max_parallel_workers: 3
  worker_failure_policy: best_effort   # fail_fast | best_effort
  aggregation: manager_summarize
```

**Execution flow:**
```
WorkflowInput
    ↓
Manager Role  →  decomposes into N SubTasks via task-control MCP tool
    ↓ dispatch
Worker Roles  (parallel agent runs, max_parallel_workers cap)
    ↓ results
Manager Role  →  aggregates, produces final WorkflowOutput
```

**Constraints:**
- Manager uses existing `task-control` MCP tool to create subtasks — no new channel
- Manager dispatches at most once (no recursive manager delegation)
- `fail_fast`: abort all workers on first failure; `best_effort`: collect all completed results

### 4.3 Event-Driven Mode

Workflow declares triggers; when matching events arrive, the assigned Role responds. The workflow lifecycle is persistent (active as long as the plugin is enabled).

**Manifest:**
```yaml
process_mode: event_driven
spec:
  triggers:
    - event: integration.im.message_received
      filter:
        channel: "general"
        contains_mention: true
      role: project-assistant
      action: reply
      max_concurrent: 2

    - event: vcs.pull_request.opened
      filter:
        base_branch: "main"
      role: code-reviewer
      action: review
      max_concurrent: 1
```

**Execution flow:**
```
EventBus receives event
    ↓
Trigger Engine matches event_driven Workflow rules
    ↓
EventDrivenExecutor finds matching trigger
    ↓
Creates agent run (role + action + event payload as input)
    ↓
Result returned via ChannelAdapter.SendOutbound (if configured)
```

**Constraints:**
- Shares event matching logic with existing Trigger Engine — no duplicate implementation
- `max_concurrent` per trigger prevents event flooding
- Persistent lifecycle: enabled state = listening; disabled state = paused

### 4.4 Manifest `process_mode` Summary

```yaml
kind: WorkflowPlugin
spec:
  process_mode: hierarchical          # sequential | hierarchical | event_driven

  # hierarchical-only
  manager_role: project-assistant
  worker_roles: [coding-agent, test-engineer]
  worker_failure_policy: best_effort

  # event_driven-only
  triggers:
    - event: "..."
      filter: {}
      role: "..."
      action: "..."
      max_concurrent: 2
```

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

ToolChainExecutor calls the existing `POST /api/v1/plugins/:id/mcp/tools/call` internally — no new protocol. It is a Go-side orchestration loop over the existing tool invocation path.

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
- MCP tool invocation path
- EventBus implementation (stays in-memory)
- DB schema
- Existing Sequential workflow manifests (fully backward compatible)
- Plugin permission/security model
