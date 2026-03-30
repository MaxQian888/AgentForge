## ADDED Requirements

### Requirement: List view renders project tasks in a dense table with core columns
The system SHALL provide a List view component that presents the project's filtered task set in a dense, scan-friendly table format. Each visible row MUST include task title, status, priority, assignee, planned dates, and labels. The List view MUST consume the same shared filter state as Board, Timeline, and Calendar views.

#### Scenario: User switches to List view
- **WHEN** a user selects List view from the project task workspace view switcher
- **THEN** the system renders the project's filtered tasks in a table with columns for title, status, priority, assignee, planned start/end dates, and labels
- **AND** the table uses the same task set and filters currently active in the workspace store

#### Scenario: List view distinguishes scheduled from unscheduled tasks
- **WHEN** some tasks have `plannedStartAt`/`plannedEndAt` and others do not
- **THEN** the List view displays date values for scheduled tasks and a visual "unscheduled" indicator for tasks without planning dates
- **AND** the scheduling state shown is consistent with Timeline and Calendar views

#### Scenario: List view supports sorting by column
- **WHEN** a user clicks a column header in the List view
- **THEN** the table sorts tasks by that column in ascending order
- **AND** clicking the same header again reverses the sort order
- **AND** the sort indicator is visible on the active sort column

#### Scenario: Project has no tasks
- **WHEN** a project contains no tasks (or no tasks match current filters)
- **THEN** the List view renders an explicit empty state with a "Create task" action
- **AND** the empty state does not display an empty table with headers only

### Requirement: List view supports custom field columns
The system SHALL render custom field values as additional columns in the List view table. Users MUST be able to toggle column visibility, and custom field columns MUST support inline display of their values.

#### Scenario: Custom field column is displayed
- **WHEN** a project has a custom field "Risk Level" and the user enables it as a visible column
- **THEN** each task row displays the Risk Level value in its column
- **AND** the column header shows the custom field name

#### Scenario: Custom field column is toggled off
- **WHEN** a user disables a custom field column from the column visibility settings
- **THEN** that column is removed from the table without affecting other columns or data

### Requirement: List view supports filter, sort, and group by custom fields
The system SHALL allow filtering, sorting, and grouping the task list by custom field values.

#### Scenario: Filter by custom field value
- **WHEN** a user applies a filter "Risk Level = High" in the List view
- **THEN** only tasks with Risk Level set to "High" are displayed
- **AND** the filter is reflected in the shared workspace filter state

#### Scenario: Group by custom field
- **WHEN** a user groups the List view by a "Module" custom field
- **THEN** tasks are grouped into sections by their Module value
- **AND** an "Unset" group is shown for tasks without a Module value

### Requirement: List view supports inline status and priority quick-change
The system SHALL allow users to change task status and priority directly from the List view row without opening the detail panel.

#### Scenario: User changes task status from list row
- **WHEN** a user clicks the status badge in a list row and selects a new status
- **THEN** the system transitions the task to the new status
- **AND** the row updates to reflect the new status without a full page refresh

#### Scenario: Status transition fails
- **WHEN** a status transition from the list row fails (e.g., invalid transition)
- **THEN** the system shows an inline error message on the affected row
- **AND** the row reverts to the previous status
