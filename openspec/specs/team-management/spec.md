# team-management Specification

## Purpose
Define the project-level team management contract for AgentForge so human and agent members share one canonical member model for listing, creation, updates, workload context, navigation into related collaboration surfaces, and project-scoped team run management.
## Requirements
### Requirement: Project team view lists human and agent members from one member model
The system SHALL provide a project-level team management view backed by a unified member model and an explicit current project scope so that human and agent members are visible in one place. Each listed member MUST identify its type, name, role, canonical availability status, and core collaboration metadata needed for project coordination. When documented collaboration identity fields are configured for a member, the team view MUST surface those IM identity cues without requiring operators to inspect raw config.

#### Scenario: Project team list loads successfully
- **WHEN** an authenticated user opens the team management view for a selected project that has members
- **THEN** the system displays both human and agent members in one list or grid for that project scope
- **AND** every member entry shows its type, display name, role, canonical status, and at least one collaboration-oriented detail such as skills, workload, last activity, or IM identity

#### Scenario: User changes the current project scope
- **WHEN** the user switches the team management view from one project to another
- **THEN** the system reloads the roster for the newly selected project scope
- **AND** the page SHALL NOT continue showing stale members from the previous project as if they belonged to the new scope

#### Scenario: Project has no members yet
- **WHEN** a selected project has no registered human or agent members
- **THEN** the team management view displays an explicit empty state for that project scope
- **AND** the empty state provides an action for adding the first member

#### Scenario: Member has documented IM identity
- **WHEN** a listed member has saved collaboration identity fields such as IM platform and IM user identifier
- **THEN** the team management roster surfaces a readable IM identity summary for that member
- **AND** operators can distinguish IM-linked members from members that have no saved IM identity

### Requirement: Project team management supports member creation and updates
The system SHALL support project-scoped member management actions needed to keep the team roster usable for both human and agent members. The creation and update experience MUST adapt to the member type: human members retain simple profile editing, while agent members MUST expose structured editing for supported profile metadata such as skills, bound role selection, activation state, and agent configuration fields persisted by the member API. Both member types MUST support the documented canonical availability state and collaboration identity fields as part of the same member contract.

#### Scenario: Human or agent member is added
- **WHEN** an authorized user submits a valid add-member request for a human or agent member
- **THEN** the system creates the member in the target project
- **THEN** the creation flow captures supported member-type-specific fields in the same interaction, including agent profile data for agent members and documented status or IM identity fields when provided
- **THEN** the new member appears in the team management view without requiring a manual page refresh

#### Scenario: Existing member details are updated
- **WHEN** an authorized user updates an existing member's editable fields
- **THEN** the system persists the supported changes for that member type, including role, canonical status, IM identity, skills, and supported agent profile metadata when applicable
- **THEN** the team management view reflects the updated profile information after save

#### Scenario: Agent member editing does not require raw JSON as the primary path
- **WHEN** an operator edits an existing agent member
- **THEN** the UI loads the persisted agent profile into labeled fields and summaries instead of requiring the operator to edit raw JSON directly
- **THEN** the operator can change supported agent profile settings through the structured editor flow

#### Scenario: Member is suspended without being removed from the roster
- **WHEN** an operator changes a member's canonical status to `suspended`
- **THEN** the member remains visible in the team management roster with a suspended state cue
- **AND** the operator can later reactivate or otherwise update that same member record without recreating it

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

### Requirement: Canonical member availability constrains recommendations and assignment entry points
The system SHALL use the canonical team member availability state when exposing member candidates to downstream collaboration flows that depend on ready collaborators. Members in `inactive` or `suspended` state MUST remain visible in team management for editing and audit purposes, but they MUST NOT be surfaced as ready recommendation candidates or runnable agent assignment targets.

#### Scenario: Suspended member is excluded from assignee recommendations
- **WHEN** a task recommendation or similar candidate-selection flow runs for a project that contains both active members and suspended members
- **THEN** the returned candidate set excludes suspended members from ready recommendations
- **AND** eligible active members remain rankable using the existing workload and skills signals

#### Scenario: Unavailable agent member cannot be treated as a ready execution target
- **WHEN** a supported assignment or dispatch entry point resolves a target member whose canonical status is `inactive` or `suspended`
- **THEN** the system returns actionable unavailable feedback instead of treating that member as a ready runnable agent collaborator
- **AND** the same member remains visible in team management so operators can reactivate or edit it without recreating the record

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

