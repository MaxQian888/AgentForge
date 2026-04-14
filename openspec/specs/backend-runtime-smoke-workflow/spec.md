# backend-runtime-smoke-workflow Specification

## Purpose
TBD - created by archiving change complete-backend-runtime-smoke-workflow. Update Purpose after archive.
## Requirements
### Requirement: Repository exposes a root-level backend runtime smoke command
The repository SHALL provide a supported root-level backend runtime smoke command that verifies the backend-only local stack without requiring the frontend or live third-party IM credentials. The command MUST run against the repo-truthful Go Orchestrator, TS Bridge, IM Bridge, and local infra surfaces used by `dev:backend`.

#### Scenario: Smoke command succeeds from a clean local environment
- **WHEN** a developer runs the supported backend smoke command on a machine where required local prerequisites are available and no healthy backend stack is already running
- **THEN** the workflow starts or prepares the backend-only local stack in the supported order
- **THEN** it verifies the required smoke stages before reporting success

#### Scenario: Smoke command reuses a healthy backend stack
- **WHEN** a developer runs the supported backend smoke command while the managed or reused backend stack is already healthy
- **THEN** the workflow reuses the existing healthy services instead of launching duplicates
- **THEN** it still executes the smoke verification stages and reports which services were reused

### Requirement: Smoke workflow proves a zero-credential IM stub command roundtrip
The backend smoke workflow SHALL prove at least one canonical IM-originated Bridge-backed command flow in stub mode without external provider credentials. At minimum, the workflow MUST inject one repo-supported stub message into the managed IM Bridge, route that command through Go-owned backend surfaces to the TS Bridge capability it depends on, and assert that a non-empty reply is captured through the stub reply surface.

#### Scenario: Canonical IM to Go to TS Bridge flow succeeds
- **WHEN** the smoke workflow runs against a healthy backend stack with the managed IM Bridge in stub mode
- **THEN** it injects a repo-supported Bridge-backed command using the existing stub test endpoints and fixtures
- **THEN** it captures a non-empty reply proving the IM Bridge, Go backend, and TS Bridge hop completed successfully

#### Scenario: Stub command roundtrip cannot complete
- **WHEN** the injected stub command cannot reach the required backend capability or no reply is captured
- **THEN** the workflow fails the smoke run instead of reporting partial success
- **THEN** it names the failing stage so the broken hop is visible to the developer

### Requirement: Smoke workflow emits stage-based diagnostics
The backend smoke workflow SHALL report its result as explicit verification stages rather than a single pass/fail line. For each failed stage, the output MUST identify the failing service or hop and point to the next diagnostic surface such as the relevant endpoint, runtime state file, or repo-local log path.

#### Scenario: Health stage fails
- **WHEN** Go, TS Bridge, or IM Bridge health verification fails during the smoke workflow
- **THEN** the output identifies the exact failing health stage and service
- **THEN** it includes the relevant endpoint or log location a developer should inspect next

#### Scenario: Smoke command completes successfully
- **WHEN** all required smoke stages succeed
- **THEN** the workflow reports each completed stage and the services involved
- **THEN** it leaves the developer with the runtime metadata or log references needed for follow-up debugging without implying broader test coverage

