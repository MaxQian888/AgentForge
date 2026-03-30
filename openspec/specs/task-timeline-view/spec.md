# task-timeline-view Specification

## Purpose
Define the schedule-aware timeline view for project tasks, including granularity switching, horizontal bar rendering, unscheduled tasks, and drag rescheduling.
## Requirements
### Requirement: Timeline view displays tasks as horizontal bars on a time axis
The system SHALL provide a Timeline (Gantt-style) view that renders scheduled tasks as horizontal bars positioned along a time axis. The time axis MUST support day, week, and month granularity. Tasks with `plannedStartAt` and `plannedEndAt` MUST appear in the correct time window.

#### Scenario: Scheduled tasks appear on the timeline
- **WHEN** a user switches to Timeline view for a project with scheduled tasks
- **THEN** the system renders each scheduled task as a horizontal bar spanning from `plannedStartAt` to `plannedEndAt`
- **AND** the bars are vertically stacked or grouped by assignee/priority as configured

#### Scenario: User changes time granularity
- **WHEN** a user switches the timeline from week view to month view
- **THEN** the time axis updates to show month columns
- **AND** task bars rescale to match the new granularity without losing position accuracy

#### Scenario: Timeline is empty
- **WHEN** a project has no scheduled tasks (or all tasks are filtered out)
- **THEN** the Timeline view shows an explicit empty state explaining no scheduled tasks exist
- **AND** the empty state suggests creating a task or scheduling existing tasks

### Requirement: Timeline view supports drag-to-reschedule
The system SHALL allow users to drag task bars along the time axis to update their planning dates. A drag operation MUST update both `plannedStartAt` and `plannedEndAt` while preserving the task's duration.

#### Scenario: User drags a task to a new date range
- **WHEN** a user drags a task bar to a new position on the timeline
- **THEN** the system updates the task's `plannedStartAt` and `plannedEndAt` to match the new position
- **AND** the task duration (end - start) remains unchanged
- **AND** the updated dates are reflected in other views (Board, List, Calendar)

#### Scenario: Drag-to-reschedule fails to persist
- **WHEN** a drag operation cannot be saved to the backend
- **THEN** the task bar snaps back to its original position
- **AND** the user receives inline feedback that the reschedule did not complete

### Requirement: Timeline view makes unscheduled tasks visible
The system SHALL surface unscheduled tasks (tasks without `plannedStartAt`/`plannedEndAt`) in a dedicated section of the Timeline view so users can identify planning gaps.

#### Scenario: Unscheduled tasks are shown separately
- **WHEN** some project tasks have no planning dates
- **THEN** the Timeline view displays an "Unscheduled" section or sidebar listing those tasks
- **AND** the user can drag an unscheduled task onto the timeline to assign dates

#### Scenario: All tasks are scheduled
- **WHEN** every task in the filtered set has planning dates
- **THEN** the "Unscheduled" section is either hidden or shows an empty confirmation
