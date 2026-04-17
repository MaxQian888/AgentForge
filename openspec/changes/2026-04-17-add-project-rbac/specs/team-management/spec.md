## ADDED Requirements

### Requirement: Team roster exposes project role alongside agent role for every member
The team management roster SHALL surface each member's project-level `projectRole` as a first-class column or attribute distinct from any agent-manifest role binding, so operators can tell at a glance whether a member is an `owner`, `admin`, `editor`, or `viewer` in the current project without opening the member editor.

#### Scenario: Roster renders project role for human and agent members
- **WHEN** the team management view loads a project's roster
- **THEN** every member row shows its `projectRole` value
- **AND** for agent members the roster continues to show agent role readiness and role manifest binding as separate columns or badges

#### Scenario: Filter by project role
- **WHEN** an operator filters the roster by project role
- **THEN** only members matching the selected role(s) are shown
- **AND** the filter state is reflected in the URL or query so deep links retain the filter

### Requirement: Member creation and update surfaces require explicit project role assignment
Member add and edit flows SHALL require the caller to explicitly set a member's `projectRole` when the caller has authority to do so. The UI and API MUST NOT silently default to a role without showing the selected value to the caller, and the caller's own `projectRole` MUST authorize the operation.

#### Scenario: Admin invites a human member with an explicit role
- **WHEN** an `admin` or `owner` submits an add-member request
- **THEN** the request payload MUST include a `projectRole` value
- **AND** the UI presents `editor` as the default selection but requires confirmation before submit

#### Scenario: Editor attempts to add a member
- **WHEN** a caller with `projectRole=editor` attempts to add a member
- **THEN** the UI hides or disables the add-member affordance
- **AND** any direct API call is rejected with `403 insufficient_project_role`

#### Scenario: Admin attempts to change an owner's role
- **WHEN** a caller with `projectRole=admin` attempts to edit an owner member's role
- **THEN** the UI disables the role control for that row with an explanatory tooltip
- **AND** the API rejects the change with `403 cannot_modify_owner_as_admin`

### Requirement: Roster SHALL protect the last remaining owner
The team management surfaces SHALL prevent operators from reaching an empty-owner state through any combination of role change and member removal, and SHALL surface the blocking reason clearly.

#### Scenario: UI blocks removal of the last owner
- **WHEN** the roster contains exactly one `owner` and an operator triggers remove or downgrade on that member
- **THEN** the UI blocks the action before any request is sent
- **AND** surfaces a message explaining the last-owner protection and suggesting the caller first promote another member to `owner`

#### Scenario: Backend enforces even when UI is bypassed
- **WHEN** a direct API request would leave the project with zero owners
- **THEN** the backend rejects the request with `409 last_owner_protected`
- **AND** no state change is applied
