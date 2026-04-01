## MODIFIED Requirements

### Requirement: Marketplace installs are normalized across item types and consumer surfaces
The system SHALL represent marketplace installation and consumption state through a typed contract that covers item identity, item type, selected version when applicable, source provenance, install or discoverability status, warning or failure reason, consumer surface, and downstream record identity or local path when available. The contract MUST support plugin, skill, and role marketplace items plus repo-owned built-in skill records without collapsing them into one generic installed-items list.

#### Scenario: Successful install returns typed consumption metadata
- **WHEN** an operator installs a marketplace item successfully
- **THEN** the system returns or exposes the installed item type, selected version, downstream consumer surface, and resulting local record or path identity
- **THEN** the marketplace workspace can render the item as installed without guessing from a string-only item id list

#### Scenario: Built-in skill surfaces as a locally discoverable consumption record
- **WHEN** the current checkout includes an official built-in skill bundle entry that resolves to `skills/<id>/SKILL.md`
- **THEN** the consumption contract returns a skill record with built-in provenance, `role-skill-catalog` as the consumer surface, and the local skill path identity
- **THEN** the marketplace workspace can render that built-in skill as already available locally without requiring a prior marketplace install action

#### Scenario: Failed install remains explicit and side-effect free
- **WHEN** an install attempt fails because of download, validation, extraction, trust, or downstream consumer errors
- **THEN** the system reports a stable failure reason tied to the affected item and version
- **THEN** it MUST NOT mark the item as installed or consumed in downstream surfaces unless the handoff completed successfully
