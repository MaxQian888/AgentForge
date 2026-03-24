## MODIFIED Requirements

### Requirement: Bridge executes real Claude Agent SDK runs
The TypeScript bridge SHALL execute agent work by invoking the real Claude-backed runtime adapter for every accepted execute request that resolves to the `claude_code` runtime, instead of emitting simulated placeholder steps. The bridge SHALL build the backend launch request from the canonical execute payload, including runtime selection, prompt, worktree, system prompt, role configuration, allowed tools, permission mode, max turns, budget context, and any runtime-specific model hints.

#### Scenario: Successful execute request starts the Claude-backed runtime
- **WHEN** the Go orchestrator sends a valid execute request that resolves to `claude_code` and the bridge has runtime capacity
- **THEN** the bridge SHALL start the real Claude-backed adapter for that task
- **THEN** the bridge SHALL return the session identifier for the started runtime
- **THEN** the bridge SHALL emit lifecycle events that move the task from starting to running to a terminal state based on the adapter result

#### Scenario: Execute request is rejected when the bridge is full
- **WHEN** the Go orchestrator sends an execute request for `claude_code` while the runtime pool is already at maximum capacity
- **THEN** the bridge SHALL reject the request with a capacity error
- **THEN** the bridge SHALL not create a new runtime entry for that task

### Requirement: Bridge and Go share one canonical execution contract
The bridge runtime SHALL expose one canonical HTTP contract for execute, status, cancel, and health operations, and the Go bridge client MUST use that same contract without route or field-name drift. The canonical execute contract MUST support explicit runtime selection while preserving defined backward-compatibility behavior for callers that do not yet send `runtime`.

#### Scenario: Go queries runtime status through the canonical route
- **WHEN** the Go bridge client requests the status of an active task
- **THEN** the request SHALL target the canonical bridge status endpoint and payload shape defined by this change
- **THEN** the bridge SHALL return the current runtime state, turn count, last tool, last activity timestamp, and spend for that task

#### Scenario: Invalid execute payload is rejected consistently
- **WHEN** the Go bridge client sends an execute request that does not satisfy the bridge schema, including runtime selection rules
- **THEN** the bridge SHALL return a validation error that identifies the payload issue
- **THEN** the bridge SHALL not start agent execution for that task

### Requirement: Bridge normalizes SDK output into AgentForge runtime events
The bridge SHALL translate Claude-backed runtime output into the existing AgentForge `AgentEvent` categories so the Go orchestrator receives structured output, tool activity, cost, status, error, and snapshot signals from a truthful runtime source, even after Claude execution is routed through the shared runtime registry.

#### Scenario: Assistant text and tool activity are streamed as structured events
- **WHEN** the Claude-backed runtime emits assistant output and tool activity during a task
- **THEN** the bridge SHALL emit `output` events for assistant text blocks
- **THEN** the bridge SHALL emit `tool_call` and `tool_result` events with stable task and session identifiers when tool activity occurs
- **THEN** the bridge SHALL update runtime bookkeeping such as turn number, last tool, and last activity timestamp

#### Scenario: Usage data updates runtime cost tracking
- **WHEN** the Claude-backed runtime returns usage data for an in-flight task
- **THEN** the bridge SHALL emit a `cost_update` event with input, output, cache-read tokens, and calculated cost
- **THEN** the bridge SHALL update the runtime's accumulated spend so status queries reflect the latest known cost

### Requirement: Bridge enforces cancellation and preserves continuity state
The bridge SHALL abort active Claude-backed execution when a cancel request, runtime abort, or local budget exhaustion occurs, and it SHALL preserve the latest known session continuity metadata for that task even when the run was selected through the shared runtime registry.

#### Scenario: Explicit cancel stops the active runtime
- **WHEN** the Go orchestrator submits a cancel request for an active `claude_code` task
- **THEN** the bridge SHALL abort the corresponding Claude-backed run through the runtime abort controller
- **THEN** the bridge SHALL emit a terminal cancellation or failure event for that task
- **THEN** the runtime SHALL be removed from the active pool after cleanup

#### Scenario: Budget exhaustion terminates execution locally
- **WHEN** the bridge detects that the task's accumulated spend has reached or exceeded the task budget during a `claude_code` run
- **THEN** the bridge SHALL stop the active Claude-backed run without waiting for Go to issue a separate cancel request
- **THEN** the bridge SHALL emit an error or terminal event that identifies budget exhaustion
- **THEN** the bridge SHALL store the latest known session continuity metadata for diagnostics or future resume work
