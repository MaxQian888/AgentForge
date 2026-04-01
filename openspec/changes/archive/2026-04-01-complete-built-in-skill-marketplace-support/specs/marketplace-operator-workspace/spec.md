## MODIFIED Requirements

### Requirement: The standalone marketplace workspace renders truthful browse, detail, and status states
The system SHALL provide a standalone `/marketplace` operator workspace for plugin, skill, and role marketplace items plus the official repo-owned built-in skill surface. The workspace MUST render list, built-in section, detail, filter, empty, loading, unavailable, blocked, and actionable states from real marketplace, built-in bundle or preview, and consumption contracts instead of silently swallowing errors or inferring installability from incomplete local state.

#### Scenario: Operator browses marketplace items with truthful states
- **WHEN** the operator opens `/marketplace`
- **THEN** the workspace loads standalone marketplace items, featured items, official built-in skill entries, and typed install or consumption state from the configured services
- **THEN** each selected item shows its type, provenance, version availability, verification state, installability or local availability, and downstream consumer status without requiring the operator to infer them from generic badges

#### Scenario: Remote marketplace service is unavailable but built-in skills remain browseable
- **WHEN** the configured standalone marketplace service is unreachable, misconfigured, or returns an invalid response while the built-in skill feed remains available
- **THEN** the workspace shows an explicit partial-unavailable state with a stable reason for the remote marketplace failure
- **THEN** the built-in skill section remains visible instead of falling back to an empty result set that implies no marketplace-relevant skills exist

#### Scenario: Skill detail renders structured package preview
- **WHEN** the operator opens the detail view for a skill item that provides a skill package preview
- **THEN** the workspace shows the skill provenance together with rendered Markdown detail and YAML panels for the skill frontmatter and supported agent configs
- **THEN** the skill detail MUST NOT collapse that package to generic description text alone when structured preview data is available

#### Scenario: Item detail distinguishes installable, locally available, installed, and used states
- **WHEN** the operator opens the detail view for an item
- **THEN** the workspace shows whether the item is installable, already installed, or already discoverable in its downstream consumer surface
- **THEN** the workspace exposes the next supported action such as install, manage, or open the downstream consumer surface
