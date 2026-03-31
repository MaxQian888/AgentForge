## ADDED Requirements

### Requirement: Scheduler panel displays job queue

The system SHALL display a list of scheduled and running jobs with status, schedule, and next run time.

#### Scenario: User views job queue
- **WHEN** user navigates to scheduler control panel
- **THEN** system displays table of jobs with name, status, schedule, next run, and last run columns
- **AND** jobs are sorted by next run time ascending

#### Scenario: Job queue is empty
- **WHEN** no scheduled jobs exist
- **THEN** system displays empty state with "Schedule your first job" call-to-action
- **AND** provides link to job creation documentation

### Requirement: Scheduler panel shows job status indicators

The system SHALL display color-coded status badges for each job (scheduled, running, completed, failed, paused).

#### Scenario: Job is running
- **WHEN** job is currently executing
- **THEN** job row displays blue "Running" badge with spinner animation
- **AND** shows elapsed time since start

#### Scenario: Job has failed
- **WHEN** job execution failed
- **THEN** job row displays red "Failed" badge
- **AND** clicking badge shows error details

#### Scenario: Job is paused
- **WHEN** job is in paused state
- **THEN** job row displays yellow "Paused" badge
- **AND** next run time shows "Paused" instead of date

### Requirement: Scheduler panel enables manual job triggering

The system SHALL allow users to manually trigger scheduled jobs on demand.

#### Scenario: User triggers job
- **WHEN** user clicks "Run Now" on a scheduled job
- **THEN** system immediately executes the job
- **AND** job status updates to "Running"

#### Scenario: Job is already running
- **WHEN** user clicks "Run Now" on a running job
- **THEN** system displays warning that job is already running
- **AND** does not trigger duplicate execution

#### Scenario: Manual trigger fails
- **WHEN** manual trigger request fails
- **THEN** system displays error message with failure reason
- **AND** job status remains unchanged

### Requirement: Scheduler panel supports job control operations

The system SHALL allow users to pause, resume, and cancel job executions.

#### Scenario: User pauses scheduled job
- **WHEN** user clicks "Pause" on an active scheduled job
- **THEN** system pauses the job schedule
- **AND** job will not run at next scheduled time until resumed

#### Scenario: User resumes paused job
- **WHEN** user clicks "Resume" on a paused job
- **THEN** system resumes the job schedule
- **AND** next run time is calculated and displayed

#### Scenario: User cancels running job
- **WHEN** user clicks "Cancel" on a running job
- **THEN** system sends cancellation signal to job
- **AND** job status updates to "Cancelling" then "Cancelled"

### Requirement: Scheduler panel displays execution history

The system SHALL show a history of recent job executions with duration and result.

#### Scenario: User views execution history
- **WHEN** user clicks on a job row to expand
- **THEN** system displays execution history for that job
- **AND** each entry shows start time, duration, status, and result summary

#### Scenario: Execution has error details
- **WHEN** execution failed with error
- **THEN** history entry shows error message and stack trace
- **AND** provides copy button for error details

#### Scenario: User clears history
- **WHEN** user clicks "Clear History" for a job
- **THEN** system removes all but last 10 executions
- **AND** confirms successful cleanup

### Requirement: Scheduler panel shows queue metrics

The system SHALL display aggregate metrics for the scheduler (total jobs, success rate, average duration).

#### Scenario: User views scheduler metrics
- **WHEN** scheduler panel loads
- **THEN** system displays summary cards showing total jobs, success rate, average duration, and queue depth
- **AND** metrics update every 10 seconds

#### Scenario: Success rate is low
- **WHEN** success rate falls below 90%
- **THEN** success rate card displays warning indicator
- **AND** suggests reviewing failed jobs

### Requirement: Scheduler panel enables job creation

The system SHALL provide a form to create new scheduled jobs with schedule, task, and configuration.

#### Scenario: User creates scheduled job
- **WHEN** user clicks "Create Job" button
- **THEN** system opens job creation form with name, schedule (cron), task type, and configuration fields
- **AND** validates cron expression syntax

#### Scenario: User submits invalid cron
- **WHEN** user enters invalid cron expression
- **THEN** form displays validation error
- **AND** shows expected format examples

#### Scenario: Job is created successfully
- **WHEN** user submits valid job configuration
- **THEN** system creates the scheduled job
- **AND** job appears in queue with calculated next run time

### Requirement: Scheduler panel supports job filtering

The system SHALL allow users to filter jobs by status, task type, and schedule frequency.

#### Scenario: User filters by status
- **WHEN** user selects "Failed" from status filter
- **THEN** system displays only failed jobs
- **AND** filter badge shows count of matching jobs

#### Scenario: User filters by task type
- **WHEN** user selects task type from dropdown
- **THEN** system displays only jobs of that task type
- **AND** filter persists until cleared

### Requirement: Scheduler panel displays upcoming schedule

The system SHALL show a calendar view of upcoming job executions.

#### Scenario: User views schedule calendar
- **WHEN** user switches to "Calendar" view
- **THEN** system displays calendar with job executions marked on upcoming dates
- **AND** clicking a date shows jobs scheduled for that day

#### Scenario: Date has many jobs
- **WHEN** more than 5 jobs are scheduled for a date
- **THEN** calendar shows "+N more" indicator
- **AND** clicking date expands to show all jobs

### Requirement: Scheduler panel enables job editing

The system SHALL allow users to modify job schedule and configuration.

#### Scenario: User edits job schedule
- **WHEN** user clicks "Edit" on a job and changes the schedule
- **THEN** system updates the job with new schedule
- **AND** next run time is recalculated

#### Scenario: User edits job configuration
- **WHEN** user modifies job task configuration
- **THEN** system saves new configuration
- **AND** next execution uses updated configuration

#### Scenario: Job is running during edit
- **WHEN** user attempts to edit a running job
- **THEN** system displays warning that changes apply to next execution
- **AND** current execution continues unchanged
