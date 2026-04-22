# Workflow Engine Completion — Design Spec

- Status: Draft, awaiting review
- Owner: Max Qian
- Created: 2026-04-22
- Scope: AgentForge Go orchestrator (`src-go/internal/workflow/*`, `src-go/internal/service/*workflow*.go`), DB migrations, system workflow templates
- Rollout: Breaking changes permitted (internal-test stage)

## 1. Position

AgentForge has already refactored from a coding-only 3-role team engine into a general-purpose DAG workflow engine (Phase 1-5 delivered per prior work). This spec closes the last gap: **the engine and the seven built-in templates do not share one contract**. Three real bugs ship in those templates today, and several engine behaviours (condition routing, idempotency, cancel cascade, secret handling, loop counter atomicity, wait_event delivery) are under-specified.

Rather than patch each symptom, this spec defines one explicit contract between node handlers, the DAG scheduler, and templates, then hardens, extends, and rewrites everything against that contract in four phases.

## 2. Goals (ordered by precedence)

1. **Zero-ambiguity contracts.** Every node type declares `Inputs / Outputs / Effects / ErrorModes / Idempotency / Scope`. Scheduler, retry, observability layers consume the contract — no handler-side side-channels.
2. **Branching semantics are singular.** `edge.Condition` stops driving routing; a new `switch` node owns multi-branch routing; `condition` becomes a gate with an explicit `onFalse` policy.
3. **Execution is robust.** Node-level retry/backoff, timeout/SLA, workflow-level idempotency keys, cancel cascade through sub-runs, wait_event de-duplication, atomic loop counter — all shipped.
4. **Collection primitives exist.** A first-class `map/foreach + reduce` pair replaces the hand-rolled `fan_out + parallel_split + parallel_join` chain used in current coding templates.
5. **Built-in templates are correct and expressive.** All seven templates (`plan-code-review`, `pipeline`, `swarm`, `content-creation`, `customer-service`, `system:code-review`, `code_fixer`) are fixed and rewritten against the new contracts.

## 3. Non-Goals (moved to backlog section)

- Frontend visual editor node-inspector UX (`lib/stores/workflow-store.ts` polish, inline validation, live overlay)
- `/debug` step-level aggregation panel (timeline heatmaps, loop iteration histograms)
- Additional node types: `delay`, `try_catch`, `emit_event`
- Cross-workflow template-variable sharing

These are enumerated in §11 Backlog so nothing is silently dropped.

## 4. Breaking-Change Baseline

Project memory records internal-test stage; `breaking changes` are expressly permitted.

- **DB schema:** breaking changes allowed. New columns: `workflow_executions.idempotency_key`, `workflow_definitions.node_matrix_version`. New table: `wait_event_deliveries`.
- **Workflow definitions:** versioned via `node_matrix_version` (1 = old, 2 = new). A one-off migration script bumps built-in seeds; in-flight executions on v1 are drained (see §10).
- **API/SDK:** `edge.Condition` remains a persisted field for visual layout fidelity, but the scheduler ignores it (§5.2). Client libs must stop relying on it for routing.

## 5. Contract Layer

### 5.1 Node Contract

Every node handler must declare a `NodeContract` alongside the existing `Execute / ConfigSchema / Capabilities`. Go interface:

```go
type NodeContractProvider interface {
    Contract() NodeContract
}

type NodeContract struct {
    Inputs     []InputDecl   // declared {{...}} references this handler reads
    Outputs    []OutputDecl  // keys the handler writes into its NodeExecResult.Output
    Effects    []EffectKind  // identical to today's Capabilities()
    ErrorModes []ErrorMode   // every named failure this handler can return
    Idempotent bool          // repeat-call safe?
    Scope      ContractScope // "execution" | "run" | "step"
}

type InputDecl  struct { Name, Path string; Required bool; Type string }
type OutputDecl struct { Name, Type string; Schema json.RawMessage }
type ErrorMode  struct { Code string; Retryable bool; Description string }
```

Relationship to existing surface:

