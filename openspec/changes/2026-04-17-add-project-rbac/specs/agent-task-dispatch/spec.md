## ADDED Requirements

### Requirement: Task dispatch SHALL authorize the initiating human before scheduling agent work
The task dispatch service SHALL resolve an initiating human's `projectRole` and verify it against the `task.dispatch` action in the RBAC matrix before any agent work is scheduled, worktree is allocated, or budget is reserved. The agent's own role manifest SHALL NOT substitute for this check.

#### Scenario: Viewer attempts to dispatch a task
- **WHEN** a human member with `projectRole=viewer` attempts to dispatch a task through the API or through any UI path that reaches the dispatch service
- **THEN** the service rejects the dispatch before agent work is scheduled
- **AND** no worktree, agent run record, or budget reservation is created

#### Scenario: Editor dispatches a task for an agent with a stricter agent role manifest
- **WHEN** a human member with `projectRole=editor` dispatches a task to an agent whose agent role manifest has narrower execution capabilities
- **THEN** the dispatch proceeds because `editor` satisfies the `task.dispatch` action requirement
- **AND** the agent's execution remains constrained by its own role manifest independently from the initiator's `projectRole`

### Requirement: Dispatch service signature SHALL require initiator identity at the type level
The task dispatch service API SHALL require a non-optional initiator identity parameter (or typed caller struct) for every dispatch call, so that dispatch cannot be invoked without an authenticated initiator or an explicit system-initiated flag. This requirement SHALL be visible in the service signature, not only enforced by runtime validation.

#### Scenario: Service caller omits initiator identity
- **WHEN** a call site invokes the dispatch service without providing an initiator identity or system-initiated marker
- **THEN** the build fails at compile time, or the service returns a structured error before acquiring any resources
- **AND** no dispatch side effects are produced

#### Scenario: Scheduler invokes dispatch as system-initiated
- **WHEN** a scheduled job calls the dispatch service as `systemInitiated=true`
- **THEN** the service additionally requires a `configuredByUserID` value representing the human who authorized the scheduled job
- **AND** the service evaluates that user's current `projectRole` for the `task.dispatch` action before scheduling
