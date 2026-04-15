## MODIFIED Requirements

### Requirement: Scheduler run history and aggregate metrics are operator-governed
The system SHALL provide operator-ready history and metrics for scheduled jobs. Run-history queries MUST support filtering by job, status, trigger source, and time window; cleanup operations MUST preserve active runs and apply only to terminal history; aggregate stats MUST expose success and failure trends plus average duration and queue depth. When a scheduled job evaluates downstream automation or workflow orchestration, the persisted run detail and metrics MUST preserve machine-readable downstream outcome counts instead of collapsing the run into a scan-only summary.

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

#### Scenario: Scheduler-backed automation job reports downstream orchestration truth
- **WHEN** a scheduled job evaluates automation rules that start, block, or fail workflow orchestration
- **THEN** the persisted run detail includes machine-readable downstream counts for those outcomes
- **THEN** operators do not have to infer orchestration impact from a generic scan-only summary

## ADDED Requirements

### Requirement: Scheduler-backed due-date automation reports downstream workflow outcomes
The system SHALL preserve downstream automation and workflow-start outcome truth for the built-in due-date automation detector. When `automation-due-date-detector` evaluates `task.due_date_approaching` rules, its scheduler run summary and metrics MUST distinguish how many tasks were evaluated, how many rules matched, and how many downstream workflow starts were started, blocked, or failed.

#### Scenario: Due-date detector starts workflows and reports counts
- **WHEN** `automation-due-date-detector` evaluates due-date rules and at least one matching rule starts a workflow run
- **THEN** the scheduler run metrics include the number of evaluated tasks, matched rules, and started workflow runs
- **THEN** the run summary reflects that downstream workflow orchestration occurred during the run

#### Scenario: Due-date detector records blocked or failed workflow starts
- **WHEN** `automation-due-date-detector` evaluates matching due-date rules but some workflow starts are blocked by duplicate guards or fail validation
- **THEN** the scheduler run metrics include blocked or failed workflow-start counts for that run
- **THEN** operators can distinguish blocked or failed orchestration from a run that merely found no matching work
