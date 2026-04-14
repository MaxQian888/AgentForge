# bridge-claude-advanced-features Specification

## Purpose
Define the Claude Code-specific advanced execution capabilities exposed through the TypeScript bridge, including hooks, subagents, extended thinking, file checkpointing, structured output, permission callbacks, and live query controls.
## Requirements
### Requirement: Claude Code hooks forwarding
The Bridge SHALL accept a `hooks_config` field in ExecuteRequest containing hook definitions (PreToolUse, PostToolUse, SubagentStart, SubagentStop, PermissionRequest). When a hook fires during Claude execution, the Bridge SHALL POST the hook event to the `hook_callback_url` specified in ExecuteRequest and await a response within `hook_timeout_ms` (default 5000ms). The hook response SHALL be forwarded back to the SDK.

#### Scenario: PreToolUse hook intercepts a tool call
- **WHEN** ExecuteRequest includes `hooks_config` with a PreToolUse matcher and `hook_callback_url`
- **THEN** the Bridge passes the hook definition to `query()` options and when the hook fires, POSTs `{ hook: "PreToolUse", tool_name, tool_input, task_id }` to `hook_callback_url`, and forwards the response (`{ continue: true }`, `{ permissionDecision: "allow" }`, or `{ updatedInput }`) back to the SDK

#### Scenario: Hook callback timeout
- **WHEN** the Go orchestrator does not respond within `hook_timeout_ms`
- **THEN** the Bridge returns `{ continue: true }` as default (fail-open for tool execution) and emits a warning event

### Requirement: Claude Code subagent definitions
The Bridge SHALL accept an `agents` field in ExecuteRequest containing a map of agent definitions (description, prompt, tools, model). These SHALL be passed directly to the `query()` options `agents` parameter, enabling the Claude SDK to spawn subagents during execution.

#### Scenario: Multi-agent execution with subagent definitions
- **WHEN** ExecuteRequest includes `agents: { "reviewer": { description: "...", prompt: "...", tools: ["Read", "Grep"], model: "haiku" } }`
- **THEN** the Bridge passes these definitions to `query()` and the SDK can spawn the "reviewer" subagent. SubagentStart and SubagentStop hooks (if configured) fire for each spawn.

### Requirement: Claude Code extended thinking configuration
The Bridge SHALL accept a `thinking_config` field in ExecuteRequest with `{ enabled: boolean, budget_tokens?: number }`. When enabled, the Bridge SHALL pass `maxThinkingTokens` to `query()` options.

#### Scenario: Enable thinking with budget
- **WHEN** ExecuteRequest includes `thinking_config: { enabled: true, budget_tokens: 10000 }`
- **THEN** the Bridge passes `maxThinkingTokens: 10000` to the SDK and the model uses extended thinking up to that token budget

#### Scenario: Thinking disabled by default
- **WHEN** ExecuteRequest does not include `thinking_config`
- **THEN** the Bridge does not pass `maxThinkingTokens` to the SDK (SDK default behavior applies)

### Requirement: Claude Code file checkpointing
The Bridge SHALL accept `file_checkpointing: boolean` in ExecuteRequest. When true, the Bridge SHALL pass `enableFileCheckpointing: true` to `query()` and retain the `Query` object reference to support subsequent `rewindFiles()` calls via the `/bridge/rollback` route.

#### Scenario: Enable file checkpointing and rewind
- **WHEN** ExecuteRequest includes `file_checkpointing: true` and execution produces file changes
- **THEN** the Bridge enables checkpointing and the `/bridge/rollback` route can call `query.rewindFiles(messageUuid)` to revert files to a previous checkpoint

### Requirement: Claude Code structured output
The Bridge SHALL accept `output_schema` in ExecuteRequest. For Claude runtime, this SHALL be mapped to the `outputFormat: { type: "json_schema", schema }` option in `query()`. The final `SDKResultMessage.structured_output` SHALL be included in the completion event.

#### Scenario: Structured JSON output
- **WHEN** ExecuteRequest includes `output_schema: { type: "json_schema", schema: { type: "object", properties: { summary: { type: "string" } } } }`
- **THEN** the Bridge passes `outputFormat` to the SDK and the final event includes `structured_output` with the parsed JSON

### Requirement: Claude Code MCP elicitation handling
The Bridge SHALL register an `onElicitation` callback when MCP servers are configured. When an MCP server requests user input (auth, form fields), the Bridge SHALL emit a `permission_request` event to Go orchestrator via WebSocket and await a response via the hook callback URL.

#### Scenario: MCP server requests authentication
- **WHEN** an MCP server triggers an elicitation for OAuth credentials
- **THEN** the Bridge emits `{ type: "permission_request", data: { elicitation_type, fields, mcp_server_id } }` and forwards the Go orchestrator's response back to the SDK

### Requirement: Claude Code dynamic tool permission callback
The Bridge SHALL accept `tool_permission_callback: boolean` in ExecuteRequest. When true, the Bridge SHALL pass a `canUseTool` callback to `query()` that POSTs tool permission checks to `hook_callback_url` and returns the orchestrator's allow/deny decision.

#### Scenario: Dynamic tool permission check
- **WHEN** `tool_permission_callback: true` and Claude attempts to use a tool
- **THEN** the Bridge POSTs `{ callback_type: "tool_permission", tool_name, tool_input }` to `hook_callback_url` and the orchestrator's response determines whether the tool executes

