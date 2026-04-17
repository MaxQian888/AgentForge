## ADDED Requirements

### Requirement: Project-scoped write routes SHALL be tagged with an ActionID and enforced by the RBAC matrix
Every project-scoped route in the backend that creates, updates, deletes, dispatches, or executes resources SHALL be associated with a canonical `ActionID`, and the request SHALL pass the RBAC middleware before the handler runs. Read-only routes MAY opt out of gating when the resource is not sensitive.

#### Scenario: Write route missing ActionID tag
- **WHEN** the routing table is constructed at server start
- **THEN** every write-capable route under `projectGroup` MUST declare an associated `ActionID`
- **AND** a missing or unknown `ActionID` fails the server smoke/wire test so the regression cannot ship

#### Scenario: Gated route passes RBAC before ownership checks
- **WHEN** a request enters a gated project-scoped route
- **THEN** the RBAC middleware runs after the project-context middleware that establishes `pid`, and its decision is independent of per-resource ownership checks performed later
- **AND** an RBAC deny short-circuits further handler work

### Requirement: Project-scoped API responses SHALL carry sufficient information for frontend gating without additional round-trips
When a caller loads a project's top-level read surface, the response contract SHALL make it possible to determine which write actions the caller can perform without issuing additional authorization round-trips per button.

#### Scenario: Project detail response references a permissions endpoint
- **WHEN** a caller requests a project's detail payload used by project-entry workspaces
- **THEN** the response either embeds the caller's `projectRole` and allowed actions OR includes a stable link to the canonical `GET /auth/me/projects/:pid/permissions` endpoint for the frontend to consume
- **AND** the frontend does not need to issue a separate permissions request per gated button

### Requirement: Agent action endpoints SHALL require and record an initiator identity
Any API route that initiates an agent action within a project — task dispatch, team run start/retry/cancel, workflow execute, agent spawn, manual automation trigger — SHALL resolve an `initiatorUserID` from the authenticated request and SHALL propagate it to the service layer. System-initiated paths (scheduler, IM webhook, scheduled automation) SHALL be explicitly marked and SHALL carry a `configuredByUserID` identifying the human whose authorization underlies the run.

#### Scenario: Human-initiated dispatch carries initiator
- **WHEN** a human calls a task dispatch endpoint
- **THEN** the handler resolves the authenticated user and passes `initiatorUserID` to the service
- **AND** the service uses that identifier to evaluate the caller's `projectRole` against the dispatch `ActionID`

#### Scenario: Scheduler-initiated run carries configured-by identity
- **WHEN** the scheduler triggers an automation that starts an agent action
- **THEN** the handler sets `systemInitiated=true` and supplies the `configuredByUserID` captured when the automation was last authorized
- **AND** the service re-evaluates that user's current `projectRole` before allowing the run
