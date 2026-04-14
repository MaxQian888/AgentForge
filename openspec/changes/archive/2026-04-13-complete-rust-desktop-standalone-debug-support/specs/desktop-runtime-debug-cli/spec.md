## ADDED Requirements

### Requirement: Rust desktop debug CLI performs explicit preflight checks
The Rust desktop debug CLI SHALL validate the configured frontend surface, the required current-host sidecar binaries, and any required runtime ports before attempting to launch the standalone Rust desktop runtime. It MUST report failing prerequisites with actionable guidance instead of deferring them to opaque Tauri startup failures.

#### Scenario: CLI preflight finds a missing frontend surface
- **WHEN** a developer runs the supported CLI preflight command and the configured frontend URL or asset surface is unavailable
- **THEN** the CLI exits non-zero
- **AND** the output identifies the missing frontend prerequisite and the supported way to satisfy it

#### Scenario: CLI preflight finds missing sidecars or conflicting listeners
- **WHEN** a developer runs the supported CLI preflight command and a required current-host sidecar binary is missing or a required runtime port is occupied by an unexpected listener
- **THEN** the CLI reports the failing runtime label and reason
- **AND** it points the developer to the supported prepare command or conflict to resolve before launch

### Requirement: Rust desktop debug CLI launches the shared runtime in foreground
The Rust desktop debug CLI SHALL launch the AgentForge desktop runtime through the shared desktop runtime host in a foreground, debug-friendly mode. It MUST surface startup progress plus ready, degraded, and exit outcomes through stdout or stderr, and MUST preserve the same backend, bridge, and IM Bridge supervision semantics as the normal desktop GUI entrypoint.

#### Scenario: Foreground CLI run succeeds against a healthy frontend
- **WHEN** a developer runs the supported CLI launch command after prerequisites pass
- **THEN** the CLI starts the Rust desktop runtime without re-running frontend startup ownership
- **AND** terminal output surfaces runtime startup progress until the desktop runtime becomes ready or degraded

#### Scenario: Foreground CLI run exits or is interrupted
- **WHEN** the standalone CLI launch is stopped by the developer or the desktop runtime exits unexpectedly
- **THEN** the CLI terminates with a truthful exit code
- **AND** it preserves the shutdown or failure context in terminal output instead of silently swallowing the reason
