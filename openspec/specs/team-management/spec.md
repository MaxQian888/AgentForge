# team-management Specification

## Purpose
Define the project-level team management contract for AgentForge so human and agent members share one canonical member model for listing, creation, updates, workload context, and navigation into related collaboration surfaces.
## Requirements
### Requirement: Project team view lists human and agent members from one member model
The system SHALL provide a project-level team management view backed by a unified member model so that human and agent members are visible in one place. Each listed member MUST identify its type, name, role, status, and core collaboration metadata needed for project coordination.

#### Scenario: Project team list loads successfully
- **WHEN** an authenticated user opens the team management view for a project that has members
- **THEN** the system displays both human and agent members in one list or grid
- **AND** every member entry shows its type, display name, role, status, and at least one collaboration-oriented detail such as skills, workload, or last activity

#### Scenario: Project has no members yet
- **WHEN** a project has no registered human or agent members
- **THEN** the team management view displays an explicit empty state
- **AND** the empty state provides an action for adding the first member

### Requirement: Project team management supports member creation and updates
The system SHALL support project-scoped member management actions needed to keep the team roster usable for both human and agent members. The creation and update experience MUST adapt to the member type: human members retain simple profile editing, while agent members MUST expose structured editing for supported profile metadata such as skills, bound role selection, activation state, and agent configuration fields persisted by the member API.

#### Scenario: Human or agent member is added
- **WHEN** an authorized user submits a valid add-member request for a human or agent member
- **THEN** the system creates the member in the target project
- **THEN** the creation flow captures supported member-type-specific fields in the same interaction, including agent profile data for agent members
- **THEN** the new member appears in the team management view without requiring a manual page refresh

#### Scenario: Existing member details are updated
- **WHEN** an authorized user updates an existing member's editable fields
- **THEN** the system persists the supported changes for that member type, including role, status, skills, and supported agent profile metadata when applicable
- **THEN** the team management view reflects the updated profile information after save

#### Scenario: Agent member editing does not require raw JSON as the primary path
- **WHEN** an operator edits an existing agent member
- **THEN** the UI loads the persisted agent profile into labeled fields and summaries instead of requiring the operator to edit raw JSON directly
- **THEN** the operator can change supported agent profile settings through the structured editor flow

### Requirement: Team management exposes member workload context for collaboration
The system SHALL surface enough member-level workload context to help users decide who should own or review work. This context MUST be consistent with dashboard team insights and usable as a navigation hub into related project work.

#### Scenario: Member workload is available
- **WHEN** the system has task ownership, active agent run, or recent activity data for a member
- **THEN** the team management view shows the member's current workload summary
- **AND** the summary distinguishes human-assigned work from active agent execution where applicable

#### Scenario: User investigates a member from team management
- **WHEN** a user selects a member entry or workload indicator
- **THEN** the system navigates to a related filtered view such as project tasks, agent details, or recent activity for that member
- **AND** the destination preserves enough context for the user to continue managing that member's work without restarting the search

### Requirement: Team roster surfaces agent role linkage and configuration readiness
The system SHALL expose enough agent-specific summary information in the team roster to help users manage silicon employees as first-class collaborators. Agent entries MUST show their bound role state and a concise readiness summary of supported execution or governance settings so operators can identify which agent members are ready to use or still need configuration.

#### Scenario: Agent row shows bound role and readiness summary
- **WHEN** the project roster contains an agent member with saved profile metadata
- **THEN** the team management view shows the agent's bound role or role state, key skills, and a concise readiness summary for supported execution-related settings
- **THEN** the operator can distinguish that agent from a generic member row without opening the editor first

#### Scenario: Agent profile is incomplete
- **WHEN** an agent member is missing required or expected profile information for the current product contract
- **THEN** the team management view surfaces an incomplete or attention-needed cue for that member
- **THEN** the view provides a direct edit path so the operator can complete the missing configuration

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

