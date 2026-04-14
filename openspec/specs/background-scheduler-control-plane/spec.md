# background-scheduler-control-plane Specification

## Purpose
Define the canonical control plane for built-in background jobs, including durable run lifecycle, deployment-aware scheduling adapters, and operator-facing metadata, pause/resume, cancellation, history, metrics, and schema-driven configuration.
## Requirements
### Requirement: System scheduled jobs are registered through one canonical control plane
The system SHALL maintain one canonical registry for built-in scheduled jobs instead of scattering cron logic across ad hoc startup hooks or in-memory loops. Each registered job MUST expose a stable job key, human-readable name, scope, schedule, enable state, overlap policy, execution mode, and the latest known run status.

#### Scenario: Built-in jobs are materialized on startup
- **WHEN** the Go Orchestrator starts and loads the built-in scheduler catalog
- **THEN** the system registers or reconciles each built-in scheduled job in the canonical scheduler registry
- **THEN** each job record includes its stable key, current schedule, enable state, and latest execution summary

#### Scenario: Unsupported schedule changes are rejected
- **WHEN** an operator attempts to save an invalid or unsupported schedule configuration for a registered job
- **THEN** the system rejects the change with a validation error
- **THEN** the existing active schedule for that job remains unchanged

### Requirement: Scheduled jobs execute through a durable run lifecycle
The system SHALL create a durable run record for every scheduled or manually triggered job execution. The run lifecycle MUST capture trigger source, start and end timestamps, result status, summary metrics, and error details, and the scheduler MUST prevent overlapping execution for jobs configured as singletons.

#### Scenario: Cron trigger creates one singleton run
- **WHEN** a singleton scheduled job reaches its next due time
- **THEN** the system creates exactly one active run record for that trigger
- **THEN** the scheduler does not start a second overlapping run for the same job key and scope until the active run finishes

#### Scenario: Failed execution is preserved for diagnosis
- **WHEN** a scheduled job run fails
- **THEN** the system records the failure status, failure time, and diagnostic summary in the run history
- **THEN** the parent job record reflects that failure in its latest known run state

### Requirement: Scheduler execution adapts to deployment mode without changing job truth
The system SHALL keep Go as the authority for scheduled-job state and business execution semantics across deployment modes. In server-mode deployments, the scheduler MAY run directly in-process. In desktop or local sidecar deployments, the system MUST support OS-level trigger registration through Bun-based scheduling while still routing effective execution through the canonical Go scheduler contract.

#### Scenario: Desktop mode registers an OS-level trigger
- **WHEN** a job is marked as requiring persistent desktop scheduling and the app is running in a Bun/Tauri local deployment mode
- **THEN** the system reconciles an OS-level scheduled trigger for that job through the Bun scheduling adapter
- **THEN** the resulting callback targets the canonical Go scheduler execution path instead of bypassing Go-owned business logic

#### Scenario: Server mode runs without Bun registration
- **WHEN** the app is deployed in a server or container mode without desktop OS scheduling support
- **THEN** the scheduler continues to execute registered jobs through the Go runtime
- **THEN** the absence of the Bun adapter does not change the job definition or run-history contract

### Requirement: Operators can inspect and control scheduler jobs
The system SHALL provide authenticated API and UI surfaces for scheduler operations. Operators MUST be able to list jobs, inspect recent runs, enable or disable a job, and trigger an on-demand execution while preserving auditability.

#### Scenario: Operator lists scheduler jobs
- **WHEN** an authenticated operator opens the scheduler management surface
- **THEN** the system returns the registered jobs with schedule, enable state, last run status, last run time, and next due time
- **THEN** the operator can distinguish healthy jobs from paused or failing ones without reading server logs

#### Scenario: Operator manually triggers a job
- **WHEN** an authenticated operator requests a manual run for a registered job
- **THEN** the system creates a new run with trigger source `manual`
- **THEN** the resulting execution is recorded in the same history stream as cron-triggered runs

### Requirement: Scheduler failures and lifecycle changes are observable in real time
The system SHALL emit realtime scheduler lifecycle events for meaningful state changes, including job enablement changes, run start, run completion, and run failure. Repeated failures MUST remain operator-visible even when the underlying job is temporarily paused or skipped.

#### Scenario: Job failure is broadcast to realtime consumers
- **WHEN** a scheduled job run ends in failure
- **THEN** the system emits a realtime scheduler event that identifies the job, run, failure status, and summary reason
- **THEN** management clients can surface that failure without polling the full run-history endpoint

#### Scenario: Job is disabled after repeated issues
- **WHEN** an operator disables a repeatedly failing job
- **THEN** the system emits a lifecycle event for that state change
- **THEN** future automatic triggers stop until the job is enabled again

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