### Requirement: Project team management supports bulk member availability governance
The system SHALL allow operators to select multiple members within the current project roster and apply canonical availability governance actions in one step. The first supported bulk actions MUST cover `active`, `inactive`, and `suspended` transitions, execute only within the current project scope, and return explicit per-member outcomes so operators can see which records changed and which failed.

#### Scenario: Bulk suspend succeeds for selected members
- **WHEN** an operator selects multiple members from the current project roster and applies the bulk `suspended` action
- **THEN** the system updates each selected member to canonical status `suspended`
- **AND** the roster refreshes without requiring a manual page reload
- **AND** the operator sees a result summary that confirms the affected members

#### Scenario: Bulk action returns mixed outcomes
- **WHEN** a bulk availability action succeeds for some selected members but fails for others
- **THEN** the system returns per-member success or failure outcomes
- **AND** the UI keeps the failure feedback actionable instead of collapsing it into one generic error
- **AND** successful members still reflect their new status after the roster refresh

#### Scenario: Project scope changes during bulk-governance workflow
- **WHEN** the operator switches the team workspace to a different project
- **THEN** any existing member selection and pending bulk-governance state are cleared
- **AND** the next bulk action cannot target members from the previous project scope

### Requirement: Team management surfaces an attention workflow for members needing operator action
The system SHALL provide a management-focused attention workflow that highlights members who need operator intervention, including setup-required agent members and members whose canonical availability is `inactive` or `suspended`. Operators MUST be able to use this workflow to filter the roster to the relevant members and jump directly into the corrective action.

#### Scenario: Operator opens the attention view
- **WHEN** the current roster contains setup-required agent members or unavailable members
- **THEN** the team workspace shows an attention summary grouped by actionable categories
- **AND** the operator can focus the roster on one of those categories without manually re-entering search filters

#### Scenario: Setup-required agent opens targeted remediation flow
- **WHEN** the operator selects a setup-required agent from the attention workflow
- **THEN** the system opens the existing member edit flow for that agent
- **AND** the missing readiness fields are highlighted
- **AND** the operator does not need to manually rediscover which fields are incomplete

### Requirement: Team roster exposes faster single-member lifecycle controls
The system SHALL expose lightweight lifecycle controls for common member-governance actions directly from the roster so operators do not need to enter the full edit form for every suspend or reactivate task. These controls MUST prevent duplicate submission while an action is already in progress and MUST keep the full edit flow available for deeper changes.

#### Scenario: Operator quickly reactivates an unavailable member
- **WHEN** an operator uses a row-level quick action to change an `inactive` or `suspended` member back to `active`
- **THEN** the system persists the new canonical status for that member
- **AND** the roster updates to reflect the new availability without requiring the operator to reopen the page

#### Scenario: Quick lifecycle action is already in flight
- **WHEN** the operator triggers a row-level lifecycle action and the request has not completed yet
- **THEN** the same quick action control becomes temporarily unavailable for repeated submission
- **AND** the workspace shows that the member is currently being updated

### Requirement: Team management validates bound roles against the authoritative role registry
The team management member contract SHALL validate agent-member role bindings against the authoritative role registry whenever a create or update flow submits a bound `roleId`. The system MUST reject saves that attempt to persist an unknown role binding, and it MUST return field-level feedback that points operators back to the role-binding control.

#### Scenario: Agent member save is rejected for an unknown role binding
- **WHEN** an operator creates or updates an agent member with a `roleId` that does not exist in the current role registry
- **THEN** the member save request is rejected instead of silently persisting the stale binding
- **THEN** the returned feedback identifies the bound role field as invalid
- **THEN** the operator can correct the role binding through the existing structured editor flow

### Requirement: Team roster distinguishes missing role setup from stale role binding
The team roster, readiness summaries, and attention workflow SHALL distinguish an agent member that has no bound role from an agent member whose stored `roleId` no longer resolves from the current role registry. A stale role binding MUST be shown as an actionable repair state rather than being collapsed into a generic setup label.

#### Scenario: Agent row shows a stale role binding cue
- **WHEN** the current project roster contains an agent member whose stored `roleId` no longer resolves from the current role registry
- **THEN** the roster shows an explicit stale role binding state for that member
- **THEN** the attention workflow can focus that member as needing repair
- **THEN** opening the edit flow highlights the role binding field instead of only generic runtime setup fields

#### Scenario: Empty role binding remains a separate setup state
- **WHEN** the current project roster contains an agent member with no bound `roleId`
- **THEN** the roster continues to show that member as missing role setup rather than as a stale bound role
- **THEN** operators can distinguish “never bound” from “bound to a deleted role” at a glance

