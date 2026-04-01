## MODIFIED Requirements

### Requirement: Skill catalog entries expose usable authoring metadata
The system SHALL expose enough metadata for each discovered skill to support authoring selection, compatibility explanation, and review, including a stable role-compatible path, a human-usable label, direct dependency paths, and declared tool requirements when present. Optional metadata such as a short description MAY come from skill package metadata, but missing optional display or compatibility fields MUST NOT prevent the skill from being discoverable.

#### Scenario: Catalog entry uses skill metadata when available
- **WHEN** a discovered skill package provides a readable title or summary in its skill metadata
- **THEN** the catalog entry exposes that title or summary alongside the canonical skill path
- **THEN** the same entry also includes any direct `requires` paths and declared `tools` so role authoring can explain the skill's compatibility contract without reading raw `SKILL.md`

#### Scenario: Catalog normalizes direct dependency paths for authoring
- **WHEN** a discovered skill package declares one or more direct dependencies in frontmatter
- **THEN** the catalog entry returns those dependencies using the same normalized `skills/<name>` path syntax that role manifests use
- **THEN** authoring surfaces can compare the direct dependency set with the role's configured skill rows without inventing their own normalization rules

#### Scenario: Missing optional metadata falls back without hiding the skill
- **WHEN** a discovered skill package lacks optional display metadata or omits compatibility metadata such as `requires` or `tools`
- **THEN** the catalog still returns that skill as a selectable entry
- **THEN** path-derived labels are used where needed and missing compatibility metadata is represented as empty authoring data instead of causing the skill to disappear from discovery
