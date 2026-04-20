## ADDED Requirements

### Requirement: Workflow runs can declare an acting employee

The workflow run record SHALL support an optional acting-employee attribution that identifies which Digital Employee the run represents. When a workflow run is started through any execution engine (DAG workflow or legacy workflow plugin) and an acting employee is provided, the system MUST persist the employee identifier on the run record so consumers can query runs by acting employee without joining through spawned agent runs.

#### Scenario: Run record persists acting employee when provided
- **WHEN** a workflow run is started with a non-null acting employee identifier
- **THEN** the run record persists that employee identifier on an `acting_employee_id` column
- **THEN** subsequent reads of the run DTO expose the acting employee identifier

#### Scenario: Run record retains null acting employee when not provided
- **WHEN** a workflow run is started without an acting employee identifier
- **THEN** the run record's `acting_employee_id` column remains null
- **THEN** spawned agent runs attribute only to any explicit step-level employee overrides

### Requirement: Step-level employee overrides supersede run-level defaults

When a workflow step (DAG node or legacy plugin step) declares an explicit employee identifier, the step-level identifier SHALL be used for the spawned run. Only when the step omits an explicit employee identifier does the run-level `acting_employee_id` flow through to the spawned run. When both are absent, the spawned agent run's `employee_id` remains null (current behavior preserved).

#### Scenario: Step-level override wins over run-level default
- **WHEN** a run declares `acting_employee_id = A` and a step declares an explicit `employee_id = B`
- **THEN** the spawned agent run's `employee_id` equals B

#### Scenario: Step falls back to run-level default
- **WHEN** a run declares `acting_employee_id = A` and a step does NOT declare any `employee_id`
- **THEN** the spawned agent run's `employee_id` equals A

#### Scenario: Both unset preserves current behavior
- **WHEN** a run has `acting_employee_id = null` and a step has no `employee_id`
- **THEN** the spawned agent run's `employee_id` remains null

### Requirement: Trigger-initiated runs propagate the trigger's acting employee

When an external trigger (IM, schedule, or any future source served by the unified dispatch router) fires, the router SHALL read the trigger row's `acting_employee_id` and pass it to the engine adapter that starts the run. The started run record MUST receive that identifier as its run-level default.

#### Scenario: IM trigger with acting employee propagates into run record
- **WHEN** an IM event fires a trigger whose `acting_employee_id = E`
- **THEN** the started workflow run record's `acting_employee_id` equals E
- **THEN** every step spawned without an explicit step-level override attributes its agent run to E

#### Scenario: Schedule trigger without acting employee leaves run-level default null
- **WHEN** a schedule tick fires a trigger whose `acting_employee_id` is null
- **THEN** the started workflow run record's `acting_employee_id` is null

### Requirement: Cross-project employee references are rejected with structured reasons

The system SHALL reject every reference to an employee whose `project_id` differs from the referencing workflow's `project_id`. Rejection MUST occur at two points: at author time (trigger sync, node-config save, plugin manifest validation) and at dispatch time. Rejections MUST produce a structured non-success outcome identifying the mismatched project; they MUST NOT silently fall back to a null attribution.

#### Scenario: Trigger sync rejects cross-project employee reference
- **WHEN** a trigger sync request binds an employee whose project does not match the workflow's project
- **THEN** the sync response reports a structured rejection that identifies the cross-project mismatch
- **THEN** the trigger row is not persisted in the enabled state with that binding

#### Scenario: Dispatch-time rejection for archived employee target
- **WHEN** a trigger with `acting_employee_id = E` fires and employee E is currently in the `archived` state
- **THEN** the router returns a structured non-success outcome identifying the archived-employee condition
- **THEN** no workflow run is started

#### Scenario: Paused employee attribution is allowed
- **WHEN** a trigger with `acting_employee_id = E` fires and employee E is in the `paused` state (not archived)
- **THEN** the trigger dispatches and the run record's `acting_employee_id` equals E

### Requirement: Run-level acting employee is observable on run read APIs

Workflow run read APIs (DAG workflow execution read, legacy plugin run read) SHALL expose the run-level `acting_employee_id` in their response DTO. Consumers MUST be able to list runs filtered by acting employee without joining to spawned agent runs.

#### Scenario: DAG run read exposes acting employee
- **WHEN** a caller reads a DAG workflow execution that has a non-null `acting_employee_id`
- **THEN** the response DTO includes the acting employee identifier

#### Scenario: Plugin run read exposes acting employee
- **WHEN** a caller reads a legacy workflow plugin run that has a non-null `acting_employee_id`
- **THEN** the response DTO includes the acting employee identifier

#### Scenario: List filter by acting employee returns matching runs
- **WHEN** a caller lists workflow runs filtered by an acting employee identifier
- **THEN** the response returns only runs whose run-level `acting_employee_id` matches
