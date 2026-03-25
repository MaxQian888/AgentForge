# desktop-sidecar-supervision Specification

## Purpose
Define how the AgentForge desktop shell supervises the Go orchestrator and TS bridge sidecars, including startup ordering, readiness, failure recovery, and degraded-state reporting.
## Requirements
### Requirement: Desktop shell starts the required sidecar topology
The Tauri desktop shell SHALL manage both the Go orchestrator sidecar and the TS bridge sidecar for local desktop mode. It MUST track each runtime separately, MUST expose a combined desktop runtime state, and MUST NOT report the desktop runtime as ready until both required sidecars have reached their ready condition.

#### Scenario: Desktop startup succeeds with both sidecars
- **WHEN** the desktop app starts and both sidecar binaries are available
- **THEN** Tauri launches the Go orchestrator first, launches the TS bridge after the Go endpoint is known, and reports backend, bridge, and overall desktop runtime state as ready

#### Scenario: A required sidecar cannot start
- **WHEN** one required sidecar binary is missing or fails to start
- **THEN** Tauri marks the failing runtime and the overall desktop runtime as degraded and preserves an error summary for frontend consumption

### Requirement: Desktop shell supervises sidecar failures with bounded recovery
The Tauri desktop shell SHALL detect unexpected sidecar termination, record restart attempts, and perform bounded recovery before declaring the runtime degraded. It MUST surface the latest restart count and last error for each managed runtime.

#### Scenario: A ready sidecar exits unexpectedly
- **WHEN** the backend or bridge sidecar exits after having reached ready state
- **THEN** Tauri emits a runtime event, increments that runtime's restart count, and attempts recovery according to the configured policy

#### Scenario: Recovery attempts are exhausted
- **WHEN** recovery attempts for a sidecar exceed the allowed threshold
- **THEN** Tauri stops automatic restart for that runtime, marks the overall desktop runtime as degraded, and exposes a stable failure snapshot instead of oscillating between ready and starting
