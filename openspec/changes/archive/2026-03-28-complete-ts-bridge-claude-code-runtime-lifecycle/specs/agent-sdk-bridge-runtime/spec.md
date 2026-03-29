## ADDED Requirements

### Requirement: Bridge resumes Claude-backed runs from persisted continuity state
The TypeScript bridge SHALL treat `/bridge/resume` for `claude_code` runs as a continuity-aware recovery operation rather than a replay of the original execute request. When a paused or interrupted Claude-backed run has persisted continuity metadata, the bridge MUST restore that run through the Claude runtime adapter using the stored continuity state while preserving the resolved runtime identity.

#### Scenario: Resume uses persisted Claude continuity state
- **WHEN** the bridge receives a valid resume request for a paused `claude_code` task whose snapshot includes resumable continuity metadata
- **THEN** the bridge SHALL call the Claude-backed runtime adapter with that persisted continuity state instead of starting a fresh execute flow from only the original prompt
- **THEN** the resumed run SHALL keep the same resolved `runtime`, `provider`, and `model` identity that was stored in the snapshot

#### Scenario: Resume fails explicitly when continuity state is missing
- **WHEN** the bridge receives a resume request for a `claude_code` task whose snapshot lacks the required continuity metadata
- **THEN** the bridge SHALL reject the request with an explicit resume-readiness error
- **THEN** the bridge SHALL NOT silently fall back to replaying the original execute request as if a true resume had succeeded

## MODIFIED Requirements

### Requirement: Bridge executes real Claude Agent SDK runs
The TypeScript bridge SHALL execute agent work by invoking the real Claude-backed runtime adapter for every accepted execute request that resolves to the `claude_code` runtime, instead of emitting simulated placeholder steps. The bridge SHALL build the backend launch request from the canonical execute payload, including runtime selection, prompt, worktree, system prompt, role configuration, allowed tools, permission mode, max turns, budget context, any runtime-specific model hints, the effective MCP/tool runtime configuration, and any supported continuity inputs needed to continue an existing Claude-backed session without losing execution identity.

#### Scenario: Successful execute request starts the Claude-backed runtime
- **WHEN** the Go orchestrator sends a valid execute request that resolves to `claude_code` and the bridge has runtime capacity
- **THEN** the bridge SHALL start the real Claude-backed adapter for that task
- **THEN** the bridge SHALL return the session identifier for the started runtime
- **THEN** the bridge SHALL emit lifecycle events that move the task from starting to running to a terminal state based on the adapter result

#### Scenario: Execute request is rejected when the bridge is full
- **WHEN** the Go orchestrator sends an execute request for `claude_code` while the runtime pool is already at maximum capacity
- **THEN** the bridge SHALL reject the request with a capacity error
- **THEN** the bridge SHALL not create a new runtime entry for that task

#### Scenario: Claude launch tuple preserves resolved execution inputs
- **WHEN** the Go orchestrator sends a valid `claude_code` execute request with resolved runtime, provider, model, permission, tool, and MCP configuration
- **THEN** the bridge SHALL pass that normalized launch tuple to the Claude-backed adapter through one canonical launch-context builder
- **THEN** the adapter SHALL NOT silently drop the resolved model, permission mode, tool allowlist, or active MCP server configuration before execution begins

### Requirement: Bridge enforces cancellation and preserves continuity state
The bridge SHALL abort active Claude-backed execution when a cancel request, runtime abort, or local budget exhaustion occurs, and it SHALL preserve the latest known session continuity metadata for that task even when the run was selected through the shared runtime registry. The persisted continuity state MUST be detailed enough for later diagnostics and for a future resume attempt to determine whether a truthful Claude-backed recovery is possible.

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

#### Scenario: Pause stores a resumable continuity snapshot
- **WHEN** an active `claude_code` runtime is paused or terminates unexpectedly after the adapter has exposed resumable continuity metadata
- **THEN** the bridge SHALL persist both the normalized execute request and the latest known continuity snapshot for that task
- **THEN** later resume or diagnostic flows SHALL be able to distinguish a resumable Claude snapshot from a request-only legacy snapshot

### Requirement: Resolved runtime identity remains visible through bridge status metadata
The bridge SHALL expose the resolved `runtime`, `provider`, and `model` in the status metadata it returns to Go for active or paused runs, and it SHALL also expose the execution-context identity needed for diagnostics, including the selected `role_id` when present plus any validated team context bound to that runtime. For Claude-backed runs, the status or snapshot metadata MUST also expose whether resumable continuity state currently exists and, when it does not, the blocking reason. This metadata MUST remain stable across pause and resume flows so backend persistence and operator-facing summaries never need to re-infer runtime or team phase identity from legacy fallback rules.

#### Scenario: Status query returns execution identity for a team coder run
- **WHEN** the Go bridge client requests the status of an active run executing through `codex` as a team coder
- **THEN** the status payload includes the resolved `runtime`, `provider`, `model`, `role_id` when present, `team_id`, and `team_role=coder`
- **THEN** the backend can persist or render that identity without re-inferring it from unrelated run records

#### Scenario: Paused run keeps the same resolved execution identity
- **WHEN** the bridge reports status for a paused or resumable run
- **THEN** the status metadata continues to return the same runtime, provider, model, and validated role or team context that were used to start the run
- **THEN** resume or summary flows do not need to guess the execution identity from legacy fallback rules

#### Scenario: Claude paused status reports resume readiness
- **WHEN** the bridge reports status or snapshot metadata for a paused `claude_code` run
- **THEN** the payload SHALL indicate whether resumable continuity state is available
- **THEN** if resume is blocked, the payload SHALL include a machine-readable reason instead of leaving callers to infer the failure from missing fields alone
