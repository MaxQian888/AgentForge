---
title: "Track A: DAG Node Type Registry"
date: 2026-04-16
status: approved-design
roadmap: docs/superpowers/roadmap/2026-04-16-plugin-extensibility-roadmap.md
track: A
stage: pre-1.0 / internal / experimental
---

# Track A: DAG Node Type Registry

## 1. 目标

把 `dag_workflow_service.go` 的 `switch node.Type { ... }` 硬编码分发替换为**两层 NodeTypeRegistry**，使插件能在不修改核心代码的前提下贡献自定义 `node.type`。同步清理 Phase 5 遗留的老 team 策略和 TeamWorkflowAdapter。

**成功标准**（见 roadmap）：

1. `NodeTypeRegistry` 接口稳定（pre-1.0，允许破坏性更改）
2. 13 个内置节点类型全部迁移至 registry，测试通过
3. `strategy*.go` / `team_workflow_adapter.go` 全部删除，TeamService 重构完成
4. 至少一个 WASM 和一个 MCP 形态的示例自定义节点可端到端跑通

## 2. 非目标

- 不保证向后兼容：此阶段项目处内部测试，所有契约随时 breaking
- 不引入 Track B（function handler registry）；Track A 的 `function` 节点仍用现有内联实现
- 不引入 Track C（execution hook）；effect 产生时只做 capability 校验，无 hook 链
- 不引入 Track D1 的前端动态加载；`ConfigSchema()` 先产出，让 D1 后续消费
- 不做跨插件 RPC、pub/sub、shared state 等高级协作

## 3. 架构总览

```
┌─────────────────────────────────────────────────────────┐
│              DAGWorkflowService.executeNode              │
│  ┌───────────────────────────────────────────────────┐  │
│  │ 1. resolveNodeConfig (template vars)              │  │
│  │ 2. registry.Resolve(projectID, node.Type)         │  │
│  │ 3. handler.Execute(ctx, req) -> result + effects  │  │
│  │ 4. validateEffects vs. handler.Capabilities()     │  │
│  │ 5. applyEffects (park/fire/control)               │  │
│  │ 6. updateNodeExecution + storeResult              │  │
│  └───────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────┘
                           │
          ┌────────────────┴────────────────┐
          │                                 │
    ┌─────▼──────┐                   ┌──────▼──────┐
    │ GoNative   │                   │   WASM /    │
    │  Handler   │                   │   MCP       │
    │ (built-in) │                   │  Adapter    │
    └────────────┘                   └──────┬──────┘
                                            │
                           ┌────────────────┴───────────────┐
                           │                                │
                     ┌─────▼──────┐                   ┌─────▼──────┐
                     │ WASMRuntime│                   │ MCPClient  │
                     │ Manager    │                   │  Hub       │
                     │ (existing) │                   │ (existing) │
                     └────────────┘                   └────────────┘
```

Registry 两层：全局（内置 13 个，进程启动注册、immutable）+ 项目级（插件 active 时注入，deactivate 时移除）。

## 4. 核心契约

### 4.1 Handler 接口

```go
// Package: internal/workflow/nodetypes
// experimental: pre-1.0, may change without notice

type NodeTypeHandler interface {
    // Execute is called once per node execution.
    // Returning an error marks the node as failed; the error message is persisted.
    Execute(ctx context.Context, req *NodeExecRequest) (*NodeExecResult, error)

    // ConfigSchema returns a JSON Schema document describing the node's config shape.
    // Consumed by workflow editor (Track D1). Return nil if schema not provided.
    ConfigSchema() json.RawMessage

    // Capabilities returns the set of effect kinds this handler may emit.
    // Used for strict capability enforcement; emitting an undeclared effect
    // causes the node to fail and records a registry_capability_violation event.
    Capabilities() []EffectKind
}
```

### 4.2 Request / Result

```go
type NodeExecRequest struct {
    Execution  *model.WorkflowExecution  // read-only snapshot
    Node       *model.WorkflowNode        // read-only
    Config     map[string]any             // template-resolved (done by service)
    DataStore  map[string]any             // read-only snapshot; handlers MUST NOT mutate
    NodeExecID uuid.UUID
    ProjectID  uuid.UUID
}

type NodeExecResult struct {
    Result  json.RawMessage  // nil → void; non-nil → stored into DataStore under nodeID
    Effects []Effect
}
```

**Return semantics** (no explicit Mode field):

