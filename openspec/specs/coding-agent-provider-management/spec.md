# coding-agent-provider-management Specification

## Purpose
Define the project-level coding-agent provider management contract for AgentForge so Claude Code, Codex, and OpenCode share one canonical runtime catalog, one resolved default selection model, and one readiness-diagnostics surface across settings, launch flows, and operator-facing summaries.
## Requirements
### Requirement: Project settings expose one coding-agent runtime catalog
The system SHALL expose one project-scoped coding-agent runtime catalog that covers `claude_code`, `codex`, and `opencode`. The catalog MUST include each runtime's identifier, display metadata, compatible provider identifiers, default model metadata, and the project's resolved default runtime/provider/model selection.

#### Scenario: Settings load with configured defaults
- **WHEN** an authenticated user opens project settings for a project that has explicit coding-agent defaults
- **THEN** the system SHALL return the runtime catalog together with the project's resolved default `runtime`, `provider`, and `model`
- **THEN** the settings UI SHALL render its runtime/provider/model selectors from that catalog instead of hard-coded local options

#### Scenario: Project has no explicit override
- **WHEN** a project has not saved custom coding-agent defaults
- **THEN** the system SHALL still return the runtime catalog with a resolved fallback default selection
- **THEN** downstream launch surfaces SHALL use that resolved fallback instead of inventing their own per-page defaults

### Requirement: Catalog reports runtime readiness and blocking diagnostics
The system SHALL report whether each coding-agent runtime is currently launchable in the active environment, and it MUST include actionable diagnostics when a runtime cannot be launched.

#### Scenario: Command runtime is not installed
- **WHEN** the catalog is generated and the configured `codex` executable cannot be discovered
- **THEN** the `codex` runtime entry SHALL be marked unavailable
- **THEN** the diagnostics SHALL identify that the required command is missing rather than returning a generic failure

#### Scenario: Claude runtime is missing credentials
- **WHEN** the catalog is generated and `claude_code` lacks a valid credential source
- **THEN** the `claude_code` runtime entry SHALL be marked unavailable
- **THEN** the diagnostics SHALL identify the missing credential requirement before the user attempts to start a run

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
