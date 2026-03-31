## MODIFIED Requirements

### Requirement: Bridge and Go share one canonical execution contract
The bridge runtime SHALL expose one canonical HTTP contract under the `/bridge/*` route family for execute, status, cancel, pause, resume, health, and runtime catalog operations, and the Go bridge client MUST use that same canonical contract without route or field-name drift. Compatibility aliases MAY remain available for legacy callers, but they MUST stay behaviorally identical and SHALL NOT replace `/bridge/*` as the primary contract used by Go, tests, or live documentation. The canonical execute contract MUST support explicit runtime selection for every runtime key published in the runtime catalog while preserving defined backward-compatibility behavior for callers that do not yet send `runtime`.

#### Scenario: Go queries runtime status through the canonical route
- **WHEN** the Go bridge client requests the status of an active task
- **THEN** the request SHALL target `/bridge/status/:id`
- **THEN** the bridge SHALL return the current runtime state, turn count, last tool, last activity timestamp, and spend for that task

#### Scenario: Invalid execute payload is rejected consistently
- **WHEN** the Go bridge client sends an execute request to `/bridge/execute` that does not satisfy the bridge schema, including runtime selection rules
- **THEN** the bridge SHALL return a validation error that identifies the payload issue
- **THEN** the bridge SHALL not start agent execution for that task

#### Scenario: Compatibility alias remains equivalent but secondary
- **WHEN** a legacy caller uses a supported alias for pause, resume, health, or another execution-adjacent bridge route
- **THEN** the alias SHALL invoke the same handler and validation behavior as the canonical `/bridge/*` route
- **THEN** project documentation and new callers SHALL still treat the `/bridge/*` route as the primary contract

#### Scenario: Execute request targets a newly supported CLI-backed runtime
- **WHEN** the Go orchestrator sends a valid execute request with `runtime=gemini`
- **THEN** the bridge SHALL accept that runtime key through the same canonical `/bridge/execute` contract used by the existing runtimes
- **THEN** downstream execution and status surfaces SHALL preserve `gemini` as the resolved runtime identity instead of remapping it to a legacy runtime

### Requirement: Go-managed execution sends explicit runtime-compatible launch tuples
The Go-managed execution path SHALL resolve and send an explicit `runtime`, `provider`, and `model` tuple for every agent run before calling the bridge, and the bridge MUST reject tuples that are incompatible or ambiguous before acquiring execution state. For additional CLI-backed runtimes, that validation MUST follow the backend profile advertised in the runtime catalog rather than assuming that all runtimes behave like Claude Code, Codex, or OpenCode.

#### Scenario: Compatible backend tuple is accepted
- **WHEN** the Go orchestrator submits an execute request with `runtime=iflow` and a provider or model combination supported for that runtime profile
- **THEN** the bridge SHALL accept the request without remapping it to another runtime
- **THEN** the started run SHALL retain the same resolved `runtime`, `provider`, and `model` identity

#### Scenario: Incompatible runtime and provider are rejected
- **WHEN** the Go orchestrator submits an execute request whose explicit `runtime` and `provider` do not form a supported coding-agent combination for the selected backend profile
- **THEN** the bridge SHALL reject the request before creating a runtime entry
- **THEN** the error SHALL identify that the runtime or provider tuple is incompatible instead of silently guessing a different runtime

#### Scenario: Unsupported model override is rejected before launch
- **WHEN** the Go orchestrator submits an execute request whose explicit `model` does not satisfy the selected runtime profile's bounded model contract
- **THEN** the bridge SHALL reject the request before execution starts
- **THEN** the error SHALL identify the unsupported model constraint instead of silently falling back to a different model
