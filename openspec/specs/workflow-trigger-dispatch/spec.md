# workflow-trigger-dispatch

## Purpose

Defines the shared contract for matching, deduplicating, and dispatching external trigger events (IM slash commands, schedule cron ticks, and any future trigger source built atop the same router) to either the DAG workflow engine or the legacy workflow plugin runtime. Covers target-kind selection on each trigger record, input-mapping templating, idempotency semantics, and the structured non-success outcomes consumers see when a trigger cannot be executed.

This capability exists so that both workflow execution engines (`workflow-engine` and `workflow-plugin-runtime`) can be first-class trigger targets without either subsystem having to know how external events reach them.

## Requirements

### Requirement: External trigger events dispatch to a declared target workflow engine

The workflow trigger router SHALL dispatch matched external trigger events (IM slash commands, schedule cron ticks, and any future trigger source implemented atop the same router) to the execution engine declared on each trigger record. Each `workflow_trigger` row MUST declare a `target_kind` that identifies the execution engine, and the dispatch router MUST consult this field — not infer the engine from workflow identifiers — when selecting the adapter that starts the workflow run.

#### Scenario: Trigger declaring the DAG engine starts a DAG execution
- **WHEN** an IM event matches a trigger whose `target_kind` is `dag`
- **THEN** the router starts a DAG workflow execution through the DAG adapter and records the resulting `WorkflowExecution` identifier on the trigger outcome

#### Scenario: Trigger declaring the plugin engine starts a workflow plugin run
- **WHEN** an IM event matches a trigger whose `target_kind` is `plugin`
- **THEN** the router starts a legacy workflow plugin run through the plugin adapter and records the resulting `workflow_plugin_run` identifier on the trigger outcome

#### Scenario: Trigger with unknown target kind never dispatches
- **WHEN** a trigger row declares a `target_kind` value that the router has no registered adapter for
- **THEN** the router MUST return a structured non-success outcome with a machine-readable reason identifying the unknown target kind
- **THEN** no workflow run is started for that trigger

### Requirement: Match-filter, input mapping, and idempotency are engine-agnostic

The workflow trigger router SHALL apply the same match-filter, input-mapping, and idempotency semantics before dispatching to any target engine. The rendered idempotency key MUST collide across target kinds for a single event so that two triggers matching the same event produce at most one fire per rendered key per dedupe window — regardless of which engine each trigger targets.

#### Scenario: Two triggers targeting different engines share idempotency
- **WHEN** one IM event matches two enabled triggers that render the same idempotency key within the dedupe window, where one trigger targets `dag` and the other targets `plugin`
- **THEN** the router fires at most one dispatch across both engines for that rendered key during the dedupe window
- **THEN** subsequent dispatches for the same key within the window are suppressed with a structured idempotency outcome

#### Scenario: Input mapping templating resolves identically per engine
- **WHEN** two triggers with the same `input_mapping` template match the same event, targeting different engines
- **THEN** both triggers receive input seeds that render from the same `{{$event.*}}` expressions against the same event payload

### Requirement: Trigger enablement validates target engine resolvability

When the registrar synchronises `workflow_triggers` from a workflow save, it SHALL validate that the referenced workflow exists in the declared target engine and is currently executable. Triggers whose target cannot be resolved MUST be persisted in a disabled state with a structured reason; they MUST NOT silently remain enabled.

#### Scenario: Plugin-target trigger for a disabled plugin is persisted disabled
- **WHEN** the registrar syncs a trigger with `target_kind='plugin'` whose referenced plugin is disabled in the plugin runtime
- **THEN** the trigger row is persisted with `enabled=false` and a structured `disabled_reason` that identifies the unresolvable plugin target
- **THEN** the sync response surfaces the structured disabled reason to the caller

#### Scenario: DAG-target trigger for a missing workflow is rejected
- **WHEN** the registrar syncs a trigger with `target_kind='dag'` whose referenced workflow identifier does not exist in the DAG workflow definitions store
- **THEN** the trigger row is persisted with `enabled=false` and a structured `disabled_reason` that identifies the unresolvable DAG target
- **THEN** the sync response surfaces the structured disabled reason to the caller

