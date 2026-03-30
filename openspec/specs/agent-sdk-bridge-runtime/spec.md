# agent-sdk-bridge-runtime Specification

## Purpose
Define the baseline contract for executing real Claude-backed runs through the TypeScript bridge, including the canonical Go-to-bridge HTTP surface, explicit runtime selection, runtime event normalization, budget and cancellation enforcement, provider/model capability alignment, normalized role execution profiles, and continuity state persistence for diagnostics and future resume work.
## Requirements
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

### Requirement: Bridge and Go share one canonical execution contract
The bridge runtime SHALL expose one canonical HTTP contract under the `/bridge/*` route family for execute, status, cancel, pause, resume, health, and runtime catalog operations, and the Go bridge client MUST use that same canonical contract without route or field-name drift. Compatibility aliases MAY remain available for legacy callers, but they MUST stay behaviorally identical and SHALL NOT replace `/bridge/*` as the primary contract used by Go, tests, or live documentation. The canonical execute contract MUST support explicit runtime selection while preserving defined backward-compatibility behavior for callers that do not yet send `runtime`.

#### Scenario: Go queries runtime status through the canonical route
- **WHEN** the Go bridge client requests the status of an active task
- **THEN** the request SHALL target `/bridge/status/:id`
- **THEN** the bridge SHALL return the current runtime state, turn count, last tool, last activity timestamp, and spend for that task

#### Scenario: Invalid execute payload is rejected consistently
- **WHEN** the Go bridge client sends an execute request to `/bridge/execute` that does not satisfy the bridge schema, including runtime selection rules
- **THEN** the bridge SHALL return a validation error that identifies the payload issue
- **THEN** the bridge SHALL not start agent execution for that task

#### Scenario: Compatibility alias remains equivalent but secondary
- **WHEN** a legacy caller uses a supported alias for pause, resume, health, or another execution-adjacent bridge route
- **THEN** the alias SHALL invoke the same handler and validation behavior as the canonical `/bridge/*` route
- **THEN** project documentation and new callers SHALL still treat the `/bridge/*` route as the primary contract

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
The bridge SHALL abort active execution when a cancel request, runtime abort, or local budget exhaustion occurs, and it SHALL preserve truthful continuity metadata for runtimes that support future resume work. For `claude_code`, continuity metadata MAY remain bridge-local snapshots. For `opencode`, continuity metadata MUST include the upstream OpenCode session binding required to continue the same run instead of replaying the original execute payload.

#### Scenario: Explicit cancel stops the active runtime through its truthful control plane
- **WHEN** the Go orchestrator submits a cancel request for an active task
- **THEN** the bridge SHALL abort the corresponding runtime through the runtime-specific control plane
- **THEN** the bridge SHALL emit a terminal cancellation or failure event for that task
- **THEN** the runtime SHALL be removed from the active pool after cleanup

#### Scenario: Paused OpenCode run remains resumable without prompt replay
- **WHEN** the bridge pauses an active `opencode` task
- **THEN** the saved continuity metadata SHALL include the bound upstream OpenCode session identity and latest known resume metadata
- **THEN** a later resume request SHALL continue that same upstream session instead of starting a fresh execute call from the original payload

#### Scenario: Budget exhaustion terminates execution locally
- **WHEN** the bridge detects that the task's accumulated spend has reached or exceeded the task budget during a runtime that is still executing
- **THEN** the bridge SHALL stop the active run without waiting for Go to issue a separate cancel request
- **THEN** the bridge SHALL emit an error or terminal event that identifies budget exhaustion
- **THEN** it SHALL store the latest truthful continuity metadata permitted for that runtime

### Requirement: Agent execution honors the Bridge provider capability contract
The TypeScript Bridge SHALL resolve `provider` and `model` for execute requests through the shared provider registry, and it MUST only start agent execution when the resolved provider supports the `agent_execution` capability.

#### Scenario: Execute request uses the default supported provider
- **WHEN** the Go orchestrator submits a valid execute request without an explicit provider override
- **THEN** the Bridge SHALL resolve the default `agent_execution` provider and model from the registry
- **THEN** the Bridge SHALL start the real Claude Agent SDK-backed runtime for that task