| Result   | Effects contain park-effect | Node state after Execute |
|----------|-----------------------------|--------------------------|
| non-nil  | no                          | **completed** (result stored) |
| nil      | no                          | **completed** (void — no DataStore write) |
| any      | yes (exactly one)           | **waiting** (park payload dispatched) |
| any      | yes (>1 park-effect)        | **failed** (registry rejects) |
| n/a      | Execute returns error       | **failed** |

### 4.3 Effect DSL (closed set)

```go
type EffectKind string

const (
    // Park-and-await (at most one per result; node enters `waiting`)
    EffectSpawnAgent         EffectKind = "spawn_agent"
    EffectRequestReview      EffectKind = "request_review"
    EffectWaitEvent          EffectKind = "wait_event"
    EffectInvokeSubWorkflow  EffectKind = "invoke_sub_workflow"

    // Fire-and-forget (any count, executed in order)
    EffectBroadcastEvent     EffectKind = "broadcast_event"
    EffectUpdateTaskStatus   EffectKind = "update_task_status"

    // Control-flow
    EffectResetNodes         EffectKind = "reset_nodes"
)

type Effect struct {
    Kind    EffectKind
    Payload json.RawMessage
}
```

**Per-effect payload schemas**:

| Kind | Payload |
|------|---------|
| `spawn_agent` | `{runtime, provider, model, roleId, memberId?, budgetUsd}` — parks; resumed by `DAGWorkflowService.HandleAgentRunCompletion` |
| `request_review` | `{prompt, context}` — parks; resumed by `ResolveHumanReview` |
| `wait_event` | `{eventType, matchKey?}` — parks; resumed by `HandleExternalEvent` |
| `invoke_sub_workflow` | `{workflowId, variables}` — parks; resumed by new `HandleSubWorkflowCompletion` (stub in Track A, wired in later) |
| `broadcast_event` | `{eventType, payload}` |
| `update_task_status` | `{targetStatus}` |
| `reset_nodes` | `{nodeIds[], counterKey?, counterValue?}` — deletes node executions for given IDs so AdvanceExecution re-picks them up; optional `counterKey/counterValue` lets the loop handler persist its iteration count through the EffectApplier instead of mutating DataStore (replaces the current `_loop_{nodeId}_count` in-place mutation) |

### 4.4 NodeTypeRegistry

```go
type NodeTypeRegistry struct {
    mu       sync.RWMutex
    global   map[string]NodeTypeEntry                  // name → entry (built-in)
    project  map[uuid.UUID]map[string]NodeTypeEntry    // projectID → {name → entry}
    reserved map[string]bool                           // set of 13 built-in names
    events   PluginEventSink                           // for audit writes
}

type NodeTypeEntry struct {
    Name           string
    Handler        NodeTypeHandler
    Source         EntrySource          // "builtin" | "plugin"
    PluginID       string               // empty for builtins
    PluginVersion  string               // empty for builtins
    DeclaredCaps   map[EffectKind]bool  // cached from Capabilities()
}

// Public API
func (r *NodeTypeRegistry) RegisterBuiltin(name string, h NodeTypeHandler) error
func (r *NodeTypeRegistry) RegisterPluginNode(projectID uuid.UUID, pluginID, pluginVersion, name string, h NodeTypeHandler) error
func (r *NodeTypeRegistry) UnregisterPlugin(projectID uuid.UUID, pluginID string) int // returns count removed
func (r *NodeTypeRegistry) Resolve(projectID uuid.UUID, name string) (NodeTypeEntry, error)
func (r *NodeTypeRegistry) ListForProject(projectID uuid.UUID) []NodeTypeEntry
```

**Rules**:

- `RegisterBuiltin` is callable only during bootstrap (enforced by a `lockedGlobal bool` flag set after bootstrap completes; post-lock calls return error).
- `RegisterPluginNode` validates:
  1. `name` MUST contain exactly one `/` separator
  2. Prefix before `/` MUST equal `pluginID`
  3. Full name MUST NOT be in `reserved`
  4. Not already present in the same project scope
  5. All declared capabilities in `h.Capabilities()` MUST be a subset of the plugin's manifest capabilities (checked at plugin activation)
- `Resolve` order: project layer → global layer → `ErrNodeTypeNotFound`.
- Name format for Resolve:
  - Built-in: simple name, e.g., `"llm_agent"`
  - Plugin: `"{pluginID}/{name}"`, e.g., `"acme-invoice/submit"`

### 4.5 Built-in reserved names

