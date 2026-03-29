## ADDED Requirements

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

## MODIFIED Requirements

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
