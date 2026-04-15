# agent-spawn-orchestration Specification

## Purpose
Define the backend requirements for service-backed agent spawn orchestration, runtime state persistence, startup failure compensation, and authenticated WebSocket lifecycle delivery.
## Requirements
### Requirement: Agent spawn starts a real execution runtime
The spawn dialog SHALL include a role selector allowing operators to choose a role from the role library when spawning an agent. The selected role's ID SHALL be included as `roleId` in the spawn request. If no role is selected, the spawn request SHALL proceed without a `roleId` (preserving current behavior). The role selector SHALL populate from the role store and fetch roles on dialog open if not already loaded.

 **Changes**:
- **ADDED**: Before spawn proceeds, the system checks Bridge runtime pool status to verify capacity
- **ADDED**: If Bridge pool is at capacity, user sees warning with option to wait or proceed anyway
- **ADDED**: Spawn flow checks Bridge runtime health before execution

#### Scenario: Operator selects a role when spawning agent
- **WHEN** operator opens the spawn dialog and selects a role from the role dropdown
- **THEN** the spawn request includes the selected role's ID as `roleId`
 (existing behavior)

#### Scenario: Spawn checks Bridge pool capacity
- **WHEN** operator attempts to spawn agent and Bridge pool is at capacity (10/10 active)
- **THEN** system calls `GET /api/v1/bridge/pool` to check status
- **THEN** system displays warning "Bridge pool at capacity (10/10 agents active)"
 if pool is full
- **AND** offers options: "Wait in queue" or "Proceed anyway" if queue is full
- **THEN** if user chooses "Proceed anyway", spawn is queued
- **OR** if user chooses "Wait", spawn is queued with higher priority

#### Scenario: Spawn verifies Bridge health before execution
- **WHEN** operator spawns agent and Bridge health check succeeds
- **THEN** system calls `GET /api/v1/bridge/health` to verify Bridge is operational
- **THEN** spawn proceeds with worktree creation and bridge execution (existing flow)

#### Scenario: Bridge health check fails
- **WHEN** operator spawns agent and Bridge health check fails
- **THEN** system displays error "Bridge is unavailable" with health diagnostics
- **AND** offers options: "Retry" or "Cancel" if Bridge is down
- **THEN** if user cancels, spawn is aborted
- **OR** if user retries, system rechecks Bridge health

#### Scenario: Operator spawns without selecting a role
- **WHEN** operator opens the spawn dialog and leaves the role selector empty
- **THEN** the spawn request proceeds without a `roleId` field (existing behavior)
- **AND** the spawn succeeds using default agent configuration

#### Scenario: Role list loads on dialog open
- **WHEN** the spawn dialog opens and the role store has no loaded roles
- **THEN** the dialog fetches the role list from the API (existing behavior)
- **AND** displays available roles in the selector once loaded

#### Scenario: Spawn with available Bridge capacity
- **WHEN** operator spawns agent and Bridge pool shows 3/10 active
- **THEN** system proceeds with spawn without capacity warning (existing behavior)
- **AND** agent starts successfully

### Requirement: Spawn failure leaves no ambiguous runtime state
The system SHALL compensate for partial startup failures by marking a failed spawn as failed and removing any worktree created during that attempt. If worktree creation succeeds but bridge startup fails after the run record is created, the system MUST mark the run as failed and remove any worktree created during that attempt. The system MUST not leave the task pointing to a branch, worktree, or session that never became active.

 **Changes**: No changes to this requirement (preserved as-is)

#### Scenario: Bridge startup fails after worktree creation
- **WHEN** the system has already created the agent run and worktree but the bridge execute call fails
- **THEN** the system marks the agent run as `failed` (existing behavior)
- **THEN** the system removes the created worktree for that spawn attempt (existing behavior)
- **THEN** the system clears or avoids persisting task runtime metadata with the failed attempt (existing behavior)

#### Scenario: Worktree creation fails
- **WHEN** worktree creation fails before bridge execution begins
- **THEN** system marks spawn as failed immediately
- **AND** no bridge execution attempt is made
- **AND** no task runtime metadata is persisted

#### Scenario: Bridge execution succeeds
- **WHEN** worktree creation succeeds and bridge execution call succeeds
- **THEN** agent run is marked as `active` (existing behavior)
- **AND** task runtime metadata is persisted (existing behavior)

### Requirement: Agent lifecycle events are delivered over the authenticated WebSocket hub
The system SHALL expose truthful AgentPool and agent lifecycle events through the authenticated WebSocket hub used by backend services. Clients connected through the server WebSocket endpoint must be able to receive server-pushed `agent.started`, `agent.failed`, and related lifecycle signals from active runs, and they MUST also be able to receive explicit queued-admission feedback when a spawn request is accepted into the AgentPool queue instead of starting immediately. **Changes**: No changes to this requirement (preserved as-is)

#### Scenario: Spawn success produces a server-push lifecycle event
- **WHEN** a client is connected to the authenticated WebSocket endpoint for the task's project and a spawn request is admitted immediately (existing behavior)
- **THEN** the backend broadcasts an `agent.started` event through the shared hub (existing behavior)
- **THEN** the connected client receives the event without sending an echo message first (existing behavior)

