## ADDED Requirements

### Requirement: Bridge executes real Claude Agent SDK runs
The TypeScript bridge SHALL execute agent work by invoking the real Claude Agent SDK for every accepted execute request, instead of emitting simulated placeholder steps. The bridge SHALL build the SDK request from the canonical execute payload, including prompt, worktree, system prompt, role configuration, allowed tools, permission mode, max turns, and budget context.

#### Scenario: Successful execute request starts a real runtime
- **WHEN** the Go orchestrator sends a valid execute request and the bridge has runtime capacity
- **THEN** the bridge starts a real Claude Agent SDK query for that task
- **THEN** the bridge returns the session identifier for the started runtime
- **THEN** the bridge emits lifecycle events that move the task from starting to running to a terminal state based on the SDK result

#### Scenario: Execute request is rejected when the bridge is full
- **WHEN** the Go orchestrator sends an execute request while the runtime pool is already at maximum capacity
- **THEN** the bridge rejects the request with a capacity error
- **THEN** the bridge does not create a new runtime entry for that task

### Requirement: Bridge and Go share one canonical execution contract
The bridge runtime SHALL expose one canonical HTTP contract for execute, status, cancel, and health operations, and the Go bridge client MUST use that same contract without route or field-name drift.

#### Scenario: Go queries runtime status through the canonical route
- **WHEN** the Go bridge client requests the status of an active task
- **THEN** the request targets the canonical bridge status endpoint and payload shape defined by this change
- **THEN** the bridge returns the current runtime state, turn count, last tool, last activity timestamp, and spend for that task

#### Scenario: Invalid execute payload is rejected consistently
- **WHEN** the Go bridge client sends an execute request that does not satisfy the bridge schema
- **THEN** the bridge returns a validation error that identifies the payload issue
- **THEN** the bridge does not start agent execution for that task

### Requirement: Bridge normalizes SDK output into AgentForge runtime events
The bridge SHALL translate Claude Agent SDK output into the existing AgentForge `AgentEvent` categories so the Go orchestrator receives structured output, tool activity, cost, status, error, and snapshot signals from a truthful runtime source.

#### Scenario: Assistant text and tool activity are streamed as structured events
- **WHEN** the Claude Agent SDK emits assistant output and tool activity during a task
- **THEN** the bridge emits `output` events for assistant text blocks
- **THEN** the bridge emits `tool_call` and `tool_result` events with stable task and session identifiers when tool activity occurs
- **THEN** the bridge updates runtime bookkeeping such as turn number, last tool, and last activity timestamp

#### Scenario: Usage data updates runtime cost tracking
- **WHEN** the Claude Agent SDK returns usage data for an in-flight task
- **THEN** the bridge emits a `cost_update` event with input, output, cache-read tokens, and calculated cost
- **THEN** the bridge updates the runtime's accumulated spend so status queries reflect the latest known cost

### Requirement: Bridge enforces cancellation and preserves continuity state
The bridge SHALL abort active Claude Agent SDK execution when a cancel request, runtime abort, or local budget exhaustion occurs, and it SHALL preserve the latest known session continuity metadata for that task.

#### Scenario: Explicit cancel stops the active runtime
- **WHEN** the Go orchestrator submits a cancel request for an active task
- **THEN** the bridge aborts the corresponding Claude Agent SDK run through the runtime abort controller
- **THEN** the bridge emits a terminal cancellation or failure event for that task
- **THEN** the runtime is removed from the active pool after cleanup

#### Scenario: Budget exhaustion terminates execution locally
- **WHEN** the bridge detects that the task's accumulated spend has reached or exceeded the task budget
- **THEN** the bridge stops the active Claude Agent SDK run without waiting for Go to issue a separate cancel request
- **THEN** the bridge emits an error or terminal event that identifies budget exhaustion
- **THEN** the bridge stores the latest known session continuity metadata for diagnostics or future resume work