```
trigger, condition, agent_dispatch, notification, status_transition,
gate, parallel_split, parallel_join, llm_agent, function, human_review,
wait_event, loop, sub_workflow
```

Note: `sub_workflow` is reserved even though not fully implemented in Track A (handler registered as a no-op that emits `invoke_sub_workflow` effect with stub payload — completes as void until downstream tracks wire it).

## 5. Plugin Kind: `NodeTypePlugin`

### 5.1 Manifest schema additions

```yaml
apiVersion: agentforge.io/v1
kind: NodeTypePlugin
metadata:
  id: acme-invoice
  name: Invoice Operations
  version: 0.1.0
spec:
  runtime: wasm                    # or mcp
  # For runtime=wasm:
  module: ./plugin.wasm
  abiVersion: v1
  # For runtime=mcp:
  command: ["node", "./server.js"]
  transport: stdio                 # or http
  # Universal:
  capabilities:
    - "effect:broadcast_event"
    - "effect:update_task_status"
  nodeTypes:
    - name: submit                 # auto-prefixed with pluginID → "acme-invoice/submit"
      description: Submit invoice to ACME ERP
      configSchema: ./schemas/submit.json
      declaredCapabilities:        # subset of top-level capabilities
        - "effect:broadcast_event"
    - name: refund
      description: Refund a submitted invoice
      configSchema: ./schemas/refund.json
```

**Validation** (enforced in both Go `internal/plugin/parser.go` and TS `src-bridge/src/plugins/schema.ts`):

- `kind == "NodeTypePlugin"` requires `runtime ∈ {wasm, mcp}`
- `spec.nodeTypes` must be non-empty
- Each entry's `declaredCapabilities` must be a subset of `spec.capabilities`
- `configSchema`, if present, must be a valid JSON Schema Draft 2020-12 document path
- For `runtime == mcp`, each entry's `name` must correspond 1:1 to an MCP tool exported by the plugin server (verified at activation)

### 5.2 Reserved kind-to-runtime mapping (updated table)

| Kind               | Allowed runtimes          | Source of truth                  |
|--------------------|---------------------------|----------------------------------|
| RolePlugin         | declarative               | unchanged                        |
| ToolPlugin         | mcp                       | unchanged                        |
| WorkflowPlugin     | wasm                      | unchanged                        |
| IntegrationPlugin  | wasm, go-plugin           | unchanged                        |
| ReviewPlugin       | mcp                       | unchanged                        |
| **NodeTypePlugin** | **wasm, mcp**             | **new — Track A**                |

Required schema updates:
- `src-go/internal/model/plugin.go`: add `PluginKindNodeType` constant; extend `isAllowedRuntime` to accept the new kind/runtime pair.
- `src-bridge/src/plugins/schema.ts`: add `"NodeTypePlugin"` to `PluginKindSchema` enum; extend kind-to-runtime enforcement block with `NodeTypePlugin → {wasm, mcp}`; add `nodeTypes[]` structure to `spec` schema.
- `src-go/internal/plugin/parser.go`: parse & validate `spec.nodeTypes[]`; enforce namespacing rules (§4.4).

## 6. Runtime Dispatch Protocols

### 6.1 WASM (runtime = wasm)

Reuses existing `invoke` operation of the Plugin SDK. No new operation.

**Invocation envelope** (sent via `AGENTFORGE_PAYLOAD` env):

```json
{
  "operation": "execute_node:{shortName}",
  "request": {
    "executionId": "uuid",
    "projectId": "uuid",
    "nodeId": "string",
    "nodeExecId": "uuid",
    "config": { ... },
    "dataStore": { ... }
  }
}
```

**Return envelope**:

```json
{
  "ok": true,
  "data": {
    "result": <json>,
    "effects": [ { "kind": "...", "payload": { ... } } ]
  }
}
```

Error envelope follows existing plugin SDK convention (`RuntimeError` with code/message/details). The error `code == "validation"` indicates a config-level failure (node permanently failed); any other code is treated as transient (still fails node in Track A; Track C may add retry).

**Plugin SDK (Go)** adds a helper:

```go
// plugin-sdk-go/nodetype.go
func RegisterNodeTypeHandler(name string, handler func(ctx *Context, req NodeExecRequest) (*NodeExecResult, error))
// Router inside Run() dispatches operation "execute_node:..." to registered handlers.
```

### 6.2 MCP (runtime = mcp)

Each declared `nodeTypes[].name` corresponds 1:1 to an MCP tool with the same short name.

