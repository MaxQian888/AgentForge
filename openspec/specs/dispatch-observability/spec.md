# dispatch-observability Specification

## Purpose
Define the canonical dispatch observability contract for project-level statistics, task-level dispatch history, and operator-facing frontend status surfaces.
## Requirements
### Requirement: Dispatch metrics endpoint exposes operational statistics
The system SHALL expose a project-scoped dispatch statistics endpoint that returns aggregated metrics about dispatch outcomes, queue performance, and success rates.

#### Scenario: Dispatch stats returns outcome distribution
- **WHEN** an authenticated caller requests dispatch stats for a project
- **THEN** the response includes counts of dispatches by outcome (`started`, `queued`, `blocked`, `skipped`) for the requested time window
- **THEN** the response includes the blocked-reason distribution (budget, pool, worktree, member-validation)

#### Scenario: Dispatch stats returns queue performance metrics
- **WHEN** an authenticated caller requests dispatch stats for a project that has had queued dispatches
- **THEN** the response includes current queue depth, median wait time from queued to promoted, and promotion success rate
- **THEN** the response includes the count of entries that expired or were cancelled without promotion

#### Scenario: Dispatch stats returns empty metrics for inactive projects
- **WHEN** an authenticated caller requests dispatch stats for a project with no dispatch activity
- **THEN** the response returns zero counts and null averages rather than an error

### Requirement: Dispatch history is queryable per task
The system SHALL expose a per-task dispatch history that records each dispatch attempt with its outcome, timestamp, and reason.

#### Scenario: Task dispatch history lists chronological attempts
- **WHEN** an authenticated caller requests dispatch history for a specific task
- **THEN** the response lists all dispatch attempts for that task ordered by timestamp descending
- **THEN** each entry includes the dispatch outcome, trigger source (assignment, manual, workflow, IM), member targeted, and reason if blocked or queued

#### Scenario: Task dispatch history is empty for undispatched tasks
- **WHEN** an authenticated caller requests dispatch history for a task that has never been dispatched
- **THEN** the response returns an empty list rather than an error

### Requirement: Frontend dispatch status is visible in agent run surfaces
The system SHALL display dispatch status and outcome details in frontend agent monitoring surfaces so operators can distinguish between started, queued, and blocked runs at a glance.

#### Scenario: Agent run table shows dispatch status badge
- **WHEN** an operator views the agent monitor page
- **THEN** each agent run row displays a dispatch status badge (`started`, `queued`, `blocked`) using the existing `event-badge-list` component pattern
- **THEN** blocked and queued badges include a tooltip with the machine-readable reason

#### Scenario: Dispatch history panel shows per-task dispatch timeline
- **WHEN** an operator selects a task in the agent monitor or task detail view
- **THEN** a dispatch history panel shows chronological dispatch attempts with outcomes
- **THEN** the panel reuses the task-context-rail layout pattern
