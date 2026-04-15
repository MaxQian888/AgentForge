## ADDED Requirements

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