### Requirement: Claude Code partial message streaming
The Bridge SHALL accept `include_partial_messages: boolean` in ExecuteRequest. When true, the Bridge SHALL pass `includePartialMessages: true` to `query()` and emit `partial_message` events for `SDKPartialAssistantMessage` types.

#### Scenario: Streaming partial responses
- **WHEN** `include_partial_messages: true`
- **THEN** the Bridge emits `{ type: "partial_message", data: { content, is_complete: false } }` events as the model generates tokens

### Requirement: Claude Code additional message type handling
The Bridge SHALL handle `SDKRateLimitEvent`, `SDKToolProgressMessage`, and `SDKCompactBoundaryMessage` message types from the SDK query stream, emitting them as `rate_limit`, `progress`, and `status_change` AgentEvents respectively.

#### Scenario: Rate limit event forwarded
- **WHEN** the SDK emits an `SDKRateLimitEvent` with utilization and reset time
- **THEN** the Bridge emits `{ type: "rate_limit", data: { utilization, reset_at } }`

#### Scenario: Tool progress event forwarded
- **WHEN** the SDK emits an `SDKToolProgressMessage` during a long-running tool
- **THEN** the Bridge emits `{ type: "progress", data: { tool_name, progress_text } }`

### Requirement: Claude Code model fallback
The Bridge SHALL accept `fallback_model?: string` in ExecuteRequest. For Claude runtime, this SHALL be passed as `fallbackModel` to `query()` options.

#### Scenario: Primary model unavailable with fallback
- **WHEN** ExecuteRequest includes `model: "opus"` and `fallback_model: "sonnet"` and the primary model is rate-limited
- **THEN** the SDK automatically falls back to the sonnet model

### Requirement: Claude Code disallowed tools
The Bridge SHALL accept `disallowed_tools?: string[]` in ExecuteRequest. For Claude runtime, this SHALL be passed as `disallowedTools` to `query()` options, explicitly preventing the agent from using specified tools.

#### Scenario: Explicitly block dangerous tools
- **WHEN** ExecuteRequest includes `disallowed_tools: ["Bash", "Write"]`
- **THEN** the SDK prevents the agent from invoking Bash or Write tools

### Requirement: Claude Code Query method exposure
The Bridge SHALL retain a reference to the `Query` object returned by `query()` and expose `interrupt()`, `setModel()`, `setMaxThinkingTokens()`, and `mcpServerStatus()` methods via Bridge HTTP routes.

#### Scenario: Interrupt running query
- **WHEN** a client calls `POST /bridge/interrupt` with `{ task_id }`
- **THEN** the Bridge calls `query.interrupt()` on the matching active query, which gracefully stops execution

#### Scenario: Change model mid-session
- **WHEN** a client calls `POST /bridge/model` with `{ task_id, model: "haiku" }`
- **THEN** the Bridge calls `query.setModel("haiku")` on the active query

### Requirement: Claude callback hook coverage aligns with the official hook event taxonomy used by AgentForge
The Bridge SHALL accept and publish the current Claude Code hook events that AgentForge depends on for orchestration, including tool lifecycle, subagent lifecycle, prompt submission, notification, and session lifecycle events. The callback payload and validation rules MUST stay aligned with the official Claude Code / Agent SDK hook surface instead of freezing the Bridge to an older subset of hook names.

#### Scenario: Request enables newer Claude hook events
- **WHEN** an execute request includes Claude hook subscriptions for events such as `SessionStart`, `SessionEnd`, `Notification`, or `UserPromptSubmit`
- **THEN** the Bridge SHALL validate those hook declarations and preserve them through the Claude runtime launch path
- **THEN** the Bridge SHALL NOT reject them merely because an older local schema only recognized tool and subagent events

#### Scenario: Hook event is surfaced through the Bridge callback contract
- **WHEN** Claude emits a supported hook event that AgentForge forwards through its orchestrator callback path
- **THEN** the Bridge SHALL emit a callback payload that identifies the hook event, task context, and relevant runtime payload fields
- **THEN** downstream orchestrators SHALL be able to make policy decisions without parsing Claude-native transport details directly

### Requirement: Claude live control publishing reflects query method availability truthfully
The Bridge SHALL publish and enforce Claude live controls based on the methods actually available on the active Query object. Controls such as interrupt, model switching, thinking-budget control, and MCP status introspection MUST only be advertised as supported when the active runtime and SDK surface can execute them.

#### Scenario: Active Query exposes a live control
- **WHEN** the active Claude Query exposes methods such as `interrupt`, `setModel`, `setMaxThinkingTokens`, or `mcpServerStatus`
- **THEN** the runtime capability metadata SHALL publish the corresponding control as supported for that active runtime
- **THEN** the canonical Bridge control route SHALL invoke the matching Query method instead of simulating the result

#### Scenario: Live control is unavailable on the active Query
- **WHEN** a caller requests a Claude live control whose underlying Query method is unavailable in the current SDK/runtime combination
- **THEN** the Bridge SHALL return an explicit unsupported or degraded response that identifies the missing Query capability
- **THEN** the runtime capability metadata SHALL match that unsupported or degraded state

