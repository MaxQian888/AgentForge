## Context

DAG workflows today cannot nest. The `sub_workflow` handler at `sub_workflow.go:9` is a "fail-fast stub" with a matching test (`sub_workflow_test.go:10` → `TestSubWorkflowHandler_ExecuteReturnsNotImplemented`) asserting the current behavior. The applier has a symmetric stub at `applier.go:350-359` with a `log.Printf("[WARN] ... TODO: sub_workflow not wired")`. Without sub-workflow invocation, a DAG author cannot reuse a curated workflow plugin as a building block — they must reproduce the plugin's steps inline, which defeats the plugin runtime's purpose.

Parking / resumption is not a novel mechanic for the DAG engine — `human_review` nodes already park for approval (`workflow_step_router.go:213-221` emits `awaiting_approval`; the applier/service handle resume via the review-resolution endpoint `workflow_handler.go:596-618`). A sub-workflow invocation is the same mechanic with a different resumption source: instead of a human review decision, the parent resumes when the child run reaches a terminal state. The engine's existing `AdvanceExecution` loop already re-checks parked nodes when called from an event path (`dag_workflow_service.go:475` from external event, `:701` from a resume).

Cycle safety matters because sub-workflows can reference each other directly (A → B → A) or transitively (A → B → C → A). The invocation chain must be tracked — either as a persisted chain on the child run record or as a defensive pre-dispatch walk.

## Goals / Non-Goals

**Goals:**
- A DAG `sub_workflow` node can invoke either another DAG workflow or a legacy workflow plugin as a child run.
- The parent node parks while the child runs and resumes when the child reaches a terminal state.
- The child run's outputs are reified into the parent's `dataStore` under the `sub_workflow` node's id, keyed by the canonical child-run output envelope.
- Recursion that would produce a cycle is rejected with a structured non-success outcome at invocation time (not lazy failure after a dispatch).
- The parent↔child linkage is persisted and exposed on run read DTOs so debugging can walk the invocation tree.

**Non-Goals:**
- Fire-and-forget sub-workflows where the parent does not wait. The payload exposes `WaitForCompletion` for future expansion, but the initial delivery always treats it as `true`. (Tracked as an "Open Question" — see below.)
- Partial data mapping (cherry-picking child outputs). Initial delivery materializes the full child output envelope; selective mapping is a future concern.
- Cross-project sub-workflow invocation. The child workflow must be in the same project as the parent. (Cross-project composition is a distinct security concern handled later if ever.)
- Editor UX for visualizing nested execution trees in the canvas. A textual linkage in the execution detail panel is enough for this change.

## Decisions

### Decision 1: Reuse the `TargetEngine` registry from `bridge-trigger-dispatch-unification`

**Chosen**: The applier resolves the child engine via `TargetEngine.Kind()` lookup and calls `Start(ctx, workflowID, seed, invocationID)`. The same code path triggers use, guaranteeing both entrypoints agree on how to spin up child runs across engines.

**Alternatives considered**:
- *Hardcode a switch on `payload.TargetKind` inside the applier* — duplicates the trigger router's dispatch logic and means any future engine has to change two places.

**Rationale**: Single source of truth for "how do we start a workflow run in engine X." The `Start` call from the trigger router and the one from the applier are semantically the same dispatch.

### Decision 2: Park at effect-apply time, resume by external completion signal

**Chosen**: When the applier processes `EffectInvokeSubWorkflow`:
1. Resolve target and validate (project match, recursion guard).
2. Insert a parent-link row: `(parent_execution_id, parent_node_id, child_run_id, child_engine_kind, status='running')`.
3. Call `TargetEngine.Start(...)` to get `childRunID`, update parent link row.
4. Mark the parent `WorkflowNodeExecution.status = 'awaiting_sub_workflow'` and return `parked=true` (parallel to the `human_review` and `wait_event` mechanics).

Resume path:
- DAG engine: when any DAG execution reaches a terminal state, check if a parent link row references it; if yes, materialize child outputs into parent's `dataStore[parent_node_id]`, mark parent node complete/failed per child outcome, and call `AdvanceExecution(parent_execution_id)`.
- Plugin engine: the same check on legacy plugin-run terminal transitions. Implementation attaches to the plugin runtime's existing terminal-state emission (per `workflow-plugin-runtime` "Workflow completion emits a terminal run state").

**Alternatives considered**:
- *Parent polls child periodically* — wasteful and racy; we already have terminal-state emissions.
- *Child explicitly notifies parent* — requires child runtimes to know they are children, which leaks the parent→child coupling into leaf engines.

**Rationale**: Parks cleanly using existing engine primitives; resumes on the event the engines already emit.

### Decision 3: Cycle guard is a pre-dispatch chain walk

