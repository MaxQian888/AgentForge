## MODIFIED Requirements

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

## ADDED Requirements

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
