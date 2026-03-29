## ADDED Requirements

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
The system SHALL expose enough metadata for each discovered skill to support authoring selection and explanation, including a stable role-compatible path plus a human-usable label. Optional metadata such as a short description MAY come from skill package metadata, but missing optional fields MUST NOT prevent the skill from being discoverable.

#### Scenario: Catalog entry uses skill metadata when available
- **WHEN** a discovered skill package provides a readable title or summary in its skill metadata
- **THEN** the catalog entry exposes that title or summary alongside the canonical skill path
- **THEN** role authoring surfaces can present a meaningful picker instead of only a raw filesystem path

#### Scenario: Missing optional metadata falls back to path-based labeling
- **WHEN** a discovered skill package lacks optional display metadata but still resolves to a valid skill path
- **THEN** the catalog still returns that skill as a selectable entry
- **THEN** the entry falls back to a path-derived label instead of being silently dropped from discovery
