## ADDED Requirements

### Requirement: Runtime registry publishes catalog metadata for upstream consumers
The TypeScript bridge SHALL expose runtime-registry metadata for `claude_code`, `codex`, and `opencode` that upstream services can use to build runtime catalogs without duplicating bridge-specific compatibility rules.

#### Scenario: Upstream requests runtime catalog metadata
- **WHEN** the backend or an equivalent upstream consumer asks the bridge for coding-agent runtime metadata
- **THEN** the bridge SHALL return one entry per supported runtime with its runtime key, default model metadata, and compatible provider identifiers
- **THEN** the upstream consumer SHALL NOT need to hard-code Claude Code, Codex, or OpenCode compatibility tables separately from the bridge

#### Scenario: Runtime catalog identifies the bridge default
- **WHEN** the bridge publishes runtime-registry metadata
- **THEN** the metadata SHALL identify which runtime is currently configured as the bridge default
- **THEN** upstream consumers SHALL be able to distinguish the bridge default from merely supported runtimes

### Requirement: Runtime registry surfaces availability diagnostics without starting execution
The TypeScript bridge SHALL evaluate runtime readiness for catalog consumers without acquiring execution state, and it MUST surface actionable diagnostics when a runtime cannot currently start.

#### Scenario: OpenCode command is unavailable during diagnostics
- **WHEN** the registry evaluates `opencode` readiness and the configured executable cannot be resolved
- **THEN** the diagnostics result SHALL mark `opencode` as unavailable
- **THEN** the bridge SHALL return the missing-command reason without creating a runtime entry or emitting running-state events

#### Scenario: Claude credentials are unavailable during diagnostics
- **WHEN** the registry evaluates `claude_code` readiness and no valid credential source is configured
- **THEN** the diagnostics result SHALL mark `claude_code` as unavailable
- **THEN** the reported reason SHALL identify the credential requirement that is missing before any execute request is attempted
