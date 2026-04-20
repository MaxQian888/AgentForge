## Context

Today the trigger router has a single-engine assumption baked into its shape. `trigger.Starter` (router.go:20-22) only declares `StartExecution(ctx, workflowID, taskID, service.StartOptions) (*model.WorkflowExecution, error)` — a signature that is DAG-specific both in argument kinds and in return type (`WorkflowExecution` is the DAG run record, not the legacy `workflow_plugin_run`). The registrar writes `workflow_triggers` rows on workflow save (`registrar.go:88` only accepts `TriggerSourceIM` and `TriggerSourceSchedule`), but nothing in the row identifies the intended execution engine, so every row is interpreted as DAG by default. Legacy workflow plugins declare their own trigger metadata through plugin manifests (per `workflow-plugin-runtime` spec) but those manifests are not reconciled with the `workflow_triggers` table today.

The existing two capabilities we must respect:
- `workflow-engine/spec.md` — the legacy `workflow_step_router` contract (`agent`, `review`, `task`, `workflow`, `approval` actions) and its run-persistence model (`workflow_plugin_run`).
- `workflow-plugin-runtime/spec.md` — workflow plugin manifests, registration, process-mode execution, retry budget.

Neither of those capabilities currently makes any claim about how external trigger events reach workflow runs — that gap is why the DAG router silently owns trigger dispatch end-to-end.

## Goals / Non-Goals

**Goals:**
- External trigger events (IM slash commands, schedule cron, future webhook) reach both DAG workflows and legacy workflow plugin runs through a single dispatch path.
- Each `workflow_triggers` row unambiguously declares its target engine.
- Match-filter, idempotency (`IdempotencyKeyTemplate` + `DedupeWindowSeconds`), and input-mapping semantics stay identical across engines.
- Unsupported or broken targets return a structured non-success outcome consistent with `task-triggered-workflow-automation` — never a silent drop.
- Adding a future source (e.g. webhook) or future engine (e.g. "mcp-workflow") does not require widening the router's engine-specific surface again.

**Non-Goals:**
- Changing DAG or legacy execution semantics (step routing, retry, approval pauses). Those stay owned by `workflow-engine` and `workflow-plugin-runtime`.
- Implementing webhook triggers or any third trigger source — this change only unifies dispatch for the two sources that are already wired.
- Building a unified run-listing API across engines — that is a separate bridge (change `bridge-unified-run-view`).
- Cross-engine sub-workflow invocation (change `bridge-sub-workflow-invocation` handles DAG→plugin calls).
- UI work beyond surfacing the new `targetKind` field in the existing Triggers tab.

## Decisions

### Decision 1: Add an explicit `target_kind` column instead of inferring engine from `workflow_id`

**Chosen**: `workflow_triggers.target_kind TEXT NOT NULL DEFAULT 'dag'` with an enum of `dag | plugin`. The trigger repo carries this field through; router matches against target-kind-aware dispatcher registries.

**Alternatives considered**:
- *Infer engine from `workflow_id`* — would require the router to consult both the DAG definitions repo and the plugin registry on every event. Adds latency and makes ambiguous-id collisions possible if both engines ever share ID space.
- *Keep DAG-only and ship a sidecar "plugin-trigger" table* — doubles the persistence surface, registrar code, and UI plumbing. Rejected as it would entrench the two-island problem we are trying to fix.

**Rationale**: An explicit column keeps the router O(1), lets the registrar validate the target before enabling, and makes the UI semantics obvious ("this trigger fires a DAG workflow / a plugin workflow").

### Decision 2: Introduce a `TargetEngine` interface with per-engine adapters

**Chosen**: Replace `Starter` with:

```go
type TargetEngine interface {
    Kind() string                                           // "dag" | "plugin"
    Start(ctx context.Context, workflowRef uuid.UUID, seed map[string]any,
          triggerID uuid.UUID) (engineRun TriggerRun, err error)
}

type TriggerRun struct {
    Engine string     // "dag" | "plugin"
    RunID  uuid.UUID  // DAG WorkflowExecution.ID or legacy workflow_plugin_run.ID
}
```

Router holds `map[string]TargetEngine` keyed by `Kind()`. A DAG adapter wraps `*service.DAGWorkflowService.StartExecution`; a plugin adapter wraps the existing plugin runtime start seam (to be introduced in this change as a small exported method on the plugin runtime service).

