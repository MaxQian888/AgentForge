## MODIFIED Requirements

### Requirement: Progress metrics feed dashboard widgets
The task progress tracking system SHALL expose aggregated progress metrics to dashboard widget data endpoints.

#### Scenario: Burndown data from progress tracking
- **WHEN** a burndown widget requests data for a sprint
- **THEN** the progress tracking service returns daily completed/remaining task counts for the sprint duration

#### Scenario: Throughput data from progress tracking
- **WHEN** a throughput widget requests data for a time range
- **THEN** the progress tracking service returns tasks completed per period with optional grouping by assignee or status
