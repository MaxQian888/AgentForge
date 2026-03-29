# dispatch-budget-governance Specification

## Purpose
Define canonical dispatch-time budget governance across task, sprint, and project scopes, including admission guardrails, runtime spend reactions, and operator-visible budget outcomes.
## Requirements
### Requirement: Dispatch admission respects layered budget readiness
The system SHALL evaluate task, sprint, and project dispatch budgets before starting a new runtime from assignment-triggered dispatch, manual spawn, or queued promotion. A dispatch start MUST NOT proceed when any governing budget for the target scope has already exhausted its available allowance. The budget check MUST be invoked by the `TaskDispatchService` layer before attempting runtime spawn or queue admission.

#### Scenario: Task budget exhaustion blocks immediate dispatch
- **WHEN** an assignment-triggered dispatch or manual spawn targets a task whose current spend is already at or above its configured task budget
- **THEN** the synchronous dispatch result returns `blocked`
- **THEN** the result includes a machine-readable budget guardrail classification for the task scope
- **THEN** the system MUST NOT create a new agent run or queue entry for that immediate request

#### Scenario: Sprint budget exhaustion blocks dispatch with sprint scope metadata
- **WHEN** an assignment-triggered dispatch targets a task whose governing sprint budget has been exhausted
- **THEN** the synchronous dispatch result returns `blocked` with `guardrailType: budget` and `guardrailScope: sprint`
- **THEN** the system MUST NOT create a new agent run or queue entry
- **THEN** the `budget.exceeded` realtime event is emitted with sprint scope details

#### Scenario: Project budget exhaustion blocks dispatch with project scope metadata
- **WHEN** an assignment-triggered dispatch targets a task whose governing project budget has been exhausted
- **THEN** the synchronous dispatch result returns `blocked` with `guardrailType: budget` and `guardrailScope: project`
- **THEN** the system MUST NOT create a new agent run or queue entry

#### Scenario: Queue promotion rechecks sprint or project budget before start
- **WHEN** a queued dispatch reaches the front of the admission order but the governing sprint or project budget no longer permits a new start
- **THEN** the system MUST NOT create a new agent run for that queue entry
- **THEN** the queue entry remains visible with its latest budget-blocked guardrail reason
- **THEN** operator-facing queue and pool views can see which budget scope prevented promotion

#### Scenario: Budget warning emitted but dispatch proceeds
- **WHEN** a dispatch would cross the 80% budget warning threshold without exceeding the hard limit
- **THEN** the system emits a `budget.warning` realtime event with the affected scope details
- **THEN** the dispatch proceeds normally (warning does not block)
- **THEN** the synchronous response includes a `budgetWarning` field alongside the successful dispatch outcome

### Requirement: Runtime cost updates govern dispatch lifecycle across budget scopes
The system SHALL treat bridge cost updates as the authoritative input for dispatch-time budget governance, updating task, sprint, and project spend snapshots and applying warning or exceeded actions for the affected scope. Exceeded budget decisions MUST prevent further admissions in that scope and MUST truthfully update the run and task lifecycle impacted by the overage.

#### Scenario: Warning threshold crossed for an active dispatch
- **WHEN** a runtime cost update causes the active dispatch to cross the configured warning threshold for the task, sprint, or project scope without exceeding the hard limit
- **THEN** the system emits a `budget.warning` realtime event with the affected scope details
- **THEN** the active run remains in progress
- **THEN** the updated spend snapshot becomes visible to synchronous and operator-facing consumers

#### Scenario: Hard budget limit exceeded during runtime execution
- **WHEN** a runtime cost update causes the active dispatch to exceed the hard limit for the task, sprint, or project scope
- **THEN** the system applies the configured exceed action for that scope to the triggering dispatch
- **THEN** the triggering run and task lifecycle are updated truthfully to reflect the budget breach
- **THEN** new dispatch starts and queued promotions for the affected scope are blocked until the governing budget recovers or is raised

### Requirement: Budget guardrail state is visible to dispatch consumers and operators
The system SHALL expose budget guardrail decisions as first-class dispatch facts so HTTP callers, WebSocket consumers, queue rosters, and IM clients can distinguish budget constraints from pool, worktree, or member-validation failures.

#### Scenario: Synchronous dispatch response carries budget guardrail metadata
- **WHEN** a dispatch request is blocked or delayed because of a task, sprint, or project budget guardrail
- **THEN** the synchronous result includes the blocked or queued dispatch outcome
- **THEN** the result includes machine-readable metadata that identifies the governing budget scope
- **THEN** consumers can render the outcome without parsing free-form reason text alone

#### Scenario: Queue roster preserves budget-blocked state
- **WHEN** a queued dispatch remains pending because a governing budget does not currently permit promotion
- **THEN** the queue roster retains the original admission context for that entry
- **THEN** the latest budget-blocked reason remains visible in operator-facing queue data
- **THEN** realtime pool lifecycle events expose that the entry is still queued because of budget guardrails

### Requirement: Budget threshold as automation event source
The dispatch budget governance system SHALL emit a budget.threshold_reached event when budget consumption crosses configured thresholds.

#### Scenario: Budget threshold event emitted
- **WHEN** a project's budget consumption crosses 80% of the allocated budget
- **THEN** the system emits a budget.threshold_reached event with the project ID, current consumption, and threshold percentage

### Requirement: Budget data feeds dashboard widgets
The dispatch budget governance system SHALL expose budget aggregation data to dashboard widget endpoints.

#### Scenario: Budget consumption widget data
- **WHEN** a budget_consumption widget requests data
- **THEN** the budget governance service returns total allocated, total spent, per-agent breakdown, and daily spend trend

### Requirement: Budget governance service is wired into the dispatch service at initialization
The system SHALL inject the `BudgetGovernanceService` into `TaskDispatchService` during server startup so that all dispatch paths (assign, spawn, queue promotion) share the same budget governance instance.

#### Scenario: TaskDispatchService receives budget checker at construction
- **WHEN** the server initializes and constructs the `TaskDispatchService`
- **THEN** the service receives a `DispatchBudgetChecker` interface that delegates to `BudgetGovernanceService`
- **THEN** the budget checker is invoked in `spawnForTask()` before any runtime spawn or queue admission attempt

#### Scenario: Budget checker unavailability does not block dispatch
- **WHEN** the `DispatchBudgetChecker` is nil (budget governance not configured)
- **THEN** dispatch proceeds without budget validation
- **THEN** no error or blocked outcome is returned for the absence of budget governance

