# workflow-run-unified-view Specification

## Purpose
Defines the cross-engine workflow-run list/detail API contract consumed by the workflow workspace UI. Merges DAG workflow executions and legacy workflow plugin runs under a single canonical row schema with an `engine` discriminator, a filter surface (engine, status, acting employee, triggered-by, time window), cursor pagination, and a matching live-update WebSocket channel. Does not replace the per-engine deep-drill-down endpoints (`workflow-engine`, `workflow-plugin-runtime`, `workflow-trigger-dispatch`) — layers a cross-engine read/presentation surface on top of them.

## Requirements

### Requirement: Unified workflow-run list exposes both engines under one schema

The system SHALL expose an authenticated, project-scoped workflow-run list endpoint that merges DAG workflow executions and legacy workflow plugin runs into a single paginated response. Each row MUST include an `engine` discriminator (`dag` or `plugin`), a run identifier, a workflow reference (id and display name), a normalized status, started-at timestamp, optional completed-at timestamp, optional acting-employee identifier, structured `triggeredBy` metadata, and optional parent-link metadata when the run is a sub-workflow child.

#### Scenario: Mixed project lists both engine kinds
- **WHEN** a project contains one running DAG execution and one running plugin run, and a caller fetches the unified list with no engine filter
- **THEN** the response includes one row with `engine='dag'` and one row with `engine='plugin'`
- **THEN** each row carries the normalized fields defined above

#### Scenario: Engine filter narrows the list
- **WHEN** a caller fetches the unified list with `engine=plugin`
- **THEN** the response excludes all DAG executions and returns only plugin runs

#### Scenario: Status normalization collapses engine-native variants
- **WHEN** the underlying engines report their native status values for a running run
- **THEN** the unified row reports `status='running'` regardless of engine

### Requirement: Unified list supports filter and cursor pagination

The unified list endpoint SHALL accept the filter parameters `engine`, `status` (repeatable), `actingEmployeeId`, `triggeredByKind`, `triggerId`, `startedAfter`, `startedBefore`, `limit`, and `cursor`. Pagination MUST use cursor-based ordering keyed on `(started_at DESC, run_id)` so that concurrent inserts in either engine do not cause duplicate or skipped rows across page boundaries.

#### Scenario: Cursor pagination does not skip rows under concurrent insertion
- **WHEN** a caller paginates a large mixed result, and new rows are inserted into either engine's table between page fetches
- **THEN** no row previously returned appears on a subsequent page
- **THEN** no row that should appear before the cursor is missing from the pages returned

#### Scenario: Acting-employee filter selects rows from both engines
- **WHEN** a caller filters `actingEmployeeId=E`
- **THEN** the response includes DAG executions and plugin runs whose run-level `acting_employee_id` equals E

#### Scenario: Triggered-by filter narrows to trigger dispatches
- **WHEN** a caller filters `triggeredByKind=trigger&triggerId=T`
- **THEN** the response includes only runs that were started by trigger T, regardless of engine

### Requirement: Unified detail endpoint routes to engine-native body

The system SHALL expose an authenticated, project-scoped unified detail endpoint keyed on `(engine, runId)` that returns a shared envelope (status, actingEmployeeId, triggeredBy, parentLink, startedAt, completedAt) together with the engine-native body produced by that engine's existing read seam. The engine-native body MUST be the same shape existing per-engine endpoints already return so existing detail UI components can consume it without change.

#### Scenario: DAG run detail returns DAG-native body under shared envelope
- **WHEN** a caller reads `(engine=dag, runId=R)` where R is an existing DAG execution
- **THEN** the response includes the shared envelope and the DAG-native body (node executions, graph snapshot, data store)

#### Scenario: Plugin run detail returns plugin-native body under shared envelope
- **WHEN** a caller reads `(engine=plugin, runId=R)` where R is an existing plugin run
- **THEN** the response includes the shared envelope and the plugin-native body (step list, attempt history)

#### Scenario: Unknown engine or missing run returns structured error
- **WHEN** a caller reads a detail with an unknown `engine` value or a run id that does not exist for the declared engine
- **THEN** the response returns a structured not-found or invalid-engine error without exposing the other engine's rows

### Requirement: Live run updates fan out on a unified WebSocket channel

The WebSocket hub SHALL emit canonical `workflow.run.*` events that mirror every DAG execution and plugin run lifecycle transition within the project. Payloads MUST use the same canonical row shape as the unified list. Engine-native channels MUST continue to be emitted unchanged; the unified channel is an additive fan-out layer.

#### Scenario: DAG run start is mirrored to unified channel
- **WHEN** a DAG workflow execution transitions from `pending` to `running`
- **THEN** a `workflow.run.status_changed` event is emitted on the unified channel with `engine='dag'` and the normalized row shape

#### Scenario: Plugin run completion is mirrored to unified channel
- **WHEN** a workflow plugin run reaches a terminal state
- **THEN** a `workflow.run.terminal` event is emitted on the unified channel with `engine='plugin'` and the normalized row shape

### Requirement: Workflow workspace consumes the unified view as its default list

The project workflow workspace UI SHALL use the unified list endpoint as its default run listing, with an engine-filter chip that lets operators narrow to a single engine. The detail view SHALL route through `(engine, runId)` and render the shared header plus engine-native body without requiring the operator to pick the engine manually.

#### Scenario: Workspace list shows both engine kinds by default
- **WHEN** an operator opens the workflow workspace for a project that has both DAG and plugin runs
- **THEN** both kinds appear in the list by default, each with a visible engine badge

#### Scenario: Engine filter chip narrows list immediately
- **WHEN** the operator clicks the engine-filter chip for `plugin`
- **THEN** only plugin runs appear in the list

#### Scenario: Clicking a row opens the engine-correct detail view
- **WHEN** the operator clicks a row with `engine='plugin'`
- **THEN** the detail view renders the plugin-native body under the shared header
