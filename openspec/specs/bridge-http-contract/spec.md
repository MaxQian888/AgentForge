# bridge-http-contract Specification

## Purpose
Define the canonical TypeScript Bridge HTTP and WebSocket contract so Go callers, bridge routes, compatibility aliases, and project documentation all describe the same live integration surface.
## Requirements
### Requirement: TS Bridge exposes one canonical HTTP route family
The TypeScript Bridge SHALL treat `/bridge/*` as the canonical HTTP route family for Go Orchestrator and operator-facing service-to-service calls. New implementation code, tests, examples, and project documentation MUST use the canonical `/bridge/*` routes for agent execution, lightweight AI calls, runtime inspection, and bridge control operations rather than inventing new primary route families.

#### Scenario: Go-managed execution uses the canonical execute route
- **WHEN** the Go bridge client starts an agent run
- **THEN** it SHALL target `/bridge/execute`
- **THEN** the documented live contract for that operation SHALL also identify `/bridge/execute` as the primary route

#### Scenario: Lightweight AI documentation uses canonical routes
- **WHEN** project documentation describes task decomposition, intent classification, or text generation through the TS Bridge
- **THEN** it SHALL identify `/bridge/decompose`, `/bridge/classify-intent`, and `/bridge/generate` as the canonical live routes
- **THEN** it SHALL NOT present `/api/*` or other legacy route families as the primary implementation contract

### Requirement: Compatibility aliases remain secondary and behaviorally identical
The TypeScript Bridge MAY keep compatibility aliases for legacy callers, but every alias SHALL delegate to the same handler, schema validation, and response semantics as its canonical `/bridge/*` route. Compatibility aliases MUST be documented as migration-only surfaces rather than equal primary contracts.

#### Scenario: Compatibility alias shares the same validation behavior
- **WHEN** a legacy caller sends an invalid request to a compatibility alias for a bridge operation
- **THEN** the bridge SHALL return the same validation failure shape that the canonical `/bridge/*` route would return
- **THEN** the operation SHALL NOT diverge because the request used an alias

#### Scenario: Documentation marks aliases as compatibility-only
- **WHEN** a document or inline route comment references a supported alias such as `/ai/decompose` or `/resume`
- **THEN** it SHALL describe that path as compatibility-only
- **THEN** it SHALL direct new callers to the corresponding canonical `/bridge/*` route

### Requirement: Live contract documentation distinguishes current transport from historical references
AgentForge documentation SHALL identify HTTP + WebSocket as the current live Go-to-Bridge transport contract and SHALL distinguish any retained gRPC, proto, or historical route examples as reference-only material.

#### Scenario: Historical protocol notes remain explicitly non-live
- **WHEN** the PRD or design documents retain proto or gRPC examples for historical context
- **THEN** those sections SHALL explicitly state that they are not the current live integration contract
- **THEN** the same document SHALL point readers to the canonical HTTP + WebSocket bridge routes for implementation truth

### Requirement: Go service layer calls Bridge status endpoint as execution fallback
Go agent service SHALL call `GET /bridge/status/:id` once after spawn to confirm execution started, as a fallback in case the WebSocket event was missed.

#### Scenario: Status confirms execution started
- **WHEN** agent is spawned and WS `agent.started` event was received
- **THEN** status check is skipped (WS event is authoritative)

#### Scenario: Status check recovers missed start event
- **WHEN** agent is spawned but no WS `agent.started` event arrives within 5 seconds
- **THEN** Go service calls `GET /bridge/status/:id` and updates agent state from response

### Requirement: Go backend exposes AI generation endpoints proxying to Bridge
Go backend SHALL expose `POST /api/v1/ai/generate` and `POST /api/v1/ai/classify-intent` endpoints that proxy to Bridge `/bridge/generate` and `/bridge/classify-intent` respectively.

#### Scenario: Generate text via API
- **WHEN** authenticated client calls `POST /api/v1/ai/generate` with `{"prompt": "...", "provider": "anthropic"}`
- **THEN** Go handler forwards to Bridge and returns generated text response

#### Scenario: Classify intent via API
- **WHEN** authenticated client calls `POST /api/v1/ai/classify-intent` with `{"text": "...", "candidates": [...]}`
- **THEN** Go handler forwards to Bridge and returns classification result

### Requirement: Go backend exposes runtime catalog endpoint
Go backend SHALL expose `GET /api/v1/bridge/runtimes` that proxies to Bridge `/bridge/runtimes` and returns the runtime catalog.

#### Scenario: Frontend fetches runtime catalog
- **WHEN** authenticated client calls `GET /api/v1/bridge/runtimes`
- **THEN** response contains array of runtime entries with key, display_name, default_provider, default_model, available, diagnostics

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