- `ConfigSchema()` remains and describes *static* config shape (JSON Schema), consumed by the visual editor and save-time validation.
- `Capabilities()` is superseded by `Contract().Effects` but kept for one release as a deprecated alias that panics if it disagrees with the contract.
- `Inputs/Outputs` are *runtime* declarations — they describe `{{...}}` templating and DataStore keys, which `ConfigSchema` cannot express.

**Worked example (`llm_agent`):**

```go
func (LLMAgentHandler) Contract() NodeContract {
    return NodeContract{
        Inputs: []InputDecl{
            {Name: "prompt", Path: "config.prompt", Required: true, Type: "string-template"},
            {Name: "runtime", Path: "config.runtime", Required: false, Type: "string-template"},
        },
        Outputs: []OutputDecl{
            {Name: "output", Type: "object", Schema: json.RawMessage(`{"type":"object"}`)},
        },
        Effects: []EffectKind{EffectSpawnAgentRun},
        ErrorModes: []ErrorMode{
            {Code: "llm_timeout",     Retryable: true,  Description: "runtime timed out"},
            {Code: "llm_budget",      Retryable: false, Description: "budget_usd exceeded"},
            {Code: "llm_bad_output",  Retryable: false, Description: "output did not parse"},
        },
        Idempotent: false,
        Scope:      "run",
    }
}
```

Consumers: scheduler (uses `ErrorMode.Retryable` to short-circuit retry-node exhaustion), observability (labels metrics by `ErrorMode.Code`), `ValidateDefinition` (§5.3), visual editor (renders inspector form from `Inputs`).

All 16 existing handlers gain a contract in P2. Derivation is not mechanical (the audit showed `ConfigSchema` alone is insufficient); each handler writes its contract by hand following the worked example.

### 5.2 Branching Semantics (Breaking Change)

**Current conflict.** Routing today is evaluated at two layers:

1. `ConditionHandler.Execute` returns `error("condition not met: ...")` when its `expression` is false, failing the entire workflow.
2. The DAG scheduler (`dag_workflow_service.go` around line 385) independently evaluates `edge.Condition` on each outgoing edge to decide which downstream node to advance.

In the `CustomerService` template, `urgent_check.config.expression` and the two outgoing `edge.Condition`s encode **the same** predicate. For non-urgent tickets the node layer fails first, so the edge layer never gets a chance to route to `auto_reply`. The bug is not "one layer is dead" — both layers are alive and duplicated, and the node layer's default-`error` policy wins. `onFalse: skip_downstream` (§7.7) plus `switch` (§8.3) remove the duplication.

**New semantics.**

| Node Type     | Downstream edges | Purpose        | Failure policy                                                            |
| ------------- | ---------------- | -------------- | ------------------------------------------------------------------------- |
| `condition`   | exactly 1        | Gate           | `onFalse: error \| skip_downstream \| proceed` — `config.onFalse` required |
| `switch` (new)| N (per case)     | Multi-branch   | One `case` match or `default_route`; no match and no default → error      |
| `edge.Condition` | —             | **Retired**    | Persisted for layout only; scheduler ignores                              |

Routing authority: the scheduler reads `output.route` (from `switch` or any handler that publishes one) to select the next edge. Edges carry a new `route` label that matches against the output. A single authority eliminates the double-write problem.

Backward compatibility: `ConditionHandler` keeps `onFalse: error` as its default, so definitions that never set it preserve today's behaviour. All six built-in templates that use conditions are migrated in P1/P4.

### 5.3 Data Flow and Variable Scope

Today the codebase mixes five kinds of `{{...}}` references with no formal spec. Formalise:

| Prefix                              | Scope                           | Lifetime     | Resolved at                             |
| ----------------------------------- | ------------------------------- | ------------ | --------------------------------------- |
| `{{<node_id>.output.<path>}}`       | run-scoped DataStore            | One run      | Node entry                              |
| `{{<template_var>}}`                | definition-scoped `templateVars`| Permanent    | Baked at clone time                     |
| `{{$event.<path>}}`                 | run-scoped trigger payload      | One run      | Node entry                              |
| `{{secret.<key>}}`                  | project-scoped secret vault     | Lazy         | At handler's outbound request only      |
| `{{$run.<meta>}}`                   | run metadata (id, startedAt, …) | One run      | Node entry                              |

