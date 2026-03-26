## ADDED Requirements

### Requirement: Project-scoped team run collection and detail surfaces stay actionable
The system SHALL provide project-scoped agent team collection and detail surfaces that respect the backend team API contract, preserve the current project scope, and distinguish loading, empty, failure, and not-found states so operators can continue managing team runs without ambiguous feedback.

#### Scenario: Team list loads for the current project scope
- **WHEN** an authenticated user opens the team run collection for a selected project
- **THEN** the system requests team runs with that project's identifier
- **AND** the resulting list only shows team runs for that project scope
- **AND** the page exposes the active project context so the operator can switch scope intentionally

#### Scenario: Team list request fails
- **WHEN** the project-scoped team run request fails
- **THEN** the system shows an actionable error state instead of an empty successful list
- **AND** the operator can retry the same project-scoped request without leaving the page

#### Scenario: Team detail is loading
- **WHEN** a user opens a team detail route and the team summary request is still in flight
- **THEN** the system shows a loading state for that detail view
- **AND** the system SHALL NOT present the team as missing until the request has completed with a not-found outcome

### Requirement: Team strategy identifiers remain canonical and backward-compatible
The system SHALL use canonical backend strategy identifiers for new team startup and team run display, while remaining able to interpret supported legacy identifiers already persisted in existing team records.

#### Scenario: New team startup uses canonical strategy identifiers
- **WHEN** a user starts a new team through the supported startup flow
- **THEN** the request and resulting team summary use a supported canonical strategy identifier
- **AND** team collection and detail views render that strategy through a human-readable label without inventing a different API value

#### Scenario: Existing team uses a legacy strategy alias
- **WHEN** a stored team record still carries the legacy `planner_coder_reviewer` identifier
- **THEN** the system normalizes that alias to the equivalent supported strategy semantics for display and follow-up actions
- **AND** the operator can still inspect, retry, or cancel that team without manual data repair

## MODIFIED Requirements

### Requirement: Project team view lists human and agent members from one member model
The system SHALL provide a project-level team management view backed by a unified member model and an explicit current project scope so that human and agent members are visible in one place. Each listed member MUST identify its type, name, role, status, and core collaboration metadata needed for project coordination.

#### Scenario: Project team list loads successfully
- **WHEN** an authenticated user opens the team management view for a selected project that has members
- **THEN** the system displays both human and agent members in one list or grid for that project scope
- **AND** every member entry shows its type, display name, role, status, and at least one collaboration-oriented detail such as skills, workload, or last activity

#### Scenario: User changes the current project scope
- **WHEN** the user switches the team management view from one project to another
- **THEN** the system reloads the roster for the newly selected project scope
- **AND** the page SHALL NOT continue showing stale members from the previous project as if they belonged to the new scope

#### Scenario: Project has no members yet
- **WHEN** a selected project has no registered human or agent members
- **THEN** the team management view displays an explicit empty state for that project scope
- **AND** the empty state provides an action for adding the first member

### Requirement: Team management exposes member workload context for collaboration
The system SHALL surface enough member-level workload context to help users decide who should own or review work. This context MUST remain consistent with dashboard team insights, include recent activity cues when available, and use drill-down navigation whose destinations preserve and consume the same member/project context.

#### Scenario: Member workload is available
- **WHEN** the system has task ownership, active agent run, or recent activity data for a member
- **THEN** the team management view shows the member's current workload summary
- **AND** the summary distinguishes human-assigned work from active agent execution where applicable
- **AND** the row surfaces the most recent collaboration cue available for that member when such activity exists

#### Scenario: User investigates a member from team management
- **WHEN** a user selects a member entry or workload indicator from the team management view
- **THEN** the system navigates to a related filtered view such as project tasks, agent details, or recent activity for that member
- **AND** the destination consumes the provided member and project context instead of ignoring it
- **AND** the user can continue managing that member's work without manually rebuilding the same filter state

### Requirement: Team startup supports explicit coding-agent runtime selection
The system SHALL allow a team run to start through a supported team strategy plus an explicit coding-agent `runtime`, `provider`, and `model` selection, and it MUST also support falling back to the project's resolved defaults when the caller omits that selection. The resulting team summary MUST expose the resolved strategy and coding-agent identity used by downstream team surfaces.

#### Scenario: Team starts with an explicit Codex selection
- **WHEN** a user starts a team with a supported strategy identifier, `runtime=codex`, a compatible provider selection, and a Codex model
- **THEN** the created team response SHALL preserve that resolved `strategy`, `runtime`, `provider`, and `model`
- **AND** the team management surfaces SHALL be able to display that selection without inferring it from unrelated fields

#### Scenario: Team start uses project defaults
- **WHEN** a user starts a team without explicitly providing runtime selection
- **THEN** the backend SHALL resolve the project's default coding-agent selection before launching the team
- **AND** the team response SHALL expose the resolved `strategy`, `runtime`, `provider`, and `model` rather than leaving them blank
