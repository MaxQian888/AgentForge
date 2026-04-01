## ADDED Requirements

### Requirement: Role workspace exposes skill compatibility impact before save
The system SHALL surface role-skill compatibility cues throughout the existing authoring flow so operators can understand not only whether a skill resolves from the repository catalog, but also what dependencies it brings in, what tool capabilities it declares, and whether the current role configuration fully covers those needs. These cues MUST remain visible in the Skills section, draft summary, role library, and review context without requiring raw YAML inspection.

#### Scenario: Skills section shows dependency and tool-demand details for a selected skill
- **WHEN** the operator selects or types a skill path in the Skills section and that skill resolves from the authoritative catalog
- **THEN** the workspace shows the skill's direct dependency paths and declared tool requirements alongside its label and provenance cues
- **THEN** any compatibility warning or blocking state for the current role configuration is visible from the same authoring flow

#### Scenario: Summary and library react to compatibility changes
- **WHEN** the operator changes skill rows, toggles `auto_load`, or edits the role's tool configuration in a way that changes skill compatibility
- **THEN** the draft summary and review context update to reflect the current direct-versus-transitive skill impact and blocking versus warning state
- **THEN** the role library can distinguish a fully compatible skill-backed role from a role that still contains compatibility warnings or blockers
