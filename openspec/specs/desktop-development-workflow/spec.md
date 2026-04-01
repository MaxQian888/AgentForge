# desktop-development-workflow Specification

## Purpose
TBD - created by archiving change complete-im-bridge-startup-debug-and-sidecar-support. Update Purpose after archive.
## Requirements
### Requirement: Repository exposes complete desktop development and packaging commands
The repository SHALL provide supported root-level desktop development and packaging commands that prepare every required AgentForge desktop dependency instead of relying on manual sidecar build steps. The supported desktop workflow MUST cover the frontend surface plus the backend, TS bridge, and IM Bridge sidecars, and it MUST distinguish current-host development builds from packaging builds for supported desktop targets.

#### Scenario: Desktop development command prepares current-host sidecars
- **WHEN** a developer runs the supported desktop development command for the local machine
- **THEN** the repository builds or refreshes the current-host backend, TS bridge, and IM Bridge sidecar binaries before Tauri dev mode starts
- **AND** the developer does not need to manually compile the IM Bridge in a separate shell

#### Scenario: Desktop packaging command includes IM Bridge in the bundle inputs
- **WHEN** a developer runs the supported desktop packaging command
- **THEN** the repository prepares the backend, TS bridge, and IM Bridge sidecar binaries required by the desktop bundle
- **AND** the resulting Tauri bundle input set includes the IM Bridge sidecar instead of packaging only the backend and TS bridge

#### Scenario: Desktop prepare step reports a sidecar build failure
- **WHEN** one required desktop sidecar cannot be built for the requested workflow
- **THEN** the supported desktop command exits non-zero
- **AND** the failure identifies which sidecar preparation step failed

### Requirement: Desktop debug entry points reuse the same repository-supported prepare contract
The repository SHALL keep CLI, Tauri config, and IDE desktop debug entry points aligned to the same prepare contract. Any supported desktop debug entry point, including `tauri:dev`, Tauri pre-commands, and maintained VS Code desktop debug tasks, MUST reuse the same command family for sidecar preparation so IM Bridge coverage does not drift between tools.

#### Scenario: VS Code desktop debug uses the shared prepare command
- **WHEN** a developer launches the maintained VS Code desktop debug configuration
- **THEN** the prelaunch task runs the same repository-supported desktop prepare command family used by the CLI desktop workflow
- **AND** the debug session does not rely on a separate IDE-only list of sidecar build steps

#### Scenario: Tauri configuration remains aligned with repository scripts
- **WHEN** Tauri dev or build mode invokes its configured pre-command hooks
- **THEN** those hooks resolve to the same repository-supported desktop prepare commands documented at the root level
- **AND** IM Bridge availability does not depend on an undocumented extra command

