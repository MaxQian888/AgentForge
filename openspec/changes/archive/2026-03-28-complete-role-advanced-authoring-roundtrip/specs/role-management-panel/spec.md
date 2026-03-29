## ADDED Requirements

### Requirement: Role workspace supports advanced role authoring without lossy saves
The system SHALL let operators inspect and edit the current documented advanced role configuration from the role workspace without requiring raw file surgery for routine authoring. The workspace MUST cover the supported advanced fields that the current Go role contract already preserves, including advanced capability settings, richer tool-host configuration, structured knowledge details, memory metadata, and controlled override authoring. Saving a draft that changes only part of the role MUST preserve the rest of the loaded advanced manifest instead of serializing a reduced UI subset.

#### Scenario: Editing a role with advanced configuration does not drop untouched fields
- **WHEN** the operator opens a role that already includes advanced custom settings, shared knowledge source details, memory metadata, or overrides
- **AND** the operator edits only one advanced subsection or a basic field in the workspace
- **THEN** the save payload still preserves the untouched advanced sections from the loaded draft
- **THEN** the role remains semantically equivalent after reload except for the fields the operator actually changed

#### Scenario: Workspace blocks malformed advanced subsection input
- **WHEN** the operator enters malformed key-value data, invalid shared-source metadata, or invalid override content in an advanced authoring subsection
- **THEN** the workspace surfaces inline validation in the relevant subsection before submission
- **THEN** the workspace SHALL NOT submit a lossy or malformed role payload to the role API

### Requirement: Role workspace reveals advanced field provenance and save impact
The system SHALL show enough provenance and impact information for advanced role fields so operators can understand whether a value is inherited, copied from a template, explicitly overridden in the current draft, or preserved unchanged. This visibility MUST remain available from the same authoring flow as YAML preview, execution summary, preview, and sandbox actions.

#### Scenario: Operator inspects inherited advanced values before saving
- **WHEN** the operator edits a child role that inherits advanced tool, knowledge, memory, or governance settings from a parent role
- **THEN** the workspace indicates which advanced values are inherited versus explicitly set in the current draft
- **THEN** the operator can review the effective impact of those advanced values without leaving the current authoring flow

#### Scenario: Operator reviews save impact for advanced role edits
- **WHEN** the operator changes advanced fields and opens the review context before saving
- **THEN** the workspace shows which advanced sections will be written back, preserved as-is, or excluded from the execution profile
- **THEN** the operator can distinguish canonical YAML persistence from runtime projection behavior before submission
