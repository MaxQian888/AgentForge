## ADDED Requirements

### Requirement: Project task workspace accepts explicit sprint-scoped handoff
The project task workspace SHALL accept an explicit sprint-scoped handoff input when opened from another project management surface so operators can continue into the existing execution workspace without manually reapplying sprint filters.

#### Scenario: Sprint workspace opens task workspace with sprint scope
- **WHEN** an operator opens the project task workspace with a valid project identifier and an explicit sprint handoff value for that project
- **THEN** the shared task workspace initializes with that sprint as the active sprint filter
- **AND** the sprint overview and sprint metrics in the task workspace resolve against the same sprint selection

#### Scenario: Sprint handoff input is invalid for the active project
- **WHEN** the project task workspace is opened with an explicit sprint handoff value that does not belong to the active project or no longer exists
- **THEN** the workspace ignores that sprint handoff input
- **AND** the task workspace falls back to its normal sprint filter and metrics resolution behavior without entering a broken state
