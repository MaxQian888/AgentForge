# team-agent-context-handoff Specification

## Purpose
Define the Team-to-Bridge execution context contract so team-managed planner, coder, and reviewer runs always carry explicit `team_id` and `team_role` across execute, resume, status, and diagnostic flows without identity guesswork.
## Requirements
### Requirement: Team-managed bridge requests carry explicit team execution context
The Go orchestrator SHALL send explicit team execution context for every Team-managed Bridge execute or resume request. That context MUST include the canonical `team_id` and a normalized `team_role` drawn from the supported phase set (`planner`, `coder`, `reviewer`). The Bridge MUST validate that context, bind it to the active runtime request state, and MUST NOT reconstruct team identity later from unrelated task or run lookups.

#### Scenario: Planner run starts with explicit team context
- **WHEN** the Team service starts a planner run for a team task
- **THEN** the execute request sent to the Bridge includes the team's `team_id` and `team_role=planner`
- **THEN** the Bridge stores that context with the active runtime before execution begins

#### Scenario: Unsupported team role is rejected
- **WHEN** a Go-managed execute or resume request declares a `team_role` outside the supported team phase set
- **THEN** the Bridge rejects the request with a validation error
- **THEN** the backend does not treat that run as started or resumed

### Requirement: Team context survives continuity and diagnostics surfaces
The system SHALL preserve the resolved team execution context through snapshots, status metadata, and resume handling so that planner, coder, and reviewer runs keep the same team identity across pauses, restarts, and operator diagnostics. Continuity surfaces MUST expose enough context for the backend to associate a runtime with the correct team phase without guessing from persisted run tables alone.

#### Scenario: Paused coder resumes with the same team context
- **WHEN** a paused coder run is resumed after the backend has already bound it to a team
- **THEN** the backend sends the same `team_id` and `team_role=coder` back to the Bridge
- **THEN** the resumed runtime snapshot and later status responses keep that same team context

#### Scenario: Status metadata exposes team phase identity for diagnostics
- **WHEN** the backend queries Bridge status for an active reviewer run
- **THEN** the status payload includes the resolved `team_id` and `team_role=reviewer`
- **THEN** operator-facing summary or recovery flows can use that context without inferring it from unrelated records
