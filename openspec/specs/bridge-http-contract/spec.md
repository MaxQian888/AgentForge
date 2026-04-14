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

### Requirement: Backend service callers SHALL reach TS Bridge only through Go-owned surfaces
All non-frontend backend service callers, including IM Bridge and operator-facing automation surfaces, SHALL access TS Bridge runtime inspection and lightweight AI capabilities only through Go backend-owned HTTP surfaces. They MUST NOT call TS Bridge service endpoints directly from IM Bridge or other backend helpers.

#### Scenario: IM Bridge queries runtime catalog through Go proxy
- **WHEN** an IM Bridge command needs the runtime catalog
- **THEN** it calls the Go backend `GET /api/v1/bridge/runtimes` endpoint
- **THEN** the Go backend proxies that request to the canonical TS Bridge `/bridge/runtimes` route
- **THEN** the returned payload preserves the upstream runtime catalog semantics without inventing a parallel contract

#### Scenario: IM Bridge requests AI classification through Go proxy
- **WHEN** an IM Bridge command or natural-language routing flow needs intent classification
- **THEN** it calls the Go backend `POST /api/v1/ai/classify-intent` endpoint
- **THEN** the Go backend forwards the request to the canonical TS Bridge `/bridge/classify-intent` route
- **THEN** the IM Bridge does not bypass the backend to call TS Bridge directly

### Requirement: Go proxy endpoints SHALL expose upstream connectivity failures truthfully
When a Go backend proxy endpoint for TS Bridge capabilities cannot reach its upstream or receives an upstream validation/runtime failure, the endpoint SHALL preserve the failure source and SHALL NOT report the issue as a successful local backend response.

#### Scenario: Runtime catalog request fails because Bridge is unavailable
- **WHEN** a caller invokes `GET /api/v1/bridge/runtimes` while the Go backend cannot reach TS Bridge
- **THEN** the response reports that the Bridge upstream is unavailable
- **THEN** operator or IM callers can distinguish that failure from an empty runtime catalog

#### Scenario: AI proxy request fails due to upstream validation
- **WHEN** a caller invokes `POST /api/v1/ai/decompose` with a payload rejected by TS Bridge validation
- **THEN** the Go backend returns the upstream validation failure as a rejected proxy request
- **THEN** the response does not claim that decomposition ran successfully in the backend

### Requirement: Runtime interaction routes return capability-aware responses
The Bridge SHALL keep runtime interaction routes canonical and capability-aware. For advanced runtime controls such as messages, command execution, model changes, shell execution, and permission callbacks, the route response MUST align with the runtime catalog support state and MUST identify unsupported or degraded operations explicitly.

#### Scenario: Canonical interaction route succeeds for a supported runtime
- **WHEN** a caller invokes a canonical Bridge interaction route whose selected runtime publishes the operation as supported
- **THEN** the Bridge SHALL delegate to the runtime-specific control path and return the runtime's normalized result
- **THEN** the route response SHALL remain consistent with the support state advertised in `/bridge/runtimes`

#### Scenario: Canonical interaction route is unsupported for the selected runtime
- **WHEN** a caller invokes a canonical Bridge interaction route whose selected runtime publishes the operation as unsupported or degraded
- **THEN** the Bridge SHALL return a structured error that includes the runtime key, requested operation, support state, and reason code
- **THEN** the response SHALL NOT collapse that outcome into an unqualified generic 500 error

### Requirement: Bridge exposes a canonical shell control route
The Bridge SHALL expose `POST /bridge/shell` for runtimes that publish shell execution support. The request MUST include the task identity plus the shell command payload, and runtimes that do not publish shell execution support MUST return an explicit unsupported response.

#### Scenario: OpenCode-backed task uses the canonical shell route
- **WHEN** a caller posts a shell command to `/bridge/shell` for an active OpenCode-backed task
- **THEN** the Bridge SHALL resolve the task runtime, verify shell support from the runtime contract, and proxy the request through the OpenCode session shell API
- **THEN** the normalized response SHALL identify the originating task and runtime session

### Requirement: OpenCode session-backed interaction routes resolve through persisted continuity after pause
The Bridge SHALL resolve OpenCode canonical interaction routes through either the active runtime or the persisted OpenCode continuity snapshot for the same task. For OpenCode-backed tasks, `messages`, `diff`, `command`, `shell`, `revert`, and `unrevert` MUST remain task-oriented canonical routes even after the task has been paused and released from the active pool.

#### Scenario: Read-only control works after the OpenCode task is paused
- **WHEN** `GET /bridge/messages/{task_id}` or `GET /bridge/diff/{task_id}` is called for a paused OpenCode task whose snapshot still contains an upstream session binding
- **THEN** the Bridge resolves the task through persisted continuity rather than active pool lookup alone
- **THEN** it returns the server-backed OpenCode result for the same upstream session

#### Scenario: Mutating control works after the OpenCode task is paused
- **WHEN** `POST /bridge/command`, `POST /bridge/shell`, or `POST /bridge/revert` targets a paused OpenCode task with valid persisted continuity
- **THEN** the Bridge forwards the request to the bound upstream OpenCode session without forcing a resume or spawning a fresh runtime
- **THEN** the route response remains consistent with the support state published for OpenCode in `/bridge/runtimes`

#### Scenario: Persisted continuity is missing for a paused OpenCode control request
- **WHEN** a caller invokes one of those canonical OpenCode interaction routes for a paused task whose snapshot no longer contains a valid upstream session binding
- **THEN** the Bridge returns an explicit continuity or non-resumable control error
- **THEN** it SHALL NOT degrade that outcome into an ambiguous `task not found` response

### Requirement: Bridge exposes canonical OpenCode provider-auth routes
The Bridge SHALL expose canonical provider-auth routes for OpenCode under the `/bridge/*` family so callers can initiate and complete provider authentication without invoking OpenCode server endpoints directly.

#### Scenario: Start OpenCode provider auth through the Bridge
- **WHEN** a caller posts to `/bridge/opencode/provider-auth/{provider}/start`
- **THEN** the Bridge returns a request-scoped auth initiation payload derived from the upstream OpenCode provider authorize surface
- **THEN** the caller does not need to call the upstream `/provider/{id}/oauth/authorize` endpoint directly

#### Scenario: Complete OpenCode provider auth through the Bridge
- **WHEN** a caller posts the callback payload to `/bridge/opencode/provider-auth/{request_id}/complete`
- **THEN** the Bridge forwards that payload to the matching upstream provider callback surface
- **THEN** the response truthfully reports success or upstream auth failure without pretending the provider is ready when callback completion failed

