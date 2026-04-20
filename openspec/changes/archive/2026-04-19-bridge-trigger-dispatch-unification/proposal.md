## Why

AgentForge now carries two workflow execution engines that are meant to complement each other: the **DAG workflow engine** (user-authored visual flows in `src-go/internal/service/dag_workflow_service.go`) and the **legacy workflow plugin engine** (declarative plugins governed by `openspec/specs/workflow-engine/spec.md` and `workflow-plugin-runtime/spec.md`). External triggers such as IM slash commands and schedule cron ticks currently land only in the DAG engine — `src-go/internal/trigger/router.go` depends on a `Starter` interface whose single method `StartExecution(ctx, workflowID, taskID, StartOptions) (*WorkflowExecution, error)` is only satisfied by `*service.DAGWorkflowService`. A registered `workflow_trigger` row therefore cannot fire a legacy workflow plugin, so operators are forced to wrap every legacy plugin behind a shim DAG workflow if they want it triggerable. That defeats the "互补 / complementary" stance: legacy plugins cannot be first-class trigger targets even though the plugin runtime is already executable.

## What Changes

- Introduce an explicit target engine selector on `model.WorkflowTrigger` so each trigger declares which engine handles it — at minimum `dag` (default, current behavior) and `plugin` (legacy workflow plugin). **BREAKING**: the workflow trigger schema gains a required `target_kind` column with a default of `dag` for in-flight rows.
- Widen the `trigger.Router` dependency contract so it can start either a DAG workflow execution or a legacy workflow plugin run, while keeping idempotency, input mapping templating, and match-filter semantics identical across both targets.
- Extend `src-go/internal/trigger/registrar.go` trigger sync logic to validate that the referenced workflow exists in the declared target engine before enabling the trigger, and to reject triggers that reference a target kind the platform does not currently support.
- Teach the IM trigger HTTP surface (`POST /api/v1/triggers/im/events`) and the schedule ticker (`schedule_ticker.go`) to dispatch through the unified router without either subsystem having to know the target engine.
- Update the workflow trigger UI store (`lib/stores/workflow-trigger-store.ts`) and the Triggers tab (`components/workflow/workflow-triggers-section.tsx`) to surface and edit `targetKind`, defaulting existing triggers to `dag`.
- Surface a structured non-success outcome (reused from `task-triggered-workflow-automation`) when a trigger references an unexecutable target, instead of silently dropping the event.

## Capabilities

### New Capabilities
- `workflow-trigger-dispatch`: Defines the shared contract for matching, deduplicating, and dispatching external trigger events (IM, schedule, and future sources) to either the DAG workflow engine or the legacy workflow plugin runtime, including target-kind selection, input mapping, idempotency, and non-success outcomes.

### Modified Capabilities
- None. `workflow-engine` and `workflow-plugin-runtime` execution semantics are not changing — only how triggers reach them.

## Impact

- **Affected backend seams**: `src-go/internal/model/workflow_trigger.go`, `src-go/internal/repository/workflow_trigger_repo.go` (plus migration), `src-go/internal/trigger/router.go`, `src-go/internal/trigger/registrar.go`, `src-go/internal/trigger/schedule_ticker.go`, `src-go/internal/handler/trigger_handler.go`, `src-go/internal/handler/workflow_handler.go` (trigger sync endpoints), `src-go/internal/service/dag_workflow_service.go` (StartExecution remains DAG-side).
- **Affected consumer seams**: `lib/stores/workflow-trigger-store.ts`, `components/workflow/workflow-triggers-section.tsx`, `src-im-bridge/commands/workflow.go` (only reads command text; should not need change).
- **Data model impact**: `workflow_triggers` table gains `target_kind TEXT NOT NULL DEFAULT 'dag'`; existing rows migrate implicitly.
- **API impact**: Trigger create/update REST payloads accept an optional `targetKind` (default `dag`); list/read responses expose it. IM and schedule event endpoints return `targetKind` in outcome metadata so operators can see which engine fired.
- **Verification impact**: router/registrar/schedule-ticker unit tests extended for both target kinds; a new integration test boots both engines and verifies a single IM event can fan out to both a DAG execution and a legacy plugin run when two triggers are registered for the same command.