#### Scenario: Execute request asks for an unsupported execution provider
- **WHEN** the Go orchestrator submits an execute request whose resolved provider does not support `agent_execution`
- **THEN** the Bridge SHALL reject the request before creating a runtime entry
- **THEN** it SHALL return an explicit error instead of ignoring the provider field or silently switching runtimes

### Requirement: Bridge execute requests accept normalized role execution profiles from Go
The TypeScript bridge SHALL treat `role_config` in execute requests as a normalized execution profile produced by the Go role-loading pipeline rather than as a raw Role YAML document. The bridge MUST apply the projected role persona, tool allowlist, bridge-consumable tool plugin identifiers, injected knowledge context, projected loaded skill context, available on-demand skill inventory, output filters, and permission constraints without needing to read YAML files, resolve inheritance, or interpret repo-local skill files locally.

#### Scenario: Expanded normalized role execution profile is honored
- **WHEN** the Go orchestrator submits an execute request with a valid normalized `role_config` that includes `allowed_tools`, `tools`, `knowledge_context`, loaded skill context, available skill inventory, and `output_filters`
- **THEN** the bridge uses that projected configuration when composing the effective system prompt, tool or plugin selection, and output filtering behavior for the runtime
- **THEN** execution uses the projected runtime-facing values instead of silently dropping the advanced fields

#### Scenario: Bridge does not need direct YAML or skill file access
- **WHEN** the bridge receives a valid execute or resume request whose role and skill tree were resolved from repo-local assets by Go
- **THEN** the bridge executes the task without reading the roles directory or parsing `skills/**/SKILL.md` itself
- **THEN** unsupported PRD-only sections remain outside the bridge request contract

#### Scenario: On-demand skills stay available without preloading full instructions
- **WHEN** the Go orchestrator submits a normalized role execution profile that contains available non-auto-load skills
- **THEN** the bridge preserves that inventory context for runtime prompt composition or diagnostics
- **THEN** the bridge does not inject the full instruction bodies for those on-demand skills unless the normalized profile explicitly marks them as loaded

### Requirement: Bridge rejects non-normalized role payloads
The bridge SHALL validate `role_config` against the normalized execution-profile contract and MUST reject payloads that omit required execution fields or attempt to send raw nested Role YAML structures that belong to the Go-side role model.

#### Scenario: Execute request with incomplete role execution data is rejected
- **WHEN** the Go orchestrator submits an execute request whose `role_config` omits required normalized execution fields such as the projected role name or system prompt inputs
- **THEN** the bridge returns a validation error and does not start execution

#### Scenario: Raw YAML-shaped role payload is rejected
- **WHEN** an execute request includes nested PRD role sections such as raw `metadata`, `knowledge`, or `security` objects where a normalized execution profile is expected
- **THEN** the bridge rejects the payload instead of trying to interpret it as runtime configuration

### Requirement: Go-managed execution sends explicit runtime-compatible launch tuples
The Go-managed execution path SHALL resolve and send an explicit `runtime`, `provider`, and `model` tuple for every agent run before calling the bridge, and the bridge MUST reject tuples that are incompatible or ambiguous before acquiring execution state.

#### Scenario: Compatible Codex tuple is accepted
- **WHEN** the Go orchestrator submits an execute request with `runtime=codex` and a provider/model combination supported for that runtime
- **THEN** the bridge SHALL accept the request without remapping it to another runtime
- **THEN** the started run SHALL retain the same resolved `runtime`, `provider`, and `model` identity

#### Scenario: Incompatible runtime and provider are rejected
- **WHEN** the Go orchestrator submits an execute request whose explicit `runtime` and `provider` do not form a supported coding-agent combination
- **THEN** the bridge SHALL reject the request before creating a runtime entry
- **THEN** the error SHALL identify that the runtime/provider tuple is incompatible instead of silently guessing a different runtime

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

### Requirement: Provider-only runtime inference is compatibility-only for non-Go callers
The system SHALL treat provider-only runtime inference as a compatibility path for legacy direct callers, and the Go-managed execution path MUST NOT depend on provider inference to determine runtime selection.

