# desktop-development-workflow Specification

## Purpose
Define the supported AgentForge desktop development, standalone Rust debugging, and packaging workflows so CLI commands, Tauri pre-commands, and maintained IDE entrypoints stay aligned around sidecar preparation and frontend ownership.

## Requirements
### Requirement: Repository exposes complete desktop development, standalone Rust debugging, and packaging commands
The repository SHALL provide supported root-level desktop development, standalone Rust debugging, and packaging commands that prepare every required AgentForge desktop dependency instead of relying on manual sidecar build steps. The supported desktop workflow MUST cover the frontend surface plus the backend, TS bridge, and IM Bridge sidecars, MUST distinguish current-host development builds from packaging builds for supported desktop targets, and MUST define a standalone Rust debug entrypoint that reuses the same current-host sidecar contract while depending on an already-available frontend surface.

#### Scenario: Desktop development command prepares current-host sidecars
- **WHEN** a developer runs the supported desktop development command for the local machine
- **THEN** the repository builds or refreshes the current-host backend, TS bridge, and IM Bridge sidecar binaries before Tauri dev mode starts
- **AND** the developer does not need to manually compile the IM Bridge in a separate shell

#### Scenario: Standalone Rust debug command reuses current-host sidecars
- **WHEN** a developer runs the supported standalone Rust debug command while the configured frontend surface is already reachable
- **THEN** the repository reuses or prepares the current-host backend, TS bridge, and IM Bridge sidecar binaries required by the Rust runtime
- **AND** it launches the Rust desktop runtime without taking ownership of frontend startup

#### Scenario: Desktop packaging command includes IM Bridge in the bundle inputs
- **WHEN** a developer runs the supported desktop packaging command
- **THEN** the repository prepares the backend, TS bridge, and IM Bridge sidecar binaries required by the desktop bundle
- **AND** the resulting Tauri bundle input set includes the IM Bridge sidecar instead of packaging only the backend and TS bridge

#### Scenario: Desktop prepare step reports a sidecar build failure
- **WHEN** one required desktop sidecar cannot be built or validated for the requested workflow
- **THEN** the supported desktop command exits non-zero
- **AND** the failure identifies which sidecar preparation or frontend prerequisite step failed

### Requirement: Desktop debug entry points reuse the same repository-supported prepare contract
The repository SHALL keep CLI, Tauri config, IDE desktop debug entry points, and the standalone Rust debug entrypoint aligned to the same prepare contract. Any supported full desktop debug entry point, including `tauri:dev`, Tauri pre-commands, and maintained VS Code desktop debug tasks, MUST reuse the same command family for sidecar preparation so IM Bridge coverage does not drift between tools. The supported standalone Rust debug entrypoint MUST reuse that same current-host sidecar preparation family, MUST keep frontend startup as an external prerequisite, and MUST surface actionable diagnostics when that prerequisite is not satisfied.

#### Scenario: VS Code full desktop debug uses the shared prepare command
- **WHEN** a developer launches the maintained VS Code full desktop debug configuration
- **THEN** the prelaunch task runs the same repository-supported desktop prepare command family used by the CLI desktop workflow
- **AND** the debug session does not rely on a separate IDE-only list of sidecar build steps

#### Scenario: Tauri configuration remains aligned with repository scripts
- **WHEN** Tauri dev or build mode invokes its configured pre-command hooks
- **THEN** those hooks resolve to the same repository-supported desktop prepare commands documented at the root level
- **AND** IM Bridge availability does not depend on an undocumented extra command

#### Scenario: Standalone Rust debug reports missing frontend prerequisite
- **WHEN** a developer launches the maintained standalone Rust debug entrypoint and the configured frontend surface is unavailable
- **THEN** the entrypoint exits non-zero before Rust runtime ownership begins
- **AND** the diagnostic output identifies the missing frontend prerequisite and the supported command or asset contract required to satisfy it
