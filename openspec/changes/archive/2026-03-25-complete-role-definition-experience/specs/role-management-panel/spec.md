## MODIFIED Requirements

### Requirement: Operators can edit roles through a structured manifest-aware form
The system SHALL provide a role management workspace that edits the normalized role manifest through labeled structured sections instead of relying on a small generic form or raw YAML as the primary path. The workspace MUST cover the current role API contract plus the advanced PRD-backed role sections needed for real authoring, including metadata, icon, version, identity, prompt fields, response style, capabilities, packages, tool host config, knowledge, memory, security, collaboration, triggers, overrides, and inheritance fields. The workspace MUST block save when required fields, nested advanced fields, or supported list inputs are invalid.

#### Scenario: Edit an existing advanced role through structured sections
- **WHEN** the operator opens an existing role for editing
- **THEN** the workspace loads the role's current metadata, identity, advanced identity fields, capability settings, knowledge sections, security settings, collaboration metadata, trigger definitions, version, and `extends` values into labeled sections
- **THEN** the operator can update those supported fields without switching to raw YAML editing

#### Scenario: Save is blocked by invalid structured data
- **WHEN** the operator enters invalid required metadata, malformed advanced nested config, or malformed supported list values in the role workspace
- **THEN** the system shows inline validation feedback before submission
- **THEN** the workspace SHALL NOT submit a partial or invalid manifest payload to the role API

### Requirement: The role panel supports template-based creation and inheritance setup
The system SHALL support role creation flows that start from a blank role, copy an existing role as a template, or inherit from an existing role. The workspace SHALL also provide a live execution-oriented summary and effective preview rail so operators can inspect the draft's prompt intent, inherited values, governance settings, and advanced execution cues before saving.

#### Scenario: Start from an existing role template
- **WHEN** the operator chooses to create a role from an existing role template
- **THEN** the workspace prefills the new draft with the template's reusable values
- **THEN** the UI makes the template source visible so the operator understands what was reused

#### Scenario: Create a child role that extends an existing role
- **WHEN** the operator chooses an existing role as a parent for inheritance
- **THEN** the draft stores that parent in the role's `extends` field
- **THEN** the workspace clearly indicates that the draft is a child role and shows which values are inherited versus locally overridden

#### Scenario: Inspect effective summary before saving
- **WHEN** the operator edits prompt, packages, tool host config, budget, permission, collaboration, or trigger fields in the role draft
- **THEN** the workspace shows a live summary of the draft's execution-relevant settings, including prompt intent, tool limits, budget or turn caps, permission mode, governance cues, and advanced authoring signals
- **THEN** the operator can review that summary without leaving the structured editor flow

## ADDED Requirements

### Requirement: Role workspace provides authoring guidance and YAML visibility
The system SHALL surface field-level guidance for advanced role definition and provide a YAML-oriented view of the current draft so operators do not have to infer how structured inputs map back to the canonical role asset.

#### Scenario: Operator reviews guidance for advanced fields
- **WHEN** the operator enters sections such as advanced identity, memory, collaboration, or trigger authoring
- **THEN** the workspace shows concise field explanations or guidance for the supported fields in that section
- **THEN** the operator can understand what the field means without leaving the role workspace

#### Scenario: Operator inspects YAML preview of the current draft
- **WHEN** the operator asks to inspect the role definition as YAML before saving
- **THEN** the workspace shows the current draft or preview response in a YAML-oriented form that maps back to the canonical role asset
- **THEN** the operator can compare structured inputs against the serialized definition without manually rebuilding the YAML in another tool

### Requirement: Role workspace can launch preview and sandbox flows from the current draft
The system SHALL let operators open authoritative preview and sandbox flows directly from the role workspace using either an existing persisted role or the current unsaved draft.

#### Scenario: Unsaved draft launches preview or sandbox
- **WHEN** the operator requests preview or sandbox while editing a new or modified role draft that has not yet been saved
- **THEN** the workspace submits the current draft to the backend preview or sandbox surface without first persisting it to `roles/<role-id>/role.yaml`
- **THEN** the operator receives authoritative preview or sandbox results tied to the current draft state
