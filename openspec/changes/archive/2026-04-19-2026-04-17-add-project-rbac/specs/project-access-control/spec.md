## ADDED Requirements

### Requirement: Project access control defines four canonical project roles
The system SHALL define exactly four project-level roles for human members — `owner`, `admin`, `editor`, `viewer` — with fixed semantics, and SHALL NOT allow custom or ad-hoc project roles. The canonical role name `projectRole` SHALL be used across persistence, API contracts, and UI without collision with agent role identifiers.

#### Scenario: Project role taxonomy is enumerated
- **WHEN** any persistence, API, or UI component references a project-level role
- **THEN** it MUST use one of `owner`, `admin`, `editor`, or `viewer`
- **AND** any other value MUST be rejected at validation time

#### Scenario: Project role is distinct from agent role
- **WHEN** a member record carries both a human project role and an agent role manifest identifier
- **THEN** the project role is exposed as `projectRole` and the agent role identifier is exposed as `roleId` (or equivalent agent-manifest field)
- **AND** no part of the system treats them as interchangeable

### Requirement: Project write actions SHALL be gated by a central action-to-role matrix
The system SHALL enforce project-scoped write authorization through a single, centrally declared `ActionID → MinRole` matrix consumed by a shared RBAC middleware. Every project-scoped write route SHALL be tagged with its `ActionID`, and the middleware SHALL reject requests whose caller does not meet the minimum role.

#### Scenario: Caller meets the required role
- **WHEN** a caller with `projectRole` ≥ the action's `MinRole` invokes a gated project-scoped route
- **THEN** the middleware permits the request and passes control to the handler

#### Scenario: Caller is below the required role
- **WHEN** a caller with `projectRole` < the action's `MinRole` invokes a gated route
- **THEN** the middleware rejects the request with `403 insufficient_project_role`
- **AND** the handler is not invoked

#### Scenario: Caller is not a member of the project
- **WHEN** an authenticated user who is not a member of the target project invokes any project-scoped route
- **THEN** the middleware rejects the request with `403 not_a_project_member`
- **AND** the response does not leak whether the project exists

### Requirement: Agent-initiated actions SHALL be authorized by the initiating human's project role
The system SHALL gate every human-triggered agent action — task dispatch, team run start/retry/cancel, workflow execution, agent spawn, and manual automation trigger — by resolving the initiating user's `projectRole` and checking it against the same matrix as direct API actions. The agent's own role manifest SHALL NOT be used to authorize the action.

#### Scenario: Viewer attempts to start an agent action
- **WHEN** a human with `projectRole=viewer` attempts to dispatch a task, start a team run, execute a workflow, or spawn an agent
- **THEN** the system rejects the action before any agent work is scheduled
- **AND** the rejection references the missing required role rather than any agent-manifest property

#### Scenario: Service receives the initiator identity
- **WHEN** any handler that starts an agent action calls its service layer
- **THEN** the service signature REQUIRES an `initiatorUserID` value (or an equivalent typed caller identity) and a boolean marker for system-initiated calls
- **AND** callers that do not provide initiator identity fail at the service boundary before the agent action begins

### Requirement: System-initiated agent actions SHALL reference a configured-by user whose current role is still sufficient
The system SHALL distinguish automation/scheduler/IM-webhook triggered agent actions from human-initiated ones. For these system-initiated actions, the system SHALL resolve the `configured-by` user who last authorized the automation and SHALL verify that user's CURRENT `projectRole` still satisfies the action's `MinRole`. If the configured-by user no longer meets the role, the system SHALL block the run and emit a signal that the automation binding is invalid.

#### Scenario: Automation runs while the configuring user still has admin role
- **WHEN** a scheduler or automation triggers an agent action whose configured-by user is currently `admin` or `owner`
- **THEN** the system allows the run
- **AND** records both the system-initiated flag and the configured-by user identity

#### Scenario: Automation runs after the configuring user has been demoted
- **WHEN** a scheduler or automation triggers an agent action whose configured-by user is now `editor` or `viewer`, or is no longer a member
- **THEN** the system rejects the run
- **AND** emits an `rbac_snapshot_invalid` signal referencing the automation and the stale configured-by user

