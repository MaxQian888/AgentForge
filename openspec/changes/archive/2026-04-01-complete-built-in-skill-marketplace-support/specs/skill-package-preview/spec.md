## ADDED Requirements

### Requirement: Skill package preview exposes structured markdown and yaml detail
The system SHALL expose a structured skill package preview contract that can be used by both repo-owned built-in skills and standalone marketplace skill items. A skill preview MUST include rendered-source-ready Markdown content from `SKILL.md`, normalized YAML text for parsed frontmatter, normalized YAML text for supported agent config files, dependency metadata, declared tool requirements, and package-part inventory so the frontend can render a truthful skill detail surface without reverse-engineering raw archives.

#### Scenario: Repo-owned built-in skill returns a local package preview
- **WHEN** an operator opens the detail view for an official built-in skill backed by `skills/<id>/SKILL.md`
- **THEN** the system returns the skill package preview with the parsed Markdown body, frontmatter YAML, supported agent YAML, and package metadata for that local skill package
- **THEN** the marketplace workspace can render the skill detail without downloading or parsing repo-local files in the browser

#### Scenario: Standalone marketplace skill returns an artifact-backed preview
- **WHEN** an operator opens the detail view for a marketplace skill item whose uploaded artifact satisfies the canonical skill package layout
- **THEN** the system returns the same skill package preview shape derived from that artifact
- **THEN** the frontend can render marketplace-published skills and built-in skills with the same detail model despite their different provenance

### Requirement: Skill package preview failures remain explicit and non-destructive
The system SHALL treat preview extraction failures as explicit preview-state errors rather than silently collapsing skill detail back to generic description-only content. A preview failure MUST NOT mutate install state, discovery state, or bundle membership state for the affected skill package.

#### Scenario: Invalid preview source stays explicit
- **WHEN** a built-in or marketplace skill package cannot produce a valid preview because `SKILL.md` or a supported agent YAML file is unreadable or malformed
- **THEN** the system returns an explicit preview-unavailable state with a stable reason for that skill item
- **THEN** the marketplace workspace shows that preview state instead of pretending the skill has no package detail

#### Scenario: Preview extraction does not change installability or provenance
- **WHEN** preview extraction fails for a skill item that is otherwise still installed or discoverable
- **THEN** the existing install or discoverability record remains unchanged
- **THEN** operators can distinguish a preview problem from a marketplace install or role-skill-catalog failure