#### Scenario: Backend resolves project defaults before calling the bridge
- **WHEN** an operator starts an agent run without explicit runtime overrides and the project defaults resolve to a supported coding-agent selection
- **THEN** the Go backend SHALL send that resolved `runtime` explicitly to the bridge
- **THEN** the bridge execution path for that run SHALL not depend on provider-only runtime inference

#### Scenario: Legacy direct call still resolves a provider hint
- **WHEN** a non-Go compatibility caller submits a direct bridge execute request without `runtime` but with a recognized legacy provider hint
- **THEN** the bridge MAY resolve the runtime through the compatibility inference rule
- **THEN** that compatibility path SHALL NOT change the requirement that Go-managed calls send explicit runtime metadata

### Requirement: Bridge executes Codex runs through a truthful Codex connector
The TypeScript bridge SHALL execute accepted `runtime=codex` requests through a bridge-owned Codex connector that targets a real Codex automation surface, and it MUST translate that connector's native output into the canonical AgentForge runtime lifecycle without relying on a placeholder stdin/stdout protocol implemented by operators out of band.

#### Scenario: Successful Codex execute request starts the dedicated connector
- **WHEN** the Go orchestrator submits a valid execute request with `runtime=codex` and compatible provider/model values
- **THEN** the bridge SHALL start the dedicated Codex connector for that task
- **THEN** the bridge SHALL return the session identifier for the started Codex run
- **THEN** the bridge SHALL emit canonical lifecycle events that move the task from starting to running to a terminal state based on the Codex connector result

#### Scenario: Codex connector launch prerequisites are not satisfied
- **WHEN** the Go orchestrator submits an execute request with `runtime=codex` but the dedicated Codex connector cannot be launched because its supported setup is incomplete
- **THEN** the bridge SHALL reject the request with an explicit runtime-configuration error
- **THEN** the bridge SHALL not create a misleading active runtime entry for that task

### Requirement: Codex pause and resume preserve native continuity metadata
The TypeScript bridge SHALL persist Codex-specific continuity metadata in its session snapshots, and `/bridge/resume` MUST use that stored continuity state when continuing a paused Codex run instead of silently replaying the original execute request as a fresh run.

#### Scenario: Paused Codex run stores continuity state for resume
- **WHEN** an active Codex run is paused through the canonical bridge pause flow
- **THEN** the persisted snapshot SHALL include the resolved runtime identity and the Codex continuity metadata required to continue that run
- **THEN** later status or resume flows SHALL be able to reference the same Codex execution identity without re-inferring it

#### Scenario: Resume fails explicitly when Codex continuity state is missing
- **WHEN** `/bridge/resume` is called for a paused Codex task whose snapshot no longer contains valid Codex continuity metadata
- **THEN** the bridge SHALL return an explicit continuity or configuration error instead of starting a duplicate fresh run
- **THEN** operators SHALL be able to distinguish a failed resume from a newly started Codex execution

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

### Requirement: Go agent service performs post-spawn status verification
After dispatching an execute request to Bridge, Go agent service SHALL verify execution started by either receiving a WS `agent.started` event or falling back to a `GET /bridge/status/:id` call within a 5-second window.

#### Scenario: WS event confirms start
- **WHEN** Bridge sends `agent.started` WS event within 5 seconds of spawn
- **THEN** agent record status is updated to `running` from the WS event (no status poll needed)

#### Scenario: Fallback status poll on missing WS event
- **WHEN** no `agent.started` WS event arrives within 5 seconds
- **THEN** Go service calls `bridge.GetStatus(taskID)` and updates agent record from response state

### Requirement: Bridge test coverage includes all active and pool endpoints
Bridge server tests SHALL cover `/bridge/active` and `/bridge/pool` endpoints with comprehensive scenarios.

#### Scenario: Active endpoint returns running agents
- **WHEN** 2 agents are running and client calls `GET /bridge/active`
- **THEN** response contains array of 2 agent summaries with task_id, runtime, state, spent_usd

#### Scenario: Pool endpoint returns slot allocation
- **WHEN** pool has 3 active and 2 warm slots
- **THEN** `GET /bridge/pool` returns `{"active": 3, "available": N, "warm": 2, "queued": 0}`

