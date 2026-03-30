## ADDED Requirements

### Requirement: Codex full event type handling
The Bridge SHALL handle all Codex JSONL event types: `thread.started`, `turn.started`, `turn.completed`, `turn.failed`, `item.started`, `item.updated`, `item.completed`, and `error`. The `turn.started` event SHALL emit a `status_change` AgentEvent. The `turn.failed` event SHALL emit an `error` AgentEvent with the error details and update runtime state to `failed`.

#### Scenario: Turn failure captured
- **WHEN** Codex emits a `turn.failed` event with `{ error: { message: "context overflow" } }`
- **THEN** the Bridge emits `{ type: "error", data: { message: "context overflow", source: "codex" } }` and sets runtime state to `failed`

#### Scenario: Item updated event for streaming progress
- **WHEN** Codex emits an `item.updated` event during command execution
- **THEN** the Bridge emits `{ type: "progress", data: { item_type, partial_output } }` with the current item state

### Requirement: Codex extended item detail parsing
The Bridge SHALL parse all ThreadItem detail variants: `AgentMessage`, `Reasoning`, `CommandExecution`, `FileChange`, `McpToolCall`, `CollabToolCall`, `WebSearch`, `TodoList`, and `Error`. Each variant SHALL be mapped to the appropriate AgentEvent type.

#### Scenario: Reasoning item emitted
- **WHEN** Codex emits an `item.completed` with `details.type: "Reasoning"`
- **THEN** the Bridge emits `{ type: "reasoning", data: { content: details.summary } }`

#### Scenario: FileChange item emitted
- **WHEN** Codex emits an `item.completed` with `details.type: "FileChange"`
- **THEN** the Bridge emits `{ type: "file_change", data: { changes: details.changes } }` with file paths and diff content

#### Scenario: McpToolCall item emitted
- **WHEN** Codex emits an `item.completed` with `details.type: "McpToolCall"`
- **THEN** the Bridge emits `{ type: "tool_call", data: { tool_name: details.tool_name, tool_input: details.input, call_id: details.id } }` followed by `{ type: "tool_result", data: { call_id: details.id, output: details.output } }`

#### Scenario: WebSearch item emitted
- **WHEN** Codex emits an `item.completed` with `details.type: "WebSearch"`
- **THEN** the Bridge emits `{ type: "tool_call", data: { tool_name: "web_search", tool_input: details.query } }` and `{ type: "tool_result", data: { output: details.results } }`

#### Scenario: TodoList item emitted
- **WHEN** Codex emits an `item.completed` with `details.type: "TodoList"`
- **THEN** the Bridge emits `{ type: "todo_update", data: { items: details.items } }`

### Requirement: Codex structured output
The Bridge SHALL accept `output_schema` in ExecuteRequest. For Codex runtime, this SHALL be written to a temporary JSON file and passed via `--output-schema <path>` CLI flag. The final agent message SHALL be parsed against the schema.

#### Scenario: Codex structured output with schema file
- **WHEN** ExecuteRequest includes `output_schema` and runtime is `codex`
- **THEN** the Bridge writes the schema to a temp file, adds `--output-schema /tmp/schema.json` to the Codex command, and parses the final output as structured JSON

### Requirement: Codex image attachment support
The Bridge SHALL accept `attachments` in ExecuteRequest containing image file paths. For Codex runtime, each image SHALL be passed via the `--image <path>` CLI flag.

#### Scenario: Image attached to Codex execution
- **WHEN** ExecuteRequest includes `attachments: [{ type: "image", path: "/tmp/screenshot.png" }]`
- **THEN** the Bridge adds `--image /tmp/screenshot.png` to the Codex CLI command

### Requirement: Codex additional writable directories
The Bridge SHALL accept `additional_directories?: string[]` in ExecuteRequest. For Codex runtime, each directory SHALL be passed via the `--add-dir <path>` CLI flag.

#### Scenario: Extra directories granted
- **WHEN** ExecuteRequest includes `additional_directories: ["/data/shared"]`
- **THEN** the Bridge adds `--add-dir /data/shared` to the Codex CLI command

### Requirement: Codex MCP server configuration passthrough
The Bridge SHALL translate active plugin manifests into Codex-compatible MCP server configuration. When MCP plugins are active, the Bridge SHALL pass them via `--config mcp_servers.<name>.command=<cmd>` CLI flags or generate a temporary Codex config file.

#### Scenario: MCP plugin forwarded to Codex
- **WHEN** the Bridge has active MCP plugins and runtime is `codex`
- **THEN** the Bridge generates Codex-compatible MCP config and passes it to the CLI, enabling the Codex agent to use the same MCP tools

### Requirement: Codex session fork support
The Bridge SHALL support forking a Codex session by invoking `codex fork <thread-id>` when the `/bridge/fork` route is called with a Codex continuity state.

#### Scenario: Fork existing Codex thread
- **WHEN** `/bridge/fork` is called with `{ task_id }` where the task has Codex continuity with `thread_id: "abc123"`
- **THEN** the Bridge spawns `codex fork abc123` and captures the new thread ID in fresh continuity state

### Requirement: Codex web search passthrough
The Bridge SHALL accept `web_search?: boolean` in ExecuteRequest. For Codex runtime, this SHALL add the `--search` flag to the CLI command.

#### Scenario: Web search enabled for Codex
- **WHEN** ExecuteRequest includes `web_search: true` and runtime is `codex`
- **THEN** the Bridge adds `--search` to the Codex CLI command