**Tool contract**:

- `tool.name` = short name (e.g., `submit`)
- `tool.inputSchema` SHOULD be the union of `NodeExecRequest` shape + any node-specific config-derived schema
- `tool.outputSchema` SHOULD match `NodeExecResult` shape

At activation time, `ToolPluginManager` verifies each declared `nodeTypes[].name` is exposed as a tool via `refreshCapabilitySurface`. Mismatch → activate fails, lifecycle `degraded`.

Dispatch: `registry.Resolve(projectID, "acme-invoice/submit")` returns an `MCPNodeAdapter` whose `Execute` calls `mcpClientHub.callTool(pluginID, shortName, req)` and unmarshals the result.

**Plugin SDK (TS)** adds a helper:

```typescript
// plugin-sdk/nodetype.ts
export function defineNodeTypePlugin(definition: {
  manifest: PluginManifest
  nodeTypes: readonly NodeTypeDefinition[]
})

export interface NodeTypeDefinition {
  name: string
  description?: string
  configSchema?: ZodTypeAny
  declaredCapabilities: EffectKind[]
  execute: (req: NodeExecRequest) => Promise<NodeExecResult>
}
```

`defineNodeTypePlugin` auto-registers one MCP tool per node type, wiring input validation and output formatting.

### 6.3 Built-in (GoNativeHandler)

Built-in handlers are plain Go types implementing `NodeTypeHandler`. No cross-boundary dispatch.

## 7. Lifecycle Rules

| Event | Registry effect |
|-------|-----------------|
| Plugin install | no registry change; manifest stored in DB |
| Plugin activate | registry reads manifest `nodeTypes[]`, calls `RegisterPluginNode` for each. On any failure: rollback all entries for this plugin, mark lifecycle `degraded` |
| Plugin deactivate | `UnregisterPlugin(projectID, pluginID)` removes all its entries |
| Plugin uninstall | must deactivate first (existing contract); uninstall idempotent |
| Workflow save | for each `nodes[i].type`, call `Resolve(projectID, type)`; any failure → 400 with list of unresolved types |
| Node running when plugin deactivates | in-flight Execute call completes (handler reference was captured); subsequent node executions referencing the same type fail with `ErrNodeTypeNotFound` → workflow → failed |
| Async node parked when plugin deactivates | callback path (HandleAgentRunCompletion / ResolveHumanReview / HandleExternalEvent) does not reconsult registry; parked nodes resume normally |
| Plugin uninstall with active executions referencing it | install endpoint returns 409 with list of `executionId`s; `?force=true` cascades to cancel them |
| Go process restart | global layer re-registered via bootstrap; project layer replays from DB (`SELECT * FROM plugins WHERE kind='NodeTypePlugin' AND lifecycle_state='active'`) |

All mutations emit `plugin_events`:
- `registry_entry_added` (on successful RegisterPluginNode)
- `registry_entry_removed` (on UnregisterPlugin)
- `registry_rejected` (on validation failure during activate)
- `registry_capability_violation` (on undeclared effect emission at runtime)

## 8. Built-in Migration Plan

### 8.1 File layout

```
src-go/internal/workflow/
├── nodetypes/
│   ├── registry.go                 # NodeTypeRegistry struct, Resolve, Register*
│   ├── types.go                    # Handler/Request/Result/Effect types
│   ├── effects.go                  # EffectKind constants, payload structs, applyEffect
│   ├── trigger.go                  # (void no-op)
│   ├── condition.go
│   ├── llm_agent.go                # emits spawn_agent effect
│   ├── agent_dispatch.go           # alias for llm_agent (share impl)
│   ├── function.go                 # sync compute
│   ├── notification.go             # emits broadcast_event effect
│   ├── status_transition.go        # emits update_task_status effect
│   ├── human_review.go             # emits request_review effect
│   ├── wait_event.go               # emits wait_event effect
│   ├── loop.go                     # emits reset_nodes effect
│   ├── gate.go                     # void no-op
│   ├── parallel_split.go           # void no-op
│   ├── parallel_join.go            # void no-op
│   ├── sub_workflow.go             # emits invoke_sub_workflow (stub body in Track A)
│   ├── bootstrap.go                # RegisterBuiltins(r *Registry) entrypoint
│   └── *_test.go                   # one test file per handler
└── adapters/
    ├── wasm_adapter.go             # WASM NodeTypeHandler wrapping WASMRuntimeManager
    └── mcp_adapter.go              # MCP NodeTypeHandler wrapping MCPClientHub (via bridge call)
```

