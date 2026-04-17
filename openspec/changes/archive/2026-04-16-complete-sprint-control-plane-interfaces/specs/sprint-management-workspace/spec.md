## MODIFIED Requirements
### Requirement: Sprint workspace is a project-scoped planning control plane
The system SHALL provide a project-scoped sprint management workspace at /sprints that uses explicit project scope when present, falls back to the selected dashboard project when it is not, and supports sprint creation handoff plus explicit sprint selection inputs without requiring the operator to rediscover the planning surface.
#### Scenario: Workspace loads with explicit project scope
- **WHEN** an authenticated operator opens /sprints?project=<project-id>
- **THEN** the sprint workspace loads sprint and milestone data for that project
- **AND** the page does not require a separate dashboard selection step before managing sprints
#### Scenario: Workspace has no available project scope
- **WHEN** an operator opens /sprints without an explicit project input and no dashboard project is selected
- **THEN** the page shows an explicit select-project prompt instead of an empty sprint canvas
- **AND** sprint data requests are not issued until project scope becomes available
#### Scenario: Bootstrap or dashboard handoff opens sprint creation
- **WHEN** an operator opens /sprints?project=<project-id>&action=create-sprint
- **THEN** the sprint creation dialog opens in that project scope
- **AND** the rest of the sprint workspace remains available behind the dialog
#### Scenario: Route-scoped sprint selection opens the requested sprint detail
- **WHEN** an operator opens /sprints?project=<project-id>&sprint=<sprint-id> and the requested sprint belongs to that project
- **THEN** the workspace selects that sprint after project-scoped sprint data loads
- **AND** the detail surface opens on the requested sprint instead of falling back to another sprint silently
#### Scenario: Route-scoped sprint selection falls back cleanly when invalid
- **WHEN** an operator opens /sprints?project=<project-id>&sprint=<sprint-id> and the requested sprint is missing or belongs to a different project
- **THEN** the workspace ignores the invalid sprint seed
- **AND** the page falls back to its normal active-or-first sprint selection without entering a broken state
### Requirement: Sprint forms round-trip truthful sprint planning fields
The sprint workspace SHALL let operators create and edit sprint records using one truthful sprint contract for name, date range, optional milestone, status, and total budget so reopening the sprint reflects the same persisted planning state. Lifecycle edits MUST also preserve the single-active-sprint invariant for the project.
#### Scenario: Operator creates a sprint with milestone and budget
- **WHEN** an operator creates a sprint with a name, start date, end date, milestone selection, and total budget
- **THEN** the system persists that sprint in the current project scope with the selected milestone and budget values
- **AND** reopening or reloading the workspace shows the same entered calendar dates and milestone association for that sprint
#### Scenario: Operator edits sprint fields and status
- **WHEN** an operator updates an existing sprint's name, date range, milestone, budget, or valid lifecycle status
- **THEN** the system persists the updated sprint fields through the same project-scoped sprint record
- **AND** the workspace list and selected sprint detail reflect the updated values after the save completes
#### Scenario: Sprint form submission is invalid
- **WHEN** sprint creation or update fails because of invalid dates, invalid milestone scope, or an invalid status transition
- **THEN** the workspace keeps the operator's form context visible
- **AND** the page shows an inline error instead of silently reverting or pretending the sprint saved
#### Scenario: Sprint activation conflicts with another active sprint
- **WHEN** an operator attempts to save a sprint with status=active while a different sprint in the same project is already active
- **THEN** the workspace keeps the edit form open with the operator's pending values intact
- **AND** the page shows an inline conflict error that explains another sprint is already active
