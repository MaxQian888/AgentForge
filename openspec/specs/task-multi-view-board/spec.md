# task-multi-view-board Specification

## Purpose
Define the shared project task workspace that supports Board, List, Timeline, and Calendar views with consistent filters, detail access, and drag-based task updates.
## Requirements
### Requirement: Project tasks support Board, List, Timeline, and Calendar views within one workspace
The system SHALL provide a project-scoped task workspace that allows authenticated users to switch between Board, List, Timeline, and Calendar views without leaving the current project context. All views MUST operate on the same underlying task set for the selected project.

#### Scenario: User switches to another task view
- **WHEN** a user opens the project task workspace and selects a different view mode
- **THEN** the system renders the selected representation for the same project
- **AND** the current project context remains active without forcing the user to navigate to another route tree

#### Scenario: Project has no tasks yet
- **WHEN** a project contains no tasks
- **THEN** each available view renders an explicit empty state rather than a blank canvas
- **AND** the empty state provides a next-step action such as creating the first task

### Requirement: Task views share filtering, identity, and detail access
The system SHALL apply shared task filters, search context, and task identities consistently across Board, List, Timeline, and Calendar views. A task selected from any view MUST resolve to the same task detail context and preserve enough state for the user to continue managing that task.

#### Scenario: Filters remain consistent across views
- **WHEN** a user applies project task filters such as status, assignee, priority, or search text and then switches views
- **THEN** the newly selected view shows the same filtered task set
- **AND** the system does not require the user to re-enter the same filter criteria

#### Scenario: User opens task details from different views
- **WHEN** a user opens a task from Board, List, Timeline, or Calendar
- **THEN** the system shows the same task identity and detail panel or destination
- **AND** closing that detail experience returns the user to the previously selected task view context

### Requirement: Board view retains status-based drag and drop
The system SHALL preserve a Kanban-style Board view where tasks are grouped by workflow status and can be moved between status columns via drag and drop. A successful drag operation MUST update the task status in the shared task data source so that subsequent views reflect the change.

#### Scenario: Task is moved to another status column
- **WHEN** a user drags a task from one Board status column to another valid status column
- **THEN** the system updates the task status to match the destination column
- **AND** the moved task appears with its new status when the user revisits Board or another task view

#### Scenario: Drag operation cannot be persisted
- **WHEN** a Board drag action fails to persist
- **THEN** the system restores or clearly marks the affected task state instead of silently losing consistency
- **AND** the user receives inline feedback that the status change did not complete successfully

### Requirement: List view provides a dense operational representation of the same tasks
The system SHALL provide a List view that presents the project's tasks in a dense, scan-friendly format while exposing the same core task fields used across other views. At minimum, each visible list item MUST identify task title, status, priority, assignee, and planning state.

#### Scenario: User opens List view
- **WHEN** a user switches to List view for a project with tasks
- **THEN** the system renders the project's tasks in a dense row- or table-like representation
- **AND** each visible entry includes enough fields to compare work without opening every task individually

#### Scenario: List view reflects scheduling state
- **WHEN** some tasks are scheduled and others are not
- **THEN** the List view distinguishes scheduled tasks from unscheduled ones
- **AND** the planning state shown in List is consistent with Timeline and Calendar

### Requirement: Timeline and Calendar views support schedule-aware planning and drag updates
The system SHALL support schedule-aware Timeline and Calendar views backed by explicit task planning fields rather than inferred placeholders. Tasks with planning dates MUST appear in the correct time window, and unscheduled tasks MUST remain visible through an explicit unscheduled representation or prompt.

#### Scenario: Scheduled task appears in Timeline and Calendar
- **WHEN** a task has valid planning dates for the selected project
- **THEN** the Timeline and Calendar views place that task in the corresponding time range
- **AND** the rendered placement is consistent with the task's stored planning fields

#### Scenario: User reschedules a task by dragging in Timeline or Calendar
- **WHEN** a user drags a scheduled or unscheduled task within Timeline or Calendar to a valid target date or range
- **THEN** the system updates the task's planning fields to match the new placement
- **AND** the new schedule is reflected when the user switches to the other task views

#### Scenario: Task has no planning dates yet
- **WHEN** a task does not yet have a valid planning window
- **THEN** the Timeline and Calendar experience makes that task's unscheduled state explicit instead of hiding it
- **AND** the user can identify that additional planning is needed before it appears on the main schedule grid

### Requirement: Custom field columns in views
The task multi-view board SHALL render custom field values as columns in list and table views, and support filtering, sorting, and grouping by custom fields.

#### Scenario: Custom field column in table view
- **WHEN** a project has a custom field "Risk Level" and user enables it as a column in table view
- **THEN** each task row displays the Risk Level value, editable inline

#### Scenario: Filter by custom field in board view
- **WHEN** user applies a filter "Risk Level = High" in any view
- **THEN** only tasks with Risk Level set to "High" are displayed

#### Scenario: Group by custom field
- **WHEN** user groups the board view by "Module" custom field
- **THEN** tasks are grouped into columns by their Module value, including an "Unset" column for tasks without a value

### Requirement: Saved view integration
The task multi-view board SHALL load and apply saved view configurations when a view is selected from the view switcher.

#### Scenario: Apply saved view
- **WHEN** user selects a saved view from the view switcher
- **THEN** the workspace applies the view's layout type, filters, sorts, groups, and column configuration

### Requirement: Linked Docs column in board views
The task multi-view board SHALL support a "Linked Docs" optional column in list and table views displaying linked document titles.

#### Scenario: Show Linked Docs column in table view
- **WHEN** user enables the "Linked Docs" column in table view settings
- **THEN** each task row displays the titles of linked documents as clickable chips

### Requirement: Doc preview popover on task cards
Task cards in all board views SHALL display a document preview popover when hovered over a linked-doc indicator.

#### Scenario: Hover doc indicator on task card
- **WHEN** user hovers over the document icon on a task card that has linked docs
- **THEN** a popover shows the first linked document's title and first 3 lines of content, with a "View" link