Hard rules:

- **Secrets never enter DataStore, logs, or Live Tail.** A `SecretRef` type carries an opaque handle; the handler calls `resolver.Materialize(ctx, ref)` at the moment of constructing the HTTP/LLM/IM request and discards the clear value immediately. `SecretRef.MarshalJSON` returns `"<secret:<key>>"`.
- **Unknown references fail fast.** No silent empty-string substitution. Opt-in allowlist via config for legitimate optional placeholders.

**Non-leakage surfaces (all must be covered by E2E — see §13).** The `SecretRef` guarantee is only as strong as the exit points it plugs. The design must scrub, on every code path below:

- Outbound HTTP response bodies included in handler `error()` strings — error wrapping must run through a scrubber that re-applies the `<secret:...>` mask on any substring that matches a materialised secret value (short-lived in-memory set held by the resolver for the duration of the call).
- Retry replay payloads (`EffectRetryInvoke.Payload`): replays store the original unresolved `SecretRef`, never the materialised value. Re-materialise per attempt.
- Bridge-side log forwarding: the Bridge's review pipeline already forwards LLM transcripts; the `llm_agent` handler must pass the unresolved `SecretRef` structure to the Bridge with resolution performed **inside the Bridge** at the final HTTP egress, never on the Go side — OR redact Bridge logs with the same scrubber. Choose per P2 handler; document in the handler's test.
- Live Tail diff panel and `/debug/trace/:id` merged timeline: audit event payloads carry `SecretRef` only; assertions in the E2E suite read the persisted `events` and `logs` rows and grep for known secret cleartext — zero matches required.

### 5.4 Idempotency and Cancel Cascade

**Idempotency key.** Every `WorkflowExecution` carries a non-empty `idempotency_key`; §7.1 defines the full resolution order (caller → trigger hash → uuid). `DAGWorkflowService.Start` and `WorkflowExecutionService.StartTriggered*` both consult `(workflow_id, idempotency_key)`. On collision, return `{id, status}` of the existing run. Aligned with the existing trigger-engine idempotency (no double-writing).

**Cancel cascade.** Cancelling a parent run enumerates its active children via `workflow_run_parent_link` + sub-invocation bookkeeping, then dispatches `EffectCancelChild` to each. Context honouring details are covered in §7.2 (surfaces to change) and §14 Risk 3 (single authoritative SLA). Three levels of nesting are required to pass CI.

**wait_event de-duplication.** New table:

```sql
wait_event_deliveries (
  execution_id UUID,
  node_id      TEXT,
  event_id     TEXT,
  delivered_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  payload_hash TEXT,
  PRIMARY KEY (execution_id, node_id, event_id)
);
```

`WaitEventResumer.Deliver` checks the table first and short-circuits on duplicates with an audit log entry.

## 6. Phase P1 — Built-in Template Hardening

Template-only changes plus new tests. Does not touch handlers or DB.

