## MODIFIED Requirements

### Requirement: Project cost summary is queryable through an authoritative project-scoped contract
The system SHALL expose an authenticated project-scoped cost summary contract at `GET /api/v1/stats/cost?projectId=<id>` for standalone operator cost surfaces. The response MUST be derived from persisted run/task/sprint/budget facts and MUST include totals, active run count, budget summary, recorded trend data, sprint breakdown, task breakdown, compact period rollups, and external runtime attribution metadata without requiring consumers to combine unrelated stores or undocumented secondary routes.

#### Scenario: Project cost summary returns authoritative aggregates
- **WHEN** an authenticated operator requests `GET /api/v1/stats/cost?projectId=<id>` for a project with runs, tasks, and sprints
- **THEN** the response includes project totals for cost, input tokens, output tokens, cache-read tokens, turns, and active run count
- **THEN** the response includes `dailyCosts`, `sprintCosts`, and `taskCosts` arrays derived from persisted repository data for that project
- **THEN** the response includes a project budget snapshot that is consistent with the existing budget query capability for the same project
- **THEN** the response includes compact period rollups for today, the most recent 7-day window, and the most recent 30-day window so lightweight consumers can render summary views from the same contract
- **THEN** the response includes external runtime attribution metadata that distinguishes authoritative, estimated, and unpriced spend coverage for that project
- **THEN** the response includes runtime/provider/model breakdown entries so operators can see which external runtime families contributed to the recorded spend

#### Scenario: Empty project cost summary stays explicit
- **WHEN** an authenticated operator requests the project cost summary for a project with no recorded cost activity
- **THEN** the system returns HTTP 200 with zero-value totals
- **THEN** `dailyCosts`, `sprintCosts`, `taskCosts`, and runtime-breakdown arrays are empty
- **THEN** the response includes a zero-value cost-coverage summary instead of omitting the attribution section
- **THEN** the response does not substitute totals or active run counts from unrelated global or client-side state

#### Scenario: Mixed coverage project summary reports attribution gaps truthfully
- **WHEN** a project's recorded runs include both priced external runtimes and runs whose billing mode or pricing alias cannot be truthfully expressed as USD
- **THEN** the summary SHALL expose both the priced totals and the unpriced coverage counts
- **THEN** the API SHALL NOT hide the unpriced runs or silently treat them as zero-cost authoritative spend
