## ADDED Requirements

### Requirement: Role governance exposes an authoritative downstream reference inventory
The system SHALL provide an authoritative reference inventory for a role across all currently supported consumer surfaces. The inventory MUST identify each consumer's type, stable identity, lifecycle state, and whether that consumer is blocking for role deletion or only advisory for audit context.

#### Scenario: Operator inspects a role with mixed consumers
- **WHEN** an operator or internal service requests the current reference inventory for a role that is used by an agent member, an installed workflow plugin, a queued execution request, and a historical agent run
- **THEN** the returned inventory includes each consumer as a distinct record or grouped entry with consumer type, human-readable label, lifecycle state, and blocking classification
- **THEN** the queued execution, member binding, and installed workflow references are marked blocking
- **THEN** the historical run reference is marked advisory rather than blocking

### Requirement: Role deletion is blocked by actionable current consumers
The system SHALL refuse deletion of a role while any blocking downstream consumer still references it. When deletion is refused, the response MUST include structured blocker details that identify the consuming surface and the remediation path instead of returning only a generic error string.

#### Scenario: Delete fails because a member and queued execution still reference the role
- **WHEN** a delete request targets a role that is still bound by at least one project agent member or queued execution request
- **THEN** the system returns a conflict response instead of deleting the role
- **THEN** the response identifies the blocking member and queued execution consumers explicitly
- **THEN** the response explains that the role must be rebound, cleared, or drained before deletion can proceed

### Requirement: Advisory historical references remain visible without preventing cleanup
The system SHALL preserve historical references to deleted roles for audit and diagnostics, but those references MUST NOT by themselves prevent role cleanup once all blocking current consumers have been removed.

#### Scenario: Delete succeeds when only historical references remain
- **WHEN** a delete request targets a role whose only remaining consumers are historical agent runs or equivalent audit records
- **THEN** the role deletion succeeds
- **THEN** the historical records continue to expose their stored `role_id` for audit context
- **THEN** the governance view distinguishes those preserved references from currently valid executable bindings
