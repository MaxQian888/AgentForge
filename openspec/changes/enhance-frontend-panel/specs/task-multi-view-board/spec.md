## ADDED Requirements

### Requirement: Task board provides multiple view modes

The system SHALL allow users to switch between kanban board, timeline, and calendar views for task visualization.

#### Scenario: User switches to kanban view
- **WHEN** user clicks "Kanban" view toggle
- **THEN** system displays tasks organized in columns by status
- **AND** view preference is saved for future visits

#### Scenario: User switches to timeline view
- **WHEN** user clicks "Timeline" view toggle
- **THEN** system displays tasks on a horizontal timeline based on due dates
- **AND** tasks without due dates are shown in a separate "Unscheduled" section

#### Scenario: User switches to calendar view
- **WHEN** user clicks "Calendar" view toggle
- **THEN** system displays tasks on a monthly calendar based on due dates
- **AND** user can navigate between months

### Requirement: Kanban board supports drag-and-drop

The system SHALL allow users to drag task cards between status columns to update task status.

#### Scenario: User drags task to new status
- **WHEN** user drags a task card from "In Progress" to "Review" column
- **THEN** system updates the task status to "review"
- **AND** task card animates to new position

#### Scenario: Drag operation is cancelled
- **WHEN** user releases task card outside valid drop zone
- **THEN** system returns task card to original position
- **AND** no status change occurs

### Requirement: Kanban board columns are customizable

The system SHALL allow users to configure which status columns are visible and their order.

#### Scenario: User hides column
- **WHEN** user clicks column menu and selects "Hide Column"
- **THEN** system removes the column from view
- **AND** tasks in hidden column are accessible via filter

#### Scenario: User reorders columns
- **WHEN** user drags column header to new position
- **THEN** system reorders columns accordingly
- **AND** column order persists across sessions

### Requirement: Timeline view shows task dependencies

The system SHALL display dependency connections between tasks on the timeline view.

#### Scenario: User views dependent tasks
- **WHEN** task A blocks task B on the timeline
- **THEN** system draws a connecting line from task A to task B
- **AND** line is highlighted when either task is hovered

#### Scenario: Timeline has no dependencies
- **WHEN** no tasks have dependencies
- **THEN** timeline displays tasks without any connecting lines

### Requirement: Task cards display key information

The system SHALL show task title, priority, assignee, due date, and tags on task cards in all views.

#### Scenario: User views task card in kanban
- **WHEN** task card is displayed in kanban column
- **THEN** card shows title, priority badge, assignee avatar, due date, and tag chips
- **AND** card is clickable to open task details

#### Scenario: Task has no assignee
- **WHEN** task card is displayed without an assignee
- **THEN** system shows "Unassigned" placeholder
- **AND** placeholder is visually distinct from assigned tasks

### Requirement: Task board supports quick filters

The system SHALL provide filter controls for assignee, priority, tags, and due date range.

#### Scenario: User filters by assignee
- **WHEN** user selects an assignee from the filter dropdown
- **THEN** system displays only tasks assigned to that user
- **AND** filter chip appears showing active filter

#### Scenario: User clears all filters
- **WHEN** user clicks "Clear Filters" button
- **THEN** system removes all active filters
- **AND** displays all tasks matching current search query

### Requirement: Task board supports search

The system SHALL allow users to search tasks by title and description.

#### Scenario: User searches for task
- **WHEN** user types "authentication" in the search box
- **THEN** system filters tasks to those matching "authentication" in title or description
- **AND** matching text is highlighted in results

#### Scenario: Search returns no results
- **WHEN** user searches for non-existent task
- **THEN** system displays "No tasks found" empty state
- **AND** suggests clearing search or filters

### Requirement: Task board enables quick task creation

The system SHALL allow users to create tasks directly from the board view with minimal required fields.

#### Scenario: User creates task from kanban
- **WHEN** user clicks "+" button on a kanban column
- **THEN** system opens quick-create form with status pre-set to column status
- **AND** form requires only title to submit

#### Scenario: User creates task with full form
- **WHEN** user clicks "Create Task" button in toolbar
- **THEN** system opens full task creation form with all fields available

### Requirement: Task board supports keyboard navigation

The system SHALL allow users to navigate and operate the task board using keyboard shortcuts.

#### Scenario: User navigates with arrow keys
- **WHEN** user presses arrow keys while board is focused
- **THEN** focus moves between task cards
- **AND** current focus is visually indicated

#### Scenario: User opens task with keyboard
- **WHEN** user presses Enter while task card is focused
- **THEN** system opens task details panel
