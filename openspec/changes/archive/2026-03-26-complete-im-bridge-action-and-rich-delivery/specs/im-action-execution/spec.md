## ADDED Requirements

### Requirement: Shared IM actions SHALL execute canonical backend workflows
The system SHALL treat normalized IM actions as executable backend workflow requests instead of placeholder acknowledgements. When the Bridge submits a supported shared action through `/api/v1/im/action`, the backend MUST execute the corresponding task, agent, or review operation, and it MUST return a canonical action result that truthfully reports whether the operation started, completed, blocked, or failed.

#### Scenario: Assign-agent action dispatches the real task workflow
- **WHEN** the Bridge submits an `assign-agent` action for a valid task with a preserved reply target
- **THEN** the backend assigns the task through the existing task-dispatch or agent-spawn workflow instead of returning a placeholder acknowledgement
- **AND** the action result reports the resulting task identity, dispatch status, and run identity when an Agent run is created

#### Scenario: Decompose action executes task decomposition
- **WHEN** the Bridge submits a `decompose` action for a task that can be decomposed
- **THEN** the backend invokes the task decomposition workflow
- **AND** the action result reports the parent task plus the created or updated decomposition outcome instead of only confirming receipt

#### Scenario: Review action updates the real review state
- **WHEN** the Bridge submits `approve` or `request-changes` for a valid review entity
- **THEN** the backend updates the persisted review outcome through the existing review workflow
- **AND** the returned action result truthfully reflects the new review state

### Requirement: Action outcomes SHALL remain explicit when execution is blocked or invalid
If a normalized IM action cannot be executed because the entity is missing, the transition is no longer valid, the binding is stale, or a downstream workflow refuses the request, the system MUST return an explicit blocked or failed outcome. It MUST NOT report success for operations that were not actually performed.

#### Scenario: Stale review action returns an explicit terminal failure
- **WHEN** a user clicks an IM review action for a review that is already completed in an incompatible state
- **THEN** the backend returns a terminal blocked or failed action outcome
- **AND** the user-visible response explains that the requested transition was not applied

#### Scenario: Missing action entity is rejected without fake success
- **WHEN** the Bridge submits an action whose task, run, or review entity cannot be resolved
- **THEN** the backend rejects the action explicitly
- **AND** the returned action outcome does not claim that assignment, decomposition, or approval succeeded

#### Scenario: Action result preserves reply-target-aware completion context
- **WHEN** an executable or blocked action result is returned to the Bridge
- **THEN** the result preserves the canonical reply target and metadata needed for follow-up delivery
- **AND** the Bridge can render the terminal outcome back into the originating conversation without guessing a new destination
