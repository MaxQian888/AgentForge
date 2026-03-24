## ADDED Requirements

### Requirement: Role workspace supports structured manifest editing with validation
The system SHALL provide a role management workspace that edits the normalized role manifest through labeled structured sections instead of relying on a small generic form or raw JSON as the primary path. The workspace MUST cover role metadata, version, identity, prompt fields, capabilities, knowledge, security, and inheritance fields that are part of the current role API contract, and it MUST block save when required fields or supported list inputs are invalid.

#### Scenario: Edit an existing role through structured sections
- **WHEN** the operator opens an existing role for editing
- **THEN** the workspace loads the role's current metadata, identity, prompt, capabilities, knowledge, security, version, and `extends` values into labeled sections
- **THEN** the operator can update those supported fields without switching to raw JSON editing

#### Scenario: Save is blocked by invalid structured data
- **WHEN** the operator enters invalid required metadata or malformed supported list values in the role workspace
- **THEN** the system shows inline validation feedback before submission
- **THEN** the workspace SHALL NOT submit a partial or invalid manifest payload to the role API

### Requirement: Role creation supports template, inheritance, and execution summary preview
The system SHALL support role creation flows that start from a blank role, copy an existing role as a template, or inherit from an existing role. The workspace SHALL also provide a live execution-oriented summary so operators can inspect the draft's prompt intent and governance settings before saving.

#### Scenario: Start a new role from an existing role template
- **WHEN** the operator chooses an existing role as a creation template
- **THEN** the workspace prefills the new draft with the template's reusable values
- **THEN** the UI makes the template source visible so the operator understands what was reused

#### Scenario: Create a role that inherits from an existing role
- **WHEN** the operator selects a parent role for inheritance
- **THEN** the draft stores that parent in the role's `extends` field
- **THEN** the workspace clearly indicates that the draft is a child role rather than an unrelated standalone role

#### Scenario: Inspect execution summary before saving
- **WHEN** the operator edits prompt, tool, budget, permission, or path-restriction fields in the role draft
- **THEN** the workspace shows a live summary of the draft's execution-relevant settings, including prompt intent, tool limits, budget or turn caps, permission mode, and governance cues
- **THEN** the operator can review that summary without leaving the structured editor flow

### Requirement: Role library highlights version and governance differences
The system SHALL present the role library as a distinguishable catalog rather than a name-only list. Each role entry MUST expose the metadata and governance cues needed to compare roles quickly, including version, tags, inheritance markers when present, and visible execution or safety signals when configured.

#### Scenario: Compare roles from the library view
- **WHEN** the operator scans the role library
- **THEN** each role entry shows enough summary information to distinguish role purpose, version, and inheritance state without opening the editor first

#### Scenario: Role has execution or safety constraints
- **WHEN** a role defines budget limits, tool restrictions, review requirements, or path restrictions
- **THEN** the role library surfaces those constraints as visible summary cues instead of hiding them only inside the edit workspace