### Requirement: Trigger outcomes expose the fired engine and run reference

For every matched trigger, the router SHALL emit a structured outcome that identifies which execution engine fired, which workflow run was started (if any), and the normalized status of the dispatch. The outcome schema MUST be consistent across trigger sources (IM, schedule, and future sources) so downstream consumers do not have to switch on trigger source to interpret the outcome.

#### Scenario: Successful dispatch outcome includes engine kind and run id
- **WHEN** the router successfully starts a workflow run for a matched trigger
- **THEN** the emitted outcome includes the `target_kind` (`dag` or `plugin`) and the started run identifier

#### Scenario: Failed dispatch outcome is structured, not silent
- **WHEN** the adapter for the trigger's target engine returns an error at dispatch time
- **THEN** the emitted outcome records a non-success status with a machine-readable reason
- **THEN** no run identifier is reported and the idempotency key is NOT marked as consumed for this dispatch

### Requirement: Trigger target kind defaults preserve existing DAG behavior

The trigger schema SHALL default `target_kind` to `dag` for any trigger persisted without an explicit target. Existing trigger rows at the time this capability is introduced MUST continue to fire DAG executions unchanged.

#### Scenario: Pre-existing trigger fires DAG execution after upgrade
- **WHEN** a trigger row that existed before this capability was introduced matches an event after the capability is shipped
- **THEN** the router treats its `target_kind` as `dag`
- **THEN** the dispatched workflow run is a DAG workflow execution identical in semantics to the pre-upgrade dispatch

### Requirement: Workflow triggers can declare an acting employee

Each `workflow_trigger` row SHALL support an optional `acting_employee_id` that references a Digital Employee within the same project as the trigger's target workflow. When the trigger fires, the dispatch router MUST pass the `acting_employee_id` through to the engine adapter so the started workflow run persists that identifier as its run-level default.

#### Scenario: Trigger with acting employee produces attributed run record
- **WHEN** a trigger whose `acting_employee_id = E` fires through the unified dispatch router
- **THEN** the started run record (DAG workflow execution or legacy workflow plugin run) persists `acting_employee_id = E`

#### Scenario: Trigger without acting employee produces unattributed run record
- **WHEN** a trigger with `acting_employee_id = null` fires
- **THEN** the started run record persists `acting_employee_id = null`

### Requirement: Trigger author-time validation rejects cross-project employees

When the registrar syncs a trigger that declares `acting_employee_id`, it SHALL resolve the employee identifier against the employees registry and confirm the employee's project matches the workflow's project. Mismatched or unresolvable employee references MUST cause the trigger to be persisted in a disabled state with a structured `disabled_reason`.

#### Scenario: Cross-project employee reference disables trigger at sync time
- **WHEN** the registrar syncs a trigger whose `acting_employee_id` belongs to a different project than the referenced workflow
- **THEN** the trigger row is persisted with `enabled = false` and a structured `disabled_reason` identifying the cross-project mismatch

#### Scenario: Unknown employee reference disables trigger at sync time
- **WHEN** the registrar syncs a trigger whose `acting_employee_id` resolves to no employee record
- **THEN** the trigger row is persisted with `enabled = false` and a structured `disabled_reason` identifying the unresolvable employee

### Requirement: Dispatch-time employee validation blocks archived targets

At dispatch time, if a trigger's `acting_employee_id` references an archived employee, the dispatch router MUST return a structured non-success outcome and MUST NOT start a workflow run. Paused employees remain valid attribution targets.

#### Scenario: Archived acting employee blocks dispatch
- **WHEN** a trigger whose `acting_employee_id = E` fires and employee E is in the `archived` state
- **THEN** the dispatch router returns a structured non-success outcome identifying the archived-employee condition
- **THEN** no workflow run is started

#### Scenario: Paused acting employee permits dispatch
- **WHEN** a trigger whose `acting_employee_id = E` fires and employee E is in the `paused` state
- **THEN** the dispatch router starts the workflow run and the run record's `acting_employee_id` equals E
