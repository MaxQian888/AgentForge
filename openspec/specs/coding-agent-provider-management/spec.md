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

### Requirement: Project catalog preserves CLI runtime lifecycle and contract diagnostics
The system SHALL include Bridge-published CLI launch-contract and lifecycle metadata inside the project-scoped coding-agent catalog returned to settings and launch surfaces. For CLI-backed runtimes, the project catalog MUST preserve degraded or unavailable reason codes, install or auth guidance, and any sunset or migration metadata instead of flattening them into a generic availability flag.

#### Scenario: Settings render a degraded CLI runtime with migration guidance
- **WHEN** project settings load a catalog containing degraded `iflow`
- **THEN** the response SHALL include the Bridge-provided sunset and migration guidance
- **THEN** the UI SHALL be able to render a warning state instead of treating the runtime as a normal selectable default

#### Scenario: Catalog preserves runtime-specific contract hints
- **WHEN** the catalog includes `cursor` or `qoder`
- **THEN** the project-scoped catalog SHALL preserve their runtime-specific launch-contract diagnostics and capability hints
- **THEN** frontend selectors SHALL NOT invent support assumptions beyond the Bridge contract

### Requirement: Launch surfaces block deprecated or contract-invalid CLI selections
Settings, single-agent launch flows, and team launch flows SHALL prevent submission of a CLI runtime selection when the project catalog marks the selected runtime as unavailable because of launch-contract mismatch, missing official auth or config prerequisites, or published sunset state. When the runtime is degraded but still launchable, the surfaces SHALL present the same warning reason before submission and preserve that resolved runtime identity if the user proceeds.

#### Scenario: Unavailable CLI runtime cannot be submitted
- **WHEN** a user selects `qoder` or `iflow` and the project catalog marks it unavailable
- **THEN** the launch surface SHALL block submission and show the runtime-specific diagnostic
- **THEN** the backend SHALL NOT receive a launch tuple for that runtime

#### Scenario: Degraded runtime warning is shown consistently
- **WHEN** a runtime remains launchable but the catalog marks it degraded because of deprecation or headless-contract caveats
- **THEN** settings and launch surfaces SHALL show the same warning reason before launch
- **THEN** any launched run SHALL still preserve the selected runtime identity rather than silently rewriting it

### Requirement: Resolved defaults skip unavailable CLI runtimes
The system SHALL resolve project-level coding-agent defaults only from runtimes that the current Bridge catalog marks launchable. If saved defaults point to a CLI runtime that is unavailable because of missing contract prerequisites or sunset state, the backend MUST surface a diagnostic and fall back to the next supported default instead of auto-launching the stale runtime.

#### Scenario: Stale iFlow default is replaced at read time
- **WHEN** project settings still persist `iflow` as the default runtime after `iflow` is marked unavailable by sunset rules
- **THEN** the returned project catalog SHALL include a diagnostic about the stale selection
- **THEN** new launch surfaces SHALL use the bridge default or another supported fallback instead of blindly using `iflow`

