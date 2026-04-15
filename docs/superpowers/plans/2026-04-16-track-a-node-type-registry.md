# Track A: DAG Node Type Registry — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace hardcoded `switch node.Type` dispatch in `dag_workflow_service.go` with a two-layer `NodeTypeRegistry` (global built-ins + per-project plugin entries), introduce new `NodeTypePlugin` kind supporting WASM and MCP runtimes via an Effects DSL, and retire old team strategies.

**Architecture:** Handlers are pure-ish: they consume a read-only request (config, dataStore snapshot), return a result plus a closed set of structured effects (spawn_agent / request_review / wait_event / invoke_sub_workflow / broadcast_event / update_task_status / reset_nodes). An `EffectApplier` owned by the DAG service translates effects back into side effects against repos / spawner / hub. Registration is bootstrap-time for built-ins (lockable global layer) and activation-time for plugins (per-project layer). All work lands in a single PR; project is in pre-1.0 internal testing so breaking changes are free.

**Tech Stack:** Go 1.23 (src-go), Bun/TypeScript (src-bridge), wazero (WASM), MCP SDK, Zod (TS validation), testify + go test, Jest (TS tests).

**Spec:** `docs/superpowers/specs/2026-04-16-track-a-node-type-registry-design.md`

**Test commands** (run from repo root unless noted):
- Go tests: `cd src-go && go test ./...`
- Go type check: `cd src-go && go build ./...`
- TS tests: `pnpm --filter @agentforge/bridge test` (or equivalent — inspect `src-bridge/package.json`)
- Full lint: `pnpm lint`

**Commit convention:** `feat(nodetypes): ...`, `refactor(workflow): ...`, `chore(team): ...`, `test(...)`. One commit per task. Keep master green between tasks where the plan notes "green-compile invariant".

---

## Phase A — Scaffold the nodetypes package (no behavior changes)

### Task 1: Create nodetypes package skeleton with Request / Result / Effect types

**Files:**
- Create: `src-go/internal/workflow/nodetypes/types.go`
- Create: `src-go/internal/workflow/nodetypes/effects.go`
- Create: `src-go/internal/workflow/nodetypes/types_test.go`

**Green-compile invariant:** yes — new package, nothing calls it.

- [ ] **Step 1: Write the failing test** (`src-go/internal/workflow/nodetypes/types_test.go`)

```go
package nodetypes

import (
    "encoding/json"
    "testing"
)

func TestEffectKind_IsPark(t *testing.T) {
    cases := map[EffectKind]bool{
        EffectSpawnAgent:        true,
        EffectRequestReview:     true,
        EffectWaitEvent:         true,
        EffectInvokeSubWorkflow: true,
        EffectBroadcastEvent:    false,
        EffectUpdateTaskStatus:  false,
        EffectResetNodes:        false,
    }
    for k, want := range cases {
        if got := k.IsPark(); got != want {
            t.Errorf("EffectKind(%q).IsPark() = %v, want %v", k, got, want)
        }
    }
}

func TestNodeExecResult_ParkCount(t *testing.T) {
    r := &NodeExecResult{
        Effects: []Effect{
            {Kind: EffectBroadcastEvent, Payload: json.RawMessage(`{}`)},
            {Kind: EffectSpawnAgent, Payload: json.RawMessage(`{}`)},
        },
    }
    if got := r.ParkCount(); got != 1 {
        t.Errorf("ParkCount() = %d, want 1", got)
    }
}
```

- [ ] **Step 2: Run test, verify fail**

```
cd src-go && go test ./internal/workflow/nodetypes/... -run 'TestEffectKind_IsPark|TestNodeExecResult_ParkCount' -v
```

Expected: compilation errors / test missing.

- [ ] **Step 3: Implement `types.go`**

```go
// Package nodetypes provides the handler contract and registry for DAG workflow node types.
//
// experimental: pre-1.0, may change without notice
package nodetypes

import (
    "context"
    "encoding/json"

    "github.com/google/uuid"
    "github.com/react-go-quick-starter/server/internal/model"
)

// NodeTypeHandler is the contract every built-in and plugin-contributed node type must satisfy.
type NodeTypeHandler interface {
    Execute(ctx context.Context, req *NodeExecRequest) (*NodeExecResult, error)
    ConfigSchema() json.RawMessage // return nil if not provided
    Capabilities() []EffectKind    // exhaustive set of effect kinds this handler may emit
}

// NodeExecRequest is the read-only input passed to a handler.
type NodeExecRequest struct {
    Execution  *model.WorkflowExecution
    Node       *model.WorkflowNode
    Config     map[string]any // already template-resolved by caller
    DataStore  map[string]any // handlers MUST NOT mutate
    NodeExecID uuid.UUID
    ProjectID  uuid.UUID
}

// NodeExecResult is what a handler returns on success.
// See spec §4.2 for the full decision table.
type NodeExecResult struct {
    Result  json.RawMessage // nil → void; non-nil → written to DataStore under nodeID
    Effects []Effect
}

// ParkCount returns the number of park-and-await effects.
// A well-formed result has ParkCount() ∈ {0, 1}; >1 causes registry rejection.
func (r *NodeExecResult) ParkCount() int {
    n := 0
    for _, e := range r.Effects {
        if e.Kind.IsPark() {
            n++
        }
    }
    return n
}
```

- [ ] **Step 4: Implement `effects.go`**

```go
package nodetypes

import "encoding/json"

// EffectKind is the closed enumeration of effect types handlers may emit.
type EffectKind string

const (
    // Park-and-await: at most one per result; node enters `waiting`.
    EffectSpawnAgent        EffectKind = "spawn_agent"
    EffectRequestReview     EffectKind = "request_review"
    EffectWaitEvent         EffectKind = "wait_event"
    EffectInvokeSubWorkflow EffectKind = "invoke_sub_workflow"

    // Fire-and-forget: any count, executed in order.
    EffectBroadcastEvent   EffectKind = "broadcast_event"
    EffectUpdateTaskStatus EffectKind = "update_task_status"

    // Control-flow.
    EffectResetNodes EffectKind = "reset_nodes"
)

// IsPark reports whether the effect parks the node (node enters `waiting` state).
func (k EffectKind) IsPark() bool {
    switch k {
    case EffectSpawnAgent, EffectRequestReview, EffectWaitEvent, EffectInvokeSubWorkflow:
        return true
    default:
        return false
    }
}

// Effect is a single structured side-effect emitted by a handler.
type Effect struct {
    Kind    EffectKind
    Payload json.RawMessage
}

// Effect payload structs (marshal targets for EffectApplier).

type SpawnAgentPayload struct {
    Runtime   string  `json:"runtime"`
    Provider  string  `json:"provider"`
    Model     string  `json:"model"`
    RoleID    string  `json:"roleId"`
    MemberID  string  `json:"memberId,omitempty"`
    BudgetUsd float64 `json:"budgetUsd"`
}

type RequestReviewPayload struct {
    Prompt  string          `json:"prompt"`
    Context json.RawMessage `json:"context,omitempty"`
}

type WaitEventPayload struct {
    EventType string `json:"eventType"`
    MatchKey  string `json:"matchKey,omitempty"`
}

type InvokeSubWorkflowPayload struct {
    WorkflowID string          `json:"workflowId"`
    Variables  json.RawMessage `json:"variables,omitempty"`
}

type BroadcastEventPayload struct {
    EventType string         `json:"eventType"`
    Payload   map[string]any `json:"payload,omitempty"`
}

type UpdateTaskStatusPayload struct {
    TargetStatus string `json:"targetStatus"`
}

type ResetNodesPayload struct {
    NodeIDs      []string `json:"nodeIds"`
    CounterKey   string   `json:"counterKey,omitempty"`
    CounterValue float64  `json:"counterValue,omitempty"`
}
```

