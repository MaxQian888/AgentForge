# agent-spawn-orchestration Specification

## Purpose
Define the backend requirements for service-backed agent spawn orchestration, runtime state persistence, startup failure compensation, and authenticated WebSocket lifecycle delivery.
## Requirements
### Requirement: Agent spawn starts a real execution runtime

The spawn dialog SHALL include a role selector allowing operators to choose a role from the role library when spawning an agent. The selected role's ID SHALL be included as `roleId` in the spawn request. If no role is selected, the spawn request SHALL proceed without a `roleId` (preserving current behavior). The role selector SHALL populate from the role store and fetch roles on dialog open if not already loaded.

#### Scenario: Operator selects a role when spawning agent
- **WHEN** operator opens the spawn dialog and selects a role from the role dropdown
- **THEN** the spawn request includes the selected role's ID as `roleId`
- **AND** the runtime selector remains independently configurable

#### Scenario: Operator spawns without selecting a role
- **WHEN** operator opens the spawn dialog and leaves the role selector empty
- **THEN** the spawn request proceeds without a `roleId` field
- **AND** the spawn succeeds using default agent configuration

#### Scenario: Role list loads on dialog open
- **WHEN** the spawn dialog opens and the role store has no loaded roles
- **THEN** the dialog fetches the role list from the API
- **AND** displays available roles in the selector once loaded

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

