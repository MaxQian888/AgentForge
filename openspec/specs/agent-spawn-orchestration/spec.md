# agent-spawn-orchestration Specification

## Purpose
Define the backend requirements for service-backed agent spawn orchestration, runtime state persistence, startup failure compensation, and authenticated WebSocket lifecycle delivery.
## Requirements
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

### Requirement: Agent spawn respects managed worktree guardrails
The spawn flow SHALL acquire task workspaces through the managed worktree lifecycle before bridge execution begins. If worktree allocation is denied because of capacity, path ownership, or unrecoverable stale state, the system MUST fail the spawn without leaving ambiguous task runtime metadata.

#### Scenario: Spawn surfaces worktree allocation denial
- **WHEN** an authenticated spawn request reaches worktree allocation and the manager returns a capacity, ownership, or stale-state error
- **THEN** the system does not mark the related task runtime as active
- **THEN** the system does not persist branch, worktree, or session metadata for the failed allocation attempt
- **THEN** the caller receives an error that reflects the worktree allocation failure

#### Scenario: Spawn reuses the healthy managed workspace for the task
- **WHEN** a spawn request targets a task whose canonical managed workspace already exists and is healthy
- **THEN** the system reuses that managed workspace instead of creating another checkout
- **THEN** the bridge execute request receives the canonical workspace path and branch for that task

### Requirement: Manual spawn reuses task assignment context
The system SHALL allow explicit agent spawn requests to reuse the task's current agent assignment context instead of requiring every caller to provide redundant member metadata. When a spawn request identifies a task but omits explicit member identity, the system MUST derive the dispatch target from the task's current assignee if that assignee is an active agent member for the same project.

#### Scenario: Task-scoped spawn resolves the assigned agent member
- **WHEN** a caller requests agent spawn for a task without explicitly providing `memberId`
- **THEN** the system reads the task's current assignee context before startup
- **THEN** the system starts runtime execution with that assigned agent member if the assignee is a valid active agent target
- **THEN** the request reuses the same startup and compensation semantics as other spawn flows

#### Scenario: Task-scoped spawn rejects tasks without a valid agent assignee
- **WHEN** a caller requests agent spawn for a task that is not currently assigned to an active agent member
- **THEN** the system MUST reject the request before runtime startup
- **THEN** the system MUST NOT create a new agent run for that request
- **THEN** the response explains that the task has no valid agent dispatch target