### Requirement: Project creation SHALL atomically grant the creator owner role
The system SHALL create a project and its creating user's `owner` member record in the same transaction, such that a project can never exist without at least one `owner` member at the moment creation returns.

#### Scenario: Project creation succeeds
- **WHEN** an authenticated user creates a new project
- **THEN** the transaction persists the project record AND inserts a `members` row for the creating user with `type=human` and `projectRole=owner`
- **AND** the response reflects the created project together with the creator's membership

#### Scenario: Project creation fails mid-transaction
- **WHEN** either the project insert or the owner-member insert fails
- **THEN** neither record is persisted
- **AND** no half-created project without an owner is left behind

### Requirement: Project SHALL always retain at least one owner
The system SHALL prevent any role change or member removal that would leave a project with zero `owner` members. The final remaining owner's role SHALL NOT be downgraded and the final remaining owner's membership SHALL NOT be deleted.

#### Scenario: Last owner is downgraded
- **WHEN** a role update request would change the project's only remaining `owner` to any non-owner role
- **THEN** the system rejects the request with `409 last_owner_protected`
- **AND** the role remains unchanged

#### Scenario: Last owner is removed
- **WHEN** a member delete request would remove the project's only remaining `owner`
- **THEN** the system rejects the request with `409 last_owner_protected`
- **AND** the membership remains intact

#### Scenario: Additional owner is added before downgrade
- **WHEN** the caller first promotes another member to `owner` and then downgrades or removes the previously sole owner
- **THEN** the system permits both operations in sequence because the project never drops below one owner

### Requirement: Admin role cannot modify owners
The system SHALL forbid callers with `projectRole=admin` from changing the `projectRole` of any `owner` member or removing any `owner` member. Only `owner` callers may alter other owners.

#### Scenario: Admin attempts to downgrade an owner
- **WHEN** a caller with `projectRole=admin` attempts to update an `owner` member's role
- **THEN** the system rejects the request with `403 cannot_modify_owner_as_admin`
- **AND** the target member's role is unchanged

#### Scenario: Owner modifies another owner
- **WHEN** a caller with `projectRole=owner` updates another owner's role or removes them (without violating last-owner protection)
- **THEN** the system permits the operation

### Requirement: Backend SHALL publish a canonical per-project permissions endpoint
The system SHALL expose `GET /auth/me/projects/:pid/permissions` returning the caller's current `projectRole` for that project and the set of allowed `ActionID` values derived from the server-side matrix. Frontend UI SHALL consume this endpoint as the source of truth for gate rendering instead of duplicating the matrix on the client.

#### Scenario: Authenticated member requests permissions
- **WHEN** a member calls `GET /auth/me/projects/:pid/permissions`
- **THEN** the response includes the caller's `projectRole` and an array of `allowedActions` matching the server-side matrix for that role

#### Scenario: Non-member requests permissions
- **WHEN** an authenticated user who is not a member of the project calls the endpoint
- **THEN** the response is `403 not_a_project_member`
- **AND** the response does not leak whether the project exists

### Requirement: Frontend SHALL gate write entry points using server-provided permissions
The system's frontend SHALL hide or disable write-capable UI actions whose corresponding `ActionID` is not present in the server-provided `allowedActions` set, and SHALL NOT rely on a client-side role matrix to make this decision. When a gated action is attempted anyway, the frontend SHALL present a consistent authorization failure message derived from the backend error code.

#### Scenario: Viewer opens a project workspace
- **WHEN** a user with `projectRole=viewer` opens a project workspace
- **THEN** write-capable entry points such as "Dispatch", "Start Team", "Invite Member", "Edit Settings", "Execute Workflow" are hidden or rendered disabled with a tooltip indicating required role
- **AND** no write request is issued if the user interacts with disabled affordances

#### Scenario: Backend rejects a write that slipped past the frontend
- **WHEN** the backend returns `403 insufficient_project_role` or `403 not_a_project_member` in response to a write action
- **THEN** the frontend surfaces a consistent authorization error notification referencing the required role
- **AND** does not retry automatically
