## MODIFIED Requirements

### Requirement: Team management exposes member workload context for collaboration
The system SHALL surface enough member-level workload context to help users decide who should own or review work. This context MUST remain consistent with dashboard team insights, include recent activity cues when available, and use drill-down navigation whose destinations preserve and consume the same member/project context.

#### Scenario: Member workload is available
- **WHEN** the system has task ownership, active agent run, or recent activity data for a member
- **THEN** the team management view shows the member's current workload summary
- **AND** the summary distinguishes human-assigned work from active agent execution where applicable
- **AND** the row surfaces the most recent collaboration cue available for that member when such activity exists

#### Scenario: User investigates a member from team management
- **WHEN** a user clicks a member's workload indicator or selects a member entry from the team management view
- **THEN** the system navigates to the project task workspace filtered by that member as assignee
- **AND** the destination consumes the provided member and project context
- **AND** the task workspace pre-selects the appropriate view mode and shows tasks assigned to that member

#### Scenario: User investigates an agent member's active runs
- **WHEN** a user clicks an agent member's active agent run count
- **THEN** the system navigates to the team runs list filtered to show runs involving that agent member
- **AND** the user can continue managing that agent's work from the destination

### Requirement: Team roster surfaces agent role linkage and configuration readiness
The system SHALL expose enough agent-specific summary information in the team roster to help users manage silicon employees as first-class collaborators. Agent entries MUST show their bound role state and a concise readiness summary of supported execution or governance settings so operators can identify which agent members are ready to use or still need configuration.

#### Scenario: Agent row shows bound role and readiness summary
- **WHEN** the project roster contains an agent member with saved profile metadata
- **THEN** the team management view shows the agent's bound role or role state, key skills, and a concise readiness summary for supported execution-related settings
- **THEN** the operator can distinguish that agent from a generic member row without opening the editor first

#### Scenario: Agent profile is incomplete
- **WHEN** an agent member is missing required profile information (runtime, provider, or model)
- **THEN** the team management view surfaces a prominent "Setup Required" badge on that member row
- **AND** the badge is clickable and opens the agent profile editor directly
- **AND** the editor highlights the missing fields

### Requirement: Project-scoped team run collection and detail surfaces stay actionable
The system SHALL provide project-scoped agent team collection and detail surfaces that respect the backend team API contract, preserve the current project scope, and distinguish loading, empty, failure, and not-found states so operators can continue managing team runs without ambiguous feedback.

#### Scenario: Team list loads for the current project scope
- **WHEN** an authenticated user opens the team run collection for a selected project
- **THEN** the system requests team runs with that project's identifier
- **AND** the resulting list only shows team runs for that project scope
- **AND** the page exposes the active project context so the operator can switch scope intentionally

#### Scenario: Team list request fails
- **WHEN** the project-scoped team run request fails
- **THEN** the system shows an actionable error state with retry button instead of an empty successful list
- **AND** the error message explains what went wrong

#### Scenario: Team detail is loading
- **WHEN** a user opens a team detail route and the team summary request is still in flight
- **THEN** the system shows a loading skeleton for that detail view
- **AND** the system SHALL NOT present the team as missing until the request has completed with a not-found outcome

#### Scenario: Team detail shows fully populated summary
- **WHEN** a user opens a team detail view
- **THEN** the system displays the team's task title (not just task ID), pipeline visualization with planner/coder/reviewer statuses, coder run count and completion state, cost tracking, and error messages
- **AND** all `AgentTeamSummaryDTO` fields are populated by the backend

## ADDED Requirements

### Requirement: Backend populates AgentTeamSummaryDTO with task and agent run data
The system SHALL populate all fields of `AgentTeamSummaryDTO` when returning team detail or team list responses. The handler MUST JOIN with the tasks table for `taskTitle` and query agent runs for `coderRuns`, `coderTotal`, `coderCompleted`, `plannerStatus`, and `reviewerStatus`.

#### Scenario: Team detail returns fully populated summary
- **WHEN** the backend receives `GET /api/v1/teams/:id`
- **THEN** the response includes `taskTitle` from the associated task record
- **AND** the response includes `coderRuns` array with status and cost for each coder agent run
- **AND** the response includes `plannerStatus` and `reviewerStatus` from their respective agent run records

#### Scenario: Team list returns summaries with task titles
- **WHEN** the backend receives `GET /api/v1/teams?projectId=X`
- **THEN** each team in the response includes its `taskTitle`
- **AND** the query uses batch lookups (IN clause) to avoid N+1 queries