**Alternatives considered**:
- *Sum-type return on a single method* — forces every caller to switch on kind; leaks engine specifics.
- *Generic `StartByKind(kind, ...)` on a single service* — merges two separate lifecycles into one service, contrary to the "互补" stance.

**Rationale**: Keeps each engine's entry-point authoritative. Router becomes a pure dispatcher. New engines (e.g. future MCP workflow) register their own adapter and gain trigger support for free.

### Decision 3: Registrar validates target at enable time, not dispatch time

**Chosen**: When `registrar.go` syncs triggers from a workflow save:
- Declared `target_kind='dag'` → resolve against DAG definitions repo.
- Declared `target_kind='plugin'` → resolve against plugin runtime registry (workflow plugin must be enabled and declare an executable process mode per `workflow-plugin-runtime`).
- Unresolvable → trigger is persisted but marked `enabled=false` with a structured failure reason surfaced through the sync endpoint.

**Rationale**: Fail-at-author-time matches the existing registrar contract and keeps the hot dispatch path lean. Dispatch-time errors still produce structured non-success outcomes for races (e.g. plugin disabled between enable and fire).

### Decision 4: Keep idempotency key storage engine-agnostic

The `IdempotencyStore` already keys on a rendered template string. No change needed — the same key collides regardless of target kind, which is the right behavior (one IM message = one trigger fire per registered match, no matter which engine fires it).

### Decision 5: Non-success outcome shape is shared with `task-triggered-workflow-automation`

Reuse that capability's structured outcome object (normalized action, matched context, result status, machine-readable reason) so consumers downstream of the IM bridge and the schedule ticker see a single outcome schema regardless of trigger source.

## Risks / Trade-offs

- **Risk**: The plugin runtime does not today expose a callable `StartByManifest(ctx, pluginID, seed)` seam — it expects starts through higher-level task/review flows. → **Mitigation**: Add a minimal `WorkflowPluginRuntime.StartTriggered(ctx, pluginID, seed, triggerID)` exported method that wraps the existing `workflow-plugin-runtime` execution entry points. This is a narrow seam, tested against `workflow-plugin-runtime` scenarios; it does not add new execution semantics.
- **Risk**: Existing `workflow_triggers` rows have no `target_kind` at rollout. → **Mitigation**: Migration defaults the column to `'dag'` for in-flight rows; rollout does not require any data backfill since no non-DAG triggers can exist today.
- **Risk**: A trigger pointing at a plugin could fire repeatedly if the plugin completes instantly and the idempotency window is short. → **Mitigation**: Idempotency already applies pre-dispatch, symmetric to DAG. No new concern introduced here.
- **Trade-off**: We keep two authoritative run stores (`workflow_executions` and `workflow_plugin_run`) rather than merging them. This change accepts that cost to preserve execution-engine independence; the cross-engine view is deferred to `bridge-unified-run-view`.
- **Trade-off**: The registrar rejects unresolvable targets at save time. If a plugin is deleted later, its triggers become disabled; consumers must handle the `enabled=false` state. This matches existing behavior for disabled plugins today.

## Migration Plan

1. Add `target_kind` column via a new reversible migration (`0NN_workflow_trigger_target_kind.up.sql` / `.down.sql`). Default `'dag'`.
2. Extend `model.WorkflowTrigger` and repository.
3. Refactor `trigger.Starter` → `trigger.TargetEngine` interface; add DAG adapter and plugin adapter.
4. Update `trigger.Router` to look up engine by `trigger.TargetKind`.
5. Extend registrar validation per Decision 3.
6. Surface `targetKind` in the trigger API payloads and in the frontend store.
7. Add a new-capability spec at `specs/workflow-trigger-dispatch/spec.md`.
8. Expand tests as listed under "Verification impact" in the proposal.
9. No rollback concerns: migration is reversible; the new column is additive; trigger behavior for default rows is unchanged (all dispatch to DAG).

## Open Questions

- Should we also record `run_kind` on the trigger-fire audit event so downstream observers do not have to join two run tables? (Intent: yes, but left to `bridge-unified-run-view` so this change stays focused.)
- Does the plugin runtime expose a seed/input contract compatible with the DAG's `StartOptions.Seed` shape? (Expected yes since both carry templated `map[string]any`, but the plugin-adapter implementation task must verify at the runtime boundary.)
