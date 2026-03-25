## ADDED Requirements

### Requirement: Go-managed execution sends explicit runtime-compatible launch tuples
The Go-managed execution path SHALL resolve and send an explicit `runtime`, `provider`, and `model` tuple for every agent run before calling the bridge, and the bridge MUST reject tuples that are incompatible or ambiguous before acquiring execution state.

#### Scenario: Compatible Codex tuple is accepted
- **WHEN** the Go orchestrator submits an execute request with `runtime=codex` and a provider/model combination supported for that runtime
- **THEN** the bridge SHALL accept the request without remapping it to another runtime
- **THEN** the started run SHALL retain the same resolved `runtime`, `provider`, and `model` identity

#### Scenario: Incompatible runtime and provider are rejected
- **WHEN** the Go orchestrator submits an execute request whose explicit `runtime` and `provider` do not form a supported coding-agent combination
- **THEN** the bridge SHALL reject the request before creating a runtime entry
- **THEN** the error SHALL identify that the runtime/provider tuple is incompatible instead of silently guessing a different runtime

### Requirement: Resolved runtime identity remains visible through bridge status metadata
The bridge SHALL expose the resolved `runtime`, `provider`, and `model` in the status metadata it returns to Go for active or paused runs so that backend persistence and operator-facing summaries use the truthful execution identity.

#### Scenario: Status query returns runtime identity for a Codex run
- **WHEN** the Go bridge client requests the status of an active run executing through `codex`
- **THEN** the status payload SHALL include the resolved `runtime`, `provider`, and `model`
- **THEN** the backend SHALL be able to persist or render that identity without re-inferring it from other fields

#### Scenario: Paused run keeps the same resolved runtime identity
- **WHEN** the bridge reports status for a paused or resumable run
- **THEN** the status metadata SHALL continue to return the same resolved `runtime`, `provider`, and `model` that were used to start the run
- **THEN** resume or summary flows SHALL not need to guess the runtime identity from legacy fallback rules

### Requirement: Provider-only runtime inference is compatibility-only for non-Go callers
The system SHALL treat provider-only runtime inference as a compatibility path for legacy direct callers, and the Go-managed execution path MUST NOT depend on provider inference to determine runtime selection.

#### Scenario: Backend resolves project defaults before calling the bridge
- **WHEN** an operator starts an agent run without explicit runtime overrides and the project defaults resolve to a supported coding-agent selection
- **THEN** the Go backend SHALL send that resolved `runtime` explicitly to the bridge
- **THEN** the bridge execution path for that run SHALL not depend on provider-only runtime inference

#### Scenario: Legacy direct call still resolves a provider hint
- **WHEN** a non-Go compatibility caller submits a direct bridge execute request without `runtime` but with a recognized legacy provider hint
- **THEN** the bridge MAY resolve the runtime through the compatibility inference rule
- **THEN** that compatibility path SHALL NOT change the requirement that Go-managed calls send explicit runtime metadata