### 8.2 DAGWorkflowService refactor

- `executeNode` becomes:
  ```go
  entry, err := s.registry.Resolve(exec.ProjectID, node.Type)
  if err != nil { /* fail node */ }
  req := &NodeExecRequest{ ... s.resolveNodeConfig(node, dataStore) ... }
  result, err := entry.Handler.Execute(ctx, req)
  if err != nil { /* fail node */ }
  // Validate effects against entry.DeclaredCaps
  // Apply effects (park/fire/control)
  // Persist result + advance or wait
  ```
- Service fields `agentSpawner`, `reviewRepo`, `mappingRepo`, `taskRepo`, `hub` are retained but consumed by the **EffectApplier**, not handlers.
- `HandleAgentRunCompletion`, `ResolveHumanReview`, `HandleExternalEvent`: unchanged externally; internally they write node state + advance, as today.
- `executeLLMAgent`, `executeFunction`, `executeHumanReview`, etc., methods on DAGWorkflowService are deleted.

### 8.3 EffectApplier

```go
type EffectApplier struct {
    agentSpawner DAGWorkflowAgentSpawner
    mappingRepo  DAGWorkflowRunMappingRepo
    reviewRepo   DAGWorkflowReviewRepo
    taskRepo     DAGWorkflowTaskRepo
    nodeRepo     DAGWorkflowNodeExecRepo
    execRepo     DAGWorkflowExecutionRepo
    hub          *ws.Hub
}

func (a *EffectApplier) Apply(ctx context.Context, exec *model.WorkflowExecution, nodeExecID uuid.UUID, node *model.WorkflowNode, effects []Effect) (parked bool, err error)
```

Returns `parked=true` iff exactly one park-effect was applied; caller skips the "mark completed" step.

## 9. Cleanup: Old Team Strategies

### 9.1 Files to delete

- `src-go/internal/service/strategy.go`
- `src-go/internal/service/strategy_plan_code_review.go`
- `src-go/internal/service/strategy_pipeline.go`
- `src-go/internal/service/strategy_swarm.go`
- `src-go/internal/service/strategy_wave_based.go`
- `src-go/internal/service/team_workflow_adapter.go`
- Strategy-related test cases in `team_service_test.go`

### 9.2 `TeamService` refactor

Delete:
- `useWorkflowEngine` field
- `SetWorkflowAdapter` method
- `workflowAdapter` field
- `resolveStrategy` method

Modify:
- `StartTeam`: remove the `if s.useWorkflowEngine && s.workflowAdapter != nil` branch and the strategy-fallback path. Always call `WorkflowTemplateService.CreateFromTemplate` using a new helper `mapStrategyToTemplateName(strategy string) string` (logic inlined from deleted adapter, or promoted into `WorkflowTemplateService`).
- `ProcessRunCompletion`: remove `strategy.HandleRunCompletion` call entirely. Run completion routing is already handled by `DAGWorkflowService.HandleAgentRunCompletion` (Phase 5 wiring).

Also delete (verified 2026-04-16 via grep: all callers live in deleted strategy files):
- `TeamService.spawnCodersForTasks`
- `TeamService.spawnCodersForTask`

Preserve:
- All other `TeamService` public methods (`CancelTeam`, `RetryTeam`, `GetSummary`, `ListByProject`, `ListSummaries`, `DeleteTeam`, `UpdateTeam`, `ListArtifacts`): untouched unless they transitively reference deleted code.

### 9.3 Initialization order (`cmd/server/main.go` or equivalent)

```go
registry := nodetypes.NewRegistry(pluginEventSink)
nodetypes.RegisterBuiltins(registry, effectApplier)   // 13 built-ins
registry.LockGlobal()                                  // prevent further built-in registration

// Replay active plugins
for _, p := range dbPlugins.ListActive(NodeTypePlugin) {
    if err := pluginRegistrar.RegisterNodeTypePlugin(ctx, registry, p); err != nil {
        log.Warnf("failed to replay node type plugin %s: %v", p.ID, err)
    }
}

dagSvc := service.NewDAGWorkflowService(..., registry, effectApplier)
```

## 10. Observability

- All mutations → one `plugin_events` row (event types: `registry_entry_added`, `registry_entry_removed`, `registry_rejected`, `registry_capability_violation`)
- `Resolve` miss → structured error `{code: "REGISTRY_MISS", nodeType, projectId}`
- Node execution metrics (duration, success/failure) continue through `workflow_node_execution` table; no new metric channel
- Logging: standard `log.WithFields` at INFO level for registry mutations; DEBUG for per-node resolve

