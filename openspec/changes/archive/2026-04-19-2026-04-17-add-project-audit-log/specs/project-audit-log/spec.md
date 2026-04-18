## ADDED Requirements

### Requirement: System SHALL persist project-scoped audit events in a dedicated store
The system SHALL persist audit events describing write actions against project-scoped resources in a dedicated table, separate from business/execution logs and plugin event logs. Each event SHALL include the initiating actor identity, the actor's project role at event time, the canonical `ActionID`, the targeted resource type and identifier, a sanitized payload snapshot of the change, and the time of occurrence.

#### Scenario: Human-initiated write produces an audit event
- **WHEN** an authenticated member performs a gated write action (project/member/task/team-run/workflow/wiki/settings/automation/dashboard change)
- **THEN** the system persists an audit event referencing the acting user, their `projectRole` at the time, the `ActionID`, the targeted resource, and a sanitized snapshot of the change

#### Scenario: Audit event is stored independently from operational logs
- **WHEN** an audit event is persisted
- **THEN** it lands in the dedicated `project_audit_events` store (or its contract-equivalent), not in the business/execution log store
- **AND** its schema supports queries keyed on `project_id + occurred_at` and on `project_id + action_id` / `project_id + actor_user_id`

### Requirement: Audit events SHALL use the same ActionID enum as RBAC
The audit event `action_id` field SHALL take values only from the canonical `ActionID` enum defined by the project access control capability. The system SHALL NOT introduce a parallel audit-only action taxonomy.

#### Scenario: Writing an event with an unknown ActionID
- **WHEN** any emission path attempts to record an event with an `action_id` not declared in the canonical enum
- **THEN** the audit service rejects the write and emits a diagnostic warning
- **AND** no event row is persisted under an undeclared action

### Requirement: Audit writes SHALL NOT block the main business path
The system SHALL treat audit persistence as a best-effort secondary write. A failure to persist an audit event SHALL NOT cause the underlying business operation to fail or be rolled back, but SHALL trigger visible diagnostic signals so lost events can be recovered.

#### Scenario: Audit store is temporarily unavailable
- **WHEN** the audit persistence layer fails to write an event (DB error, transient unavailability)
- **THEN** the originating business operation still succeeds and returns its normal response
- **AND** the event enters a bounded retry queue for later persistence
- **AND** a warning diagnostic is emitted

#### Scenario: Retry queue exceeds sustained failure threshold
- **WHEN** the retry queue has been unable to drain to the store for more than the configured degradation window
- **THEN** the system spills queued events to an append-only local file as a durable fallback
- **AND** emits an elevated `audit_sink_degraded` signal to operations

### Requirement: RBAC deny events SHALL be audited
The RBAC middleware SHALL emit a `rbac_denied` audit event whenever a gated request is rejected due to insufficient role or non-membership, including the attempted `ActionID` and the caller identity. To prevent abuse-driven noise, the system SHALL deduplicate identical `(actor_user_id, action_id, resource_id)` denials within a short window before persistence.

#### Scenario: Viewer attempts a gated write
- **WHEN** a caller with `projectRole=viewer` attempts a write action requiring `editor+`
- **THEN** the middleware rejects the request AND emits a `rbac_denied` event identifying the attempted `ActionID` and the rejected caller
- **AND** repeated identical attempts within the dedup window do not produce additional events

### Requirement: Audit payload snapshots SHALL be sanitized and bounded
The system SHALL sanitize audit payload snapshots before persistence by redacting known sensitive field names (including but not limited to `secret`, `token`, `api_key`, `password`, `access_token`, `refresh_token`) and SHALL bound payload size to a fixed maximum, truncating with a visible marker when exceeded.

#### Scenario: Payload contains a redactable field
- **WHEN** an emission payload contains a sensitive field matched by the denylist
- **THEN** the persisted snapshot replaces the sensitive value with a redacted marker
- **AND** leaves non-sensitive sibling fields intact

#### Scenario: Payload exceeds the size limit
- **WHEN** an emission payload exceeds the configured size limit
- **THEN** the persisted snapshot is truncated
- **AND** the truncation is represented by an explicit marker within the stored JSON so readers can detect it

### Requirement: Audit query API SHALL be gated by admin-or-higher project role
The system SHALL expose a project-scoped audit query API (list + detail) that requires the caller to hold `projectRole ≥ admin`. The list endpoint SHALL support filtering by `actionId`, `actorUserId`, `resourceType`, `resourceId`, and time range, and SHALL return results paginated by cursor in reverse chronological order.

#### Scenario: Admin queries the audit log
- **WHEN** a caller with `projectRole=admin` or `owner` requests the audit list endpoint with valid filters
- **THEN** the system returns matching events in reverse chronological order with a cursor for pagination
- **AND** each returned event includes the sanitized payload snapshot reference

#### Scenario: Editor or viewer queries the audit log
- **WHEN** a caller with `projectRole=editor` or `viewer` requests the audit list or detail endpoints
- **THEN** the system rejects the request with `403 insufficient_project_role`
- **AND** does not return any events or metadata

### Requirement: Audit UI SHALL present events within the project workspace without making write operations fail
The system SHALL provide a project-scoped audit log view within the project workspace (under settings or an equivalent governance location) presenting filterable, paginated events and a detail drawer with the full sanitized payload. The UI SHALL be hidden from callers below `admin` and SHALL not attempt any write operations against the audit store.

#### Scenario: Admin opens the audit log view
- **WHEN** a caller with `projectRole=admin` or `owner` opens the project workspace
- **THEN** an audit log entry point is visible
- **AND** selecting it renders the filterable list and a detail drawer populated from the query API

#### Scenario: Viewer or editor opens the project workspace
- **WHEN** a caller with `projectRole=viewer` or `editor` opens the project workspace
- **THEN** the audit log entry point is hidden or disabled
- **AND** direct URL access returns a 403 from the backend
