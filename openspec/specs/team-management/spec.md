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
The system SHALL support project-scoped member management actions needed to keep the team roster usable. At minimum, users with access to the project MUST be able to create a member record and update editable member fields such as role, status, or profile metadata.

#### Scenario: Human or agent member is added
- **WHEN** an authorized user submits a valid add-member request for a human or agent member
- **THEN** the system creates the member in the target project
- **AND** the new member appears in the team management view without requiring a manual page refresh

#### Scenario: Existing member details are updated
- **WHEN** an authorized user updates an existing member's editable fields
- **THEN** the system persists the changes
- **AND** the team management view reflects the updated role, status, or profile metadata

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
