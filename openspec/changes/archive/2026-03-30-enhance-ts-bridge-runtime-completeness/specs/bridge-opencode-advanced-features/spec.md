## ADDED Requirements

### Requirement: OpenCode session fork
The Bridge SHALL support forking an OpenCode session by calling `POST /session/:id/fork` with an optional `messageID` to specify the fork point. The new session ID SHALL be captured in fresh continuity state.

#### Scenario: Fork session at specific message
- **WHEN** `/bridge/fork` is called with `{ task_id, message_id: "msg-42" }` where the task has OpenCode continuity
- **THEN** the Bridge calls `POST /session/{upstream_session_id}/fork` with `{ messageID: "msg-42" }` and stores the new session ID in continuity state

### Requirement: OpenCode message revert and unrevert
The Bridge SHALL support reverting and unreverting messages by calling `POST /session/:id/revert` and `POST /session/:id/unrevert` through the OpenCode transport layer.

#### Scenario: Revert a message
- **WHEN** `/bridge/revert` is called with `{ task_id, message_id: "msg-42" }`
- **THEN** the Bridge calls `POST /session/{upstream_session_id}/revert` with `{ messageID: "msg-42" }`

#### Scenario: Unrevert all messages
- **WHEN** `/bridge/unrevert` is called with `{ task_id }`
- **THEN** the Bridge calls `POST /session/{upstream_session_id}/unrevert`

### Requirement: OpenCode diff retrieval
The Bridge SHALL expose file diffs for an OpenCode session by calling `GET /session/:id/diff` through the transport layer and returning the result via the `/bridge/diff/:id` route.

#### Scenario: Retrieve session diffs
- **WHEN** `GET /bridge/diff/{task_id}` is called for a task with OpenCode continuity
- **THEN** the Bridge calls `GET /session/{upstream_session_id}/diff` and returns the file diff array

### Requirement: OpenCode todo list sync
The Bridge SHALL retrieve and forward todo lists from OpenCode sessions by calling `GET /session/:id/todo`. The Bridge SHALL also handle `todo.updated` SSE events and emit them as `todo_update` AgentEvents.

#### Scenario: Fetch session todos
- **WHEN** the Bridge receives a `todo.updated` SSE event for the active session
- **THEN** the Bridge emits `{ type: "todo_update", data: { items: event.todos } }`

### Requirement: OpenCode message history retrieval
The Bridge SHALL support fetching message history by calling `GET /session/:id/message` through the transport layer, returning the full conversation for a session.

#### Scenario: Retrieve message history
- **WHEN** `GET /bridge/messages/{task_id}` is called
- **THEN** the Bridge calls `GET /session/{upstream_session_id}/message` and returns the message list

### Requirement: OpenCode slash command execution
The Bridge SHALL support executing slash commands by calling `POST /session/:id/command` through the transport layer.

#### Scenario: Execute slash command
- **WHEN** `/bridge/command` is called with `{ task_id, command: "/compact" }`
- **THEN** the Bridge calls `POST /session/{upstream_session_id}/command` with `{ name: "compact" }`

### Requirement: OpenCode permission response forwarding
The Bridge SHALL support forwarding permission decisions by calling `POST /session/:id/permissions/:permissionID` when the Go orchestrator responds to a `permission_request` event.

#### Scenario: Permission granted
- **WHEN** the Bridge emitted a `permission_request` event and receives a callback with `{ permission_id: "perm-1", decision: "allow" }`
- **THEN** the Bridge calls `POST /session/{upstream_session_id}/permissions/perm-1` with `{ allow: true }`

### Requirement: OpenCode agent and skill discovery
The Bridge SHALL query `GET /agent` and `GET /skill` from the OpenCode server and include them in the runtime catalog response.

#### Scenario: Runtime catalog includes agents and skills
- **WHEN** `GET /bridge/runtimes` is called and OpenCode server is available
- **THEN** the catalog includes `opencode.agents` and `opencode.skills` arrays with available agents and skills

### Requirement: OpenCode extended SSE event handling
The Bridge SHALL handle `session.status`, `todo.updated`, `message.updated`, `command.executed`, and `vcs.branch.updated` SSE events in addition to existing event types.

#### Scenario: Session status event
- **WHEN** OpenCode emits a `session.status` event with `{ sessionID, status: "busy" }`
- **THEN** the Bridge emits `{ type: "status_change", data: { state: "running" } }` if it matches the active session

#### Scenario: Command executed event
- **WHEN** OpenCode emits a `command.executed` event with `{ name: "compact", sessionID }`
- **THEN** the Bridge emits `{ type: "output", data: { content: "Command /compact executed", content_type: "text" } }`

### Requirement: OpenCode extended part type parsing
The Bridge SHALL parse `ReasoningPart`, `FilePart`, `AgentPart`, `CompactionPart`, and `SubtaskPart` in addition to existing `TextPart` and `ToolPart`. Each part type SHALL be mapped to the appropriate AgentEvent type.

#### Scenario: Reasoning part emitted
- **WHEN** a `message.part.delta` or `message.part.updated` event contains a `ReasoningPart`
- **THEN** the Bridge emits `{ type: "reasoning", data: { content: part.reasoning } }`

#### Scenario: File part emitted
- **WHEN** a `message.part.updated` event contains a `FilePart`
- **THEN** the Bridge emits `{ type: "file_change", data: { files: part.files } }`

#### Scenario: Subtask part emitted
- **WHEN** a `message.part.updated` event contains a `SubtaskPart`
- **THEN** the Bridge emits `{ type: "output", data: { content: part.subtask_description, content_type: "subtask" } }`

### Requirement: OpenCode provider OAuth flow support
The Bridge SHALL support initiating and completing OAuth flows for OpenCode providers by calling `POST /provider/{id}/oauth/authorize` and `POST /provider/{id}/oauth/callback`.

#### Scenario: OAuth authorization initiated
- **WHEN** the Bridge detects an OpenCode provider needs OAuth and `hook_callback_url` is configured
- **THEN** the Bridge calls the authorize endpoint, emits a `permission_request` event with the auth URL, and completes the callback when the orchestrator responds

### Requirement: OpenCode runtime configuration updates
The Bridge SHALL support updating OpenCode runtime configuration by calling `PATCH /config` when execution parameters change (e.g., model switch).

#### Scenario: Update OpenCode model during session
- **WHEN** the Go orchestrator sends a model change request for an active OpenCode session
- **THEN** the Bridge calls `PATCH /config` with the updated provider/model configuration