| Template              | Fix                                                                                                                                                                                                                      | Ground truth                                                            |
| --------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------- |
| `SystemCodeReview`    | `finalize.config` from `{from, to, reason}` → `{targetStatus: "{{decision.output.decision}}", reason: "{{decision.output.comment}}"}`. `decision` human_review declares `decision_schema: {approved: bool, comment: string}` and prompt demands it. | `status_transition` handler only accepts `targetStatus`; current template 100% fails. |
| `CustomerService`     | P1 (before `switch` exists): pull `onFalse: skip_downstream` from §7.7 forward into P1 so `urgent_check` no longer errors; `classify` prompt emits discrete `urgency_band: "urgent" \| "normal"` (no floats); `edge.Condition` is kept in this intermediate release and the scheduler's existing edge-condition logic selects `human_review` vs `auto_reply`. P4 rewrites to `switch` and removes `edge.Condition`. | Avoid float-threshold edge cases + avoid the `condition` false-path crash. Two-step migration because §5.2's "edge.Condition retired" cannot land before P2 ships scheduler changes. |
| `ContentCreation`     | `editor` prompt required output `{approved: bool, feedback: string}`. `edit_loop.exit_condition` preserved. `writer/editor` outputs referenced explicitly (no implicit last-output).                                   | Loop may never exit or exit wrong today.                                |
| `PlanCodeReview` / `Swarm` | `fan_out.output` schema formalised as `{subtasks: [{id, title, prompt, budget_usd}]}`. P4 replaces this fan-out with a `map` node.                                                                                   | `coder` inputs currently unspecified.                                   |
| `Pipeline`            | `coder.inputs` explicit: `{plan: "{{planner.output.plan}}"}`.                                                                                                                                                           | No implicit template pass-through.                                      |
| `CodeFixer`           | `has_prebaked` expression changed from `input.suggested_patch != null` to `{{$event.suggested_patch}} != null`; trigger payload schema declares `suggested_patch`. Verify `validate` function handler exists — if not, implement or remove. | Current reference is not resolvable.                                    |
| All LLM nodes         | Template documents selection priority in the handler contract: `employee_id` > `role_id` > `templateVars.runtime/provider/model`.                                                                                        | Today the "who wins" requires reading source.                           |

## 7. Phase P2 — Engine Hardening (no new node types)

### 7.1 Idempotency

- Migration: `ALTER TABLE workflow_executions ADD COLUMN idempotency_key VARCHAR(128) NOT NULL;` (no default) plus unique index `UNIQUE(workflow_id, idempotency_key)`.
- Every caller supplies a non-empty key. Resolution order:
  1. Explicit `req.IdempotencyKey` from the caller (API clients, trigger payload).
  2. For trigger-originated runs: fallback `sha256(trigger_id | canonical_json(payload))[:32]` computed inside `WorkflowExecutionService.StartTriggered*` before insertion.
  3. For manual starts without an explicit key: fallback `uuid.NewString()` (single-shot, never collides, effectively disables dedup for that call).
- Legacy-row backfill: the P2 migration fills existing rows with `uuid.NewString()` so historical executions never collide with incoming traffic.
- On collision return `{id, status}` of the existing run.

### 7.2 Cancel Cascade

**Current state (verified by audit).** `llm_agent.go`, `http_call.go`, `wait_event.go`, and `applier_http_call.go` do not currently read `ctx.Done()`. The applier does not propagate cancellation. Handlers are pure effect producers; cancellation therefore has to live in (a) the applier's effect dispatch, (b) the Bridge round-trip, and (c) any polling/tick loop.

**New effect.** `EffectCancelChild{ ChildRunID uuid.UUID, Reason string }`. The applier reads the child's engine (DAG vs plugin_run) from `workflow_run_parent_link` and dispatches via the appropriate cancellation surface.

**Surfaces that must gain cancellation in this phase:**

