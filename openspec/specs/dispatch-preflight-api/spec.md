# dispatch-preflight-api Specification

## Purpose
Define the read-only dispatch preflight contract that reports budget readiness, pool availability, and admission likelihood before a caller commits to dispatch.
## Requirements
### Requirement: Dispatch preflight returns budget and pool readiness before commit
The system SHALL expose a read-only preflight endpoint that evaluates the same canonical dispatch guardrails used by real dispatch for a given task-member combination and optional candidate runtime tuple. The preflight response MUST return an advisory outcome hint, machine-readable guardrail classification for non-start outcomes, current task or sprint or project budget pressure, and current pool readiness without reserving capacity or mutating queue state.

#### Scenario: Preflight returns canonical advisory snapshot for an eligible dispatch
- **WHEN** an authenticated caller requests dispatch preflight for a valid task and active agent member within the same project, with or without explicit runtime or provider or model or role or budget input
- **THEN** the system evaluates the same task, member, budget, active-run, and pool guardrails that an immediate dispatch would use for that candidate
- **THEN** the response includes the current pool active count, available slots, and queued count together with an `admissionLikely` advisory boolean
- **THEN** the response indicates whether the current advisory outcome would most likely start immediately or queue under present conditions

#### Scenario: Preflight warns when task or sprint or project budget is near threshold
- **WHEN** an authenticated caller requests dispatch preflight and the projected spend for the candidate dispatch would cross the warning threshold for task, sprint, or project budget scope without exceeding the hard limit
- **THEN** the response includes a `budgetWarning` summary for each affected scope that is near threshold
- **THEN** the advisory outcome remains non-blocking if no hard guardrail is exceeded
- **THEN** the response does not misclassify a warning-only state as blocked

#### Scenario: Preflight returns canonical blocked guardrail metadata
- **WHEN** an authenticated caller requests dispatch preflight for a task-member candidate that would currently be blocked by budget exhaustion, invalid assignment context, an active run conflict, or a transient system guardrail
- **THEN** the response indicates the advisory dispatch outcome would be `blocked`
- **THEN** the response includes machine-readable guardrail type and scope metadata together with a human-readable reason
- **THEN** non-budget blocked states are surfaced through their true guardrail classification instead of being folded into a budget-only field

#### Scenario: Preflight for a non-agent member returns skipped indication
- **WHEN** an authenticated caller requests dispatch preflight for a member that is not of type `agent`
- **THEN** the response indicates the dispatch outcome would be `skipped`
- **THEN** pool and runtime-only budget fields are omitted because no runtime startup would be attempted

### Requirement: Preflight is advisory and does not reserve resources
The system SHALL treat preflight as a pure read operation even when it reuses the canonical dispatch evaluation path. A preflight request MUST NOT create queue entries, dispatch attempts, notifications, or run records, and it MUST NOT hold any admission reservation.

#### Scenario: Concurrent preflight and dispatch do not conflict
- **WHEN** a caller requests preflight and another caller triggers dispatch for the same task concurrently
- **THEN** the preflight response reflects the guardrail and pool state visible at read time only
- **THEN** the real dispatch still performs its own authoritative commit-time recheck
- **THEN** no deadlock, queue mutation, reservation leak, or double-counting occurs because of the advisory read