- [ ] **Step 5: Run test, verify pass**

```
cd src-go && go test ./internal/workflow/nodetypes/... -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```
rtk git add src-go/internal/workflow/nodetypes/
rtk git commit -m "feat(nodetypes): scaffold handler contract and effect DSL"
```

---

### Task 2: Implement NodeTypeRegistry with tests

**Files:**
- Create: `src-go/internal/workflow/nodetypes/registry.go`
- Create: `src-go/internal/workflow/nodetypes/registry_test.go`

**Green-compile invariant:** yes.

- [ ] **Step 1: Write failing tests** covering: register built-in, register plugin node with namespace, resolve order (project → global), reject reserved names, reject unnamespaced plugin entries, reject mismatched prefix, reject duplicate within same project, lock-global enforcement, unregister plugin removes all its entries.

```go
package nodetypes

import (
    "context"
    "encoding/json"
    "testing"

    "github.com/google/uuid"
)

type fakeHandler struct{ caps []EffectKind }

func (h *fakeHandler) Execute(ctx context.Context, req *NodeExecRequest) (*NodeExecResult, error) {
    return &NodeExecResult{}, nil
}
func (h *fakeHandler) ConfigSchema() json.RawMessage { return nil }
func (h *fakeHandler) Capabilities() []EffectKind    { return h.caps }

func newFake() *fakeHandler { return &fakeHandler{} }

func TestRegistry_RegisterBuiltin_ThenResolve(t *testing.T) {
    r := NewRegistry(nil)
    if err := r.RegisterBuiltin("llm_agent", newFake()); err != nil {
        t.Fatalf("RegisterBuiltin: %v", err)
    }
    got, err := r.Resolve(uuid.New(), "llm_agent")
    if err != nil {
        t.Fatalf("Resolve: %v", err)
    }
    if got.Name != "llm_agent" || got.Source != SourceBuiltin {
        t.Fatalf("got %+v", got)
    }
}

func TestRegistry_LockGlobal_PreventsFurtherBuiltinRegistration(t *testing.T) {
    r := NewRegistry(nil)
    _ = r.RegisterBuiltin("trigger", newFake())
    r.LockGlobal()
    if err := r.RegisterBuiltin("gate", newFake()); err == nil {
        t.Fatal("expected error after LockGlobal")
    }
}

func TestRegistry_RegisterPluginNode_RejectsReservedName(t *testing.T) {
    r := NewRegistry(nil)
    _ = r.RegisterBuiltin("llm_agent", newFake())
    r.LockGlobal()
    err := r.RegisterPluginNode(uuid.New(), "acme", "0.1", "acme/llm_agent", newFake())
    if err == nil {
        t.Fatal("expected error for reserved name collision via prefix, got nil")
    }
}

func TestRegistry_RegisterPluginNode_RequiresPluginIDPrefix(t *testing.T) {
    r := NewRegistry(nil)
    r.LockGlobal()
    projectID := uuid.New()

    // No slash at all
    if err := r.RegisterPluginNode(projectID, "acme", "0.1", "submit", newFake()); err == nil {
        t.Fatal("expected error for missing prefix")
    }
    // Wrong prefix
    if err := r.RegisterPluginNode(projectID, "acme", "0.1", "notacme/submit", newFake()); err == nil {
        t.Fatal("expected error for mismatched prefix")
    }
    // Happy path
    if err := r.RegisterPluginNode(projectID, "acme", "0.1", "acme/submit", newFake()); err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
}

func TestRegistry_ResolveOrder_ProjectBeforeGlobal(t *testing.T) {
    r := NewRegistry(nil)
    _ = r.RegisterBuiltin("probe", newFake())
    r.LockGlobal()
    projectID := uuid.New()

    // Built-in resolves for any project
    if _, err := r.Resolve(projectID, "probe"); err != nil {
        t.Fatalf("built-in should resolve: %v", err)
    }

    // Plugin name in project does not shadow built-in (they are different names due to namespacing)
    _ = r.RegisterPluginNode(projectID, "acme", "0.1", "acme/probe", newFake())
    got, _ := r.Resolve(projectID, "acme/probe")
    if got.Source != SourcePlugin {
        t.Fatalf("expected plugin entry, got %+v", got)
    }
}

func TestRegistry_UnregisterPlugin_RemovesAllEntries(t *testing.T) {
    r := NewRegistry(nil)
    r.LockGlobal()
    projectID := uuid.New()
    _ = r.RegisterPluginNode(projectID, "acme", "0.1", "acme/submit", newFake())
    _ = r.RegisterPluginNode(projectID, "acme", "0.1", "acme/refund", newFake())
    n := r.UnregisterPlugin(projectID, "acme")
    if n != 2 {
        t.Fatalf("expected 2 removals, got %d", n)
    }
    if _, err := r.Resolve(projectID, "acme/submit"); err == nil {
        t.Fatal("expected not found after unregister")
    }
}
```

- [ ] **Step 2: Run tests, verify fail**

```
cd src-go && go test ./internal/workflow/nodetypes/... -run TestRegistry -v
```

- [ ] **Step 3: Implement `registry.go`** (see spec §4.4 for struct shape; write full impl with sync.RWMutex, lockedGlobal flag, validation).

Key points:
- `NewRegistry(events PluginEventSink) *NodeTypeRegistry` — sink may be nil in tests
- `RegisterBuiltin`: error if `lockedGlobal`, error if name collision, records entry with `Source=SourceBuiltin` and adds to `reserved`
- `RegisterPluginNode`: validate single `/`, prefix match, not in reserved, not duplicated in project; emit `registry_entry_added` on success; emit `registry_rejected` on failure
- `Resolve`: RLock; check project map first, then global
- `UnregisterPlugin`: take Lock; iterate project map, remove all entries whose `PluginID == id`; return count; emit `registry_entry_removed` per removal
- `ListForProject(projectID)`: returns merged view (project entries + all built-ins), useful for workflow-save validation and debug endpoints
- `EntrySource` constants: `SourceBuiltin`, `SourcePlugin`
- `PluginEventSink` interface stub: `RecordEvent(ctx, eventType string, payload map[string]any) error`

- [ ] **Step 4: Run all registry tests, verify pass**

```
cd src-go && go test ./internal/workflow/nodetypes/... -v
```

- [ ] **Step 5: Commit**

```
rtk git add src-go/internal/workflow/nodetypes/registry.go src-go/internal/workflow/nodetypes/registry_test.go
rtk git commit -m "feat(nodetypes): implement two-layer node type registry"
```

---

### Task 3: Implement EffectApplier (fire-forget + control-flow effects)

**Files:**
- Create: `src-go/internal/workflow/nodetypes/applier.go`
- Create: `src-go/internal/workflow/nodetypes/applier_test.go`

**Green-compile invariant:** yes.

Applier depends on interfaces the DAG service already owns (agentSpawner, reviewRepo, hub etc.). We re-declare them locally in this package (duplicated minimal interfaces are fine — Go conventional style; keeps package import tree clean).

**Field naming**: the applier uses **exported** fields (`Hub`, `TaskRepo`, `NodeRepo`, ...) so tests can construct it as a composite literal with selected deps. This deviates from spec §8.3 (which showed unexported fields) but is the pragmatic choice — the applier is instantiated from `routes.go` with all its deps at once and never mutated after, so information-hiding buys nothing but test friction.

- [ ] **Step 1: Write failing tests** covering fire-forget path (broadcast_event, update_task_status) and reset_nodes. Park effects get a separate test in Task 4.

```go
package nodetypes

import (
    "context"
    "encoding/json"
    "testing"

    "github.com/google/uuid"
    "github.com/react-go-quick-starter/server/internal/model"
)

