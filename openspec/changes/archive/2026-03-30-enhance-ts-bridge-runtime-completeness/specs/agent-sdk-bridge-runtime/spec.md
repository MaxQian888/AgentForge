## MODIFIED Requirements

### Requirement: ExecuteRequest type definition
The ExecuteRequest type SHALL include all existing fields plus the following optional fields:
- `thinking_config?: { enabled: boolean; budget_tokens?: number }` — extended thinking configuration
- `output_schema?: { type: "json_schema"; schema: object }` — structured output schema
- `hooks_config?: { hooks: HookDefinition[]; callback_url: string; timeout_ms?: number }` — hook definitions with callback
- `hook_callback_url?: string` — URL for hook and permission callbacks
- `hook_timeout_ms?: number` — timeout for hook callbacks (default 5000)
- `attachments?: Array<{ type: "image" | "file"; path: string; mime_type?: string }>` — file/image attachments
- `file_checkpointing?: boolean` — enable file state snapshots
- `agents?: Record<string, { description: string; prompt: string; tools?: string[]; model?: string }>` — subagent definitions
- `disallowed_tools?: string[]` — explicit tool blocklist
- `fallback_model?: string` — model failover
- `additional_directories?: string[]` — extra directory access
- `include_partial_messages?: boolean` — stream partial tokens
- `tool_permission_callback?: boolean` — enable dynamic tool permission checks
- `web_search?: boolean` — enable web search
- `env?: Record<string, string>` — custom environment variables

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
