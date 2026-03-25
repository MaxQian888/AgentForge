## ADDED Requirements

### Requirement: Team startup supports explicit coding-agent runtime selection
The system SHALL allow a team run to start with an explicit coding-agent `runtime`, `provider`, and `model` selection, and it MUST also support falling back to the project's resolved defaults when the caller omits that selection.

#### Scenario: Team starts with an explicit Codex selection
- **WHEN** a user starts a team with `runtime=codex`, a compatible provider selection, and a Codex model
- **THEN** the created team response SHALL preserve that resolved `runtime`, `provider`, and `model`
- **THEN** the team management view SHALL be able to display that selection without inferring it from unrelated fields

#### Scenario: Team start uses project defaults
- **WHEN** a user starts a team without explicitly providing runtime selection
- **THEN** the backend SHALL resolve the project's default coding-agent selection before launching the team
- **THEN** the team response SHALL expose the resolved `runtime`, `provider`, and `model` rather than leaving them blank

### Requirement: Team execution preserves coding-agent selection across planner, coder, and reviewer phases
The system SHALL preserve one resolved coding-agent `runtime`, `provider`, and `model` selection across the planner, coder, and reviewer phases of a team run unless a future capability explicitly allows per-role overrides.

#### Scenario: Planner selection is inherited by downstream phases
- **WHEN** a team run starts with a resolved coding-agent selection and the planner phase completes successfully
- **THEN** every coder or reviewer run spawned for that team SHALL inherit the same resolved `runtime`, `provider`, and `model`
- **THEN** downstream runs SHALL NOT silently revert to empty values or unrelated defaults

#### Scenario: Team retry preserves the same runtime identity
- **WHEN** a failed team is retried
- **THEN** the retry flow SHALL reuse the team's previously resolved `runtime`, `provider`, and `model`
- **THEN** the retry SHALL NOT force the operator to rediscover or re-enter the coding-agent selection unless they explicitly change it through a supported edit flow
