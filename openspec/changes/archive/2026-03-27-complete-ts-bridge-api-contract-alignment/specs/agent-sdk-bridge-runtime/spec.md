## MODIFIED Requirements

### Requirement: Bridge and Go share one canonical execution contract
The bridge runtime SHALL expose one canonical HTTP contract under the `/bridge/*` route family for execute, status, cancel, pause, resume, health, and runtime catalog operations, and the Go bridge client MUST use that same canonical contract without route or field-name drift. Compatibility aliases MAY remain available for legacy callers, but they MUST stay behaviorally identical and SHALL NOT replace `/bridge/*` as the primary contract used by Go, tests, or live documentation. The canonical execute contract MUST support explicit runtime selection while preserving defined backward-compatibility behavior for callers that do not yet send `runtime`.

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
