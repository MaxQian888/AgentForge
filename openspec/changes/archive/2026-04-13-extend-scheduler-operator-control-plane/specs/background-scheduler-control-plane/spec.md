## ADDED Requirements

### Requirement: Scheduler jobs expose explicit operator control metadata
The system SHALL return explicit operator-facing control metadata for each registered built-in scheduled job. This metadata MUST include control state, active-run summary when present, supported actions, editable configuration metadata, and upcoming schedule preview derived from the canonical scheduler parser.

#### Scenario: Operator opens scheduler job details
- **WHEN** an authenticated operator requests the details for a registered scheduled job
- **THEN** the response includes the job's explicit control state, supported actions, and editable configuration metadata
- **THEN** the response includes upcoming scheduled occurrences computed from the same schedule truth used by the scheduler registry

#### Scenario: Unsupported operator action is surfaced truthfully
- **WHEN** an operator requests metadata for a job that does not support a control action such as cancellation or config editing
- **THEN** the response marks that action as unsupported
- **THEN** the response includes a reason that the frontend can render without guessing from missing fields

### Requirement: Operators can pause and resume registered built-in jobs
The system SHALL allow authenticated operators to pause and resume registered built-in scheduled jobs without redefining the job catalog. A paused job MUST stop automatic triggers until resumed, and resuming MUST recalculate the next due time from the canonical schedule.

#### Scenario: Operator pauses a scheduled job
- **WHEN** an authenticated operator pauses an active scheduled job
- **THEN** the job transitions to an explicit paused control state
- **THEN** the scheduler does not start new automatic runs for that job until the operator resumes it

#### Scenario: Operator resumes a paused job
- **WHEN** an authenticated operator resumes a paused scheduled job
- **THEN** the job transitions back to the active control state
- **THEN** the system recalculates and returns the next due time based on the canonical schedule configuration

### Requirement: Running scheduled jobs can be cooperatively cancelled
The system SHALL support truthful cancellation for running scheduled-job executions that declare cancellation support. Cancellation MUST be recorded in the run lifecycle, and unsupported or unavailable cancellations MUST be rejected explicitly.

#### Scenario: Operator cancels a cancellable running job
- **WHEN** an authenticated operator requests cancellation for a running scheduled-job execution that supports cooperative cancellation
- **THEN** the run enters a cancel-requested lifecycle state and the handler receives the cancellation signal
- **THEN** the final persisted run status becomes cancelled if the handler exits cooperatively

#### Scenario: Operator attempts to cancel an unsupported run
- **WHEN** an authenticated operator requests cancellation for a scheduled-job execution that is not running or does not support cancellation
- **THEN** the system rejects the request with an explicit unsupported or unavailable reason
- **THEN** no optimistic cancelled status is recorded for that run

### Requirement: Scheduler run history and aggregate metrics are operator-governed
The system SHALL provide operator-ready history and metrics for scheduled jobs. Run-history queries MUST support filtering by job, status, trigger source, and time window; cleanup operations MUST preserve active runs and apply only to terminal history; aggregate stats MUST expose success and failure trends plus average duration and queue depth.

#### Scenario: Operator filters scheduler run history
- **WHEN** an authenticated operator queries scheduler run history with job, status, trigger source, or time-window filters
- **THEN** the system returns only matching scheduled-job runs
- **THEN** each returned run includes enough lifecycle detail to show duration, terminal outcome, and error summary when present

#### Scenario: Operator cleans up terminal run history
- **WHEN** an authenticated operator requests scheduler history cleanup with a retain or time-bound policy
- **THEN** the system deletes only terminal scheduled-job runs that fall outside the requested retention boundary
- **THEN** active runs and the retained recent history remain intact

#### Scenario: Operator loads scheduler metrics
- **WHEN** an authenticated operator requests scheduler aggregate metrics
- **THEN** the response includes queue depth, success and failure counts or rates, and average run duration over the reported window
- **THEN** paused jobs remain distinguishable from active or failing jobs without requiring client-side recomputation

### Requirement: Scheduler configuration remains built-in and schema-driven
The system SHALL keep the built-in scheduler catalog as the only source of job definitions. Operator configuration APIs MUST allow only schema-driven edits for registered jobs and MUST reject arbitrary job creation or unsupported config fields.

#### Scenario: Operator updates a supported built-in job setting
- **WHEN** an authenticated operator submits a configuration update that matches the registered editable metadata for a built-in scheduled job
- **THEN** the system validates and persists the allowed fields
- **THEN** the response returns the updated job details with refreshed preview or validation state

#### Scenario: Operator attempts to create or mutate an unsupported job shape
- **WHEN** an authenticated operator attempts to create an ad hoc scheduled job or submit config fields outside the registered editable metadata
- **THEN** the system rejects the request with a validation or unsupported-action error
- **THEN** the existing built-in scheduler registry remains unchanged
