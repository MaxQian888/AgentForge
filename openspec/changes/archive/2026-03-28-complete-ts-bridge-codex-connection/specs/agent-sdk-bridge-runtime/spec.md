## ADDED Requirements

### Requirement: Bridge executes Codex runs through a truthful Codex connector
The TypeScript bridge SHALL execute accepted `runtime=codex` requests through a bridge-owned Codex connector that targets a real Codex automation surface, and it MUST translate that connector's native output into the canonical AgentForge runtime lifecycle without relying on a placeholder stdin/stdout protocol implemented by operators out of band.

#### Scenario: Successful Codex execute request starts the dedicated connector
- **WHEN** the Go orchestrator submits a valid execute request with `runtime=codex` and compatible provider/model values
- **THEN** the bridge SHALL start the dedicated Codex connector for that task
- **THEN** the bridge SHALL return the session identifier for the started Codex run
- **THEN** the bridge SHALL emit canonical lifecycle events that move the task from starting to running to a terminal state based on the Codex connector result

#### Scenario: Codex connector launch prerequisites are not satisfied
- **WHEN** the Go orchestrator submits an execute request with `runtime=codex` but the dedicated Codex connector cannot be launched because its supported setup is incomplete
- **THEN** the bridge SHALL reject the request with an explicit runtime-configuration error
- **THEN** the bridge SHALL not create a misleading active runtime entry for that task

### Requirement: Codex pause and resume preserve native continuity metadata
The TypeScript bridge SHALL persist Codex-specific continuity metadata in its session snapshots, and `/bridge/resume` MUST use that stored continuity state when continuing a paused Codex run instead of silently replaying the original execute request as a fresh run.

#### Scenario: Paused Codex run stores continuity state for resume
- **WHEN** an active Codex run is paused through the canonical bridge pause flow
- **THEN** the persisted snapshot SHALL include the resolved runtime identity and the Codex continuity metadata required to continue that run
- **THEN** later status or resume flows SHALL be able to reference the same Codex execution identity without re-inferring it

#### Scenario: Resume fails explicitly when Codex continuity state is missing
- **WHEN** `/bridge/resume` is called for a paused Codex task whose snapshot no longer contains valid Codex continuity metadata
- **THEN** the bridge SHALL return an explicit continuity or configuration error instead of starting a duplicate fresh run
- **THEN** operators SHALL be able to distinguish a failed resume from a newly started Codex execution
