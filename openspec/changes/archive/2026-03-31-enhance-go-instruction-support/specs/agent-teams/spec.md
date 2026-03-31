# Agent Teams Specification

## ADDED Requirements

### Requirement: Team creation persists strategy, budget, and runtime selection
The system SHALL create agent teams from an existing task and project, resolve the coding-agent runtime selection from project settings and request overrides, persist that selection into the team config, and broadcast team creation.

#### Scenario: Start team with project coding-agent defaults
- **WHEN** the caller starts a team without explicitly overriding runtime selection and the project defines coding-agent defaults
- **THEN** the created team persists the resolved runtime, provider, and model in its config

#### Scenario: Default missing team name and strategy
- **WHEN** the caller omits the team name or strategy
- **THEN** the team name defaults to a task-derived label
- **THEN** the strategy defaults to `plan-code-review`

### Requirement: Team strategy resolution supports current built-in strategies with safe fallback
The system SHALL resolve the current built-in team strategies `plan-code-review`, `wave-based`, `pipeline`, and `swarm`, and fall back to `plan-code-review` when an unknown strategy is requested.

#### Scenario: Resolve built-in strategy
- **WHEN** the team is created with one of the supported built-in strategy names
- **THEN** the team service uses the corresponding strategy implementation

#### Scenario: Unknown strategy falls back safely
- **WHEN** the team is created with an unknown strategy name
- **THEN** the team service logs the unexpected strategy
- **THEN** the service falls back to `plan-code-review`

### Requirement: Team startup delegates initial execution to the resolved strategy
The team service SHALL delegate startup behavior to the resolved strategy after the team record is created.

#### Scenario: Strategy starts planner-first execution
- **WHEN** a team starts under a planner-oriented strategy such as `plan-code-review` or `wave-based`
- **THEN** the strategy begins team execution by spawning a planner run with team execution context

### Requirement: Team run completion is routed back through the owning strategy
The team service SHALL process agent run completion only for runs that belong to a team, refresh team cost, and delegate follow-up behavior to the owning strategy unless the team is already terminal.

#### Scenario: Ignore completion for non-team runs
- **WHEN** a completed run does not carry a team ID
- **THEN** the team service does not process it as team orchestration state

#### Scenario: Skip processing for terminal teams
- **WHEN** a run completes for a team that is already in a terminal team status
- **THEN** the team service does not invoke additional strategy logic

### Requirement: Team child agent spawning preserves resolved runtime configuration
The current team strategies SHALL propagate the resolved runtime, provider, and model to downstream team-member spawns.

#### Scenario: Planner completion propagates runtime selection to coder spawns
- **WHEN** a planner run completes for a `plan-code-review` team
- **THEN** downstream coder spawns inherit the same runtime, provider, and model stored on the team config

#### Scenario: Child coder spawns use the canonical coding role
- **WHEN** the team service spawns coder runs for child tasks
- **THEN** those runs use the canonical role ID `coding-agent`

### Requirement: Team agent runs carry explicit team execution context
The system SHALL pass team execution context into spawned runs so that downstream runtime execution can identify the owning team and the member's team role.

#### Scenario: Spawn-for-team includes team context
- **WHEN** the system spawns an agent run for a team member
- **THEN** the resulting run carries the owning team ID
- **THEN** the resulting run carries the assigned team role such as planner or coder
