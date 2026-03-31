# resource-governor Specification

## Purpose
Define the current runtime budget-governance contract for task-level cost accumulation, warning thresholds, budget exhaustion handling, and persisted spend updates.
## Requirements
### Requirement: Task runtime cost is accumulated and evaluated against budget thresholds
The system SHALL accumulate runtime token and cost usage per task and evaluate that spend against configured task budgets.

#### Scenario: Record task runtime cost
- **WHEN** runtime cost data is reported for a task
- **THEN** the cost tracker accumulates input tokens, output tokens, cache-read tokens, total cost, and turn count for that task

#### Scenario: Budget warning threshold is detected
- **WHEN** accumulated task spend reaches the configured warning threshold for the task budget
- **THEN** the tracker reports a warning threshold action instead of immediate termination

#### Scenario: Budget hard limit is detected
- **WHEN** accumulated task spend reaches or exceeds the configured task budget
- **THEN** the tracker reports a hard-stop action for that task

### Requirement: Runtime handlers emit warning and hard-stop behavior for task budgets
The runtime bridge handlers SHALL emit budget-threshold cost updates and abort execution once the task budget is exhausted.

#### Scenario: Emit budget threshold warning
- **WHEN** runtime spend reaches the warning ratio for the current task budget
- **THEN** the runtime emits a `cost_update` event carrying the budget-threshold warning signal

#### Scenario: Abort runtime when budget is exhausted
- **WHEN** runtime spend reaches or exceeds the task budget
- **THEN** the runtime aborts execution with a budget-exceeded error

### Requirement: Go orchestration updates persisted spend from runtime cost events
The Go agent service SHALL process runtime cost updates and propagate the resulting spend into persisted run and task state.

#### Scenario: Process runtime cost update event
- **WHEN** Go receives a bridge `cost_update` event for an active run
- **THEN** the run token totals, run cost, and turn count are updated
- **THEN** the owning task's spent budget state is recalculated from persisted run cost

#### Scenario: Mark task budget-exceeded status when spend crosses task budget
- **WHEN** recalculated task spend reaches or exceeds the task budget
- **THEN** the task runtime status is updated to reflect budget exhaustion

### Requirement: Current resource-governor support is limited to task-level runtime budget tracking
The system SHALL treat the current resource-governor capability as covering task-level runtime budget tracking, warning thresholds, and budget exhaustion handling. Redis-backed global quotas, monthly rollovers, provider-wide fair queuing, and generalized governance APIs are not yet guaranteed by this capability.

#### Scenario: Do not assume multi-level quota management from this capability
- **WHEN** a caller relies on the current resource-governor capability definition
- **THEN** only task-level runtime budget tracking and exhaustion handling are guaranteed