type fakeHub struct{ events []map[string]any }
func (h *fakeHub) BroadcastEvent(eventType, projectID string, payload map[string]any) {
    h.events = append(h.events, map[string]any{"type": eventType, "payload": payload})
}

type fakeTaskRepo struct{ status string }
func (t *fakeTaskRepo) TransitionStatus(ctx context.Context, id uuid.UUID, s string) error {
    t.status = s; return nil
}
func (t *fakeTaskRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Task, error) { return nil, nil }

type fakeNodeRepo struct{ deletedForIDs []string }
func (n *fakeNodeRepo) DeleteNodeExecutionsByNodeIDs(ctx context.Context, execID uuid.UUID, ids []string) error {
    n.deletedForIDs = append(n.deletedForIDs, ids...); return nil
}
// Stubs for the other methods of DAGWorkflowNodeExecRepo go here…

func TestApplier_BroadcastEvent(t *testing.T) {
    hub := &fakeHub{}
    a := &EffectApplier{Hub: hub}
    exec := &model.WorkflowExecution{ID: uuid.New(), ProjectID: uuid.New()}
    payload, _ := json.Marshal(BroadcastEventPayload{EventType: "workflow.demo", Payload: map[string]any{"hi": 1}})
    parked, err := a.Apply(context.Background(), exec, uuid.New(), &model.WorkflowNode{ID: "n1"}, []Effect{{Kind: EffectBroadcastEvent, Payload: payload}})
    if err != nil || parked {
        t.Fatalf("apply: parked=%v err=%v", parked, err)
    }
    if len(hub.events) != 1 {
        t.Fatalf("expected 1 event, got %d", len(hub.events))
    }
}

func TestApplier_UpdateTaskStatus(t *testing.T) {
    tr := &fakeTaskRepo{}
    a := &EffectApplier{TaskRepo: tr}
    taskID := uuid.New()
    exec := &model.WorkflowExecution{ID: uuid.New(), TaskID: &taskID}
    payload, _ := json.Marshal(UpdateTaskStatusPayload{TargetStatus: "done"})
    _, err := a.Apply(context.Background(), exec, uuid.New(), &model.WorkflowNode{}, []Effect{{Kind: EffectUpdateTaskStatus, Payload: payload}})
    if err != nil || tr.status != "done" {
        t.Fatalf("apply: err=%v status=%q", err, tr.status)
    }
}

func TestApplier_ResetNodes(t *testing.T) {
    nr := &fakeNodeRepo{}
    a := &EffectApplier{NodeRepo: nr}
    exec := &model.WorkflowExecution{ID: uuid.New()}
    payload, _ := json.Marshal(ResetNodesPayload{NodeIDs: []string{"a", "b"}})
    _, err := a.Apply(context.Background(), exec, uuid.New(), &model.WorkflowNode{}, []Effect{{Kind: EffectResetNodes, Payload: payload}})
    if err != nil || len(nr.deletedForIDs) != 2 {
        t.Fatalf("apply: err=%v deleted=%v", err, nr.deletedForIDs)
    }
}
```

- [ ] **Step 2: Run tests, verify fail**

```
cd src-go && go test ./internal/workflow/nodetypes/... -run TestApplier -v
```

- [ ] **Step 3: Implement `applier.go`** with local interface declarations:

```go
package nodetypes

// Minimal interfaces the applier consumes. Any of these fields may be nil
// in test contexts; the applier checks for nil and records an error.

type BroadcastHub interface {
    BroadcastEvent(eventType, projectID string, payload map[string]any)
}
type TaskTransitioner interface {
    TransitionStatus(ctx context.Context, id uuid.UUID, newStatus string) error
}
type NodeExecDeleter interface {
    DeleteNodeExecutionsByNodeIDs(ctx context.Context, execID uuid.UUID, ids []string) error
}
type ExecutionDataStoreWriter interface {
    UpdateExecutionDataStore(ctx context.Context, id uuid.UUID, dataStore json.RawMessage) error
    GetExecution(ctx context.Context, id uuid.UUID) (*model.WorkflowExecution, error)
}
// AgentSpawner, ReviewStarter, EventWaiter — see Task 4

type EffectApplier struct {
    Hub        BroadcastHub
    TaskRepo   TaskTransitioner
    NodeRepo   NodeExecDeleter
    ExecRepo   ExecutionDataStoreWriter
    // (park-effect deps populated in Task 4)
}

// Apply executes all effects in order. Returns parked=true iff exactly one
// park-effect was applied (caller must skip the "mark completed" step).
// Caller is responsible for ensuring at most one park effect is present
// (registry validates this before calling Apply).
func (a *EffectApplier) Apply(ctx context.Context, exec *model.WorkflowExecution, nodeExecID uuid.UUID, node *model.WorkflowNode, effects []Effect) (parked bool, err error) {
    for _, e := range effects {
        switch e.Kind {
        case EffectBroadcastEvent:
            // unmarshal, call hub
        case EffectUpdateTaskStatus:
            // unmarshal, call taskRepo
        case EffectResetNodes:
            // unmarshal, call nodeRepo; if CounterKey set, update DataStore
        case EffectSpawnAgent, EffectRequestReview, EffectWaitEvent, EffectInvokeSubWorkflow:
            // handled in Task 4 (currently stub with error)
        default:
            return parked, fmt.Errorf("unknown effect kind: %s", e.Kind)
        }
    }
    return parked, nil
}
```

Write full implementation for BroadcastEvent, UpdateTaskStatus, ResetNodes (the three above). Leave park effects as stubs that return `errors.New("park effect not yet implemented")` — they'll be fleshed out in Task 4.

- [ ] **Step 4: Run tests, verify pass**

- [ ] **Step 5: Commit**

```
rtk git add src-go/internal/workflow/nodetypes/applier.go src-go/internal/workflow/nodetypes/applier_test.go
rtk git commit -m "feat(nodetypes): implement effect applier (fire-forget + control-flow)"
```

---

### Task 4: EffectApplier — park effects (spawn_agent, request_review, wait_event, invoke_sub_workflow)

**Files:**
- Modify: `src-go/internal/workflow/nodetypes/applier.go`
- Modify: `src-go/internal/workflow/nodetypes/applier_test.go`

**Green-compile invariant:** yes.

- [ ] **Step 1: Write failing tests** for each park effect. Mock the AgentSpawner, ReviewStarter, and WaitStarter interfaces; assert the applier:
  - Calls the right dep with correct payload
  - Marks the node execution as `waiting` via the node repo (if a `waiting` API exists) OR returns `parked=true` without altering node state (let the DAG service flip status)
  - Returns `parked=true`

Decision for this plan: the applier **does not mutate node execution state**. It records the intent (by calling the spawner / review starter / event waiter for bookkeeping like `WorkflowRunMapping` creation) and returns `parked=true`. The DAG service sees `parked=true` and flips node status to `waiting` / `running` as appropriate and updates `WorkflowExecution.Status` to `paused` for reviews. This keeps state transitions centralized in the DAG service and matches current behavior (human_review is the only case that sets `status=paused`; applier signals it via `parked=true` + the event kind).

Test example (spawn_agent):

```go
type fakeSpawner struct{ run *model.AgentRun }
func (s *fakeSpawner) Spawn(ctx context.Context, taskID, memberID uuid.UUID, runtime, provider, model string, budget float64, roleID string) (*model.AgentRun, error) {
    s.run = &model.AgentRun{ID: uuid.New()}
    return s.run, nil
}

type fakeMappingRepo struct{ mapping *model.WorkflowRunMapping }
func (m *fakeMappingRepo) Create(ctx context.Context, mapping *model.WorkflowRunMapping) error {
    m.mapping = mapping; return nil
}

