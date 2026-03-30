## ADDED Requirements

### Requirement: Fork route
The Bridge SHALL expose `POST /bridge/fork` accepting `{ task_id: string, message_id?: string }` and returning `{ new_task_id: string, continuity: RuntimeContinuityState }`. The route SHALL delegate to the runtime-specific fork mechanism based on the task's active runtime.

#### Scenario: Fork route succeeds
- **WHEN** `POST /bridge/fork` is called with `{ task_id: "task-1" }` for an active task
- **THEN** the Bridge returns 200 with new continuity state for the forked session

#### Scenario: Fork route for non-existent task
- **WHEN** `POST /bridge/fork` is called with a task_id that has no active runtime
- **THEN** the Bridge returns 404 with `{ error: "task not found" }`

### Requirement: Rollback route
The Bridge SHALL expose `POST /bridge/rollback` accepting `{ task_id: string, checkpoint_id?: string, turns?: number }` and returning `{ success: boolean }`.

#### Scenario: Rollback succeeds
- **WHEN** `POST /bridge/rollback` is called with `{ task_id: "task-1", checkpoint_id: "uuid-42" }` for a task with file checkpointing
- **THEN** the Bridge returns 200 with `{ success: true }`

### Requirement: Revert route
The Bridge SHALL expose `POST /bridge/revert` accepting `{ task_id: string, message_id: string }` for reverting a specific message in the session.

#### Scenario: Revert message
- **WHEN** `POST /bridge/revert` is called for an OpenCode task
- **THEN** the Bridge delegates to `POST /session/{id}/revert` and returns 200

### Requirement: Unrevert route
The Bridge SHALL expose `POST /bridge/unrevert` accepting `{ task_id: string }` for undoing all reverts in the session.

#### Scenario: Unrevert all messages
- **WHEN** `POST /bridge/unrevert` is called for an OpenCode task
- **THEN** the Bridge delegates to `POST /session/{id}/unrevert` and returns 200

### Requirement: Diff route
The Bridge SHALL expose `GET /bridge/diff/:task_id` returning the file diffs for the task's session.

#### Scenario: Diff retrieval
- **WHEN** `GET /bridge/diff/task-1` is called for an OpenCode task
- **THEN** the Bridge returns 200 with the file diff array from OpenCode

### Requirement: Messages route
The Bridge SHALL expose `GET /bridge/messages/:task_id` returning the message history for the task's session.

#### Scenario: Message history retrieval
- **WHEN** `GET /bridge/messages/task-1` is called
- **THEN** the Bridge returns 200 with the full message list from the runtime

### Requirement: Command route
The Bridge SHALL expose `POST /bridge/command` accepting `{ task_id: string, command: string, arguments?: string }` for executing slash commands in the session.

#### Scenario: Slash command execution
- **WHEN** `POST /bridge/command` is called with `{ task_id: "task-1", command: "/compact" }`
- **THEN** the Bridge delegates to the runtime's command mechanism and returns 200

### Requirement: Interrupt route
The Bridge SHALL expose `POST /bridge/interrupt` accepting `{ task_id: string }` for gracefully interrupting a running query.

#### Scenario: Interrupt running task
- **WHEN** `POST /bridge/interrupt` is called for an active Claude Code task
- **THEN** the Bridge calls `query.interrupt()` and returns 200

### Requirement: Model switch route
The Bridge SHALL expose `POST /bridge/model` accepting `{ task_id: string, model: string }` for changing the model mid-session.

#### Scenario: Switch model
- **WHEN** `POST /bridge/model` is called with `{ task_id: "task-1", model: "haiku" }`
- **THEN** the Bridge calls the runtime-specific model switch method and returns 200

### Requirement: Permission response route
The Bridge SHALL expose `POST /bridge/permission-response/:request_id` accepting `{ decision: "allow" | "deny", reason?: string }` for responding to permission requests emitted by the Bridge.

#### Scenario: Permission response received
- **WHEN** the Bridge has a pending permission request with `request_id: "req-1"` and `POST /bridge/permission-response/req-1` is called with `{ decision: "allow" }`
- **THEN** the Bridge resolves the pending callback and the runtime receives the allow decision
