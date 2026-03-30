## 1. Types & Schema Foundation

- [x] 1.1 Extend `ExecuteRequest` in `types.ts` with all new optional fields: `thinking_config`, `output_schema`, `hooks_config`, `hook_callback_url`, `hook_timeout_ms`, `attachments`, `file_checkpointing`, `agents`, `disallowed_tools`, `fallback_model`, `additional_directories`, `include_partial_messages`, `tool_permission_callback`, `web_search`, `env`
- [x] 1.2 Extend `AgentEvent` in `types.ts` with new event types: `reasoning`, `file_change`, `todo_update`, `progress`, `rate_limit`, `partial_message`, `permission_request`
- [x] 1.3 Extend `AgentStatus` in `types.ts` with: `structured_output`, `thinking_enabled`, `file_checkpointing`, `active_hooks`, `subagent_count`
- [x] 1.4 Extend continuity state types: `ClaudeContinuityState` (query_ref, fork_available), `CodexContinuityState` (fork_available, rollback_turns), `OpenCodeContinuityState` (fork_available, revert_message_ids)
- [x] 1.5 Update Zod schemas in `schemas.ts` to validate all new ExecuteRequest fields
- [x] 1.6 Add new event data schemas for each new AgentEvent type
- [x] 1.7 Write tests for schema validation of new fields (accept valid, reject invalid)

## 2. Bridge HTTP Routes

- [x] 2.1 Add `POST /bridge/fork` route in `server.ts` with request validation and runtime delegation
- [x] 2.2 Add `POST /bridge/rollback` route with checkpoint-based and turn-based rollback support
- [x] 2.3 Add `POST /bridge/revert` and `POST /bridge/unrevert` routes for message-level undo
- [x] 2.4 Add `GET /bridge/diff/:task_id` route for file diff retrieval
- [x] 2.5 Add `GET /bridge/messages/:task_id` route for message history
- [x] 2.6 Add `POST /bridge/command` route for slash command execution
- [x] 2.7 Add `POST /bridge/interrupt` route for graceful query interruption
- [x] 2.8 Add `POST /bridge/model` route for mid-session model switching
- [x] 2.9 Add `POST /bridge/permission-response/:request_id` route for permission callback resolution
- [x] 2.10 Write route-level tests for all new endpoints (happy path + error cases)

## 3. Runtime Adapter Interface Extension

- [x] 3.1 Define optional adapter methods in `RuntimeAdapter` interface: `fork`, `rollback`, `revert`, `getMessages`, `getDiff`, `executeCommand`, `interrupt`, `setModel`
- [x] 3.2 Create `UnsupportedOperationError` class in `errors.ts`
- [x] 3.3 Add default throwing implementations for unsupported operations per runtime
- [x] 3.4 Wire route handlers to call adapter methods through `AgentRuntimeRegistry`
- [x] 3.5 Write adapter interface tests for supported/unsupported operation dispatch

## 4. Hook & Permission Callback Infrastructure

- [x] 4.1 Create `HookCallbackManager` class that registers pending callbacks, POSTs to callback URL, and resolves/rejects with timeout
- [x] 4.2 Wire `POST /bridge/permission-response/:request_id` to resolve pending callbacks in HookCallbackManager
- [x] 4.3 Write tests for callback lifecycle: register, resolve, timeout, reject

## 5. Claude Code Runtime — Advanced Features

- [x] 5.1 Wire `agents` field from ExecuteRequest into `buildClaudeQueryOptions()` as SDK `agents` option
- [x] 5.2 Wire `hooks_config` into SDK `hooks` option with callback-based hook handlers that POST to `hook_callback_url`
- [x] 5.3 Wire `thinking_config` into SDK `maxThinkingTokens` option
- [x] 5.4 Wire `file_checkpointing` into SDK `enableFileCheckpointing` option; retain `Query` reference for `rewindFiles()`
- [x] 5.5 Wire `output_schema` into SDK `outputFormat` option; extract `structured_output` from `SDKResultMessage`
- [x] 5.6 Wire `onElicitation` callback that emits `permission_request` events and awaits callback response
- [x] 5.7 Wire `tool_permission_callback` into SDK `canUseTool` callback using HookCallbackManager
- [x] 5.8 Wire `include_partial_messages` into SDK option; handle `SDKPartialAssistantMessage` as `partial_message` events
- [x] 5.9 Wire `disallowed_tools` into SDK `disallowedTools` option
- [x] 5.10 Wire `fallback_model` into SDK `fallbackModel` option
- [x] 5.11 Wire `additional_directories` into SDK `additionalDirectories` option
- [x] 5.12 Wire `env` into SDK `env` option
- [x] 5.13 Handle `SDKRateLimitEvent` → emit `rate_limit` AgentEvent
- [x] 5.14 Handle `SDKToolProgressMessage` → emit `progress` AgentEvent
- [x] 5.15 Handle `SDKCompactBoundaryMessage` → emit `status_change` AgentEvent
- [x] 5.16 Implement `interrupt()` adapter method via `Query.interrupt()`
- [x] 5.17 Implement `setModel()` adapter method via `Query.setModel()`
- [x] 5.18 Implement `fork()` adapter method via `forkSession` option
- [x] 5.19 Implement `rollback()` adapter method via `Query.rewindFiles()`
- [x] 5.20 Write tests for each new Claude runtime feature (mock SDK, verify option passthrough and event emission)

