## ADDED Requirements

### Requirement: Go-managed runtime flows SHALL preserve canonical execution context across hops
For every backend-managed coding-agent runtime flow, the Go backend and TS Bridge SHALL preserve the canonical execution context needed by downstream status, resume, diagnostics, and IM observability surfaces. That context MUST include the resolved runtime identity and any required launch tuple fields such as provider, model, team execution context when present, and runtime diagnostics metadata rather than reconstructing them later from unrelated persisted records.

#### Scenario: Execute request preserves runtime identity for an external runtime
- **WHEN** the Go backend starts a valid runtime request with `runtime=codex` or another supported external coding-agent runtime
- **THEN** the request sent to TS Bridge preserves that explicit runtime identity together with the resolved launch tuple fields needed by the runtime
- **THEN** subsequent status or diagnostics responses keep the same runtime identity instead of remapping it to another runtime family

#### Scenario: Resume request rejects context drift
- **WHEN** the Go backend attempts to resume a paused runtime with continuity metadata that conflicts with the stored runtime identity or required launch context
- **THEN** TS Bridge rejects the resume request explicitly
- **THEN** the backend does not report the runtime as resumed successfully

### Requirement: Bridge diagnostics SHALL remain consumable by backend and IM observability flows
TS Bridge runtime status and diagnostics surfaces SHALL expose enough normalized metadata for Go backend, IM Bridge, and operator-facing backend flows to explain runtime readiness and execution state without runtime-specific guesswork.

#### Scenario: Runtime readiness failure is visible to IM-facing diagnostics
- **WHEN** an IM-facing command requests runtime diagnostics for a runtime whose executable or authentication precondition is missing
- **THEN** the TS Bridge diagnostics surfaced through the backend identify the affected runtime and the readiness failure reason
- **THEN** the IM-facing response can explain why the runtime is unavailable without inventing synthetic status values

#### Scenario: Status metadata stays stable after execution starts
- **WHEN** the backend queries runtime status for an active coding-agent run
- **THEN** the returned status metadata includes the resolved runtime identity and execution diagnostics needed by backend summaries
- **THEN** those fields remain stable across repeated status checks unless the upstream runtime state actually changes
