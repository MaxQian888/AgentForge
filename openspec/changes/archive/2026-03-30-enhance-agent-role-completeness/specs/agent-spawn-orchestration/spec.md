## MODIFIED Requirements

### Requirement: Agent spawn starts a real execution runtime

The spawn dialog SHALL include a role selector allowing operators to choose a role from the role library when spawning an agent. The selected role's ID SHALL be included as `roleId` in the spawn request. If no role is selected, the spawn request SHALL proceed without a `roleId` (preserving current behavior). The role selector SHALL populate from the role store and fetch roles on dialog open if not already loaded.

#### Scenario: Operator selects a role when spawning agent
- **WHEN** operator opens the spawn dialog and selects a role from the role dropdown
- **THEN** the spawn request includes the selected role's ID as `roleId`
- **AND** the runtime selector remains independently configurable

#### Scenario: Operator spawns without selecting a role
- **WHEN** operator opens the spawn dialog and leaves the role selector empty
- **THEN** the spawn request proceeds without a `roleId` field
- **AND** the spawn succeeds using default agent configuration

#### Scenario: Role list loads on dialog open
- **WHEN** the spawn dialog opens and the role store has no loaded roles
- **THEN** the dialog fetches the role list from the API
- **AND** displays available roles in the selector once loaded
