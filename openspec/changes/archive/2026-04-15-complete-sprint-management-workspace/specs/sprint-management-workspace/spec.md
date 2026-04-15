## ADDED Requirements

### Requirement: Sprint workspace is a project-scoped planning control plane
The system SHALL provide a project-scoped sprint management workspace at `/sprints` that uses explicit project scope when present, falls back to the selected dashboard project when it is not, and supports sprint creation handoff actions without requiring the operator to rediscover the planning surface.

#### Scenario: Workspace loads with explicit project scope
- **WHEN** an authenticated operator opens `/sprints?project=<project-id>`
- **THEN** the sprint workspace loads sprint and milestone data for that project
- **AND** the page does not require a separate dashboard selection step before managing sprints

#### Scenario: Workspace has no available project scope
- **WHEN** an operator opens `/sprints` without an explicit project input and no dashboard project is selected
- **THEN** the page shows an explicit select-project prompt instead of an empty sprint canvas
- **AND** sprint data requests are not issued until project scope becomes available

#### Scenario: Bootstrap or dashboard handoff opens sprint creation
- **WHEN** an operator opens `/sprints?project=<project-id>&action=create-sprint`
- **THEN** the sprint creation dialog opens in that project scope
- **AND** the rest of the sprint workspace remains available behind the dialog

### Requirement: Sprint forms round-trip truthful sprint planning fields
The sprint workspace SHALL let operators create and edit sprint records using one truthful sprint contract for name, date range, optional milestone, status, and total budget so reopening the sprint reflects the same persisted planning state.

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

### Requirement: Selected sprint detail surfaces authoritative health and budget state
The sprint workspace SHALL provide a selected sprint detail surface that combines sprint metrics and sprint budget detail for the active selection without requiring the operator to open a separate budget or analytics page.

#### Scenario: Operator selects a sprint
- **WHEN** an operator selects a sprint card in the workspace
- **THEN** the page shows that sprint's burndown, completion metrics, velocity, and budget threshold state
- **AND** the detail surface includes the sprint's per-task budget breakdown for the selected sprint

#### Scenario: Workspace chooses a default sprint detail
- **WHEN** the workspace loads in a project that has sprints but no explicit sprint selection
- **THEN** the detail surface defaults to the active sprint if one exists
- **AND** otherwise the workspace falls back to the first available sprint instead of rendering an empty detail panel

#### Scenario: Sprint has no configured budget
- **WHEN** the selected sprint has no sprint budget allocation
- **THEN** the detail surface still renders the sprint metrics
- **AND** the budget section shows a truthful zero-value or unconfigured budget state rather than hiding the panel

### Requirement: Sprint workspace hands operators into sprint-scoped execution work
The sprint workspace SHALL provide an explicit action that opens the existing project task workspace in the same project and sprint scope so operators can continue from planning into execution without manually rebuilding filters.

#### Scenario: Operator opens sprint tasks from the selected sprint
- **WHEN** an operator triggers the sprint execution handoff for a selected sprint
- **THEN** the system navigates to the existing `/project` workspace with the same project scope and an explicit sprint-scoped handoff input
- **AND** the destination task workspace opens with that sprint already applied as the active sprint filter

### Requirement: Workspace detail remains truthful after realtime sprint updates
The sprint workspace SHALL refresh selected sprint detail when realtime sprint events change the current sprint record so list state and detail state do not drift apart.

#### Scenario: Selected sprint receives a realtime update
- **WHEN** the currently selected sprint receives a realtime sprint update or sprint transition event
- **THEN** the sprint list updates in place
- **AND** the selected sprint detail refreshes its metrics and budget state for that same sprint
