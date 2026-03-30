## ADDED Requirements

### Requirement: Budget consumption is queryable via dedicated API endpoints
The system SHALL expose dedicated budget query endpoints that return budget consumption state without requiring dashboard widget configuration. These endpoints MUST provide the same authoritative budget data used by dispatch admission, aggregated for operator consumption.

#### Scenario: Project budget summary endpoint returns aggregated state
- **WHEN** an authenticated operator requests `GET /api/v1/projects/:pid/budget/summary`
- **THEN** the system returns project-level budget allocation, total spend, remaining budget, and threshold status
- **THEN** the response includes active sprint budget state and task-level aggregate metrics
- **THEN** the data is consistent with the budget governance checks used by the dispatch admission path

#### Scenario: Sprint budget detail endpoint returns per-task breakdown
- **WHEN** an authenticated operator requests `GET /api/v1/sprints/:sid/budget`
- **THEN** the system returns sprint allocated budget, total spend, per-task breakdown, and threshold status
- **THEN** the threshold status uses the same 80% warning and 100% exceeded thresholds as dispatch admission

#### Scenario: Budget endpoints respect authorization
- **WHEN** an unauthenticated request targets a budget query endpoint
- **THEN** the system returns HTTP 401 Unauthorized
- **THEN** no budget data is exposed