func TestApplier_SpawnAgent_CreatesRunAndMapping(t *testing.T) {
    sp := &fakeSpawner{}
    mr := &fakeMappingRepo{}
    a := &EffectApplier{AgentSpawner: sp, MappingRepo: mr}
    taskID := uuid.New()
    exec := &model.WorkflowExecution{ID: uuid.New(), TaskID: &taskID}
    payload, _ := json.Marshal(SpawnAgentPayload{Runtime: "claude_code", Provider: "anthropic", Model: "claude-sonnet-4-6", BudgetUsd: 5})
    parked, err := a.Apply(context.Background(), exec, uuid.New(), &model.WorkflowNode{ID: "n1"}, []Effect{{Kind: EffectSpawnAgent, Payload: payload}})
    if err != nil || !parked {
        t.Fatalf("parked=%v err=%v", parked, err)
    }
    if sp.run == nil || mr.mapping == nil || mr.mapping.AgentRunID != sp.run.ID {
        t.Fatalf("expected spawner + mapping recorded: run=%v mapping=%v", sp.run, mr.mapping)
    }
}
```

Similar tests for `request_review` (fakeReviewRepo.Create called), `wait_event` (no persistent store needed — just returns parked=true and emits a standard "waiting for event" log/broadcast), `invoke_sub_workflow` (Track A stub: returns `parked=true` but records a `TODO: sub_workflow not wired` log; spec §13.1 allows this).

- [ ] **Step 2-5**: implement + run + commit

```
rtk git commit -m "feat(nodetypes): implement effect applier park effects"
```

---

## Phase B — Built-in handlers (each task follows TDD, small commits)

For every handler task, the shape is identical:

1. Write a table-driven test in `<handler>_test.go` exercising all branches. Assert: `result.Effects` matches expected kinds+payloads; `result.Result` matches expected output; declared `Capabilities()` contains every emitted kind (test helper `assertCapsCoverEffects(t, h, result)`).
2. Run and verify the test fails.
3. Implement the handler. Each handler is a simple struct with no fields (all deps come from the request or emitted effects).
4. Run and verify pass.
5. Commit with message `feat(nodetypes): add <name> built-in handler`.

**Test helper** (write once in Task 5, reuse everywhere):

```go
func assertCapsCoverEffects(t *testing.T, h NodeTypeHandler, effects []Effect) {
    t.Helper()
    caps := make(map[EffectKind]bool)
    for _, k := range h.Capabilities() { caps[k] = true }
    for _, e := range effects {
        if !caps[e.Kind] {
            t.Fatalf("handler emitted undeclared capability: %s", e.Kind)
        }
    }
}
```

### Task 5: Structural no-op handlers (trigger, gate, parallel_split, parallel_join)

**Files:**
- Create: `trigger.go`, `gate.go`, `parallel_split.go`, `parallel_join.go` under `src-go/internal/workflow/nodetypes/`
- Create: one combined test file `structural_test.go`

Each handler: `Execute` returns `&NodeExecResult{}` (nil Result, no Effects). `ConfigSchema` returns nil. `Capabilities` returns `nil`.

Commit: `feat(nodetypes): add structural built-in handlers (trigger, gate, parallel_split, parallel_join)`

### Task 6: sub_workflow stub handler

**Files:**
- Create: `sub_workflow.go`, `sub_workflow_test.go`

**Decision (aligned with spec §13.1)**: the handler **returns an error at Execute time** — `"sub_workflow not implemented in Track A; slated for a future track"`. This fails the node fast instead of parking indefinitely, which is the behavior a workflow author needs during development.

`Capabilities()` still returns `[EffectInvokeSubWorkflow]` so the capability declaration is ready for the future track that wires actual sub-workflow invocation. `ConfigSchema()` returns the payload shape (`{workflowId, variables}`) so the future wiring has zero schema drift.

Tests:
- `Execute` returns a non-nil error with a recognizable message (`"sub_workflow not implemented"`)
- `Capabilities()` returns exactly `[EffectInvokeSubWorkflow]`
- `ConfigSchema()` returns a non-empty JSON schema

Commit: `feat(nodetypes): add sub_workflow stub handler (fail-fast until wired)`

### Task 7: function handler

**Files:**
- Create: `function.go`, `function_test.go`

Port logic from `dag_workflow_service.executeFunction` (lines 551-574): evaluate `config["expression"]` via existing `resolveTemplateVars` + `evaluateExpression`. Since `resolveTemplateVars` is currently package-private in service, either:
  - Move it to `internal/workflow/nodetypes/expr.go` as an exported helper (preferred — removes duplication), or
  - Keep it in service and pass through the applier (undesirable — handlers should be pure).

Choose option 1: extract `resolveTemplateVars`, `lookupPath`, `evaluateExpression` into `nodetypes/expr.go`. Service and handlers both use them. Write tests for the extracted helpers first.

Commit: `feat(nodetypes): extract expression helpers and add function handler`

### Task 8: condition handler

**Files:**
- Create: `condition.go`, `condition_test.go`
- Modify: `src-go/internal/workflow/nodetypes/expr.go` (extend, created in Task 7)

**Depends on Task 7**: Task 7 created `nodetypes/expr.go` with `resolveTemplateVars`, `lookupPath`, `evaluateExpression`. Task 8 adds `evaluateCondition` to the same file.

Port logic from `executeCondition` + `evaluateCondition` (service lines 616-625 + 910-958). Handler returns error if condition is false (current behavior — this fails the node and stops DAG advance on that branch).

Commit: `feat(nodetypes): add condition handler`

### Task 9: notification handler

**Files:**
- Create: `notification.go`, `notification_test.go`

Emits `EffectBroadcastEvent` with `{eventType: "workflow.notification", payload: {executionId, nodeId, message}}`. Declares `Capabilities() = [EffectBroadcastEvent]`.

Commit: `feat(nodetypes): add notification handler`

### Task 10: status_transition handler

**Files:**
- Create: `status_transition.go`, `status_transition_test.go`

Emits `EffectUpdateTaskStatus{targetStatus}`. Returns error if config missing `targetStatus`. Declares `Capabilities() = [EffectUpdateTaskStatus]`.

Commit: `feat(nodetypes): add status_transition handler`

### Task 11: loop handler

**Files:**
- Create: `loop.go`, `loop_test.go`

Port logic from `executeLoop` (service lines 628-703). Key change: instead of mutating `dataStore` directly for the counter, emit `EffectResetNodes{nodeIds, counterKey, counterValue}`. The applier updates DataStore via `ExecRepo.UpdateExecutionDataStore` — handler stays pure.

Also computes `nodesToReset` using a copy of `findNodesBetween` (extract helper into `nodetypes/topology.go`). Declared capabilities: `[EffectResetNodes]`.

Test cases:
- First iteration (counter absent): emits reset with `counterValue=1`
- Exit on max iterations (counter ≥ max): returns nil result, nil effects (ends loop)
- Exit on condition: returns nil result, nil effects
- Missing target_node in config: returns error

Commit: `feat(nodetypes): add loop handler with effect-based iteration counter`

### Task 12: llm_agent + agent_dispatch handlers

**Files:**
- Create: `llm_agent.go` — exports both `LLMAgentHandler` and `AgentDispatchHandler` (agent_dispatch is a thin alias type that embeds LLMAgentHandler)
- Create: `llm_agent_test.go`

**Deviation from spec §8.1 file layout, intentional**: spec listed `llm_agent.go` and `agent_dispatch.go` as separate files. Since `agent_dispatch` is literally an alias that delegates 100% to llm_agent logic (see `dag_workflow_service.go:432-434`), keeping them in one file removes ~40 lines of duplication and ~20 lines of near-identical test scaffolding. Both handler types are still exported and registered under their respective names in `bootstrap.go` (Task 15).

Port logic from `executeLLMAgent` (service lines 496-548). Emit `EffectSpawnAgent{runtime, provider, model, roleId, memberId, budgetUsd}`. Return error if `exec.TaskID == nil`. Declared capabilities: `[EffectSpawnAgent]`.

Commit: `feat(nodetypes): add llm_agent and agent_dispatch handlers`

### Task 13: human_review handler

**Files:**
- Create: `human_review.go`, `human_review_test.go`

Port logic from `executeHumanReview` (service lines 706-746). Handler emits `EffectRequestReview{prompt, context}`. The applier:
- Creates the `WorkflowPendingReview` row
- Broadcasts `ws.EventWorkflowReviewRequested`
- Signals `parked=true`

(The DAG service then marks node as `waiting` and execution as `paused` — same as before.)

Capabilities: `[EffectRequestReview]`.

Commit: `feat(nodetypes): add human_review handler`

### Task 14: wait_event handler

**Files:**
- Create: `wait_event.go`, `wait_event_test.go`

Emits `EffectWaitEvent{eventType, matchKey}`. Applier broadcasts `ws.EventWorkflowNodeWaiting` and returns `parked=true`. Capabilities: `[EffectWaitEvent]`.

Commit: `feat(nodetypes): add wait_event handler`

### Task 15: Bootstrap — RegisterBuiltins

**Files:**
- Create: `src-go/internal/workflow/nodetypes/bootstrap.go`
- Create: `src-go/internal/workflow/nodetypes/bootstrap_test.go`

- [ ] **Step 1: Test**

```go
func TestRegisterBuiltins_RegistersAll13(t *testing.T) {
    r := NewRegistry(nil)
    RegisterBuiltins(r)
    r.LockGlobal()

    want := []string{
        "trigger", "condition", "agent_dispatch", "notification",
        "status_transition", "gate", "parallel_split", "parallel_join",
        "llm_agent", "function", "human_review", "wait_event", "loop",
        "sub_workflow",
    }
    for _, name := range want {
        if _, err := r.Resolve(uuid.New(), name); err != nil {
            t.Errorf("built-in %q not registered: %v", name, err)
        }
    }
}
```

- [ ] **Step 2-4**: Implement `RegisterBuiltins(r *NodeTypeRegistry)` — one call to `r.RegisterBuiltin(name, handler)` per built-in.

- [ ] **Step 5: Commit**

```
rtk git commit -m "feat(nodetypes): bootstrap registers all 13 built-in handlers"
```

---

## Phase C — Switch over DAGWorkflowService to use the registry

### Task 16: Refactor `executeNode` to route through registry + applier

**Files:**
- Modify: `src-go/internal/service/dag_workflow_service.go`
- Modify: `src-go/internal/server/routes.go`
- Modify: `src-go/internal/service/dag_workflow_service_test.go`

**Green-compile invariant:** yes — behavior preserved, test suite should pass unchanged.

**High-risk task.** This is the big switch-over.

- [ ] **Step 1: Update DAGWorkflowService struct and constructor**

Add:
```go
type DAGWorkflowService struct {
    // … existing fields …
    registry *nodetypes.NodeTypeRegistry
    applier  *nodetypes.EffectApplier
}