**Chosen**: Before inserting the parent-link row, walk from the parent execution up through the `workflow_run_parent_link` table (iterative, bounded by a max depth of N=8 to cap worst-case latency). If any ancestor's `workflow_id` equals the child's target `workflow_id`, reject with a structured outcome.

**Alternatives considered**:
- *Persist a denormalized chain array on each run* — faster check but every insert has to update downstream chains on resume; error-prone.
- *No cycle guard; rely on an overall run-count limit* — unsafe for long-running plugin workflows that could burn resources before the limit trips.

**Rationale**: A bounded walk is O(depth) with a small constant. Simpler to maintain than denormalized chains. The N=8 cap matches human comprehensibility of nested call stacks.

### Decision 4: Child outputs materialize under parent node id in `dataStore`

**Chosen**: When a child completes successfully, the applier writes `parent.exec.dataStore[parent.node.id] = { subWorkflow: { runId, engine, outputs } }`. The `outputs` shape equals the canonical run envelope exposed by the run read DTO. No mapping or cherry-picking in this version.

**Rationale**: Mirrors how other node types already write to `dataStore`. A downstream DAG node can reach into `$.dataStore[<sub_workflow_node_id>].subWorkflow.outputs.*` via the existing template syntax.

### Decision 5: Same-project constraint enforced at save time and dispatch time

**Chosen**: Both the node-config save path (in `workflow_handler.go` on workflow update) and the applier's pre-dispatch validation reject a `sub_workflow` node whose target workflow belongs to a different project than the parent workflow. Matches the scoping stance adopted in `bridge-employee-attribution-legacy`.

## Risks / Trade-offs

- **Risk**: If the child run is cancelled externally while the parent is parked, the parent could hang indefinitely. → **Mitigation**: The cancellation hook on `WorkflowExecution` / `workflow_plugin_run` must also trigger parent resumption with `failed` status. Covered in tasks.
- **Risk**: Terminal-state listener in the plugin engine does not exist as a clean hook today. → **Mitigation**: Add a small exported `OnTerminalState` callback seam on the plugin runtime service — same seam that `bridge-trigger-dispatch-unification` adds `StartTriggered` on. Narrow, testable.
- **Risk**: Parent execution might be modified (edge conditions evaluated, data store mutated) between park and resume; `AdvanceExecution` must be idempotent on already-completed parent nodes. → **Mitigation**: Existing behavior — `WorkflowNodeExecution.status` transitions guard against double-completion. Apply the same guard to sub-workflow resumption.
- **Trade-off**: We materialize the entire child output envelope into the parent's `dataStore`. Large outputs inflate the parent run's JSON payload. Accepted for now; a future `OutputProjection` field on the payload can narrow this if it becomes a problem.
- **Trade-off**: Same-project constraint prevents a marketplace-distributed plugin from being invoked across projects. Acceptable because the plugin runtime already resolves plugins per project; cross-project invocation is a security concern we are not ready to take on.

## Migration Plan

1. Add migration introducing `workflow_run_parent_link` table with columns `(id, parent_execution_id, parent_node_id, child_engine_kind, child_run_id, status, started_at, terminated_at)` + unique index on `(parent_execution_id, parent_node_id)`.
2. Extend `InvokeSubWorkflowPayload` with `TargetKind`, `InputMapping`, `WaitForCompletion` (always true in v1).
3. Replace `SubWorkflowHandler.Execute` stub with real validation + effect emission.
4. Wire `applier.go:applyInvokeSubWorkflow` to perform pre-dispatch validation, recursion walk, parent-link insert, engine start, and node-parking.
5. Extend DAG service's terminal-state path to look up parent-link rows and resume parents.
6. Add `OnTerminalState` seam on plugin runtime; extend its terminal-state path similarly.
7. Add cancellation hook for both engines to failure-resume parked parents.
8. Update DAG editor config panel for `sub_workflow` nodes.
9. Add tests per "Verification impact."

Rollback: drop the new table and revert `sub_workflow` handler to its stub; no data backfill required because no live sub-workflow runs exist before this change.

## Open Questions

- Do we want to support fire-and-forget sub-workflows (where parent continues immediately without waiting)? Currently out of scope; `WaitForCompletion` flag is reserved for a future change if real use emerges. If added, it will not require schema changes — only alternate applier branch.
- How should the WebSocket broadcast represent parent↔child linkage? (Expected: emit a `sub_workflow.started` event carrying child run ref; let the frontend treat it as any other run start. Treated as follow-up; not required for this change's correctness.)
- Should we expose a dedicated read API `/executions/{id}/tree` that walks parent links for UI rendering? (Not in this change; a small read helper is sufficient for v1.)
