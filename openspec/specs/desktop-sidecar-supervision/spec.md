# desktop-sidecar-supervision Specification

## Purpose
Define how the AgentForge desktop shell supervises the Go orchestrator and TS bridge sidecars, including startup ordering, readiness, failure recovery, and degraded-state reporting.
## Requirements
### Requirement: Desktop shell starts the required sidecar topology
The Tauri desktop shell SHALL manage the Go orchestrator sidecar, the TS bridge sidecar, and the IM Bridge sidecar for local desktop mode. It MUST track each runtime separately, MUST expose a combined desktop runtime state, and MUST NOT report the desktop runtime as ready until all required sidecars have reached their ready condition.

#### Scenario: Desktop startup succeeds with all required sidecars
- **WHEN** the desktop app starts and the backend, TS bridge, and IM Bridge sidecar binaries are available
- **THEN** Tauri launches the Go orchestrator first
- **AND** after the Go endpoint is known it launches the TS bridge and the IM Bridge using the desktop-supported local configuration
- **AND** it reports backend, bridge, IM Bridge, and overall desktop runtime state as ready once all required sidecars are healthy

#### Scenario: A required sidecar cannot start
- **WHEN** one required sidecar binary is missing or fails to start
- **THEN** Tauri marks the failing runtime and the overall desktop runtime as degraded
- **AND** it preserves an error summary for frontend consumption instead of silently treating the missing IM Bridge as optional

### Requirement: Desktop shell supervises sidecar failures with bounded recovery
The Tauri desktop shell SHALL detect unexpected sidecar termination, record restart attempts, and perform bounded recovery before declaring the runtime degraded. It MUST surface the latest restart count and last error for each managed runtime, including IM Bridge.

#### Scenario: A ready sidecar exits unexpectedly
- **WHEN** the backend, bridge, or IM Bridge sidecar exits after having reached ready state
- **THEN** Tauri emits a runtime event, increments that runtime's restart count, and attempts recovery according to the configured policy

#### Scenario: Recovery attempts are exhausted
- **WHEN** recovery attempts for a sidecar exceed the allowed threshold
- **THEN** Tauri stops automatic restart for that runtime, marks the overall desktop runtime as degraded, and exposes a stable failure snapshot instead of oscillating between ready and starting