### Requirement: ExecuteRequest type definition
The ExecuteRequest type SHALL include all existing fields plus the following optional fields:
- `thinking_config?: { enabled: boolean; budget_tokens?: number }` - extended thinking configuration
- `output_schema?: { type: "json_schema"; schema: object }` - structured output schema
- `hooks_config?: { hooks: HookDefinition[]; callback_url: string; timeout_ms?: number }` - hook definitions with callback
- `hook_callback_url?: string` - URL for hook and permission callbacks
- `hook_timeout_ms?: number` - timeout for hook callbacks (default 5000)
- `attachments?: Array<{ type: "image" | "file"; path: string; mime_type?: string }>` - file/image attachments
- `file_checkpointing?: boolean` - enable file state snapshots
- `agents?: Record<string, { description: string; prompt: string; tools?: string[]; model?: string }>` - subagent definitions
- `disallowed_tools?: string[]` - explicit tool blocklist
- `fallback_model?: string` - model failover
- `additional_directories?: string[]` - extra directory access
- `include_partial_messages?: boolean` - stream partial tokens
- `tool_permission_callback?: boolean` - enable dynamic tool permission checks
- `web_search?: boolean` - enable web search
- `env?: Record<string, string>` - custom environment variables

#### Scenario: Backward-compatible request
- **WHEN** an ExecuteRequest is sent without any new optional fields
- **THEN** the Bridge processes it identically to the current behavior with no changes

#### Scenario: New fields accepted
- **WHEN** an ExecuteRequest includes `thinking_config`, `output_schema`, and `agents`
- **THEN** the Bridge validates the fields via Zod schema and passes them to the runtime adapter

### Requirement: AgentEvent type definition
The AgentEvent type SHALL include all existing event types plus: `reasoning`, `file_change`, `todo_update`, `progress`, `rate_limit`, `partial_message`, `permission_request`. Each new type SHALL follow the existing `{ task_id, session_id, timestamp_ms, type, data }` envelope format.

#### Scenario: New event types emitted
- **WHEN** a runtime produces a reasoning trace
- **THEN** the Bridge emits `{ type: "reasoning", data: { content: string } }` via WebSocket

#### Scenario: Unknown event types handled by Go
- **WHEN** Go orchestrator receives an event type it does not recognize
- **THEN** Go logs and ignores the event (no crash)

### Requirement: AgentStatus extended fields
The AgentStatus type SHALL include optional fields: `structured_output?: object`, `thinking_enabled?: boolean`, `file_checkpointing?: boolean`, `active_hooks?: string[]`, `subagent_count?: number`.

#### Scenario: Status includes advanced features
- **WHEN** a task is running with thinking enabled and file checkpointing on
- **THEN** `GET /bridge/status/:id` returns `{ thinking_enabled: true, file_checkpointing: true }` in the status

### Requirement: RuntimeContinuityState extended fields
Each runtime's continuity state type SHALL include additional fields where applicable:
- ClaudeContinuityState: `query_ref?: string` (opaque reference for Query method calls), `fork_available?: boolean`
- CodexContinuityState: `fork_available?: boolean`, `rollback_turns?: number` (number of turns that can be rolled back)
- OpenCodeContinuityState: `fork_available?: boolean`, `revert_message_ids?: string[]` (message IDs that can be reverted)

#### Scenario: Continuity state captures fork availability
- **WHEN** a Claude Code session completes a turn with file checkpointing enabled
- **THEN** the continuity state includes `fork_available: true`

### Requirement: Runtime adapter extended interface
Each runtime adapter SHALL support optional methods: `fork(runtime, params)`, `rollback(runtime, params)`, `revert(runtime, params)`, `getMessages(runtime)`, `getDiff(runtime)`, `executeCommand(runtime, command)`, `interrupt(runtime)`, `setModel(runtime, model)`. Methods not supported by a runtime SHALL throw `UnsupportedOperationError`.

#### Scenario: Unsupported operation
- **WHEN** `fork()` is called on a runtime that does not support forking
- **THEN** the adapter throws `UnsupportedOperationError` with `{ operation: "fork", runtime: "claude_code" }`

#### Scenario: Supported operation delegates correctly
- **WHEN** `getDiff()` is called on an OpenCode runtime
- **THEN** the adapter calls `GET /session/{id}/diff` and returns the result

