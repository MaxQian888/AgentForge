# agent-sdk-bridge-runtime Specification

## Purpose
Define the baseline contract for executing real Claude-backed runs through the TypeScript bridge, including the canonical Go-to-bridge HTTP surface, explicit runtime selection, runtime event normalization, budget and cancellation enforcement, provider/model capability alignment, normalized role execution profiles, and continuity state persistence for diagnostics and future resume work.
## Requirements
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
The TypeScript bridge SHALL treat `role_config` in execute requests as a normalized execution profile produced by the Go role-loading pipeline rather than as a raw Role YAML document. The bridge MUST apply the projected role persona, tool allowlist, bridge-consumable tool plugin identifiers, injected knowledge context, output filters, and permission constraints without needing to read YAML files, resolve inheritance, or interpret PRD-only role metadata locally.

#### Scenario: Expanded normalized role execution profile is honored
- **WHEN** the Go orchestrator submits an execute request with a valid normalized `role_config` that includes `allowed_tools`, `tools`, `knowledge_context`, and `output_filters`
- **THEN** the bridge uses that projected configuration when composing the effective system prompt, tool or plugin selection, and output filtering behavior for the runtime
- **THEN** execution uses the projected runtime-facing values instead of silently dropping the advanced fields

#### Scenario: Bridge does not need direct YAML access
- **WHEN** the bridge receives a valid execute or resume request whose role was resolved from YAML by Go
- **THEN** the bridge executes the task without reading the roles directory or parsing the source YAML itself
- **THEN** unsupported PRD-only sections remain outside the bridge request contract

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
The bridge SHALL expose the resolved `runtime`, `provider`, and `model` in the status metadata it returns to Go for active or paused runs, and it SHALL also expose the execution-context identity needed for diagnostics, including the selected `role_id` when present plus any validated team context bound to that runtime. This metadata MUST remain stable across pause and resume flows so backend persistence and operator-facing summaries never need to re-infer runtime or team phase identity from legacy fallback rules.

#### Scenario: Status query returns execution identity for a team coder run
- **WHEN** the Go bridge client requests the status of an active run executing through `codex` as a team coder
- **THEN** the status payload includes the resolved `runtime`, `provider`, `model`, `role_id` when present, `team_id`, and `team_role=coder`
- **THEN** the backend can persist or render that identity without re-inferring it from unrelated run records

#### Scenario: Paused run keeps the same resolved execution identity
- **WHEN** the bridge reports status for a paused or resumable run
- **THEN** the status metadata continues to return the same runtime, provider, model, and validated role or team context that were used to start the run
- **THEN** resume or summary flows do not need to guess the execution identity from legacy fallback rules

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

