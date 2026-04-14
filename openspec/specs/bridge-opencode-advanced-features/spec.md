# bridge-opencode-advanced-features Specification

## Purpose
Define the advanced OpenCode session, transport, catalog, event, and permission capabilities exposed through the TypeScript bridge.
## Requirements
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
The Bridge SHALL support updating OpenCode runtime configuration by calling `PATCH /config` when execution parameters change before or during a session. Model switches MUST use this path for active sessions. Execute-time provider or capability settings such as provider selection, environment/config overlays, or web-search intent MUST also flow through official session or config surfaces before prompt submission whenever the server exposes them.

#### Scenario: Update OpenCode model during session
- **WHEN** the Go orchestrator sends a model change request for an active OpenCode session
- **THEN** the Bridge calls `PATCH /config` with the updated provider or model configuration

#### Scenario: Apply execute-time config before prompt submission
- **WHEN** an OpenCode execute request includes provider or capability settings that require server-side configuration before prompt submission
- **THEN** the Bridge patches or initializes the official OpenCode config for that session before calling `prompt_async`
- **THEN** the resulting session state remains aligned with the runtime catalog and execute preflight outcome

### Requirement: OpenCode runtime catalog includes server-backed provider and session control metadata
The Bridge SHALL publish OpenCode's server-backed control-plane metadata in the runtime catalog, including discovered agents, skills, provider availability, provider-auth readiness, parity-sensitive execute input support, and session control surfaces such as rollback, messages, command execution, shell execution, and permission-response loops whenever the official OpenCode server makes them available.

#### Scenario: OpenCode server is healthy and exposes discovery surfaces
- **WHEN** the Bridge refreshes runtime catalog metadata while the OpenCode server is reachable
- **THEN** the OpenCode catalog entry SHALL include discovered agents, skills, provider metadata, and the published support state of server-backed session controls and execute inputs
- **THEN** upstream consumers SHALL be able to distinguish “server reachable but auth or config required” from “control not supported by the current Bridge contract”

#### Scenario: OpenCode provider auth or config update is required before execution
- **WHEN** the selected OpenCode provider requires OAuth or equivalent configuration changes before a run can start
- **THEN** the runtime diagnostics and capability metadata SHALL identify the auth or config prerequisite explicitly
- **THEN** the Bridge SHALL NOT report the runtime as simply unavailable without indicating the missing provider-auth handshake or config step

### Requirement: OpenCode shell execution is exposed through the canonical Bridge control plane
The Bridge SHALL expose shell execution for OpenCode through a canonical Bridge route and SHALL proxy that control through the official `POST /session/:id/shell` server API when the selected runtime publishes shell support.

#### Scenario: Shell execution requested for an active OpenCode session
- **WHEN** a caller invokes the canonical Bridge shell route for a task backed by an active OpenCode session
- **THEN** the Bridge SHALL call the official OpenCode shell endpoint for the bound session and return the resulting assistant message payload
- **THEN** the runtime capability metadata SHALL publish shell execution as supported for that runtime

#### Scenario: Shell execution requested for a runtime without shell support
- **WHEN** a caller invokes the canonical Bridge shell route for a runtime that does not publish shell control support
- **THEN** the Bridge SHALL reject the request with a structured unsupported response
- **THEN** it SHALL NOT emulate shell execution through a different non-canonical route

### Requirement: OpenCode permission requests bind canonical Bridge request IDs to upstream permission identifiers
The Bridge SHALL normalize each OpenCode permission request into a Bridge-owned pending interaction record that stores the originating upstream session binding plus the upstream permission identifier. When OpenCode asks for approval, the Bridge SHALL emit a canonical `permission_request` event containing a Bridge-generated `request_id` and SHALL resolve `POST /bridge/permission-response/:request_id` by forwarding the decision to the matching upstream OpenCode permission endpoint instead of treating the request as a Claude-only callback.

