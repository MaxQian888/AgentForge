# bridge-cross-runtime-extensions Specification

## Purpose
Define the request, event, and lifecycle extensions shared across bridge runtimes, including structured output, attachments, fork and rollback flows, permission callbacks, and runtime-agnostic execution controls.

## Requirements
### Requirement: Structured output in ExecuteRequest
The Bridge SHALL accept an optional `output_schema` field in ExecuteRequest with shape `{ type: "json_schema"; schema: object }`. Each runtime adapter SHALL map this to its native structured output mechanism. The final completion event SHALL include a `structured_output` field with the parsed result.

#### Scenario: Structured output requested and returned
- **WHEN** ExecuteRequest includes `output_schema` and the runtime supports it
- **THEN** the final `status_change` event to `completed` state includes `structured_output` with the parsed JSON matching the schema

#### Scenario: Runtime does not support structured output
- **WHEN** ExecuteRequest includes `output_schema` but the runtime has no structured output mechanism
- **THEN** the Bridge ignores the field and completes normally without `structured_output` in the response

### Requirement: Attachment support in ExecuteRequest
The Bridge SHALL accept an optional `attachments` field in ExecuteRequest containing an array of `{ type: "image" | "file"; path: string; mime_type?: string }`. Each runtime adapter SHALL map attachments to its native mechanism (Claude: multi-modal messages; Codex: `--image` flags; OpenCode: message parts).

#### Scenario: Image attachment provided
- **WHEN** ExecuteRequest includes `attachments: [{ type: "image", path: "/tmp/screen.png" }]`
- **THEN** the runtime adapter passes the image to the underlying platform in its native format

### Requirement: Extended AgentEvent types
The Bridge SHALL support emitting these additional AgentEvent types: `reasoning` (chain-of-thought content), `file_change` (file modification details), `todo_update` (task list changes), `progress` (tool execution progress), `rate_limit` (rate limiting info), `partial_message` (streaming tokens), `permission_request` (permission/auth requests from runtime to orchestrator).

#### Scenario: New event type forwarded to Go
- **WHEN** a runtime emits a `reasoning` event
- **THEN** the Bridge sends `{ type: "reasoning", data: { content } }` via WebSocket to Go, and Go can handle or ignore it

### Requirement: Session fork operation
The Bridge SHALL expose `POST /bridge/fork` accepting `{ task_id, message_id?: string }`. The Bridge SHALL delegate to the runtime-specific fork mechanism and return `{ new_task_id, continuity }` with the forked session state.

#### Scenario: Fork delegates to correct runtime
- **WHEN** `/bridge/fork` is called for a task using Claude Code runtime
- **THEN** the Bridge uses Claude SDK's `forkSession` option to create a forked session
- **WHEN** `/bridge/fork` is called for a task using Codex runtime
- **THEN** the Bridge spawns `codex fork <thread-id>`
- **WHEN** `/bridge/fork` is called for a task using OpenCode runtime
- **THEN** the Bridge calls `POST /session/{id}/fork`

### Requirement: Session rollback operation
The Bridge SHALL expose `POST /bridge/rollback` accepting `{ task_id, checkpoint_id?: string, turns?: number }`. For Claude Code, this SHALL call `query.rewindFiles()`. For Codex, this maps to thread rollback. For OpenCode, this maps to message revert.

#### Scenario: Rollback to checkpoint
- **WHEN** `/bridge/rollback` is called with `{ task_id, checkpoint_id: "msg-uuid-42" }` for a Claude Code task with file checkpointing enabled
- **THEN** the Bridge calls `query.rewindFiles("msg-uuid-42")` and returns success

### Requirement: Tool permission callback pathway
The Bridge SHALL support a bidirectional tool permission flow: when a runtime needs a permission decision, the Bridge emits a `permission_request` event via WebSocket and provides a callback mechanism (HTTP POST to a temporary Bridge endpoint) for the Go orchestrator to respond. The response SHALL be forwarded to the runtime within `hook_timeout_ms`.

#### Scenario: Permission request round-trip
- **WHEN** Claude Code's `canUseTool` fires or OpenCode emits a permission request
- **THEN** the Bridge emits `{ type: "permission_request", data: { request_id, tool_name, context } }` and registers a pending callback. When Go POSTs to `/bridge/permission-response/{request_id}`, the Bridge resolves the pending callback and forwards the decision.

### Requirement: Additional directories in ExecuteRequest
The Bridge SHALL accept an optional `additional_directories?: string[]` in ExecuteRequest. For Claude Code, this maps to `additionalDirectories`. For Codex, this maps to `--add-dir` flags. OpenCode does not support this field.

#### Scenario: Extra directories granted across runtimes
- **WHEN** ExecuteRequest includes `additional_directories: ["/shared/data"]`
- **THEN** the runtime adapter passes the directories to the underlying platform in its native format

### Requirement: Web search toggle in ExecuteRequest
The Bridge SHALL accept an optional `web_search?: boolean` in ExecuteRequest. For Codex, this maps to `--search`. For Claude Code and OpenCode, this maps to enabling web search tools in the allowed tools list.

#### Scenario: Web search enabled
- **WHEN** ExecuteRequest includes `web_search: true`
- **THEN** the runtime adapter enables web search capability in the underlying platform

### Requirement: Environment variable injection
The Bridge SHALL accept an optional `env?: Record<string, string>` in ExecuteRequest. For Claude Code, this maps to the `env` option. For Codex, this maps to process environment. For OpenCode, this is included in session creation metadata.

#### Scenario: Custom environment variables
- **WHEN** ExecuteRequest includes `env: { "DATABASE_URL": "postgres://..." }`
- **THEN** the runtime adapter injects the environment variables into the execution context
