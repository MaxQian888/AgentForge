# marketplace-item-consumption Specification

## Purpose
Define how AgentForge represents marketplace install and consumption state across plugin, role, and skill assets, including typed provenance, downstream handoff, and canonical repo-local materialization rules.
## Requirements
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

### Requirement: Marketplace-installed plugins remain manageable in the plugin control plane
Marketplace-installed plugins SHALL enter the existing plugin control plane using marketplace provenance instead of masquerading as anonymous local installs. The plugin control plane MUST preserve the marketplace item identity, selected version, and source metadata needed for operators to manage, inspect, and update the installed plugin.

#### Scenario: Installed marketplace plugin appears in the plugin panel
- **WHEN** a marketplace plugin install completes successfully
- **THEN** the plugin appears in the installed section of the plugin management panel with `marketplace` provenance and the selected version
- **THEN** operators can continue lifecycle management from the plugin panel without losing the source metadata established by the marketplace install

#### Scenario: Plugin detail links back to the originating marketplace item
- **WHEN** the operator opens the detail view for a plugin that originated from the standalone marketplace workspace
- **THEN** the plugin detail shows the originating marketplace item identity and source metadata
- **THEN** the operator can navigate back to the marketplace item or management workspace from the plugin panel

### Requirement: Marketplace-installed roles and skills become discoverable in role authoring surfaces
Marketplace-installed role and skill assets SHALL be materialized into the repository's authoritative role and skill discovery seams so they can be used by existing role authoring workflows. A role install MUST become discoverable through the role listing surface, and a skill install MUST become discoverable through the authoritative role skill catalog surface.

#### Scenario: Installed marketplace role appears in the roles workspace
- **WHEN** a marketplace role install completes successfully
- **THEN** the role is discoverable through the existing roles API and visible in the roles workspace catalog
- **THEN** operators can open, clone, or use that role through the same workflows as other repository-backed roles

#### Scenario: Installed marketplace skill appears in the role skill catalog
- **WHEN** a marketplace skill install completes successfully
- **THEN** the skill is discoverable through the authoritative role skill catalog contract used by role authoring
- **THEN** operators can add that skill to a role without manual path guessing or hidden filesystem steps

### Requirement: Role and skill marketplace artifacts use canonical repo-local package layouts
The system SHALL treat role and skill marketplace artifacts as canonical package archives rather than arbitrary raw files. A role marketplace artifact MUST be a zip archive whose package root contains `role.yaml` and can be materialized into `roles/<role-id>/role.yaml`. A skill marketplace artifact MUST be a zip archive whose package root contains `SKILL.md` and can be materialized into `skills/<skill-id>/SKILL.md` plus any companion package files. The install flow MUST reject artifacts that do not satisfy these canonical repo-local layouts.

#### Scenario: Role artifact extracts into the canonical role directory
- **WHEN** an operator installs a marketplace role whose artifact is a valid zip archive with `role.yaml` at the package root
- **THEN** the installer materializes the package into `roles/<role-id>/`
- **THEN** the roles API discovers the role through `roles/<role-id>/role.yaml` without requiring a second normalization pass outside the existing role store

#### Scenario: Skill artifact extracts into the canonical skill directory
- **WHEN** an operator installs a marketplace skill whose artifact is a valid zip archive with `SKILL.md` at the package root
- **THEN** the installer materializes the package into `skills/<skill-id>/`
- **THEN** the authoritative role skill catalog discovers the installed skill through `skills/<skill-id>/SKILL.md` and its companion package files

#### Scenario: Invalid role or skill artifact is rejected before discovery state changes
- **WHEN** a role or skill marketplace artifact is not a zip archive or does not contain the required root file for its type
- **THEN** the install flow returns a stable invalid-artifact failure category tied to that item version
- **THEN** no partial role or skill discovery state is created in the downstream consumer surfaces

### Requirement: Post-install handoff keeps marketplace assets actionable
The marketplace workspace SHALL expose post-install handoff actions for each supported item type so operators can use installed marketplace assets in their native product surfaces. Installed assets MUST remain actionable even when the operator chooses not to stay on the marketplace page.

#### Scenario: Marketplace workspace offers downstream entry points after install
- **WHEN** an item finishes installing successfully
- **THEN** the marketplace workspace shows the relevant downstream action such as open plugin console, open roles workspace, or open role skill authoring
- **THEN** the operator can continue using the item from its native consumer surface without searching for it manually

#### Scenario: Installed but not yet used asset remains distinguishable
- **WHEN** an item has been installed but has not yet been opened or managed through its downstream consumer surface
- **THEN** the marketplace workspace shows it as installed with the next available handoff action
- **THEN** the workspace does not falsely report that the asset is already in active use
