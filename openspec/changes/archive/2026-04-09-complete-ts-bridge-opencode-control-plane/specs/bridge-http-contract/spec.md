## ADDED Requirements

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
