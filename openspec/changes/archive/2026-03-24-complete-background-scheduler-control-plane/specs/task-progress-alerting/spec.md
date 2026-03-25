## ADDED Requirements

### Requirement: Progress detector execution is managed by the scheduler control plane
The task progress detector SHALL run as a named scheduled job managed by the scheduler control plane instead of as an opaque in-process ticker. Its cadence, enable state, latest run result, and manual reruns MUST be operator-visible without changing the existing task-risk evaluation semantics.

#### Scenario: Scheduled detector run evaluates open tasks
- **WHEN** the scheduler triggers the registered progress-detector job
- **THEN** the system evaluates open tasks using the existing warning, stalled, and recovery rules for task progress
- **THEN** the detector run is recorded with a summary of how many tasks were checked and how many task progress states changed

#### Scenario: Manual rerun preserves alert deduplication
- **WHEN** an operator manually triggers the progress-detector job after a suspected missed run
- **THEN** the system re-evaluates open tasks immediately
- **THEN** unchanged stalled or warning conditions do not generate duplicate notifications beyond the existing deduplication contract

#### Scenario: Disabled detector pauses future automatic evaluations
- **WHEN** the progress-detector job is disabled in the scheduler control plane
- **THEN** future automatic detector executions stop
- **THEN** existing task progress snapshots remain readable until the detector is enabled again or a manual run is requested