func NewDAGWorkflowService(
    defRepo DAGWorkflowDefinitionRepo,
    execRepo DAGWorkflowExecutionRepo,
    nodeRepo DAGWorkflowNodeExecRepo,
    hub *ws.Hub,
    registry *nodetypes.NodeTypeRegistry,
    applier *nodetypes.EffectApplier,
) *DAGWorkflowService { … }
```

- [ ] **Step 2: Rewrite `executeNode` to route via registry** (replace the switch):

```go
func (s *DAGWorkflowService) executeNode(ctx context.Context, exec *model.WorkflowExecution, node *model.WorkflowNode, dataStore map[string]any) error {
    now := time.Now().UTC()
    nodeExec := &model.WorkflowNodeExecution{
        ID: uuid.New(), ExecutionID: exec.ID, NodeID: node.ID,
        Status: model.NodeExecRunning, StartedAt: &now,
    }
    if err := s.nodeRepo.CreateNodeExecution(ctx, nodeExec); err != nil {
        return fmt.Errorf("create node execution: %w", err)
    }

    entry, err := s.registry.Resolve(exec.ProjectID, node.Type)
    if err != nil {
        _ = s.nodeRepo.UpdateNodeExecution(ctx, nodeExec.ID, model.NodeExecFailed, nil, err.Error())
        return fmt.Errorf("resolve node type %q: %w", node.Type, err)
    }

    config := s.resolveNodeConfig(node, dataStore)
    dsCopy := cloneDataStore(dataStore) // handlers see a defensive copy

    req := &nodetypes.NodeExecRequest{
        Execution: exec, Node: node, Config: config, DataStore: dsCopy,
        NodeExecID: nodeExec.ID, ProjectID: exec.ProjectID,
    }

    result, execErr := entry.Handler.Execute(ctx, req)
    if execErr != nil {
        _ = s.nodeRepo.UpdateNodeExecution(ctx, nodeExec.ID, model.NodeExecFailed, nil, execErr.Error())
        return execErr
    }

    // Validate declared capabilities
    if bad := firstUndeclaredEffect(entry.DeclaredCaps, result.Effects); bad != nil {
        msg := fmt.Sprintf("handler emitted undeclared capability: %s", bad.Kind)
        _ = s.nodeRepo.UpdateNodeExecution(ctx, nodeExec.ID, model.NodeExecFailed, nil, msg)
        s.recordCapabilityViolation(ctx, entry, node, bad.Kind)
        return fmt.Errorf(msg)
    }

    // Reject multi-park
    if result.ParkCount() > 1 {
        _ = s.nodeRepo.UpdateNodeExecution(ctx, nodeExec.ID, model.NodeExecFailed, nil, "multiple park effects")
        return fmt.Errorf("handler returned %d park effects (max 1)", result.ParkCount())
    }

    parked, applyErr := s.applier.Apply(ctx, exec, nodeExec.ID, node, result.Effects)
    if applyErr != nil {
        _ = s.nodeRepo.UpdateNodeExecution(ctx, nodeExec.ID, model.NodeExecFailed, nil, applyErr.Error())
        return applyErr
    }

    if parked {
        // Flip waiting/paused state based on which park effect was emitted
        switch firstParkEffect(result.Effects).Kind {
        case nodetypes.EffectRequestReview:
            _ = s.nodeRepo.UpdateNodeExecution(ctx, nodeExec.ID, model.NodeExecWaiting, nil, "")
            _ = s.execRepo.UpdateExecution(ctx, exec.ID, model.WorkflowExecStatusPaused, nil, "")
        case nodetypes.EffectWaitEvent:
            _ = s.nodeRepo.UpdateNodeExecution(ctx, nodeExec.ID, model.NodeExecWaiting, nil, "")
        // EffectSpawnAgent, EffectInvokeSubWorkflow: node stays "running"; callback moves to completed
        }
        return nil
    }

    _ = s.nodeRepo.UpdateNodeExecution(ctx, nodeExec.ID, model.NodeExecCompleted, result.Result, "")
    if len(result.Result) > 0 {
        s.storeNodeResult(ctx, exec.ID, node.ID, result.Result)
    }
    s.broadcastEvent(ws.EventWorkflowNodeCompleted, exec.ProjectID.String(), map[string]any{
        "executionId": exec.ID.String(), "nodeId": node.ID, "nodeType": node.Type,
        "status": model.NodeExecCompleted,
    })
    return nil
}
```

- [ ] **Step 3: Delete the old `execute*` methods**:
  - `executeLLMAgent`, `executeFunction`, `executeNotification`, `executeStatusTransition`, `executeCondition`, `executeLoop`, `executeHumanReview`, `executeWaitEvent`
  - Keep `HandleAgentRunCompletion`, `ResolveHumanReview`, `HandleExternalEvent` — callbacks are unchanged
  - Keep `resolveNodeConfig`, `storeNodeResult`, `broadcastEvent`
  - Remove `findNodesBetween` if the nodetypes package now owns it (added in Task 11 / Task 8 extraction)

- [ ] **Step 4: Wire the registry + applier in `routes.go`** (around line 307)

**Before starting, read `src-go/internal/server/routes.go:295-340`** to confirm: `agentSvc` is declared before line 308 (used at 312-314); `dagRunMappingRepo` is declared at 310; `wfReviewRepo` is declared at 320 — it must be **hoisted above** the applier constructor. `taskRepo.DB()` is the DB handle.

If `wfReviewRepo` has other callers between its current declaration and its current use, hoist carefully — test after each move.

```go
// Build registry
nodetypeReg := nodetypes.NewRegistry(pluginEventSink) // event sink from plugin service
nodetypes.RegisterBuiltins(nodetypeReg)
nodetypeReg.LockGlobal()

