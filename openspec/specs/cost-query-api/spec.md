# cost-query-api Specification

## Purpose
Define the authoritative project-scoped cost query contracts that power standalone operator cost surfaces and lightweight summary consumers.
## Requirements
### Requirement: Project cost summary is queryable through an authoritative project-scoped contract
The system SHALL expose an authenticated project-scoped cost summary contract at `GET /api/v1/stats/cost?projectId=<id>` for standalone operator cost surfaces. The response MUST be derived from persisted run/task/sprint/budget facts and MUST include totals, active run count, budget summary, recorded trend data, sprint breakdown, task breakdown, and compact period rollups without requiring consumers to combine unrelated stores or undocumented secondary routes.

#### Scenario: Project cost summary returns authoritative aggregates
- **WHEN** an authenticated operator requests `GET /api/v1/stats/cost?projectId=<id>` for a project with runs, tasks, and sprints
- **THEN** the response includes project totals for cost, input tokens, output tokens, cache-read tokens, turns, and active run count
- **THEN** the response includes `dailyCosts`, `sprintCosts`, and `taskCosts` arrays derived from persisted repository data for that project
- **THEN** the response includes a project budget snapshot that is consistent with the existing budget query capability for the same project
- **THEN** the response includes compact period rollups for today, the most recent 7-day window, and the most recent 30-day window so lightweight consumers can render summary views from the same contract

#### Scenario: Empty project cost summary stays explicit
- **WHEN** an authenticated operator requests the project cost summary for a project with no recorded cost activity
- **THEN** the system returns HTTP 200 with zero-value totals
- **THEN** `dailyCosts`, `sprintCosts`, and `taskCosts` are empty arrays
- **THEN** the response does not substitute totals or active run counts from unrelated global or client-side state

### Requirement: Velocity statistics expose cost-aligned period points
The system SHALL expose authenticated velocity statistics at `GET /api/v1/stats/velocity?projectId=<id>` using a typed response whose `points` entries carry both throughput and cost semantics for the same date window. Each point MUST include `period`, `tasksCompleted`, and `costUsd`, and any summary metadata in the response MUST stay consistent with those points.

#### Scenario: Velocity points include task completion cost
- **WHEN** an authenticated operator requests velocity statistics for a project over a date range
- **THEN** each returned point includes the period label, the number of tasks completed in that period, and the cost attributed to those completed tasks for the same period
- **THEN** the response shape is stable enough for the standalone cost workspace to render velocity without inventing missing `costUsd` values client-side

#### Scenario: Velocity endpoint returns explicit empty stats
- **WHEN** an authenticated operator requests velocity statistics for a project and no tasks were completed in the selected range
- **THEN** the response returns an empty `points` list and zero-value summary metadata
- **THEN** the endpoint does not omit the wrapper or change shape based on emptiness

### Requirement: Performance statistics expose truthful execution-bucket entries
The system SHALL expose authenticated performance statistics at `GET /api/v1/stats/agent-performance?projectId=<id>` using entries that represent the actual persisted aggregation bucket rather than an invented per-run or per-session identity. Each entry MUST include a stable identifier, a display label, run count, normalized success rate, average cost, average duration in minutes, and total cost.

#### Scenario: Performance entries use stable bucket identity
- **WHEN** an authenticated operator requests performance statistics for a project with historical runs
- **THEN** each entry includes a stable bucket identifier and a human-readable label for that same bucket
- **THEN** the entry includes run count, success rate, average cost, average duration in minutes, and total cost
- **THEN** the response does not imply a more granular historical agent identity than the persisted aggregation actually supports

#### Scenario: Performance label falls back safely when richer metadata is unavailable
- **WHEN** the server cannot resolve a richer display label for an aggregated execution bucket
- **THEN** the response still returns a stable identifier and a non-empty fallback label
- **THEN** the fallback remains truthful to the underlying persisted grouping dimension
