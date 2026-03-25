## ADDED Requirements

### Requirement: Repository exposes a unified full-stack local development startup command
The repository SHALL provide a supported root-level full-stack development startup command that prepares the AgentForge local web development surface in one flow. The command MUST cover the repo-truthful frontend dev server, Go Orchestrator, TS Bridge, and the local PostgreSQL/Redis infrastructure needed by the default development path, and MUST print the resulting service endpoints when startup succeeds.

#### Scenario: Full stack starts from a clean local environment
- **WHEN** a developer runs the supported full-stack startup command on a machine where the required toolchain and Docker runtime are available
- **THEN** the repository starts or prepares PostgreSQL, Redis, the Go Orchestrator, the TS Bridge, and the frontend dev server in a valid order and prints the reachable endpoints for each service

#### Scenario: Startup reuses an already healthy service
- **WHEN** the supported full-stack startup command detects that one or more required services are already healthy on their expected local endpoints
- **THEN** the command reuses those services instead of starting duplicates and reports which services were reused

### Requirement: Full-stack development startup is health-aware and idempotent
The supported startup workflow SHALL validate service readiness before reporting success. The workflow MUST use repo-supported health probes or equivalent readiness checks for each service, MUST fail fast when a required dependency cannot be started or verified, and MUST distinguish missing prerequisites from unhealthy subprocesses or unknown external listeners.

#### Scenario: Startup waits for service readiness
- **WHEN** the startup workflow launches a managed frontend, Go, or TS Bridge process
- **THEN** the command waits for the configured readiness checks to pass before marking that service ready

#### Scenario: Startup reports a missing prerequisite
- **WHEN** a required local dependency such as Docker, Go, Bun, Node.js, or pnpm is unavailable for a service that cannot be reused
- **THEN** the startup command exits non-zero and identifies the missing prerequisite and the affected service

#### Scenario: Startup reports an unknown listener conflict
- **WHEN** a required port is already occupied but the listener does not satisfy the expected health probe for the corresponding AgentForge service
- **THEN** the startup command exits non-zero and reports the port conflict as an external or unknown listener instead of assuming the service is healthy

### Requirement: Repository exposes status and stop commands for the managed local stack
The repository SHALL provide supported root-level status and stop commands for the same full-stack local development surface. The status command MUST report the current source, health, pid metadata when available, and log locations for each known service. The stop command MUST terminate only services previously marked as managed by the workflow and MUST leave reused or external services untouched.

#### Scenario: Status reports managed and reused services distinctly
- **WHEN** a developer runs the supported status command after a previous startup flow
- **THEN** the command reports each known service with its current health, whether it is managed or reused, and any available pid, endpoint, and log metadata

#### Scenario: Stop terminates only managed services
- **WHEN** a developer runs the supported stop command after the startup workflow managed some services and reused others
- **THEN** the command stops only the managed services, preserves the reused services, and reports which services were stopped versus preserved

### Requirement: Repository persists runtime metadata and diagnostics for the local stack
The supported local development workflow SHALL persist repo-local runtime metadata and diagnostic outputs needed by follow-up status and troubleshooting commands. The workflow MUST store per-service log locations and enough state to reconcile stale processes, and MUST direct developers to the relevant log or status surface when startup or health verification fails.

#### Scenario: Managed startup records runtime metadata and logs
- **WHEN** the supported startup workflow launches or reuses services for the local stack
- **THEN** it records repo-local runtime metadata for those services and ensures each managed service has a discoverable log location

#### Scenario: Status reconciles stale runtime state
- **WHEN** the stored runtime metadata references a process id or service state that no longer matches the current machine state
- **THEN** the status workflow detects the stale state, updates the reported status accordingly, and avoids treating the stale record as a healthy managed service

#### Scenario: Failure output points to diagnostics
- **WHEN** startup or readiness verification fails for a managed service
- **THEN** the workflow exits non-zero and reports the failing service, the reason category, and the log or status location a developer should inspect next