- `applier_http_call.go`: outbound `http.Request` wrapped with `req.WithContext(ctx)`; the applier-level `ctx` is the run-level cancellable context.
- LLM applier dispatch to Bridge: the Bridge HTTP call uses `req.WithContext(ctx)`. Bridge-side runtimes already support cancellation token hand-off — no Bridge work is required *beyond* confirming each runtime adapter forwards the cancellation (audit and test, don't re-implement).
- `wait_event` resumer tick loop: when ticking, `select` on `ctx.Done()` in addition to the timer channel; on cancel, mark the waiting node `cancelled` without emitting resume effects.
- `DAGWorkflowService.Cancel` enumerates active children from `workflow_run_parent_link` plus the `sub_invocation` registry and emits one `EffectCancelChild` per row inside the same transaction that flips the parent to `cancelled`.

**SLA.** There is one SLA: **best-effort 10 s target, force-terminate at 30 s with an alert event**. §14 Risk 3 is the canonical statement; §5.4's earlier "within 10 s" is a target, not a hard guarantee. Tests assert the 30 s force-terminate ceiling, not the 10 s target.

**CI requirement:** three-level nested cancel test (A → sub_workflow B → sub_invocation C, cancel A, all end in `cancelled` within 30 s).

### 7.3 wait_event De-duplication

Table defined in §5.4. Retention and garbage collection:

- `FOREIGN KEY (execution_id) REFERENCES workflow_executions(id) ON DELETE CASCADE` so dedup rows die with their run.
- Janitor job `workflow_wait_event_janitor` (registered with the existing scheduler) runs daily at 03:00 local; deletes `wait_event_deliveries` rows older than `max(executions.completed_at + 14 days, now() - 30 days)`. A completed run keeps its dedup history for two weeks (debugging window), an orphan row (shouldn't happen with the FK, belt-and-braces) is cleaned after 30 days unconditionally.

### 7.4 Secret Pipeline

- Extend `template/secret_resolver.go` to recognise `{{secret.xxx}}` and wrap as `SecretRef`.
- Modify `llm_agent`, `http_call`, `im_send` handlers to call `resolver.Materialize(ctx, ref)` only at the last moment; discard clear value after building the outbound request.
- JSON marshalling of `SecretRef` returns `"<secret:<key>>"` everywhere — logs, Live Tail, audit.

### 7.5 Definition Validation

`ValidateDefinition(def, projectID)` runs on save and on engine start:

- Reject unknown `node_id` references (hard-fail at both save and start)
- Reject unknown `templateVar` references (hard-fail at both save and start)
- Secret-key policy: **lazy resolve at handler entry** — the save-time check is informational only; at run time, `secret.Materialize` returns a structured `ErrorMode{Code: "secret_missing", Retryable: false}` that the scheduler surfaces through `NodeContract.ErrorModes`. Rationale: a definition may legitimately reference a secret that will be populated before the first run (e.g. a new integration pending credential issuance). Hard-failing at save creates an ordering trap; lazy resolution keeps the failure observable and retry-able once the secret arrives.

Shared by frontend save and backend start.

### 7.6 Atomic Loop Counter

Today the loop handler computes `nextIter` and embeds it in `EffectResetNodes.Payload`; the applier writes the counter back. If the applier crashes between counter write and node reset, resume double-iterates.

Fix: combine counter write and node reset into a single DB transaction in the applier. Expose `ApplyResetNodes(tx, payload)`; callers wrap in the same transaction as other state changes.

### 7.7 Condition `onFalse` Policy

- `ConditionHandler` gains `config.onFalse ∈ {error, skip_downstream, proceed}`, default `error` (preserves today's behaviour).
- Built-in templates using condition (CodeFixer's `has_prebaked`, `decide`) migrate to `skip_downstream` or `switch`.

### 7.8 Observability Baseline

Each P2 change adds metrics: `workflow.retry_total` (reserved), `workflow.cancel_cascade_total`, `workflow.secret_resolve_total`. Structured errors expose `ErrorMode.Code` as a metric label. Live Tail UI is unchanged; the secret placeholder format is the only surface change.

### 7.9 New Effect Payload Schemas

Every new effect persists through the existing `workflow_node_executions` effect row and replays on resume. Payload schemas:

```go
// Emitted by retry node to schedule the next attempt of an inline child.
type RetryInvokePayload struct {
    WrappedNodeID   string            // internal subgraph node id
    Attempt         int               // 1-based, the attempt about to run
    MaxAttempts     int
    BackoffUntil    time.Time         // computed backoff + jitter deadline
    UnresolvedInput map[string]any    // original {{...}} references, SecretRef kept opaque
}

// Emitted by timeout node to register a deadline timer.
type TimeoutSchedulePayload struct {
    WrappedNodeID string
    DeadlineAt    time.Time
    OnTimeout     string   // "fail" | "route:<edge_label>"
}

// Emitted by map/foreach to spawn N iteration subgraphs.
type SpawnMapIterationsPayload struct {
    ParentNodeID string
    Items        []any     // concrete expanded items (no SecretRef here — scrubbed)
    ItemVar      string
    Concurrency  int
    Collect      string    // "all" | "first_ok" | "fail_fast"
    BodyNodeIDs  []string  // internal subgraph node ids per iteration
}

// Emitted by cancel to terminate a specific child.
type CancelChildPayload struct {
    ChildRunID uuid.UUID
    ChildKind  string    // "dag" | "plugin_run" — selects dispatch surface
    Reason     string
}
```

Persistence: each payload is written to `workflow_node_executions.effect_payload` (existing JSONB column). All payload types implement `MarshalJSON` such that any nested `SecretRef` renders as `<secret:<key>>`; the §13 DoD adds one regression test per payload that asserts no known secret cleartext appears in persisted rows.

Scheduler coordination: `SpawnMapIterationsPayload.BodyNodeIDs` are synthesised at effect-emit time and registered into the parent execution's `currentNodes` JSONB so the scheduler's existing traversal logic handles them as first-class nodes (no special case per map — iteration subgraphs are indistinguishable from hand-authored parallel branches).

## 8. Phase P3 — Node Matrix Expansion

Five new node types, each shipping with handler, schema, effect(s), contract, and tests.

### 8.1 `retry` — Retry Wrapper

```
config: {
  target_node: string,
  max_attempts: int (default 3),
  backoff: { kind: "fixed"|"exponential", base_ms, factor, max_ms, jitter: bool },
  retryable_errors: [code]?,
  on_exhausted: "fail" | "proceed" | "route:<edge_label>",
}
effects: EffectRetryInvoke (new)
```

Implementation: inline. The retry node wraps `target_node` as a subgraph child at execution time; DAG topology is unchanged. Each attempt is a distinct step record for Live Tail clarity (`attempt 2/3`).

### 8.2 `timeout` — SLA Wrapper

```
config: { target_node, timeout_ms, on_timeout: "fail" | "route:<edge_label>" }
effects: EffectTimeoutSchedule (new)
```

Implementation: applier registers a timer via the existing scheduler tick. On fire, cancel the step's `ctx`. Combined with `retry`, nest as `timeout(retry(target))` so the timeout bounds a single attempt, not the whole retry chain. P4 templates demonstrate the pattern.

### 8.3 `switch` — Multi-branch

```
config: {
  input:        string,                   // e.g. "{{classify.output.urgency_band}}"
  cases:        [{ match: any, route: string }],
  default_route: string?,
}
output: { route: string }
```

Scheduler selects the next edge by matching `output.route` to `edge.route` (edge gains a `route` field). No match and no default → `ErrorMode{Code: "no_switch_match", Retryable: false}`.

### 8.4 `map` / `foreach` — Collection Parallelism

```
config: {
  items:       string,              // "{{planner.output.subtasks}}"
  body:        { nodes, edges },    // inline subgraph
  concurrency: int (default 5),
  item_var:    string (default "item"),
  collect:     "all" | "first_ok" | "fail_fast" (default "all"),
}
effects: EffectSpawnMapIterations (new)
output: { results: [body_output], errors: [...] }
```

`map` and `foreach` are the same node — the name reflects intent (functional collection vs side-effect iteration). Concurrency integrates with the DAGWorkflowService global pool to honour tenant limits.

### 8.5 `reduce` — Aggregation

```
config: {
  items:        string,              // "{{map_node.output.results}}"
  initial:      any,
  reducer:      { kind: "llm" | "expression", ... },
  output_path:  string (default "value"),
}
```

Two reducer kinds:

- `expression`: pure expression over `accumulator` and `current`, reuses `expr.go`.
- `llm`: inline `llm_agent` sub-invocation per pair, prompt receives both values.

### 8.6 Composition Rules

- `retry` / `timeout` may wrap `llm_agent`, `http_call`, `sub_workflow`, `map`, `reduce`. They may not wrap each other or themselves.
- `map.body` may contain `switch`, `retry`, `timeout` but not another `map` (multi-level collections are in the backlog).

## 9. Phase P4 — Template Rewrite

| Template          | Rewrite                                                                                                                                                                       |
| ----------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `PlanCodeReview`  | `fan_out + split + coder + join` → `map(items="{{planner.output.subtasks}}", body=coder)`, wrapped in `timeout(30m)`.                                                          |
| `Swarm`           | Same as PlanCodeReview, higher concurrency (10) and planner prompt emphasises independence.                                                                                    |
| `Pipeline`        | Linear chain preserved; `coder` wrapped in `retry(max_attempts=2, backoff=exponential)`.                                                                                        |
| `ContentCreation` | `writer ↔ editor` loop retained (iterative polish semantics differ from `map`). `editor` wrapped in `retry`; final `seo` wrapped in `timeout`.                                  |
| `CustomerService` | P1 migrated to `switch`. P4 wraps `auto_reply` in `retry`; urgent branch wraps `human_review` in `timeout(24h)` to prevent indefinite hang.                                    |
| `SystemCodeReview`| P1 fixed `targetStatus`. P4 wraps `review` in `timeout(15m)` and `retry(max=2)`; `on_exhausted: route:manual_escalation` to a new `human_review` branch.                        |
| `CodeFixer`       | P1 fixed references. P4 wraps `validate` in `retry`; `execute` wrapped in `timeout`.                                                                                           |

All rewritten templates bump to `node_matrix_version=2`.

**v1 retention (correcting the earlier proposal):** `workflow_executions.workflow_id` has `ON DELETE CASCADE` against `workflow_definitions(id)`. Dropping v1 definition rows would cascade-destroy historical execution history, cost rows, memory rows, and review rows that reference those runs. Therefore:

- v1 definition rows are **retained permanently** with `status='deprecated'` so historical `workflow_executions` stay readable.
- v1 definitions are view-only (cannot be instantiated): `WorkflowDefinitionRepository.Instantiate` rejects rows with `status='deprecated'` at the service layer, not via DB delete.
- No FK change, no cascade-delete risk.

### 9.1 ContentCreation: `loop` vs `map` post-P4

ContentCreation keeps the `loop` node type. Iterative polish semantics (writer → editor → writer → editor until approved or max_iterations) are fundamentally stateful: each iteration depends on the prior attempt's feedback. `map` is stateless parallel-over-collection and cannot express this. The P2-f atomic-counter fix applies to this loop. The P4 change on this template is the `retry` wrap on `editor` and `timeout` wrap on `seo`; the loop body is untouched.

## 10. Migration and Release Order

1. **Pre-flight migration.** New columns + tables: `idempotency_key` with partial unique index, `node_matrix_version`, `wait_event_deliveries`.
2. **P1 lands on master.** Template files + new tests only. `node_matrix_version` stays at `1`.
3. **P2 lands.** Engine hardening. The engine continues to execute `node_matrix_version=1` definitions (back-compat).
4. **P3 lands.** New node types registered in `nodetypes/registry.go`. Templates do not yet use them; each new handler has ≥ 90% unit coverage.
5. **P4 lands.** Templates bump to `node_matrix_version=2`. Run the drain workflow (next paragraph). Old seeds kept with `status=deprecated`. `AllSystemTemplates()` returns the new versions.

### Drain Workflow

New binary `cmd/workflow-drain/main.go`:

```
workflow-drain --dry-run   # list executions in states {running, waiting} with node_matrix_version=1
workflow-drain --execute   # fail-fast those executions and emit one audit event per run
```

The operator confirms the dry-run list before executing. In the internal-test environment this is acceptable; a stronger "freeze + migrate" path is deferred to when external users exist.

## 11. Backlog (captured, not shipped this cycle)

- Frontend visual editor polish (node inspector form, inline validation, live overlay) — scope-E
- `/debug` step-level aggregation panel — scope-D
- Additional node types: `delay`, `try_catch`, `emit_event`
- Multi-level `map` (map inside map)
- Cross-workflow templateVar sharing
- Per-tenant concurrency quotas inside `map`

## 12. Testing Matrix

| Layer            | Coverage                                                                                                                                                          |
| ---------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Contract         | `NodeContract` field schema tests; `ValidateDefinition` negative cases (unknown node_id / templateVar / secret).                                                  |
| Unit (handler)   | Every node type (existing 16 + new 5): golden path, error, ctx cancel, idempotent re-entry.                                                                       |
| Integration (DAG)| Idempotency same-key returns existing run; cancel cascade 3 levels deep; wait_event duplicate delivery short-circuits; loop counter recovers after applier crash. |
| Template E2E     | Per-template scenarios (explicit list — not every template has retry or timeout surface): see §12.1.                                                              |
| Secret           | LLM / HTTP / IM handlers — outbound requests carry clear value, logs and DataStore never do.                                                                      |
| Regression       | Existing 20+ tests (including `workflow_legacy_dag_invocation_test.go`) stay green.                                                                                |

### 12.1 Per-Template E2E Scenarios

| Template           | Golden | Failure (business)       | Retry-exhausted         | Timeout fire         | Cancel cascade       |
| ------------------ | ------ | ------------------------ | ----------------------- | -------------------- | -------------------- |
| PlanCodeReview     | ✔      | coder LLM bad output     | n/a (no retry wrap)     | ✔ (30m outer)        | ✔ (split branch)     |
| Pipeline           | ✔      | coder LLM bad output     | ✔ (coder retry=2)       | n/a                  | n/a                  |
| Swarm              | ✔      | one branch fails         | n/a                     | ✔ (30m outer)        | ✔                    |
| ContentCreation    | ✔      | editor never approves    | ✔ (editor retry)        | ✔ (seo timeout)      | n/a                  |
| CustomerService    | ✔ urgent + ✔ normal | classify bad output | ✔ (auto_reply retry) | ✔ (urgent 24h)  | n/a                  |
| SystemCodeReview   | ✔      | decision request_changes | ✔ (review retry=2 → escalate) | ✔ (15m review) | n/a              |
| CodeFixer          | ✔ prebaked + ✔ generated | validate fails | ✔ (validate retry)      | ✔ (execute timeout)  | n/a                  |

Total: 22 distinct E2E runs. Cell `n/a` means that template does not wrap the relevant behaviour, so the scenario does not exist — not a gap.

## 13. Definition of Done

- Seven built-in templates: zero `TODO`, zero dead references, full E2E suite green.
- Every node handler declares `NodeContract`; `ValidateDefinition` returns zero issues for all built-in templates.
- Idempotency, cancel cascade, wait_event de-duplication, secret non-leakage — each has at least one E2E proof.
- `grep -r 'edge.Condition' src-go/` in production code only reads for visual layout, never for routing decisions.
- Each of the five new node types has ≥ 90 % unit-test coverage.
- Migration script rehearsed in CI drain environment without issue.

## 14. Risks

- **Breaking `edge.Condition` routing** may surprise any external consumer reading the field as routing. Mitigation: release note + grep audit before P4; scheduler only stops routing-by-`edge.Condition` in P4 and only for `node_matrix_version=2` definitions — v1 runs continue using `edge.Condition` for their lifetime (the scheduler branches on version).
- **`node_matrix_version=2` definition cutover** must not cascade-destroy history. Mitigation: v1 definition rows retained permanently with `status='deprecated'` (§9 v1 retention); no FK cascade fires; historical `workflow_executions`, `cost`, `memory`, `reviews` stay readable indefinitely.
- **Cancel cascade timing.** Single authoritative SLA: 10 s is a best-effort target, 30 s is the hard force-terminate (§7.2). Tests assert the 30 s ceiling; dashboards track the 10 s target.
- **Secret leakage via new effects.** Mitigation: `RetryInvokePayload`, `TimeoutSchedulePayload`, `SpawnMapIterationsPayload`, and `CancelChildPayload` (§7.9) carry `SecretRef` only; the materialised value never traverses persisted rows. The non-leakage E2E suite (§5.3 non-leakage surfaces) greps the `events`, `logs`, `workflow_node_executions`, and `workflow_executions.data_store` rows for known fixture secrets and asserts zero matches.

## 15. References

- `docs/PRD.md`
- `src-go/internal/workflow/nodetypes/*.go`
- `src-go/internal/service/dag_workflow_service.go`
- `src-go/internal/service/workflow_templates.go`
- `src-go/internal/workflow/template/secret_resolver.go`
- Memory: Unified Workflow Engine Redesign (Phases 1-5 delivered)
- Memory: API stability stage (breaking changes permitted internal-test)
