## ADDED Requirements

### Requirement: Calendar view displays tasks by planned dates in a month/week grid
The system SHALL provide a Calendar view that renders tasks on a date grid (month or week mode). Tasks with `plannedStartAt` MUST appear on their start date cell. Multi-day tasks MUST span across the appropriate date cells.

#### Scenario: Tasks appear on their planned dates
- **WHEN** a user switches to Calendar view for a project with scheduled tasks
- **THEN** each task with `plannedStartAt` appears on its start date cell as a compact chip
- **AND** multi-day tasks visually span from start to end date

#### Scenario: User switches between month and week mode
- **WHEN** a user toggles between month view and week view in the Calendar
- **THEN** the grid updates to show the selected time range
- **AND** tasks remain in their correct date positions

#### Scenario: Calendar has no tasks for visible range
- **WHEN** no tasks fall within the currently visible calendar date range
- **THEN** the Calendar displays the empty date grid with a prompt to navigate to a date range with tasks or create new tasks

### Requirement: Calendar view supports drag-to-reschedule
The system SHALL allow users to drag task chips between date cells to update planning dates.

#### Scenario: User drags a task to a different date
- **WHEN** a user drags a task chip from one date cell to another
- **THEN** the system updates the task's `plannedStartAt` (and shifts `plannedEndAt` proportionally)
- **AND** the change persists and is reflected in Timeline, List, and Board views

#### Scenario: Calendar drag fails
- **WHEN** a calendar drag operation fails to persist
- **THEN** the task chip returns to its original date cell
- **AND** the user receives feedback that the operation failed

### Requirement: Calendar view surfaces unscheduled tasks
The system SHALL make unscheduled tasks visible within the Calendar view so users can schedule them.

#### Scenario: Unscheduled tasks are accessible
- **WHEN** the project has tasks without planning dates
- **THEN** the Calendar view shows an "Unscheduled" section listing these tasks
- **AND** users can drag unscheduled tasks onto a date cell to assign a `plannedStartAt`

### Requirement: Calendar view uses shared workspace filters
The Calendar view MUST consume the same filter state as Board, List, and Timeline views.

#### Scenario: Filters applied in another view carry over to Calendar
- **WHEN** a user applies filters in Board view and then switches to Calendar
- **THEN** the Calendar displays only tasks matching those filters
- **AND** filter controls show the same active filter state
