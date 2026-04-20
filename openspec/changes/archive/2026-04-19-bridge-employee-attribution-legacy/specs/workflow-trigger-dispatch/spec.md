## ADDED Requirements

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
