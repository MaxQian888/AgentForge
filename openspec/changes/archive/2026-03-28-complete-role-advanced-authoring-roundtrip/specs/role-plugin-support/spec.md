## ADDED Requirements

### Requirement: Role APIs preserve advanced authoring fields across partial edits
The system SHALL preserve advanced YAML-backed role sections across list, get, create, update, preview, and sandbox flows even when the current authoring surface edits only a subset of those fields. When an existing role already contains advanced sections such as `capabilities.custom_settings`, structured tool-host metadata, `knowledge.memory`, detailed shared knowledge sources, or `overrides`, a subsequent update that changes other supported fields MUST NOT silently drop or flatten the untouched advanced sections.

#### Scenario: Updating basic fields preserves untouched advanced sections
- **WHEN** an operator opens a role that already contains advanced custom settings, memory metadata, and overrides
- **AND** the operator edits only basic metadata or identity fields before saving
- **THEN** the saved canonical YAML still contains the untouched advanced sections in semantically equivalent form
- **THEN** a subsequent get, preview, or list operation returns those advanced sections instead of omitting them

#### Scenario: Previewing an unsaved draft preserves untouched advanced sections
- **WHEN** an operator previews or sandboxes an unsaved draft derived from an existing advanced role
- **AND** the draft does not explicitly change some advanced sections
- **THEN** the preview pipeline carries those untouched sections forward into the normalized or effective manifest
- **THEN** the authoring helper does not treat missing UI controls as an instruction to remove stored advanced data

### Requirement: Role APIs define safe editing boundaries for open-ended advanced fields
The system SHALL distinguish between advanced fields that are fully editable, fields that remain preserved but read-only in the current authoring surface, and fields that require a controlled raw editing surface such as YAML-oriented override input. API behavior, validation errors, and normalized responses MUST make these boundaries explicit so operators can understand whether a value was edited, preserved, or rejected.

#### Scenario: Controlled override edit is validated authoritatively
- **WHEN** an operator submits a role update that modifies `overrides` through the supported advanced authoring flow
- **THEN** the system validates that override payload using the authoritative role parser or preview path
- **THEN** invalid override structure is rejected with a role-section-specific validation error instead of being silently ignored or coerced

#### Scenario: Preserved read-only advanced data remains visible after save
- **WHEN** a role contains advanced data that is not directly editable through the current structured controls
- **AND** the operator saves other supported changes successfully
- **THEN** the API returns that preserved advanced data in normalized role reads after the save
- **THEN** the system does not rewrite the canonical YAML in a way that erases the preserved section
