## MODIFIED Requirements

### Requirement: Board view retains status-based drag and drop
The system SHALL preserve a Kanban-style Board view where tasks are grouped by workflow status and can be moved between status columns via drag and drop. A successful drag operation MUST update the task status in the shared task data source so that subsequent views reflect the change.

#### Scenario: Task is moved to another status column
- **WHEN** a user drags a task from one Board status column to another valid status column
- **THEN** the system updates the task status to match the destination column
- **AND** the moved task appears with its new status when the user revisits Board or another task view

#### Scenario: Drag operation cannot be persisted
- **WHEN** a Board drag action fails to persist (network error, invalid transition, etc.)
- **THEN** the system restores the task to its original status column with an animated snap-back
- **AND** the user receives an inline toast or banner explaining that the status change did not complete
- **AND** the error message includes the reason (e.g., "Cannot transition from inbox to done")

#### Scenario: Drag operation is in progress
- **WHEN** a user drops a task on a new column and the status transition API call is pending
- **THEN** the task card shows a subtle loading indicator in its new column
- **AND** the card is not draggable again until the transition completes or fails

### Requirement: Task views share filtering, identity, and detail access
The system SHALL apply shared task filters, search context, and task identities consistently across Board, List, Timeline, and Calendar views. A task selected from any view MUST resolve to the same task detail context and preserve enough state for the user to continue managing that task.

#### Scenario: Filters remain consistent across views
- **WHEN** a user applies project task filters such as status, assignee, priority, or search text and then switches views
- **THEN** the newly selected view shows the same filtered task set
- **AND** the system does not require the user to re-enter the same filter criteria

#### Scenario: User opens task details from different views
- **WHEN** a user opens a task from Board, List, Timeline, or Calendar
- **THEN** the system shows the same task identity and detail panel
- **AND** closing that detail experience returns the user to the previously selected task view context

#### Scenario: View switch preserves selected task
- **WHEN** a user has a task selected in Board view and switches to List view
- **THEN** the selected task remains highlighted/selected in the List view if visible
- **AND** the task detail panel (if open) stays open showing the same task

### Requirement: Linked Docs column in board views
The task multi-view board SHALL support a "Linked Docs" optional column in list and table views displaying linked document titles.

#### Scenario: Show Linked Docs column in table view
- **WHEN** user enables the "Linked Docs" column in table view settings
- **THEN** each task row displays the titles of linked documents as clickable chips
- **AND** clicking a chip navigates to the linked document

#### Scenario: Task has no linked docs
- **WHEN** a task has no linked documents
- **THEN** the Linked Docs column cell is empty (no placeholder text needed)

### Requirement: Doc preview popover on task cards
Task cards in all board views SHALL display a document preview popover when hovered over a linked-doc indicator.

#### Scenario: Hover doc indicator on task card
- **WHEN** user hovers over the document icon on a task card that has linked docs
- **THEN** a popover shows the first linked document's title and first 3 lines of content
- **AND** the popover includes a "View" link to open the full document

#### Scenario: Task card has no linked docs
- **WHEN** a task card has no linked documents
- **THEN** no document icon or indicator is shown on the card

## ADDED Requirements

### Requirement: Board view loading and empty states
The Board view SHALL display explicit loading and empty states per spec requirements.

#### Scenario: Board is loading tasks
- **WHEN** the task fetch for the project is in progress
- **THEN** the Board shows skeleton column placeholders instead of empty columns
- **AND** the skeleton state clears once tasks are loaded

#### Scenario: Board has no tasks
- **WHEN** the project has no tasks or all tasks are filtered out
- **THEN** the Board shows an empty state with a "Create Task" action
- **AND** the empty state appears in place of the column layout, not as empty columns with headers

### Requirement: Bulk operations from Board multi-select
The Board view SHALL support multi-select of task cards with bulk status change, bulk assign, and bulk delete operations.

#### Scenario: User multi-selects tasks and changes status
- **WHEN** a user selects multiple task cards (via Ctrl/Cmd+click) and applies a bulk status change
- **THEN** the system transitions all selected tasks to the new status
- **AND** tasks that cannot transition (invalid transition) are reported individually
- **AND** successfully transitioned tasks move to their new columns

#### Scenario: User bulk-assigns tasks
- **WHEN** a user selects multiple tasks and applies a bulk assign operation
- **THEN** the system updates the assignee for all selected tasks
- **AND** the assignment change is reflected immediately in the Board
