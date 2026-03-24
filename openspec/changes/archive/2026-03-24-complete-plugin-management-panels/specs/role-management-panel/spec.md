## ADDED Requirements

### Requirement: Operators can edit roles through a structured manifest-aware form
The system SHALL provide a structured role management panel for creating and editing roles without requiring raw JSON editing for the primary flow. The panel SHALL expose editable controls for metadata, identity, capabilities, knowledge, security, and inheritance fields that are part of the normalized Role manifest contract used by the current role APIs.

#### Scenario: Edit an existing role with structured sections
- **WHEN** the operator opens an existing role for editing
- **THEN** the panel SHALL load its current metadata, identity, capabilities, knowledge, security, and `extends` values into labeled structured form sections
- **THEN** saving the form SHALL persist those fields back through the role API as one normalized manifest payload

#### Scenario: Create a role with execution and security settings
- **WHEN** the operator creates a new role
- **THEN** the panel SHALL allow configuration of allowed tools, permission mode, budget or turn limits, review requirement, and path restrictions alongside the role's identity and description
- **THEN** the saved role SHALL remain compatible with subsequent list and get operations through the same role API

### Requirement: The role panel supports template-based creation and inheritance setup
The system SHALL allow operators to start a new role from an existing role template or establish inheritance from an existing role without manually copying every field. The panel SHALL make the selected template or parent role visible in the creation flow so operators can understand what is being reused.

#### Scenario: Start from an existing role template
- **WHEN** the operator chooses to create a role from an existing role template
- **THEN** the creation flow SHALL prefill the structured editor with the selected role's reusable values
- **THEN** the operator SHALL be able to modify the draft before saving it as a new role

#### Scenario: Create a child role that extends an existing role
- **WHEN** the operator chooses an existing role as a parent for inheritance
- **THEN** the editor SHALL store the parent identifier in the role's `extends` field
- **THEN** the panel SHALL clearly indicate that the new role is inheriting from that parent instead of being an unrelated standalone role

### Requirement: The role library summarizes execution-relevant properties for selection
The system SHALL present the role list as a management library rather than a name-only directory. Each role entry SHALL summarize the execution-relevant properties needed for selection and review, including tags, role summary, inheritance marker when present, tool or budget constraints when available, and review or permission safety cues when configured.

#### Scenario: Review role differences from the list view
- **WHEN** the operator scans the role library
- **THEN** each role card SHALL expose enough summary information to distinguish execution intent and governance settings without opening the edit form

#### Scenario: Role uses review or path restrictions
- **WHEN** a role requires review or defines allowed or denied paths
- **THEN** the role list or detail summary SHALL surface those constraints as visible safety cues instead of hiding them inside the edit form only