#### Scenario: Queued spawn produces an explicit admission event
- **WHEN** a client is connected to the authenticated WebSocket endpoint for the task's project and a spawn request is accepted into the AgentPool queue (existing behavior)
- **THEN** the backend broadcasts an explicit queued-admission event for that project scope (existing behavior)
- **THEN** the connected client can distinguish the queued outcome from a failed or started spawn without relying on missing lifecycle events (existing behavior)

### Requirement: Agent spawn respects managed worktree guardrails
The spawn flow SHALL acquire task workspaces through the managed worktree lifecycle before bridge execution begins. If worktree allocation is denied because of capacity, path ownership, or unrecoverable stale state, the system MUST fail the spawn without leaving ambiguous task runtime metadata. **Changes**: No changes to this requirement (preserved as-is)

#### Scenario: Spawn surfaces worktree allocation denial
- **WHEN** worktree allocation is denied due to capacity limits (existing behavior)
- **THEN** spawn fails with clear error message (existing behavior)
- **AND** no task runtime metadata is persisted (existing behavior)

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
The system SHALL route task-scoped manual spawn requests and queue promotions through the same canonical dispatch preflight used by assignment-triggered dispatch. Manual spawn and promotion MUST reuse task or member context resolution, task or sprint or project budget admission checks, active-run conflict checks, transient system guardrail classification, and structured non-start outcomes, and they MUST emit matching history and queue metadata for every decisive verdict.

#### Scenario: Manual spawn returns a structured queued outcome
- **WHEN** an operator requests manual spawn for a task and AgentPool admission has no immediate slot available
- **THEN** the synchronous spawn result returns `queued`
- **THEN** the result includes the queue reference, priority, resolved runtime tuple, and machine-readable dispatch context used for that admission decision
- **THEN** the system MUST NOT create a real agent run until that queued request is later admitted

#### Scenario: Manual spawn is blocked by canonical dispatch guardrails before runtime startup
- **WHEN** an operator requests manual spawn for a task but canonical dispatch preflight fails because of budget, task or member validity, active-run conflict, or other control-plane guardrails
- **THEN** the synchronous spawn result returns `blocked`
- **THEN** the result carries the same machine-readable guardrail classification used by assignment-triggered dispatch
- **THEN** the resulting dispatch history entry preserves the same canonical metadata instead of degrading to a free-form failure string
- **THEN** the system MUST NOT create a new agent run for that request

#### Scenario: Queue promotion requeues recoverable guardrail failures with a matching history verdict
- **WHEN** a queued dispatch reaches promotion revalidation and the latest canonical preflight fails because of a recoverable budget or transient infrastructure guardrail
- **THEN** the queue entry remains queued with refreshed latest guardrail metadata and recovery disposition
- **THEN** the system records a new dispatch history verdict for that promotion recheck instead of silently mutating queue state only
- **THEN** consumers can distinguish a still-queued recoverable recheck from the original admission event

#### Scenario: Queue promotion records terminal invalidation truthfully
- **WHEN** a queued dispatch reaches promotion revalidation and the latest canonical preflight fails because task or member context is irrecoverably invalid
- **THEN** the queue entry transitions to a terminal failed state with machine-readable guardrail metadata
- **THEN** the system records a terminal dispatch history verdict for that promotion recheck
- **THEN** the control plane MUST NOT retry that entry as though it were still recoverable

#### Scenario: Successful promotion persists queue linkage and started verdict together
- **WHEN** a queued dispatch passes promotion revalidation and starts runtime execution successfully
- **THEN** the system records the resulting start through the canonical dispatch history path with linkage back to the originating queue entry
- **THEN** the queue lifecycle is completed with the promoted run linkage before promotion payloads are emitted to consumers
- **THEN** operator and realtime consumers can relate the started run back to the original queued admission without reconstructing it from free-form text

### Requirement: Spawn preflight rejects stale effective role bindings before admission
The spawn and dispatch entrypoints SHALL validate the effective role binding before queue admission or runtime startup, regardless of whether that binding came from an explicit request `roleId` or from the selected member's saved agent profile. If the effective role binding no longer resolves from the authoritative role registry, the system MUST return actionable preflight feedback and MUST NOT enqueue or start the run.

#### Scenario: Explicit role selection references a deleted role
- **WHEN** an operator submits a spawn request with an explicit `roleId` that no longer exists in the current role registry
- **THEN** the system rejects the request before runtime startup
- **THEN** no queue entry or agent run is created for that request
- **THEN** the response explains that the selected role binding is stale or missing

#### Scenario: Member-derived role binding is stale
- **WHEN** a spawn or dispatch flow resolves its effective role from the target member's saved agent profile and that bound `roleId` no longer exists in the current role registry
- **THEN** the system rejects the request before queue admission or runtime startup
- **THEN** the response identifies the member role binding as the failing preflight dependency
- **THEN** the operator is directed to repair the member's bound role instead of receiving a late runtime failure

