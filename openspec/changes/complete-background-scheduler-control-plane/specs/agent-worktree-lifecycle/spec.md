## ADDED Requirements

### Requirement: Managed worktree garbage collection is available as a scheduled repair job
The system SHALL expose managed worktree stale-state inspection and garbage collection as a registered scheduler job rather than only as a startup-time sweep or ad hoc repair path. The job MUST support recurring execution and operator-triggered runs while preserving the existing repair-oriented cleanup semantics.

#### Scenario: Recurring garbage-collection run repairs stale worktrees
- **WHEN** the scheduler triggers the registered worktree garbage-collection job
- **THEN** the system inspects managed worktree state for stale entries and performs repair-oriented cleanup for each stale workspace it finds
- **THEN** the job records a summary of how many stale worktrees were inspected, repaired, and left unresolved

#### Scenario: Operator triggers a cleanup run after repository drift
- **WHEN** an operator manually triggers the worktree garbage-collection job after detecting stale workspace state
- **THEN** the system executes the same canonical inspection and cleanup path used by recurring runs
- **THEN** the resulting run history identifies the trigger as manual and preserves the cleanup summary for later diagnosis

#### Scenario: No stale worktrees still produces a truthful run result
- **WHEN** the scheduled garbage-collection job runs and finds no stale managed worktree state
- **THEN** the system records the run as successful with a zero-cleanup summary
- **THEN** operators can confirm the job is healthy without inferring success from missing logs