// Build applier
effectApplier := &nodetypes.EffectApplier{
    Hub:          hub,
    TaskRepo:     taskRepo,
    NodeRepo:     dagNodeExecRepo,
    ExecRepo:     dagExecRepo,
    AgentSpawner: agentSvc,        // agentSvc implements nodetypes.AgentSpawner
    MappingRepo:  dagRunMappingRepo,
    ReviewRepo:   wfReviewRepo,    // populated below; pass pointer
}

dagWorkflowSvc := service.NewDAGWorkflowService(dagDefRepo, dagExecRepo, dagNodeExecRepo, hub, nodetypeReg, effectApplier)
```

Note: `wfReviewRepo` is declared after the service today (line 320). Reorder so applier gets the final value, or wire it via a `SetReviewRepo` on the applier. Cleanest: hoist `wfReviewRepo` declaration above the applier constructor.

- [ ] **Step 5: Run full regression suite**

```
cd src-go && go test ./internal/service/... -v
cd src-go && go test ./... -count=1
```

Expected: every test passes. Any failure = behavior divergence → fix before committing.

- [ ] **Step 6: Commit**

```
rtk git commit -m "refactor(workflow): route node execution through NodeTypeRegistry + EffectApplier"
```

---

## Phase D — Old team strategies cleanup

### Task 17: Delete strategy files and refactor TeamService

**Files:**
- Delete: `src-go/internal/service/strategy.go`
- Delete: `src-go/internal/service/strategy_plan_code_review.go`
- Delete: `src-go/internal/service/strategy_pipeline.go`
- Delete: `src-go/internal/service/strategy_swarm.go`
- Delete: `src-go/internal/service/strategy_wave_based.go`
- Delete: `src-go/internal/service/team_workflow_adapter.go`
- Modify: `src-go/internal/service/team_service.go`
- Modify: `src-go/internal/service/team_service_test.go`
- Modify: `src-go/internal/server/routes.go` (remove `SetWorkflowAdapter` wiring)

- [ ] **Step 1: Delete strategy files and team_workflow_adapter.go**

```
rm src-go/internal/service/strategy.go src-go/internal/service/strategy_*.go src-go/internal/service/team_workflow_adapter.go
```

- [ ] **Step 2: Refactor `TeamService`**

Remove: `useWorkflowEngine` field, `workflowAdapter` field, `SetWorkflowAdapter` method, `resolveStrategy` method, `spawnCodersForTasks` method, `spawnCodersForTask` method.

Promote the strategy→template mapping into `WorkflowTemplateService.CreateFromStrategy(ctx, projectID, taskID, strategyName, variables)`. Implementation (inline from deleted adapter):

```go
// In workflow_template_service.go
func (s *WorkflowTemplateService) CreateFromStrategy(ctx context.Context, projectID, taskID uuid.UUID, strategy string, variables map[string]any) (*model.WorkflowExecution, error) {
    name := mapStrategyToTemplate(strategy)
    templates, err := s.repo.ListTemplatesByName(ctx, name)
    if err != nil || len(templates) == 0 {
        return nil, fmt.Errorf("no system template for strategy %q", strategy)
    }
    return s.CreateFromTemplate(ctx, templates[0].ID, projectID, &taskID, variables)
}

func mapStrategyToTemplate(strategy string) string {
    switch strategy {
    case "pipeline":  return TemplatePipeline
    case "swarm":     return TemplateSwarm
    case "plan-code-review", "wave-based", "":
        fallthrough
    default:
        return TemplatePlanCodeReview
    }
}
```

Modify `TeamService.StartTeam`: remove the strategy-fallback branch. Always call `s.templateSvc.CreateFromStrategy(...)`. Add `templateSvc *WorkflowTemplateService` field to `TeamService`, wire it in `NewTeamService`.

Modify `TeamService.ProcessRunCompletion`: remove `strategy.HandleRunCompletion` call. Agent run completion is already routed via `DAGWorkflowService.HandleAgentRunCompletion` — team just updates its cost and artifact. Keep the artifact-store + cost-update logic; delete the strategy dispatch.

- [ ] **Step 3: Update team_service_test.go**

Remove all tests that reference `strategy.Start`, `PlanCodeReviewStrategy`, `PipelineStrategy`, `SwarmStrategy`, `WaveBasedStrategy`. Keep tests that exercise other TeamService responsibilities (cost tracking, team-size enforcement, artifact storage).

- [ ] **Step 4: Update `routes.go`**

Remove the `SetWorkflowAdapter` call (was around line 326). Wire `templateSvc` into `TeamService` constructor.

- [ ] **Step 5: Build + test**

```
cd src-go && go build ./...
cd src-go && go test ./... -count=1
```

Expected: green. If TeamService has transitive references elsewhere (e.g., handler wiring), fix call sites.

- [ ] **Step 6: Commit**

```
rtk git commit -m "chore(team): remove strategy interface and delegate fully to workflow templates"
```

---

## Phase E — New NodeTypePlugin kind

### Task 18: Add `NodeTypePlugin` kind to Go plugin model + parser

**Files:**
- Modify: `src-go/internal/model/plugin.go`
- Modify: `src-go/internal/plugin/parser.go`
- Modify: `src-go/internal/plugin/parser_test.go`

- [ ] **Step 1: Write failing test** for parser that validates a NodeTypePlugin manifest: WASM variant passes, MCP variant passes, RolePlugin-with-wasm-runtime still fails, NodeTypePlugin with missing `nodeTypes[]` fails, NodeTypePlugin with `declaredCapabilities` not subset of top-level `capabilities` fails.

- [ ] **Step 2: Run, verify fail.**

- [ ] **Step 3: Implement**

In `model/plugin.go`, add:
```go
const PluginKindNodeType PluginKind = "NodeTypePlugin"

type NodeTypeSpec struct {
    Name                  string   `yaml:"name" json:"name"`
    Description           string   `yaml:"description,omitempty" json:"description,omitempty"`
    ConfigSchema          string   `yaml:"configSchema,omitempty" json:"configSchema,omitempty"`
    DeclaredCapabilities  []string `yaml:"declaredCapabilities" json:"declaredCapabilities"`
}