## 6. Codex Runtime — Advanced Features

- [x] 6.1 Handle `turn.started` event → emit `status_change` AgentEvent
- [x] 6.2 Handle `turn.failed` event → emit `error` AgentEvent with error details
- [x] 6.3 Handle `item.updated` event → emit `progress` AgentEvent with partial state
- [x] 6.4 Parse `Reasoning` item detail → emit `reasoning` AgentEvent
- [x] 6.5 Parse `FileChange` item detail → emit `file_change` AgentEvent
- [x] 6.6 Parse `McpToolCall` item detail → emit `tool_call` + `tool_result` AgentEvents
- [x] 6.7 Parse `WebSearch` item detail → emit `tool_call` + `tool_result` AgentEvents
- [x] 6.8 Parse `TodoList` item detail → emit `todo_update` AgentEvent
- [x] 6.9 Parse `CollabToolCall` and `Error` item details → emit appropriate events
- [x] 6.10 Wire `output_schema` → write temp schema file, add `--output-schema` flag
- [x] 6.11 Wire `attachments` → add `--image` flags for image attachments
- [x] 6.12 Wire `additional_directories` → add `--add-dir` flags
- [x] 6.13 Wire `web_search` → add `--search` flag
- [x] 6.14 Wire MCP plugin manifests → generate Codex-compatible config, pass via `--config`
- [x] 6.15 Implement `fork()` adapter method via `codex fork <thread-id>`
- [x] 6.16 Write tests for all new Codex event parsing and CLI flag generation

## 7. OpenCode Runtime — Advanced Features

- [x] 7.1 Add `forkSession(sessionId, messageId?)` method to `OpenCodeTransport`
- [x] 7.2 Add `revertMessage(sessionId, messageId)` and `unrevertMessages(sessionId)` methods to transport
- [x] 7.3 Add `getDiff(sessionId)` method to transport
- [x] 7.4 Add `getTodos(sessionId)` method to transport
- [x] 7.5 Add `getMessages(sessionId)` method to transport
- [x] 7.6 Add `executeCommand(sessionId, command, args?)` method to transport
- [x] 7.7 Add `respondToPermission(sessionId, permissionId, allow)` method to transport
- [x] 7.8 Add `getAgents()` and `getSkills()` methods to transport
- [x] 7.9 Add `updateConfig(config)` method to transport
- [x] 7.10 Handle `session.status` SSE event → emit `status_change` AgentEvent
- [x] 7.11 Handle `todo.updated` SSE event → emit `todo_update` AgentEvent
- [x] 7.12 Handle `message.updated` SSE event → emit updated message content
- [x] 7.13 Handle `command.executed` SSE event → emit `output` AgentEvent
- [x] 7.14 Handle `vcs.branch.updated` SSE event → emit `status_change` AgentEvent
- [x] 7.15 Parse `ReasoningPart` → emit `reasoning` AgentEvent
- [x] 7.16 Parse `FilePart` → emit `file_change` AgentEvent
- [x] 7.17 Parse `AgentPart` → emit `output` AgentEvent with agent context
- [x] 7.18 Parse `CompactionPart` → emit `status_change` AgentEvent
- [x] 7.19 Parse `SubtaskPart` → emit `output` AgentEvent with subtask content
- [x] 7.20 Wire adapter methods: `fork`, `revert`, `getMessages`, `getDiff`, `executeCommand`, `setModel`
- [x] 7.21 Include agents and skills in runtime catalog response
- [x] 7.22 Write tests for all new transport methods and event/part parsing

## 8. Integration & Verification

- [x] 8.1 Update `execute.ts` handler to pass new ExecuteRequest fields to runtime adapters
- [x] 8.2 Update `event-stream.ts` to handle new AgentEvent types in WebSocket serialization
- [x] 8.3 Update `session/manager.ts` to persist extended continuity state fields
- [x] 8.4 Update runtime catalog (`GET /bridge/runtimes`) to include supported advanced features per runtime
- [x] 8.5 Run full test suite, fix any regressions from type changes
- [x] 8.6 Add integration smoke test that validates all new routes return proper error for missing tasks
