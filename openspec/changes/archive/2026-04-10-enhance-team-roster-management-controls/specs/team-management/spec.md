## ADDED Requirements

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