## 11. Security / Capability Enforcement

### 11.1 Capability declaration

- Plugin-level capabilities are declared in manifest `spec.capabilities` (e.g., `"effect:broadcast_event"`)
- Per-node-type capabilities are declared in `spec.nodeTypes[].declaredCapabilities`, MUST be a subset of plugin-level
- Handler's runtime `Capabilities()` method MUST return the same set as the manifest (verified at registration)

### 11.2 Strict mode enforcement

At effect-application time, `EffectApplier.Apply` iterates effects. For each effect:

- Look up `DeclaredCaps` from registry entry
- If `effect.Kind ∉ DeclaredCaps`:
  - Record a `registry_capability_violation` event with `{pluginId, nodeType, effectKind}`
  - **Fail the node** with error code `CAPABILITY_VIOLATION`
  - Do NOT apply any remaining effects from this execution

### 11.3 Built-in handlers

Built-in handlers' `Capabilities()` must accurately declare every effect they may emit. Integration tests enforce this: each built-in is exercised across its conditional paths and the actual emitted effects are asserted to be ⊆ declared.

## 12. Testing Strategy

### 12.1 Unit tests (per handler)

- One `*_test.go` file per built-in handler
- Tests cover: happy path, config validation, declared-capability compliance, async-park correctness
- Table-driven for handlers with multiple code paths (loop, condition)

### 12.2 Registry tests

- `registry_test.go`: CRUD, concurrency (RWMutex correctness under parallel Register/Resolve), name collision, reserved name rejection, namespace validation, lock-global enforcement

### 12.3 Effect applier tests

- `effects_test.go`: each effect kind's apply behavior, park detection, multi-park rejection, capability violation path

### 12.4 Integration tests

Two reference plugins, both shipped in `plugins/examples/`:

1. **WASM example**: `plugins/examples/wasm-echo-node/` — single node type `echo/noop` returning result unchanged
2. **MCP example**: `plugins/examples/mcp-http-probe/` — single node type `probe/get` calling an external HTTP endpoint, returning status code

End-to-end test: install → activate → create workflow referencing the type → execute → assert result + plugin_events trail.

### 12.5 Regression tests

- All existing `dag_workflow_service_test.go` scenarios pass unchanged (behavior must be preserved)
- All existing `team_service_test.go` scenarios (non-strategy) pass
- A representative workflow from each system template (`TemplatePlanCodeReview`, `TemplatePipeline`, `TemplateSwarm`) runs end-to-end via the new registry path

## 13. Risks & Open Questions

### 13.1 Known risks

1. **`sub_workflow` stub**: registered as reserved but not functionally implemented in Track A. A workflow referencing `sub_workflow` will park indefinitely. Mitigation: the default handler returns an error at Execute time until a future track wires it.
2. **TeamService.spawnCodersForTasks callers**: if pruning reveals hidden usage outside the deleted strategies, retain and add a TODO with follow-up task.
3. **WASM `sub_workflow` callback**: `invoke_sub_workflow` effect payload design is provisional — may need adjustment when actually wired.
4. **Plugin manifest schema change**: TS `schema.ts` and Go `parser.go` must stay in lockstep. CI needs a cross-validation test.

### 13.2 Explicit out-of-scope (deferred to later tracks)

- Function node's handler discovery → Track B
- Before/after-node hooks, retry semantics, circuit breaker → Track C
- Frontend dynamic loading of `configSchema` → Track D1
- Skill-backed node types → Track D2
- Scaffolding CLI templates for NodeTypePlugin → Track D3

## 14. Milestones / Acceptance

Track A is considered **done** when:

1. All files in §8.1 exist and compile (`go build ./...` green)
2. All files in §9.1 deleted (`git ls-files | grep strategy_` returns nothing in `src-go/`)
3. Unit tests pass: `cd src-go && go test ./internal/workflow/nodetypes/...`
4. Regression tests pass: `cd src-go && go test ./internal/service/...`
5. Both reference plugins (`wasm-echo-node`, `mcp-http-probe`) install → activate → execute successfully
6. `docs/guides/plugin-development.md` updated with NodeTypePlugin section
7. `MEMORY.md` entry updated: Track A done, contract snapshot committed
8. Roadmap table updated: row A columns all checked with commit SHAs
