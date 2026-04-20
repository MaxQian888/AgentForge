## Context

The `workflow` step action in `workflow_step_router.go:185-211` takes `trigger.pluginId` (or step config's `plugin_id`) and calls `e.workflows.Start(ctx, childPluginID, WorkflowExecutionRequest{Trigger: ...})`. The `WorkflowChildSpawner` interface (`workflow_step_router.go:26-28`) is typed by `pluginID string` — inherently single-engine. The output envelope reports `child_run_id`, `child_plugin`, and `status`; any consumer reading these keys from a downstream step expects them to be present.

After `bridge-sub-workflow-invocation` lands, the DAG side has the complete picture: a DAG node parks on sub-invocation, a parent-link row persists `(parent_execution_id, parent_node_id, child_engine_kind, child_run_id)`, the DAG engine's `OnTerminalState` path resumes the parent. We want the legacy `workflow` action to gain the same capability when the child is a DAG workflow, using the same linkage table and the same resume contract, but with `parent_kind='plugin_run'` so the resume seam knows where to resume.

Today's `workflow_step_router` tests (`workflow_step_router_test.go`) exercise the plugin-only child path. They will continue to pass because plugin-id input is the default — we only add a branch for the DAG target.

## Goals / Non-Goals

**Goals:**
- A legacy workflow plugin can embed a DAG workflow as a child of its `workflow` step, with the parent step parking while the DAG child runs and resuming when the DAG child reaches a terminal state.
- The parent↔child linkage is persisted in the same table used by `bridge-sub-workflow-invocation`, just with `parent_kind='plugin_run'` so the engines agree on one linkage schema.
- Existing plugin manifests and existing `workflow` action consumers continue to work unchanged — the change is purely additive at the input surface.
- Recursion guard and same-project guard apply symmetrically to cross-engine invocation (plugin → DAG).

**Non-Goals:**
- Rewriting `WorkflowChildSpawner` into a generic N-engine registry on the plugin side. We add exactly one new branch — legacy plugin invoking a DAG — via the `TargetEngine` registry introduced in `bridge-trigger-dispatch-unification`.
- Extending plugin manifests with arbitrary new fields. Only the `workflow` step's target discriminator changes.
- Propagating DAG-side execution context (node-level `dataStore`) into the legacy parent step's output beyond what already fits the existing envelope (`child_run_id`, `status`, plus new `child_engine`).
- Back-compat cruft for the legacy `plugin_id`-only shape — it stays the default and most plugins keep using it.

## Decisions

### Decision 1: Parent side reuses `workflow_run_parent_link` with a `parent_kind` column

**Chosen**: Extend the `workflow_run_parent_link` table (introduced by `bridge-sub-workflow-invocation`) with `parent_kind TEXT NOT NULL DEFAULT 'dag_execution'`. New rows inserted by legacy → DAG invocations set `parent_kind='plugin_run'` and use `parent_run_id` (aliased to `parent_execution_id` at query time, or renamed if the earlier change hasn't landed yet) to refer to a `workflow_plugin_run.id`. Resume path switches on `parent_kind`.

**Alternatives considered**:
- *Two tables, one per parent kind* — doubles repo code, doubles tests, makes cross-engine listings awkward.
- *Single-kind table and deep-link through plugin run metadata* — breaks the clean "walk the linkage" debugging story.

**Rationale**: One linkage table is simplest; one column tells the resume seam which engine owns the parent.

### Decision 2: Target-discriminator shape matches sub_workflow

**Chosen**: `workflow` action trigger input becomes:

```json
{
  "targetKind": "plugin",      // default if omitted
  "pluginId": "xyz",
  // --- OR ---
  "targetKind": "dag",
  "targetWorkflowId": "<uuid>"
}
```

When `targetKind` is omitted, the router falls back to the legacy single-field resolution (`pluginId` or `plugin_id`) for full back-compat. New authorings should pass `targetKind`.

**Rationale**: Symmetric with the DAG `sub_workflow` node payload (`InvokeSubWorkflowPayload`) so mental models stay aligned across engines.

### Decision 3: Parent plugin step parks via a new `awaiting_sub_workflow` step status

**Chosen**: The plugin runtime's step state adds an `awaiting_sub_workflow` intermediate status, analogous to `awaiting_approval`. The step router's `executeWorkflow` returns that status when the child is a DAG workflow (not when the child is a plugin — existing synchronous behavior preserved by default, though we could unify later). The plugin engine's terminal-state hook looks up parent-link rows and resumes the plugin step with the child's outcome.

**Alternatives considered**:
- *Always park for both DAG and plugin children* — changes existing plugin-child behavior (today plugin-child is synchronous). Too risky; defer unifying to a later change if wanted.

**Rationale**: Minimal-surface change: only DAG children require parking because DAG runs can be long-lived and involve human review. Plugin children stay synchronous for now.

### Decision 4: Output envelope extension with `child_engine`

**Chosen**: Existing output keys (`child_run_id`, `child_plugin`, `status`) stay. New key `child_engine` (`"dag" | "plugin"`) added; `child_plugin` is only populated when `child_engine='plugin'`. A new `child_workflow_id` key is populated when `child_engine='dag'`.

**Rationale**: Additive; existing downstream consumers reading `child_run_id` keep working; ones that want engine awareness read the new keys.

### Decision 5: Cycle guard uses the existing bounded chain walk

**Chosen**: Before dispatching the DAG child, walk the parent link chain (with `parent_kind` respected — plugin_run nodes contribute to the chain) up to the same depth (`maxSubWorkflowDepth = 8`) used by `bridge-sub-workflow-invocation`. Reject cycles consistently regardless of which engine's run is the "parent" at each hop.

**Rationale**: One cycle-guard policy across the whole sub-invocation graph — prevents an operator from creating `DAGa → PluginB → DAGa` via the cross-engine path.

## Risks / Trade-offs

- **Risk**: The plugin step-state machine is not today designed for long parks. → **Mitigation**: Add `awaiting_sub_workflow` as an explicit new status value guarded by a narrow state transition; cancel-while-parked already has a documented path in `workflow-plugin-runtime` scenario "Failed step retry is recorded" — the same persistence discipline applies.
- **Risk**: Existing plugin-manifest static validators (if any) might reject the new `{targetKind, targetWorkflowId}` shape as unknown fields. → **Mitigation**: Extend the manifest schema JSON in `workflow-plugin-runtime` scope as part of this change.
- **Risk**: Cancelling the parent plugin run while a DAG child is running must also cancel the DAG child (or at least detach the linkage cleanly). → **Mitigation**: Covered by the cancellation hook introduced in `bridge-sub-workflow-invocation`, extended here to call DAG cancellation when a plugin parent is cancelled and still parked.
- **Trade-off**: Keeping plugin children synchronous while DAG children park creates two slightly different semantics for the `workflow` action depending on child kind. This is an explicit decision — unifying will be its own change if a real use case demands it.

## Migration Plan

1. Extend `workflow_run_parent_link` with `parent_kind` column (default `'dag_execution'` for forward compat). Requires a migration.
2. Widen `WorkflowChildSpawner` interface, OR introduce a new `crossEngineChildStarter` alongside the existing interface and choose which to call in `executeWorkflow` based on `targetKind`. (Design prefers the second: additive, less disruption.)
3. Wire the new starter to the `TargetEngine` registry (DAG adapter).
4. Add `awaiting_sub_workflow` plugin step status and corresponding persistence path.
5. Extend plugin-engine terminal-state hook to respect `parent_kind` when resuming.
6. Extend cancellation hooks symmetrically.
7. Extend manifest validation to accept the new step shape.
8. Update `workflow_step_router_test.go` and `workflow_plugin_runtime` tests.

Rollback is clean: the new column is additive; the new target shape is additive; rolling back the binary and dropping the column disables the new path without orphaning data.

## Open Questions

- Should plugin-child invocations also be unified to park (removing the synchronous-plugin-child special case)? Deferred; requires migration of existing plugin-child semantics. Not in this change's scope.
- Do we want to propagate DAG node-level outputs into the parent plugin step's trigger payload for downstream steps to read? Current answer: the output envelope's `child_run_id` is enough — consumers can deep-fetch the child's run via the unified detail endpoint when they need more.
