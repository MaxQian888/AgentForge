# dispatch-preflight-api Specification

## Purpose
Define the read-only dispatch preflight contract that reports budget readiness, pool availability, and admission likelihood before a caller commits to dispatch.
## Requirements
### Requirement: Dispatch preflight returns budget and pool readiness before commit
The system SHALL expose a read-only preflight endpoint that returns budget readiness, pool availability, and admission likelihood for a given task-member combination, so callers can make informed dispatch decisions without committing to a spawn.

#### Scenario: Preflight returns budget and pool snapshot for eligible dispatch
- **WHEN** an authenticated caller requests dispatch preflight for a valid task and agent member within the same project
- **THEN** the system returns the current sprint budget remaining and project budget remaining for the task's project
- **THEN** the system returns current pool active count, available slots, and queued count
- **THEN** the system returns an `admissionLikely` boolean indicating whether dispatch would probably succeed based on current state

#### Scenario: Preflight warns when budget is near threshold
- **WHEN** an authenticated caller requests dispatch preflight and the projected spend would cross the 80% budget warning threshold for the sprint or project scope
- **THEN** the response includes a `budgetWarning` field with the affected scope and utilization percentage
- **THEN** the `admissionLikely` boolean remains true (warning does not block)

#### Scenario: Preflight indicates budget would block dispatch
- **WHEN** an authenticated caller requests dispatch preflight and the projected spend would exceed the hard budget limit for any governing scope
- **THEN** the response includes a `budgetBlocked` field with the affected scope and reason
- **THEN** the `admissionLikely` boolean is false

#### Scenario: Preflight for a non-agent member returns skipped indication
- **WHEN** an authenticated caller requests dispatch preflight for a member that is not of type `agent`
- **THEN** the response indicates the dispatch outcome would be `skipped`
- **THEN** pool and budget fields are omitted since no runtime would start

### Requirement: Preflight is advisory and does not reserve resources
The system SHALL treat preflight as a pure read operation that does not create queue entries, modify pool state, or hold any reservations.

#### Scenario: Concurrent preflight and dispatch do not conflict
- **WHEN** a caller requests preflight and another caller triggers dispatch for the same task concurrently
- **THEN** the preflight response reflects the pool/budget state at the time of the read
- **THEN** the dispatch proceeds with its own authoritative admission check
- **THEN** no deadlock, reservation conflict, or double-counting occurs
