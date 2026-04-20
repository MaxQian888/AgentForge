## Why

The newly-added Digital Employee subsystem (`src-go/internal/model/employee.go`, `src-go/internal/employee/service.go`) already binds to DAG workflows through the `llm_agent` node's optional `employeeId` config, and `model.AgentRun.EmployeeID` is already the FK that attributes a run to its originating employee. However the **legacy workflow step router** (`src-go/internal/service/workflow_step_router.go`) only threads `memberId` through its `agent` / `review` / `task` step actions, and the **trigger router** (`src-go/internal/trigger/router.go`) only records `TriggeredBy *uuid.UUID` against a trigger row — neither surface accepts or propagates `employeeId`. As a consequence, any run spawned through a legacy workflow plugin or through a trigger-initiated dispatch has `agent_runs.employee_id = NULL` even when the operator clearly intends the run to "represent" a specific silicon employee. That creates a factual gap in attribution and blocks the two-engine complementary story: a plugin workflow and a triggered workflow can never be said to have been run "by" an employee, even though the data model already has the column.

## What Changes

- Extend the legacy workflow step router contract so `agent`, `review`, and `task` step actions accept an optional `employee_id` input (alongside the existing `member_id`) and propagate it into the spawned `AgentRun` / review / task dispatch.
- Extend trigger dispatch so each `workflow_trigger` row can declare an optional `acting_employee_id`; when present, the router passes it into the initiated workflow run (DAG and plugin alike) so downstream step executions can attribute their spawned runs to that employee without every step having to repeat the field.
- Add an `acting_employee_id` column to `workflow_executions` and `workflow_plugin_run` (or equivalent legacy run table) so a run is unambiguously attributed to its originating employee and the column is queryable directly on the run record.
- Update the DAG workflow engine's step executor to forward `acting_employee_id` from the run record into any downstream step that does not override it — preserving per-step explicit overrides while providing a run-level default.
- Guard employee attribution: only employees active in the same project as the workflow / trigger MAY be referenced; mismatched or archived employee references MUST produce a structured rejection (no silent drop, no null fallback).
- Update the trigger UI (`components/workflow/workflow-triggers-section.tsx`) and the trigger store to read/write `actingEmployeeId`, defaulting to unset.

## Capabilities

### New Capabilities
- `employee-runtime-attribution`: Defines how a workflow run, workflow step, or trigger-initiated dispatch attributes its spawned agent runs to a Digital Employee — including inheritance from run → step, explicit per-step overrides, cross-project guards, and persistence of the originating employee on run records.

### Modified Capabilities
- `workflow-engine`: Adds `employee_id` to the legacy step router's `agent`, `review`, and `task` action payloads and requires the router to propagate it into the spawned run.
- `workflow-trigger-dispatch` (pending from `bridge-trigger-dispatch-unification`): Adds an optional `acting_employee_id` on trigger rows and requires the router to thread it into the initiated run. Depends on change `bridge-trigger-dispatch-unification` landing first.

## Impact

- **Affected backend seams**: `src-go/internal/model/workflow_trigger.go`, `src-go/internal/model/workflow_definition.go` (WorkflowExecution), `src-go/internal/model/workflow_plugin_run.go`, migration adding `acting_employee_id` columns, `src-go/internal/service/workflow_step_router.go`, `src-go/internal/service/dag_workflow_service.go`, `src-go/internal/trigger/router.go`, `src-go/internal/workflow/nodetypes/llm_agent.go` (now reads run-level default), `src-go/internal/employee/service.go` (cross-project guard helper).
- **Affected consumer seams**: `lib/stores/workflow-trigger-store.ts`, `components/workflow/workflow-triggers-section.tsx`, `components/workflow-editor/config-panel/node-configs/llm-agent-config.tsx` (document that unset = inherit from run).
- **Data model impact**: `workflow_triggers.acting_employee_id UUID NULL REFERENCES employees(id)`; same-named column on `workflow_executions` and `workflow_plugin_run`. All nullable with default NULL; existing rows unaffected.
- **API impact**: Trigger create/update DTOs accept `actingEmployeeId`; run read DTOs expose it; step action trigger payloads accept `employeeId` alongside `memberId`. All additive, no removals.
- **Verification impact**: Router, dag-service, and plugin-runtime tests extended to cover employee propagation and cross-project rejection; frontend store test covers round-trip; an integration test confirms a triggered IM event with `actingEmployeeId` produces `agent_runs.employee_id` set on every spawned run across both engines.

## Dependency

This change depends on `bridge-trigger-dispatch-unification` landing first; the `actingEmployeeId` extension on triggers piggybacks on the unified dispatch path introduced there.
