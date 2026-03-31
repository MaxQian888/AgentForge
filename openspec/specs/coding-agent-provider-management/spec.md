# coding-agent-provider-management Specification

## Purpose
Define the project-level coding-agent provider management contract for AgentForge so Claude Code, Codex, and OpenCode share one canonical runtime catalog, one resolved default selection model, and one readiness-diagnostics surface across settings, launch flows, and operator-facing summaries.
## Requirements
### Requirement: Project settings expose one coding-agent runtime catalog
The system SHALL expose one project-scoped coding-agent runtime catalog within the unified project settings response so `claude_code`, `codex`, `opencode`, `cursor`, `gemini`, `qoder`, and `iflow` share one canonical runtime catalog across the expanded settings control plane, launch flows, and operator-facing summaries. The catalog MUST include each runtime's identifier, display metadata, compatible provider identifiers, default selection metadata, suggested model options or fixed-model hints when the backend exposes them, supported feature flags, and the project's resolved default runtime/provider/model selection.

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

#### Scenario: Backend-specific selector metadata is returned with the catalog
- **WHEN** the project settings response includes a runtime whose provider or model choices are fixed, bounded, or capability-gated by that backend profile
- **THEN** the runtime catalog entry SHALL surface the corresponding selection metadata and supported feature flags
- **THEN** settings and launch surfaces SHALL NOT invent provider or model options that are absent from that entry

### Requirement: Catalog reports runtime readiness and blocking diagnostics
The system SHALL report whether each coding-agent runtime is currently launchable in the active environment, and it MUST include actionable diagnostics when a runtime cannot be launched. Diagnostics MUST cover backend-specific prerequisites such as missing executables, missing login or API-key state, incompatible provider profiles, and unsupported model selections for the additional CLI-backed runtimes as well as the existing Claude, Codex, and OpenCode runtimes.

#### Scenario: CLI-backed runtime is not installed
- **WHEN** the catalog is generated and the configured `cursor` executable cannot be discovered
- **THEN** the `cursor` runtime entry SHALL be marked unavailable
- **THEN** the diagnostics SHALL identify the missing command or install prerequisite rather than returning a generic failure

#### Scenario: Backend authentication profile is incomplete
- **WHEN** the catalog is generated and `gemini` lacks a supported login or API-key configuration
- **THEN** the `gemini` runtime entry SHALL be marked unavailable
- **THEN** the diagnostics SHALL identify the missing authentication requirement before the user attempts to start a run

#### Scenario: Backend provider or model profile is incompatible
- **WHEN** the catalog is generated for a runtime such as `iflow` whose configured provider profile or resolved model does not satisfy that backend's supported combinations
- **THEN** the runtime entry SHALL be marked unavailable
- **THEN** the diagnostics SHALL identify the incompatible provider or model prerequisite before launch

### Requirement: Agent launch surfaces preserve resolved runtime identity
The system SHALL preserve the resolved `runtime`, `provider`, and `model` for every agent launch that originates from project settings defaults or an explicit user selection, and it MUST return that resolved identity in the launch response surfaces consumed by the frontend.

#### Scenario: Single-agent launch uses project defaults
- **WHEN** a user starts an agent run without overriding runtime selection and the project default resolves to `claude_code`
- **THEN** the backend SHALL persist the resolved `runtime`, `provider`, and `model` for that run
- **THEN** the created run summary returned to the frontend SHALL include the same resolved `runtime`, `provider`, and `model`

#### Scenario: Explicit runtime selection overrides the project default
- **WHEN** a user starts an agent run with an explicit `runtime`, `provider`, and `model` that differ from the project default
- **THEN** the backend SHALL preserve the explicit resolved selection for that run instead of rewriting it to the project default
- **THEN** any subsequent run detail or summary response SHALL identify the same resolved runtime identity

### Requirement: Selection surfaces honor backend-specific choice constraints
The system SHALL use runtime catalog metadata to decide whether provider or model choices are fixed, suggested, or user-editable for each backend profile. A settings or launch surface MUST NOT offer provider or model combinations that the selected runtime entry does not advertise.

#### Scenario: Fixed-provider runtime does not offer extra provider choices
- **WHEN** a user selects a runtime whose catalog entry exposes only one compatible provider
- **THEN** the provider selection surface SHALL keep that provider fixed or collapsed to the single supported value
- **THEN** the user SHALL NOT be able to submit a different provider for that runtime

#### Scenario: Suggested model list constrains selection
- **WHEN** a user selects a runtime whose catalog entry exposes a bounded set of suggested model options without custom-model support
- **THEN** the model selector SHALL offer only those catalog-provided model options
- **THEN** the launch flow SHALL reject values outside that list before sending the request to the bridge

