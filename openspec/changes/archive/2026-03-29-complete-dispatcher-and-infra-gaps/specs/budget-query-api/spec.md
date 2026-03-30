## ADDED Requirements

### Requirement: Project budget summary is queryable via dedicated endpoint
The system SHALL expose an authenticated endpoint that returns the current budget consumption summary for a project, aggregating across task, sprint, and project scopes. The response MUST include total allocated budget, total spent, remaining budget, threshold status (warning or exceeded), and per-scope breakdown.

#### Scenario: Project budget summary with active sprints
- **WHEN** an authenticated operator requests the budget summary for a project with active sprints and tasks
- **THEN** the system returns a summary including: project-level allocated and spent, active sprint allocated and spent, count of tasks at or near budget, warning threshold percentage, and whether any scope is currently exceeded
- **THEN** the response includes a `scopes` array with entries for each active budget scope (project, sprint, task aggregate)

#### Scenario: Project budget summary with no budget configured
- **WHEN** an authenticated operator requests the budget summary for a project that has no budget limits configured
- **THEN** the system returns a summary with zero values for allocated and spent
- **THEN** the threshold status indicates no budget governance is active

#### Scenario: Budget summary reflects real-time spend
- **WHEN** an authenticated operator requests the budget summary after a runtime cost update has been processed
- **THEN** the returned spend values reflect the most recent cost data
- **THEN** threshold status is consistent with the current spend-to-allocation ratio

### Requirement: Sprint budget detail is queryable via dedicated endpoint
The system SHALL expose an authenticated endpoint that returns budget detail for a specific sprint, including allocated budget, total spent across all tasks, per-task budget breakdown, and threshold status.

#### Scenario: Sprint budget detail with tasks
- **WHEN** an authenticated operator requests the budget detail for a sprint with tasks
- **THEN** the system returns: sprint allocated budget, total spent, remaining, number of tasks with budget, and a per-task breakdown of allocated vs spent
- **THEN** the response indicates whether the sprint budget is in warning (≥80%) or exceeded state

#### Scenario: Sprint budget detail for sprint without budget
- **WHEN** an authenticated operator requests the budget detail for a sprint that has no budget allocation
- **THEN** the system returns a response indicating no budget is configured for the sprint
- **THEN** HTTP status is 200 with zero-value budget fields
