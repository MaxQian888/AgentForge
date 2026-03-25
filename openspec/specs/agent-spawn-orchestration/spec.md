# agent-spawn-orchestration Specification

## Purpose
Define the backend requirements for service-backed agent spawn orchestration, runtime state persistence, startup failure compensation, and authenticated WebSocket lifecycle delivery.
## Requirements
### Requirement: Agent spawn starts a real execution runtime
The system SHALL turn an authenticated spawn request into an AgentPool admission flow that either starts a real runtime immediately or records a truthful queued admission outcome when capacity is temporarily unavailable. When the request is admitted immediately, the spawn flow MUST create a new agent run in `starting` state, resolve the canonical execution context for that run, provision an isolated worktree, call the configured Bridge execute endpoint with the task, member, runtime and provider tuple, expanded role execution profile, and any applicable team execution context, persist the resulting branch, worktree, and session identifiers on the task, and mark the run as `running` only after the Bridge accepts the execution request.

#### Scenario: Successful spawn provisions runtime state immediately
- **WHEN** an authenticated client submits a valid spawn request for a task that has no active agent run and AgentPool admission has an available slot
- **THEN** the system creates a new agent run in `starting` state
- **THEN** the system resolves the execution context for that run, including role-derived runtime fields and any team-aware context that applies to the spawn
- **THEN** the system provisions a worktree and deterministic agent branch for that task
- **THEN** the system invokes the configured Bridge execute API with the canonical execution context and managed workspace information
- **THEN** the system stores `agent_branch`, `agent_worktree`, and `agent_session_id` on the task
- **THEN** the system updates the agent run status to `running`

#### Scenario: Spawn request is queued by AgentPool admission
- **WHEN** an authenticated client submits a valid spawn request for a task that has no active agent run but AgentPool admission has no immediate slot available
- **THEN** the system records a queue entry for that spawn request that preserves the canonical runtime selection and role context needed for later admission
- **THEN** the synchronous result reports that the request is `queued`
- **THEN** the system MUST NOT create a real agent run until the queued request is later admitted

### Requirement: Spawn failure leaves no ambiguous runtime state
The system SHALL compensate for partial startup failures so that a failed spawn does not leave stale runtime metadata behind. If worktree creation or bridge startup fails after the run record is created, the system MUST mark the run as failed and remove any worktree created for that attempt. The system MUST NOT leave the task pointing at a branch, worktree, or session that never became active.

#### Scenario: Bridge startup fails after worktree creation
- **WHEN** the system has already created the agent run and worktree but the bridge execute call fails
- **THEN** the system marks the agent run as `failed`
- **THEN** the system removes the created worktree for that spawn attempt
- **THEN** the system clears or avoids persisting task runtime metadata for the failed attempt

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

### Requirement: Manual spawn and queued promotion reuse dispatch control-plane guardrails
The system SHALL route task-scoped manual spawn requests and queue promotions through the same dispatch control-plane preflight used by assignment-triggered dispatch. Manual spawn and promotion MUST reuse task/member context resolution, budget admission checks, worktree readiness, and structured non-started outcomes instead of bypassing the task-centered dispatch contract.

#### Scenario: Manual spawn returns a structured queued outcome
- **WHEN** an operator requests manual spawn for a task and AgentPool admission has no immediate slot available
- **THEN** the synchronous spawn result returns `queued`
- **THEN** the result includes the queue reference and resolved dispatch context used for that admission decision
- **THEN** the system MUST NOT create a real agent run until that queued request is later admitted

#### Scenario: Manual spawn is blocked by dispatch guardrails before runtime startup
- **WHEN** an operator requests manual spawn for a task but dispatch preflight fails because of budget, task/member validity, or other control-plane guardrails
- **THEN** the synchronous spawn result returns `blocked`
- **THEN** the result carries the same machine-readable guardrail classification used by assignment-triggered dispatch
- **THEN** the system MUST NOT create a new agent run for that request

#### Scenario: Queue promotion revalidates the canonical dispatch preflight
- **WHEN** a queued dispatch becomes eligible for promotion after capacity is released
- **THEN** the system re-runs the canonical dispatch preflight before creating runtime state
- **THEN** only a passing decision may create a new agent run and persist task runtime metadata
- **THEN** a failing recheck is surfaced through the queue lifecycle without leaving ambiguous runtime state behind

