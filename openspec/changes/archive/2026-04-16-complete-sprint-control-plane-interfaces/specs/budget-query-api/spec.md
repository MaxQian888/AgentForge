## MODIFIED Requirements

### Requirement: Sprint budget detail is queryable via dedicated endpoint
The system SHALL expose an authenticated project-scoped endpoint that returns budget detail for a specific sprint in the active project, including allocated budget, total spent across all tasks, per-task budget breakdown, and threshold status. Consumers MUST resolve sprint budget detail through the canonical project-scoped sprint route instead of sid-only reads that bypass project scope.

#### Scenario: Sprint budget detail with tasks
- **WHEN** an authenticated operator requests the budget detail for an in-scope sprint with tasks through the canonical project-scoped sprint budget route
- **THEN** the system returns sprint allocated budget, total spent, remaining, number of tasks with budget, and a per-task breakdown of allocated vs spent
- **AND** the response indicates whether the sprint budget is in warning (>=80%) or exceeded state

#### Scenario: Sprint budget detail for sprint without budget
- **WHEN** an authenticated operator requests the budget detail for an in-scope sprint that has no budget allocation
- **THEN** the system returns a response indicating no budget is configured for the sprint
- **AND** HTTP status is 200 with zero-value budget fields

#### Scenario: Sprint budget detail rejects out-of-scope sprint ids
- **WHEN** an authenticated operator requests sprint budget detail for a sprint that does not belong to the active project scope
- **THEN** the system returns a scope-safe not-found or conflict outcome
- **AND** no sprint budget data from another project is disclosed
