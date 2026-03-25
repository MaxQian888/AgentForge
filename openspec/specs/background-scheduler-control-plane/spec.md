# background-scheduler-control-plane Specification

## Purpose
TBD - created by archiving change complete-background-scheduler-control-plane. Update Purpose after archive.
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

