## MODIFIED Requirements

### Requirement: Budget threshold as automation event source
The dispatch budget governance system SHALL emit a budget.threshold_reached event when budget consumption crosses configured thresholds.

#### Scenario: Budget threshold event emitted
- **WHEN** a project's budget consumption crosses 80% of the allocated budget
- **THEN** the system emits a budget.threshold_reached event with the project ID, current consumption, and threshold percentage

### Requirement: Budget data feeds dashboard widgets
The budget governance system SHALL expose budget aggregation data to dashboard widget endpoints.

#### Scenario: Budget consumption widget data
- **WHEN** a budget_consumption widget requests data
- **THEN** the budget governance service returns total allocated, total spent, per-agent breakdown, and daily spend trend
