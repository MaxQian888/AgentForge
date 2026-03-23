## ADDED Requirements

### Requirement: Agent spawn starts a real execution runtime
The system SHALL turn an authenticated agent spawn request into a real runtime startup flow instead of only persisting an `agent_runs` record. The spawn flow MUST create an agent run in `starting` state, provision an isolated worktree for the task, call the configured bridge execute endpoint, persist the resulting branch/worktree/session identifiers on the task, and mark the run as `running` only after the bridge accepts the execution request.

#### Scenario: Successful spawn provisions runtime state
- **WHEN** an authenticated client submits a valid spawn request for a task that has no active agent run
- **THEN** the system creates a new agent run in `starting` state
- **THEN** the system provisions a worktree and deterministic agent branch for that task
- **THEN** the system invokes the configured bridge execute API with the task, member, model, budget, and worktree context
- **THEN** the system stores `agent_branch`, `agent_worktree`, and `agent_session_id` on the task
- **THEN** the system updates the agent run status to `running`

### Requirement: Spawn failure leaves no ambiguous runtime state
The system SHALL compensate for partial startup failures so that a failed spawn does not leave stale runtime metadata behind. If worktree creation or bridge startup fails after the run record is created, the system MUST mark the run as failed and remove any worktree created for that attempt. The system MUST NOT leave the task pointing at a branch, worktree, or session that never became active.

#### Scenario: Bridge startup fails after worktree creation
- **WHEN** the system has already created the agent run and worktree but the bridge execute call fails
- **THEN** the system marks the agent run as `failed`
- **THEN** the system removes the created worktree for that spawn attempt
- **THEN** the system clears or avoids persisting task runtime metadata for the failed attempt

### Requirement: Agent lifecycle events are delivered over the authenticated WebSocket hub
The system SHALL expose agent lifecycle events through the authenticated WebSocket hub used by backend services. Clients connected through the server WebSocket endpoint MUST be able to receive server-pushed `agent.started`, `agent.failed`, and later lifecycle events emitted by the agent service for the relevant project scope.

#### Scenario: Spawn success produces a server-push lifecycle event
- **WHEN** a client is connected to the authenticated WebSocket endpoint for the task's project and a spawn request succeeds
- **THEN** the backend broadcasts an `agent.started` event through the shared hub
- **THEN** the connected client receives the event without sending an echo message first
