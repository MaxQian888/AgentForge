# task-progress-tracking Specification

## Purpose
Define the baseline contract for task progress snapshots, inactivity evaluation, and realtime progress-health delivery so task workflow status and execution health remain separate but consistent across backend APIs and interactive clients.

## Requirements
### Requirement: Task progress snapshots capture meaningful activity separately from workflow status
The system SHALL maintain a progress snapshot for each active task that records the latest meaningful activity time, latest workflow transition time, and current progress-health metadata independently from the task's workflow `status`.

#### Scenario: Task activity refreshes the progress snapshot
- **WHEN** a task is created, assigned, transitions workflow status, receives agent execution activity, or changes review state
- **THEN** the system updates that task's progress snapshot with the latest activity timestamp and activity source
- **AND** subsequent task reads expose the refreshed progress metadata together with the task record

#### Scenario: Terminal task stops participating in progress tracking
- **WHEN** a task enters a terminal workflow state such as `done` or `cancelled`
- **THEN** the system marks the task as no longer subject to active progress evaluation
- **AND** any previously active stalled state for that task is cleared or closed

### Requirement: The system detects stalled work from configured inactivity thresholds
The system SHALL evaluate non-terminal tasks against configured inactivity thresholds so it can distinguish normal waiting from progress risk and explicit stalled work.

#### Scenario: Active task crosses a configured inactivity threshold
- **WHEN** a non-terminal task has no qualifying activity for longer than the configured threshold for its current workflow state
- **THEN** the system marks the task as stalled or at-risk with a machine-readable reason
- **AND** the task's progress snapshot records when that risk state began

#### Scenario: Stalled task becomes active again
- **WHEN** a task that was marked stalled receives qualifying activity or transitions into a state that clears the inactivity condition
- **THEN** the system removes the stalled flag from the progress snapshot
- **AND** the task records that it has recovered from the prior risk condition

### Requirement: Progress state changes are visible to realtime consumers
The system SHALL expose task progress-health changes through the same task surfaces used by interactive clients so that the UI can react without polling a separate risk service.

#### Scenario: Client requests task data after a progress state change
- **WHEN** a task list or task detail response includes a task whose progress-health state has changed
- **THEN** the response includes the updated progress-health fields, last activity metadata, and current risk reason
- **AND** clients do not need a separate follow-up call to learn whether the task is healthy, at risk, or stalled

#### Scenario: Subscribed client observes a progress state transition
- **WHEN** a task's progress-health state changes because of new activity or inactivity evaluation
- **THEN** the system emits a realtime event for that task and project scope
- **AND** the event payload includes enough information for the client to update progress indicators and linked task views in place

### Requirement: Progress metrics feed dashboard widgets
The task progress tracking system SHALL expose aggregated progress metrics to dashboard widget data endpoints.

#### Scenario: Burndown data from progress tracking
- **WHEN** a burndown widget requests data for a sprint
- **THEN** the progress tracking service returns daily completed/remaining task counts for the sprint duration

#### Scenario: Throughput data from progress tracking
- **WHEN** a throughput widget requests data for a time range
- **THEN** the progress tracking service returns tasks completed per period with optional grouping by assignee or status
