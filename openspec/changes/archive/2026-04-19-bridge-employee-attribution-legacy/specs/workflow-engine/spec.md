## ADDED Requirements

### Requirement: Legacy workflow step router accepts optional employee attribution

The current workflow step router SHALL accept an optional `employee_id` on the trigger payload of its `agent`, `review`, and `task` actions, in addition to the existing `member_id`. When `employee_id` is provided, the spawned agent run, review, or task dispatch MUST carry that employee identifier so downstream records attribute the work to that Digital Employee. When `employee_id` is absent, the step router falls back to the run-level `acting_employee_id` recorded on the workflow run, and when that too is absent, the spawned run attributes only through the existing `member_id` pathway.

#### Scenario: Agent action with step-level employee attribution
- **WHEN** a legacy workflow step of action `agent` declares an explicit `employee_id = E` on its trigger payload
- **THEN** the step router spawns an agent run whose `employee_id` equals E

#### Scenario: Agent action falls back to run-level acting employee
- **WHEN** a legacy workflow step of action `agent` omits `employee_id` and the workflow run declares `acting_employee_id = E`
- **THEN** the step router spawns an agent run whose `employee_id` equals E

#### Scenario: Review action with step-level employee attribution
- **WHEN** a legacy workflow step of action `review` declares an explicit `employee_id = E`
- **THEN** the triggered review carries E as the acting employee on its record

#### Scenario: Task action with step-level employee attribution
- **WHEN** a legacy workflow step of action `task` declares an explicit `employee_id = E`
- **THEN** the dispatched task attributes its spawned runs to E

#### Scenario: Absent employee attribution preserves existing behavior
- **WHEN** neither the step nor the workflow run declares any employee identifier
- **THEN** the spawned agent run's `employee_id` remains null and the step continues to execute through the existing `member_id` pathway unchanged