// Extend PluginSpec with:
NodeTypes []NodeTypeSpec `yaml:"nodeTypes,omitempty" json:"nodeTypes,omitempty"`
```

In `internal/plugin/parser.go`:
- Extend `isAllowedRuntime` to accept `NodeTypePlugin ↔ {wasm, mcp}`
- Add `validateNodeTypePlugin(spec PluginSpec) error`: checks non-empty `NodeTypes`, each entry's `Name` is non-empty, each entry's `DeclaredCapabilities` ⊆ `spec.Capabilities` (after stripping `effect:` prefix and matching against the EffectKind enum)
- Invoke from the main `ValidateManifest` dispatch

- [ ] **Step 4: Run tests, verify pass.**

- [ ] **Step 5: Commit**

```
rtk git commit -m "feat(plugin): add NodeTypePlugin kind and manifest validation"
```

### Task 19: Add NodeTypePlugin to TS schema

**Files:**
- Modify: `src-bridge/src/plugins/schema.ts`
- Modify: `src-bridge/src/plugins/schema.test.ts` (or equivalent)

Mirror the Go changes in Zod. Add `"NodeTypePlugin"` to `PluginKindSchema`. Extend the kind-to-runtime refine rule to allow `(NodeTypePlugin, wasm)` and `(NodeTypePlugin, mcp)`. Add `nodeTypes` schema on `spec`. Write Zod parse tests for valid + invalid manifests.

Commit: `feat(bridge): add NodeTypePlugin kind to schema validation`

### Task 20: Plugin activation wires nodeTypes into the registry

**Files:**
- Modify: `src-go/internal/service/plugin_service.go`
- Modify: `src-go/internal/handler/plugin_handler.go` (if needed for error propagation)
- Modify: `src-go/internal/service/plugin_service_test.go`

- [ ] **Step 1: Write failing integration test** — install a fake NodeTypePlugin manifest, call `ActivatePlugin`, assert registry now contains entries. Call `DeactivatePlugin`, assert entries gone.

- [ ] **Step 2: Implement**

Plugin service gets a new dependency: `registry *nodetypes.NodeTypeRegistry`. On activate:
1. Parse manifest (already done by existing flow)
2. For `kind=NodeTypePlugin`: for each `spec.nodeTypes[]`, construct a handler based on runtime (see Task 21/22 adapters), then call `registry.RegisterPluginNode(projectID, pluginID, version, fullName, handler)`
3. If any registration fails, roll back previously-registered entries and set lifecycle to `degraded`
4. Persist activation state to `plugins` table as usual

On deactivate: call `registry.UnregisterPlugin(projectID, pluginID)` and log the count removed.

- [ ] **Step 3-5**: run tests, commit.

Commit: `feat(plugin): wire NodeTypePlugin activation to registry`

---

## Phase F — WASM and MCP adapters (make plugin-contributed handlers actually work)

### Task 21: WASM adapter + Go SDK helper

**Files:**
- Create: `src-go/internal/workflow/adapters/wasm_adapter.go`
- Create: `src-go/internal/workflow/adapters/wasm_adapter_test.go`
- Create: `src-go/plugin-sdk-go/nodetype.go`
- Create: `src-go/plugin-sdk-go/nodetype_test.go`

- [ ] **Step 1: Write failing adapter test** — given a fake WASMRuntimeManager returning a canned envelope, the adapter's `Execute` parses it into `NodeExecResult` correctly (including Effects array).

- [ ] **Step 2: Implement `WASMNodeAdapter`**

```go
type WASMNodeAdapter struct {
    pluginID     string
    nodeName     string              // short name (after /)
    runtime      WASMRuntime         // interface over wazero WASMRuntimeManager
    caps         []nodetypes.EffectKind
    configSchema json.RawMessage
}

func (a *WASMNodeAdapter) Execute(ctx context.Context, req *nodetypes.NodeExecRequest) (*nodetypes.NodeExecResult, error) {
    payload, _ := json.Marshal(map[string]any{
        "executionId": req.Execution.ID.String(),
        "projectId":   req.ProjectID.String(),
        "nodeId":      req.Node.ID,
        "nodeExecId":  req.NodeExecID.String(),
        "config":      req.Config,
        "dataStore":   req.DataStore,
    })
    inv := plugin.Invocation{
        Operation: "execute_node:" + a.nodeName,
        Payload:   payload,
    }
    env, err := a.runtime.Invoke(ctx, a.pluginID, inv)
    if err != nil { return nil, err }
    if !env.OK { return nil, fmt.Errorf("plugin error: %s", env.Error.Message) }

    var out struct {
        Result  json.RawMessage          `json:"result"`
        Effects []nodetypes.Effect       `json:"effects"`
    }
    if err := json.Unmarshal(env.Data, &out); err != nil {
        return nil, fmt.Errorf("parse plugin response: %w", err)
    }
    return &nodetypes.NodeExecResult{Result: out.Result, Effects: out.Effects}, nil
}

func (a *WASMNodeAdapter) ConfigSchema() json.RawMessage    { return a.configSchema }
func (a *WASMNodeAdapter) Capabilities() []nodetypes.EffectKind { return a.caps }
```

- [ ] **Step 3: SDK helper `plugin-sdk-go/nodetype.go`**

```go
type NodeHandler func(ctx *Context, req NodeExecRequest) (*NodeExecResult, error)

type NodeExecRequest struct {
    ExecutionID string                 `json:"executionId"`
    ProjectID   string                 `json:"projectId"`
    NodeID      string                 `json:"nodeId"`
    NodeExecID  string                 `json:"nodeExecId"`
    Config      map[string]any         `json:"config"`
    DataStore   map[string]any         `json:"dataStore"`
}

type NodeExecResult struct {
    Result  json.RawMessage `json:"result,omitempty"`
    Effects []Effect        `json:"effects,omitempty"`
}

var nodeHandlers = map[string]NodeHandler{}

func RegisterNodeTypeHandler(name string, h NodeHandler) {
    nodeHandlers[name] = h
}

// Augment the existing Runtime.Run to dispatch "execute_node:<name>" ops
// through the nodeHandlers map.
```

Modify existing `Runtime.Run()` to check if `Operation` has prefix `execute_node:` and dispatch.

- [ ] **Step 4: Test** (happy path + malformed envelope)

- [ ] **Step 5: Commit**

```
rtk git commit -m "feat(nodetypes): WASM adapter and Go SDK RegisterNodeTypeHandler helper"
```

### Task 22: MCP adapter + TS SDK helper

**Files:**
- Create: `src-go/internal/workflow/adapters/mcp_adapter.go`
- Create: `src-go/internal/workflow/adapters/mcp_adapter_test.go`
- Create: `src-bridge/src/plugin-sdk/nodetype.ts`
- Create: `src-bridge/src/plugin-sdk/nodetype.test.ts`

MCP adapter dispatches via a bridge call: the Go service issues an HTTP request to the bridge `POST /plugins/:pluginId/tools/:toolName` with the NodeExecRequest as body; the bridge forwards to the MCP server, returns the result.

- [ ] **Step 1: Adapter test** — fake HTTP bridge client, canned response, assert result parsing.

- [ ] **Step 2: Implement `MCPNodeAdapter`**

Adapter holds: `pluginID`, `shortName`, `bridgeClient`, `caps`, `configSchema`.

`Execute` serializes request to JSON, POSTs to `bridgeClient.CallPluginTool(pluginID, shortName, body)`, parses `NodeExecResult` from the tool's `content` field (per MCP convention — tool output is typically a `content: [{type:"text", text: "...json..."}]` array).

- [ ] **Step 3: TS SDK helper `plugin-sdk/nodetype.ts`**

```typescript
export interface NodeTypeDefinition<C = unknown> {
  name: string
  description?: string
  configSchema?: ZodTypeAny
  declaredCapabilities: EffectKind[]
  execute: (req: NodeExecRequest<C>) => Promise<NodeExecResult>
}

