## MODIFIED Requirements

### Requirement: Agent spawn starts a real execution runtime
The system SHALL turn an authenticated spawn request into an AgentPool admission flow that either starts a real runtime immediately or records a truthful queued admission outcome when capacity is temporarily unavailable. When the request is admitted immediately, the spawn flow MUST create a new agent run in `starting` state, provision an isolated worktree for the task, call the configured bridge execute endpoint, persist the resulting branch, worktree, and session identifiers on the task, and mark the run as `running` only after the bridge accepts the execution request.

#### Scenario: Successful spawn provisions runtime state immediately
- **WHEN** an authenticated client submits a valid spawn request for a task that has no active agent run and AgentPool admission has an available slot
- **THEN** the system creates a new agent run in `starting` state
- **THEN** the system provisions a worktree and deterministic agent branch for that task
- **THEN** the system invokes the configured bridge execute API with the task, member, model, budget, and worktree context
- **THEN** the system stores `agent_branch`, `agent_worktree`, and `agent_session_id` on the task
- **THEN** the system updates the agent run status to `running`

#### Scenario: Spawn request is queued by AgentPool admission
- **WHEN** an authenticated client submits a valid spawn request for a task that has no active agent run but AgentPool admission has no immediate slot available
- **THEN** the system records a queue entry for that spawn request
- **THEN** the synchronous result reports that the request is `queued`
- **THEN** the system MUST NOT create a real agent run until the queued request is later admitted

### Requirement: Agent lifecycle events are delivered over the authenticated WebSocket hub
The system SHALL expose truthful AgentPool and agent lifecycle events through the authenticated WebSocket hub used by backend services. Clients connected through the server WebSocket endpoint MUST be able to receive server-pushed `agent.started`, `agent.failed`, and related lifecycle signals for active runs, and they MUST also be able to receive explicit queued-admission feedback when a spawn request is accepted into the AgentPool queue instead of starting immediately.

#### Scenario: Spawn success produces a server-push lifecycle event
- **WHEN** a client is connected to the authenticated WebSocket endpoint for the task's project and a spawn request is admitted immediately
- **THEN** the backend broadcasts an `agent.started` event through the shared hub
- **THEN** the connected client receives the event without sending an echo message first

#### Scenario: Queued spawn produces an explicit admission event
- **WHEN** a client is connected to the authenticated WebSocket endpoint for the task's project and a spawn request is accepted into the AgentPool queue
- **THEN** the backend broadcasts an explicit queued-admission event for that project scope
- **THEN** the connected client can distinguish the queued outcome from a failed or started spawn without relying on missing lifecycle events
