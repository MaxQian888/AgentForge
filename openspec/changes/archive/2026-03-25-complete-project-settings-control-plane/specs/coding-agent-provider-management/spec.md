## MODIFIED Requirements

### Requirement: Project settings expose one coding-agent runtime catalog
The system SHALL expose one project-scoped coding-agent runtime catalog within the unified project settings response so `claude_code`, `codex`, and `opencode` share one canonical runtime catalog across the expanded settings control plane, launch flows, and operator-facing summaries. The catalog MUST include each runtime's identifier, display metadata, compatible provider identifiers, default model metadata, and the project's resolved default runtime/provider/model selection.

#### Scenario: Settings load with configured defaults
- **WHEN** an authenticated user opens project settings for a project that has explicit coding-agent defaults
- **THEN** the system SHALL return the runtime catalog together with the broader project settings document and the project's resolved default `runtime`, `provider`, and `model`
- **THEN** the settings UI SHALL render its runtime/provider/model selectors from that catalog inside the coding-agent section instead of hard-coded local options

#### Scenario: Project has no explicit override
- **WHEN** a project has not saved custom coding-agent defaults
- **THEN** the system SHALL still return the runtime catalog with a resolved fallback default selection
- **THEN** downstream launch surfaces SHALL use that resolved fallback instead of inventing their own per-page defaults

#### Scenario: Unified settings save preserves coding-agent selection
- **WHEN** an operator saves unrelated project governance settings without changing the coding-agent selection
- **THEN** the persisted coding-agent `runtime`, `provider`, and `model` remain unchanged
- **THEN** later settings reads and launch flows continue to expose the same resolved coding-agent defaults