export function defineNodeTypePlugin(definition: {
  manifest: PluginManifest
  nodeTypes: readonly NodeTypeDefinition[]
}): ToolPluginDefinition {
  // Translates each nodeType into an MCP tool:
  // - tool.name = nodeType.name (short, without pluginID prefix)
  // - tool.inputSchema = NodeExecRequest shape
  // - tool.execute wraps nodeType.execute, returning stringified JSON result
  //   in { content: [{type:"text", text: JSON.stringify(result)}] }
  return defineToolPlugin({
    manifest: { ...definition.manifest, kind: "NodeTypePlugin" },
    tools: definition.nodeTypes.map((n) => ({
      name: n.name,
      description: n.description,
      inputSchema: NodeExecRequestZodSchema,
      execute: async (args) => {
        const result = await n.execute(args)
        return {
          content: [{ type: "text", text: JSON.stringify(result) }]
        }
      }
    }))
  })
}
```

- [ ] **Step 4-5**: test + commit

```
rtk git commit -m "feat(nodetypes): MCP adapter and TS SDK defineNodeTypePlugin helper"
```

---

## Phase G — Workflow save-time validation

### Task 23: Reject workflow definitions referencing unknown node types

**Files:**
- Modify: `src-go/internal/service/dag_workflow_service.go` (Create + Update methods)
- Modify: `src-go/internal/service/dag_workflow_service_test.go`
- Modify: `src-go/internal/handler/workflow_handler.go` (status code mapping)

- [ ] **Step 1: Test** — create a workflow with `nodes: [{type: "bogus/unknown"}]` → API returns 400 with error body `{error: "unknown node types", types: ["bogus/unknown"]}`.

- [ ] **Step 2: Implement** — at Create + Update, iterate `nodes[].type`, call `s.registry.Resolve(projectID, type)`, collect failures, return structured error.

- [ ] **Step 3: Handler maps error to HTTP 400.**

- [ ] **Step 4: Test pass.**

- [ ] **Step 5: Commit**

```
rtk git commit -m "feat(workflow): validate node types at save time"
```

---

## Phase H — Reference plugins + E2E tests

### Task 24: Reference plugin `wasm-echo-node`

**Files:**
- Create: `plugins/examples/wasm-echo-node/` (directory tree)
  - `manifest.yaml`
  - `main.go`
  - `go.mod`, `go.sum`
  - `README.md`
  - `schemas/noop.json` (JSON Schema for the single node type's config)
  - `test/wasm-echo-node.e2e_test.go` (or under `src-go/cmd/plugin-debugger/`)

Single node type: `echo/noop` — takes `config.input` and returns it as `result.output`. No effects.

Manifest:
```yaml
apiVersion: agentforge.io/v1
kind: NodeTypePlugin
metadata:
  id: echo
  name: Echo Node Type
  version: 0.1.0
spec:
  runtime: wasm
  module: ./plugin.wasm
  abiVersion: v1
  capabilities: []
  nodeTypes:
    - name: noop
      description: Echo input as output
      configSchema: ./schemas/noop.json
      declaredCapabilities: []
```

Go plugin `main.go` uses the SDK's `RegisterNodeTypeHandler("noop", handler)` + `Autorun`.

Build artifact: `plugin.wasm` produced via `GOOS=wasip1 GOARCH=wasm go build -o plugin.wasm .`

Commit: `feat(examples): wasm-echo-node reference plugin`

### Task 25: Reference plugin `mcp-http-probe`

**Files:**
- Create: `plugins/examples/mcp-http-probe/`
  - `manifest.yaml`
  - `package.json`
  - `src/server.ts`
  - `README.md`
  - `schemas/get.json`

Single node type: `probe/get` — takes `config.url`, performs GET, returns `{status, body_snippet}`. Emits `broadcast_event` with the URL being probed.

TS server uses `defineNodeTypePlugin` (Task 22 helper).

Commit: `feat(examples): mcp-http-probe reference plugin`

### Task 26: E2E test — WASM plugin install → activate → execute

**Files:**
- Create: `src-go/internal/server/nodetype_plugin_e2e_test.go`

Integration test:
1. Build `wasm-echo-node` during test setup (use `go:generate` or a `TestMain` that shells out to `go build -target=wasip1`)
2. Start a test HTTP server with real dependencies (in-memory or test DB)
3. POST the manifest to `/api/v1/plugins/install` with local path
4. POST to `/api/v1/plugins/{id}/activate`
5. Create a workflow using `echo/noop` node
6. Start execution
7. Assert execution completes, DataStore contains expected output
8. Assert `plugin_events` table has `registry_entry_added` row

Commit: `test(nodetypes): E2E test for WASM plugin execution`

### Task 27: E2E test — MCP plugin install → activate → execute

**Files:**
- Create: `src-go/internal/server/nodetype_plugin_mcp_e2e_test.go`

Same structure as Task 26 but for `mcp-http-probe`. Requires bridge to be running in test context; consider using a test mode that stubs the HTTP call to a known local endpoint.

Commit: `test(nodetypes): E2E test for MCP plugin execution`

---

## Phase I — Documentation

### Task 28: Update `docs/guides/plugin-development.md`

**Files:**
- Modify: `docs/guides/plugin-development.md`

Add a new section "NodeTypePlugin (Track A)" covering:
- Purpose
- Manifest example (pulled from reference plugin)
- Effect DSL — complete enumeration with payload shapes
- Handler contract (Go + TS SDK usage)
- Capability declaration
- Namespacing rules
- Lifecycle (install → activate → register → invoke → deactivate)
- Link to reference plugins

Commit: `docs(plugin): add NodeTypePlugin authoring guide`

### Task 29: Mark Track A done in roadmap + update MEMORY

**Files:**
- Modify: `docs/superpowers/roadmap/2026-04-16-plugin-extensibility-roadmap.md`
- Modify: `C:/Users/qwdma/.claude/projects/D--Project-AgentForge/memory/MEMORY.md`
- Create: memory file `track_a_done.md` snapshotting the final contracts (handler interface, effect set, namespacing rule) for future sessions

Roadmap table update: Track A row — `Spec ✅`, `Plan ✅`, `Impl ✅`, `Merged ✅` with commit SHAs.

Commit: `docs: mark Track A complete in roadmap`

---

## Final Acceptance (spec §14)

- [ ] **AC1**: All files in spec §8.1 exist and compile — `cd src-go && go build ./...` green
- [ ] **AC2**: All files in spec §9.1 deleted — `rtk grep -rn "PlanCodeReviewStrategy\|PipelineStrategy\|SwarmStrategy\|WaveBasedStrategy\|TeamWorkflowAdapter" src-go/` returns nothing
- [ ] **AC3**: Unit tests pass — `cd src-go && go test ./internal/workflow/nodetypes/... -v`
- [ ] **AC4**: Regression tests pass — `cd src-go && go test ./... -count=1`
- [ ] **AC5**: WASM reference plugin E2E green
- [ ] **AC6**: MCP reference plugin E2E green
- [ ] **AC7**: `docs/guides/plugin-development.md` has the NodeTypePlugin section
- [ ] **AC8**: `MEMORY.md` has Track A contract snapshot; roadmap row updated with commit SHAs

When all AC pass, open the PR. PR title: `Track A: DAG Node Type Registry + team strategy retirement`. PR body should reference the spec and plan paths and include a checklist matching the AC above.

---

## Notes for the executor

- **Task 16 is the highest-risk step** — the whole regression suite must remain green when routing flips. If a test fails that isn't obviously a behavior bug in the new registry path, stop and diff the old executor code against the new handler code carefully. Do NOT silently "fix" the test.
- **Do not skip tests**. Every handler has its own test file; every test path exercises an actual code branch. If a handler looks too trivial to test (structural no-ops), test at minimum that `Capabilities()` returns the right empty set and that `Execute` returns `&NodeExecResult{}`.
- **Commits stay small and buildable**. If a task ends up needing two commits to keep the tree green, split it — don't push a broken intermediate commit.
- **When in doubt about a spec detail, re-read the spec section**. The plan references the spec by §; the spec is authoritative.
