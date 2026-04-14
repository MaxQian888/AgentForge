# opencode-runtime-bridge Specification

## Purpose
Define the canonical TypeScript bridge contract for executing OpenCode-backed coding-agent runs through the official OpenCode automation interfaces, including session binding, event normalization, and truthful pause or resume or cancel behavior behind the shared `/bridge/*` surface.
## Requirements
### Requirement: Bridge executes OpenCode through an official OpenCode automation transport
The TypeScript bridge SHALL execute requests resolved to runtime `opencode` by using an OpenCode automation interface supported by upstream documentation, with `opencode serve` HTTP APIs as the canonical integration surface for Bridge-managed execution. The bridge MUST NOT depend on a Bridge-private stdin JSON protocol as the primary execution contract for OpenCode.

#### Scenario: OpenCode execute request starts through the configured server transport
- **WHEN** the bridge accepts a valid execute request whose resolved runtime is `opencode` and the configured OpenCode server transport is reachable
- **THEN** the adapter SHALL create or reuse an upstream OpenCode session through the official transport before execution starts
- **THEN** the bridge SHALL bind the AgentForge task to that upstream OpenCode session for later lifecycle control

#### Scenario: OpenCode transport prerequisites are not satisfied
- **WHEN** the bridge resolves `opencode` for execution but the configured server URL, authentication, or required upstream capabilities are unavailable
- **THEN** the bridge SHALL reject the request with an explicit runtime-configuration error
- **THEN** it SHALL NOT create an active runtime entry or emit misleading running-state events

### Requirement: Bridge normalizes OpenCode session activity into canonical AgentForge runtime events
The TypeScript bridge SHALL translate OpenCode session and message activity into the canonical AgentForge runtime event model so Go callers keep consuming `output`, `tool_call`, `tool_result`, `cost_update`, `status_change`, `snapshot`, and `error` events without OpenCode-specific parsing.

#### Scenario: OpenCode emits assistant output and tool activity
- **WHEN** the upstream OpenCode session produces assistant text, tool invocation, tool result, or usage activity during an active task
- **THEN** the bridge SHALL emit the corresponding canonical AgentForge runtime events with stable task and session identifiers
- **THEN** runtime bookkeeping such as turn count, last tool, and spend SHALL stay synchronized with the normalized event stream

#### Scenario: OpenCode event streaming is interrupted before a terminal event
- **WHEN** the bridge loses the upstream OpenCode event stream while the task is still in progress
- **THEN** the bridge SHALL reconcile the latest upstream session or message state before deciding the terminal task status
- **THEN** it SHALL emit a truthful terminal or degraded error signal instead of silently treating the run as completed

### Requirement: OpenCode pause, resume, and cancel preserve truthful upstream session semantics
The TypeScript bridge SHALL manage `opencode` pause, resume, and cancel through the same bound upstream OpenCode session so resume continues prior work instead of replaying the original execute payload.

#### Scenario: Pause preserves resumable OpenCode session binding
- **WHEN** the Go orchestrator pauses an active `opencode` task
- **THEN** the bridge SHALL stop the current upstream OpenCode generation through the official control plane
- **THEN** it SHALL persist the upstream session binding and resume metadata needed to continue that same session later

#### Scenario: Resume continues the same upstream OpenCode session
- **WHEN** the Go orchestrator resumes a paused `opencode` task with a saved continuity snapshot
- **THEN** the bridge SHALL continue the bound upstream OpenCode session instead of starting a fresh session from the original prompt
- **THEN** the resumed task SHALL retain the same resolved runtime identity and continuity state lineage

#### Scenario: Cancel drops resumable continuity for OpenCode
- **WHEN** the Go orchestrator cancels an active or paused `opencode` task
- **THEN** the bridge SHALL abort the upstream OpenCode work and clear the resumable session binding for that task
- **THEN** subsequent resume attempts SHALL fail with an explicit non-resumable error

### Requirement: OpenCode continuity snapshots preserve server-backed control bindings after pause
The TypeScript bridge SHALL persist enough OpenCode continuity metadata to service server-backed control operations after a task is paused, including the upstream session identity and any related control-plane state needed for messages, diffs, commands, shell execution, and revert operations. Those controls MUST continue to target the same upstream session lineage rather than requiring a fresh execute or implicit resume.

#### Scenario: Pause captures session binding for later control operations
- **WHEN** the Bridge pauses an active `opencode` task
- **THEN** the saved continuity snapshot retains the upstream OpenCode session identity required for messages, diff, command, shell, revert, and unrevert controls
- **THEN** later control routes can act on that same upstream session even though the active runtime has been released

#### Scenario: Control request uses the saved OpenCode session lineage
- **WHEN** a caller targets a paused `opencode` task with a canonical server-backed control operation
- **THEN** the Bridge uses the persisted continuity snapshot to resolve the original upstream session lineage
- **THEN** it SHALL NOT start a fresh OpenCode session or replay the original execute payload merely to satisfy that control request

### Requirement: OpenCode auth and permission interactions are task- or session-bound bridge state
The TypeScript bridge SHALL persist Bridge-owned identifiers for OpenCode auth and permission interactions separately from Claude callback state, and each pending interaction MUST remain bound to the originating provider or upstream session context until it resolves or expires.

#### Scenario: Pending OpenCode permission interaction stays bound to its session context
- **WHEN** the Bridge emits a permission request for an OpenCode session
- **THEN** the Bridge stores the request with the originating upstream session identity and permission identifier
- **THEN** a later permission response resolves against that same OpenCode session context

#### Scenario: Pending OpenCode auth interaction expires without leaking a false ready state
- **WHEN** a provider-auth interaction is started but never completed
- **THEN** the Bridge expires the pending interaction after its timeout window
- **THEN** the runtime catalog continues to report the provider as unauthenticated or auth-required rather than falsely promoting it to ready

