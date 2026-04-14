## ADDED Requirements

### Requirement: Repository exposes a verify command for the managed backend stack
The repository SHALL provide a supported root-level `dev:backend:verify` command for the same managed backend stack covered by `dev:backend`, `dev:backend:status`, `dev:backend:logs`, and `dev:backend:stop`. The verify command MUST reuse the existing backend service definitions, runtime state semantics, and repo-local log locations instead of maintaining a parallel launcher or separate state model.

#### Scenario: Verify command starts missing backend services through the managed workflow
- **WHEN** a developer runs `dev:backend:verify` and one or more required backend services are not already healthy
- **THEN** the workflow starts or prepares those services using the same supported backend profile as `dev:backend`
- **THEN** the resulting runtime metadata and log paths remain discoverable through the existing backend status and logs commands

#### Scenario: Verify command runs against an already managed backend stack
- **WHEN** a developer runs `dev:backend:verify` after a prior `dev:backend` startup already produced a healthy managed stack
- **THEN** the workflow reuses that stack instead of rebuilding a parallel runtime graph
- **THEN** it preserves the same repo-local state and log surfaces that `dev:backend:status` and `dev:backend:logs` report

### Requirement: Backend verify preserves the managed debugging surface by default
The supported backend verify command SHALL preserve the running managed backend stack by default after verification so developers can continue live debugging. If cleanup is needed, the workflow MUST direct developers to the supported backend stop path rather than silently tearing down reused or managed services.

#### Scenario: Verify succeeds after starting managed services
- **WHEN** `dev:backend:verify` starts one or more managed backend services and all smoke stages succeed
- **THEN** the managed services remain available after the command exits by default
- **THEN** the output tells the developer how to inspect status, logs, or stop the stack explicitly

#### Scenario: Verify fails after partial startup
- **WHEN** `dev:backend:verify` fails after starting or reusing backend services
- **THEN** the workflow reports the current managed runtime state instead of silently removing it
- **THEN** it directs the developer to the supported status, logs, or stop commands for follow-up action