#### Scenario: OpenCode permission request round-trip resolves against the correct session permission
- **WHEN** an active OpenCode session emits a permission request with upstream `permissionID` `perm-42`
- **THEN** the Bridge emits a canonical `permission_request` event that includes a new Bridge `request_id`
- **THEN** `POST /bridge/permission-response/{request_id}` forwards the caller's allow or deny decision to `POST /session/{upstream_session_id}/permissions/perm-42`

#### Scenario: Permission response is rejected when the Bridge no longer has a live pending mapping
- **WHEN** a caller posts to `/bridge/permission-response/{request_id}` after the pending OpenCode permission mapping expired or was already resolved
- **THEN** the Bridge returns an explicit pending-request-not-found error
- **THEN** it SHALL NOT claim the permission decision succeeded locally

### Requirement: OpenCode provider auth handshake is exposed through canonical Bridge control routes
The Bridge SHALL expose additive canonical routes for OpenCode provider authentication under the `/bridge/*` family so callers can initiate and complete provider OAuth or equivalent upstream auth without bypassing the Bridge runtime contract. The Bridge SHALL use the upstream OpenCode provider authorize and callback surfaces behind those routes and SHALL publish the resulting provider-auth state back through the runtime catalog.

#### Scenario: Start provider auth for a disconnected OpenCode provider
- **WHEN** a caller posts to `POST /bridge/opencode/provider-auth/{provider}/start` for an OpenCode provider whose catalog entry reports `auth_required=true`
- **THEN** the Bridge requests the upstream provider authorize surface and returns a Bridge-owned `request_id` plus the authorization URL or equivalent auth payload
- **THEN** the pending auth interaction remains bound to that provider until completion or expiry

#### Scenario: Complete provider auth and refresh catalog readiness
- **WHEN** a caller posts the callback payload to `POST /bridge/opencode/provider-auth/{request_id}/complete` for a pending OpenCode provider-auth interaction
- **THEN** the Bridge forwards the opaque callback payload to the matching upstream provider callback surface
- **THEN** subsequent OpenCode runtime catalog reads reflect the updated provider connectivity or remaining auth failure truthfully

### Requirement: OpenCode execute inputs are handled through official transport surfaces
The Bridge SHALL handle parity-sensitive ExecuteRequest inputs for OpenCode through official transport surfaces before `prompt_async` begins. Supported attachments MUST be encoded as official OpenCode prompt parts. Provider or session config such as `env` or `web_search` MUST be applied through official session bootstrap or config update surfaces when the selected OpenCode server exposes them. Inputs that cannot be represented truthfully MUST be rejected before prompt submission rather than silently dropped.

#### Scenario: OpenCode attachment is forwarded as a prompt part
- **WHEN** ExecuteRequest includes a supported attachment for an OpenCode run
- **THEN** the Bridge encodes that attachment as an official OpenCode prompt part for the bound session
- **THEN** the agent receives the attachment within the same run

#### Scenario: Unsupported OpenCode execute input is rejected before prompt submission
- **WHEN** ExecuteRequest asks OpenCode to use an input that the selected server or provider does not expose truthfully
- **THEN** the Bridge returns an explicit validation or configuration error before `prompt_async`
- **THEN** no OpenCode prompt is sent for that task

### Requirement: OpenCode rollback uses continuity-backed revert targets
The Bridge SHALL resolve canonical `/bridge/rollback` for OpenCode by translating `checkpoint_id` or `turns` into revertable message targets stored in OpenCode continuity or recovered from the bound session history. The Bridge MUST call the official OpenCode revert or unrevert endpoints and MUST preserve enough continuity metadata to explain rollback failures truthfully.

#### Scenario: Rollback to explicit message checkpoint
- **WHEN** `/bridge/rollback` is called for an OpenCode task with a message checkpoint that maps to the bound upstream session
- **THEN** the Bridge calls the official OpenCode revert endpoint for that session and message
- **THEN** the rollback request returns success without degrading to a blanket unsupported error

#### Scenario: Rollback target cannot be resolved
- **WHEN** `/bridge/rollback` is called for an OpenCode task whose continuity lacks a resolvable message target
- **THEN** the Bridge returns a structured runtime-specific rollback error
- **THEN** the response identifies the missing continuity or history prerequisite
