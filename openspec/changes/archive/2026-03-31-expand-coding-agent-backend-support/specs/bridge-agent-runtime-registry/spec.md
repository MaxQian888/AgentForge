## MODIFIED Requirements

### Requirement: Bridge resolves coding-agent execution through one runtime registry
The TypeScript bridge SHALL maintain one canonical runtime registry for coding-agent execution, and that registry MUST define the supported runtime keys, default runtime selection, adapter family, and runtime metadata needed to launch Claude Code, Codex, OpenCode, Cursor Agent, Gemini CLI, Qoder CLI, and iFlow CLI behind the `/bridge/execute` endpoint.

#### Scenario: Execute request omits runtime
- **WHEN** the bridge receives a valid execute request without an explicit `runtime`
- **THEN** it SHALL resolve the runtime from the configured default in the runtime registry
- **THEN** downstream execution code SHALL consume the resolved runtime metadata instead of assuming a Claude-only path

#### Scenario: Execute request targets an additional supported runtime
- **WHEN** the bridge receives an execute request with `runtime` set to `cursor`
- **THEN** it SHALL resolve the Cursor runtime adapter profile from the registry before execution starts
- **THEN** the bridge SHALL use the same canonical execute endpoint and response shape as every other supported runtime

### Requirement: Bridge validates runtime availability before acquiring execution state
The TypeScript bridge SHALL validate requested runtime keys and runtime-specific launch prerequisites before it acquires an active runtime from the pool, and it MUST reject requests with explicit errors when the runtime is unknown, disabled, or missing required integration prerequisites. For additional CLI-backed runtimes, launch prerequisites MUST validate executable discovery, authentication or login state, provider-profile setup, and model compatibility according to the selected runtime profile.

#### Scenario: Request targets an unknown runtime
- **WHEN** the bridge receives an execute request with a `runtime` value that is not registered
- **THEN** it SHALL reject the request with a validation or configuration error that identifies the unknown runtime
- **THEN** it SHALL NOT acquire a runtime entry for that task

#### Scenario: CLI-backed runtime prerequisites are incomplete
- **WHEN** the bridge resolves `gemini` for execution but the configured executable, authentication profile, or requested provider or model combination is incomplete for that runtime
- **THEN** it SHALL reject the request with an explicit runtime-configuration error that identifies the missing prerequisite
- **THEN** it SHALL NOT start execution or emit misleading running-state events

### Requirement: Runtime registry publishes catalog metadata for upstream consumers
The TypeScript bridge SHALL expose runtime-registry metadata for all supported coding-agent runtimes that upstream services can use to build runtime catalogs without duplicating bridge-specific compatibility rules. That metadata MUST include each runtime's key, default selection metadata, compatible providers, supported feature flags, and any suggested model options that are safe to present upstream.

#### Scenario: Upstream requests runtime catalog metadata
- **WHEN** the backend or an equivalent upstream consumer asks the bridge for coding-agent runtime metadata
- **THEN** the bridge SHALL return one entry per supported runtime with its runtime key, default selection metadata, compatible provider identifiers, and supported feature flags
- **THEN** the upstream consumer SHALL NOT need to hard-code Claude Code, Codex, OpenCode, Cursor, Gemini, Qoder, or iFlow compatibility tables separately from the bridge

#### Scenario: Runtime catalog identifies the bridge default
- **WHEN** the bridge publishes runtime-registry metadata
- **THEN** the metadata SHALL identify which runtime is currently configured as the bridge default
- **THEN** upstream consumers SHALL be able to distinguish the bridge default from merely supported runtimes

#### Scenario: Catalog returns bounded model options when available
- **WHEN** a runtime profile exposes a bounded set of safe model choices for upstream selection
- **THEN** the catalog entry SHALL include those suggested model options together with the runtime's default model metadata
- **THEN** upstream consumers SHALL be able to render model selection without probing backend-specific CLIs themselves

### Requirement: Runtime registry surfaces availability diagnostics without starting execution
The TypeScript bridge SHALL evaluate runtime readiness for catalog consumers without acquiring execution state, and it MUST surface actionable diagnostics when a runtime cannot currently start. For additional CLI-backed runtimes, diagnostics MUST describe whether the blocking issue is command discovery, login or API-key state, provider-profile setup, or model compatibility.

#### Scenario: CLI runtime executable is unavailable during diagnostics
- **WHEN** the registry evaluates `qoder` readiness and the required Qoder executable cannot be discovered
- **THEN** the diagnostics result SHALL mark `qoder` as unavailable
- **THEN** the reported reason SHALL identify the missing executable or install prerequisite without creating a runtime entry or emitting running-state events

#### Scenario: CLI runtime authentication is unavailable during diagnostics
- **WHEN** the registry evaluates `iflow` readiness and no supported authentication or provider profile is configured for the selected backend mode
- **THEN** the diagnostics result SHALL mark `iflow` as unavailable
- **THEN** the reported reason SHALL identify the missing authentication or provider-profile prerequisite before any execute request is attempted

### Requirement: Frontend uses runtime catalog for agent spawn configuration
Frontend agent store SHALL fetch and cache the runtime catalog from `GET /api/v1/bridge/runtimes`. The `RuntimeSelector` component SHALL use this data to populate runtime, provider, and model controls, and it MUST honor backend-specific selection limits and supported feature hints from the catalog instead of assuming that every runtime exposes the same editable fields.

#### Scenario: Agent store loads catalog on first access
- **WHEN** `RuntimeSelector` renders and catalog is not yet loaded
- **THEN** agent store fetches catalog from API and populates the selector options

#### Scenario: Catalog shows runtime diagnostics
- **WHEN** a runtime has `available: false` with blocking diagnostics
- **THEN** RuntimeSelector shows runtime as disabled with a visible diagnostic hint explaining why that runtime cannot start

#### Scenario: Fixed or bounded runtime selections remain constrained
- **WHEN** the selected runtime exposes only one provider or a bounded model set in the catalog
- **THEN** `RuntimeSelector` SHALL constrain the provider and model controls to those catalog values
- **THEN** the component SHALL NOT emit a launch tuple that violates the selected runtime entry's advertised constraints
