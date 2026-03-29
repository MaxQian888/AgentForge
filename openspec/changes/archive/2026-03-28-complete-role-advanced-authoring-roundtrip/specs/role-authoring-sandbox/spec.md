## ADDED Requirements

### Requirement: Preview and sandbox expose advanced field provenance and persistence context
The system SHALL let preview and sandbox results explain the effective state of advanced role fields in a way that is useful for authoring. For advanced sections such as custom settings, inherited tool-host data, structured knowledge or memory metadata, and overrides, the role authoring flow MUST expose enough context to tell the operator whether the effective value comes from the current draft, an inherited parent, or preserved existing manifest state, and whether that section persists only in canonical YAML or also affects the current execution profile.

#### Scenario: Preview shows effective advanced values for a child role
- **WHEN** the operator previews an unsaved child draft that inherits advanced fields and overrides part of them
- **THEN** the preview result shows the effective advanced values after inheritance and override resolution
- **THEN** the authoring flow can indicate which advanced values are inherited versus explicitly supplied by the draft

#### Scenario: Review context distinguishes persisted role data from runtime projection
- **WHEN** the operator inspects preview or sandbox output for a role that includes memory, collaboration, trigger, or override metadata beyond the current runtime contract
- **THEN** the authoring flow identifies that those sections remain part of the canonical role definition
- **THEN** the operator can see that unsupported runtime-only omissions are projection behavior rather than data loss

### Requirement: Preview and sandbox report advanced authoring validation failures without silent fallback
The system SHALL reject malformed advanced authoring input through preview or sandbox with section-specific validation feedback instead of silently dropping the invalid subsection and continuing with a degraded manifest.

#### Scenario: Sandbox rejects malformed advanced override input
- **WHEN** the operator runs sandbox on a draft whose advanced override block is malformed or semantically incompatible with the current role schema
- **THEN** the system returns a validation issue that points to the override section
- **THEN** the sandbox flow does not proceed as if the invalid override had simply been omitted

#### Scenario: Preview rejects malformed advanced metadata rows
- **WHEN** the operator previews a draft with invalid advanced subsection content such as malformed custom settings entries or inconsistent shared knowledge source metadata
- **THEN** the system returns authoritative validation feedback for the failing subsection
- **THEN** the operator can correct the issue before any save attempt or sandbox probe
