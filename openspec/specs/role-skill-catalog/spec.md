# role-skill-catalog Specification

## Purpose
Define the authoritative repo-local skill catalog used by role authoring so operators can discover supported skill references from the current repository checkout without depending on machine-specific global skill installs.
## Requirements
### Requirement: Role authoring can browse an authoritative repo-local skill catalog
The system SHALL discover role-authoring skills from canonical repo-owned skill roots and expose them as normalized catalog entries using the same relative path syntax that role manifests use in `capabilities.skills`. The catalog MUST be authoritative for the current repository checkout and MUST NOT depend on the operator's machine-specific global skill installation state.

#### Scenario: Catalog returns repo-local skills using role-compatible paths
- **WHEN** the repository contains canonical skill packages such as `skills/react/SKILL.md` and `skills/testing/SKILL.md`
- **THEN** the catalog returns normalized entries whose selectable paths are `skills/react` and `skills/testing`
- **THEN** each entry identifies the repo-local source root that produced that path so authoring surfaces do not need hard-coded skill lists

#### Scenario: Empty skill root does not break role authoring
- **WHEN** the canonical repo-local skill root is missing or contains no valid skill packages
- **THEN** the catalog returns an empty result instead of failing the role authoring flow
- **THEN** operators can still continue with manual skill-path authoring while the UI makes clear that no catalog-backed skills were discovered

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

