## MODIFIED Requirements

### Requirement: Skill package preview exposes structured markdown and yaml detail
The system SHALL expose a structured skill package preview contract that can be used by repo-owned built-in skills, repo-assistant skills, workflow-mirror skills, and standalone marketplace skill items. A skill preview MUST include rendered-source-ready Markdown content from `SKILL.md`, normalized YAML text for parsed frontmatter, normalized YAML text for supported agent config files, dependency metadata, declared tool requirements, and package-part inventory so frontend surfaces can render truthful skill detail without reverse-engineering raw archives or local filesystem layouts.

#### Scenario: Repo-owned built-in skill returns a local package preview
- **WHEN** an operator opens the detail view for an official built-in skill backed by `skills/<id>/SKILL.md`
- **THEN** the system returns the skill package preview with the parsed Markdown body, frontmatter YAML, supported agent YAML, and package metadata for that local skill package
- **THEN** the consuming workspace can render the skill detail without downloading or parsing repo-local files in the browser

#### Scenario: Governed repo-assistant or workflow-mirror skill returns a package preview
- **WHEN** an operator opens the detail view for a governed internal skill backed by `.agents/skills/<id>/SKILL.md` or `.codex/skills/<id>/SKILL.md`
- **THEN** the system returns the same skill package preview shape together with the skill's package metadata and supported agent config YAML when present
- **THEN** the consuming workspace can render that governed skill with the same preview model used for built-in and marketplace skills despite the different canonical roots

#### Scenario: Standalone marketplace skill returns an artifact-backed preview
- **WHEN** an operator opens the detail view for a marketplace skill item whose uploaded artifact satisfies the canonical skill package layout
- **THEN** the system returns the same skill package preview shape derived from that artifact
- **THEN** the frontend can render marketplace-published skills and governed local skills with the same detail model despite their different provenance

### Requirement: Skill package preview failures remain explicit and non-destructive
The system SHALL treat preview extraction failures as explicit preview-state errors rather than silently collapsing skill detail back to generic description-only content. A preview failure MUST NOT mutate install state, discovery state, bundle membership state, registry membership state, or mirror/provenance alignment state for the affected skill package.

#### Scenario: Governed internal skill with invalid preview source stays explicit
- **WHEN** a built-in, repo-assistant, or workflow-mirror skill package cannot produce a valid preview because `SKILL.md` or a supported agent YAML file is unreadable or malformed
- **THEN** the system returns an explicit preview-unavailable state with a stable reason for that governed skill
- **THEN** the management workspace shows that preview state instead of pretending the skill has no package detail or has disappeared from governance

#### Scenario: Marketplace skill with invalid preview source stays explicit
- **WHEN** a marketplace skill package cannot produce a valid preview because `SKILL.md` or a supported agent YAML file is unreadable or malformed
- **THEN** the system returns an explicit preview-unavailable state with a stable reason for that skill item
- **THEN** the consuming workspace shows that preview state instead of pretending the skill has no package detail

#### Scenario: Preview extraction does not change installability, discoverability, or governance alignment
- **WHEN** preview extraction fails for a skill item or governed skill that is otherwise still installed, discoverable, or registry-declared
- **THEN** the existing install, discoverability, bundle, and governance records remain unchanged
- **THEN** operators can distinguish a preview problem from a marketplace install, role-skill-catalog, or internal-governance failure
