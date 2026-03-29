## MODIFIED Requirements

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

## ADDED Requirements

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
