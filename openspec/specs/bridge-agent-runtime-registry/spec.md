# bridge-agent-runtime-registry Specification

## Purpose
Define the canonical TypeScript bridge runtime registry for coding-agent execution, including supported runtime keys, default runtime selection, centralized runtime availability validation, and normalization expectations for Claude Code, Codex, and OpenCode behind one `/bridge/execute` surface.
## Requirements
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

### Requirement: Runtime adapters normalize native execution into the canonical bridge event model
The TypeScript bridge SHALL require every registered coding runtime adapter to translate its native execution stream into the canonical AgentForge runtime lifecycle contract, including normalized status, output, tool activity, cost, error, cancellation, and snapshot semantics.

#### Scenario: Non-Claude runtime emits native execution output
- **WHEN** a Codex or OpenCode adapter emits backend-native progress or tool activity during execution
- **THEN** the bridge SHALL normalize that activity into the existing `AgentEvent` categories consumed by Go
- **THEN** the Go orchestrator SHALL NOT need runtime-specific event parsing to observe the run

#### Scenario: Runtime adapter cannot support a canonical signal
- **WHEN** a registered runtime lacks a native equivalent for a canonical signal such as cost or tool-call detail
- **THEN** the adapter SHALL provide the closest truthful normalized data permitted by the bridge contract
- **THEN** the bridge SHALL preserve consistent lifecycle completion and error semantics instead of omitting required terminal behavior

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

### Requirement: Codex resolves through a dedicated bridge-owned adapter
The TypeScript bridge SHALL resolve `runtime=codex` through a dedicated Codex adapter owned by the bridge, and it MUST NOT treat the bare external `codex` executable as if it natively implemented the bridge's command-runtime JSONL contract.

#### Scenario: Execute request targets Codex
- **WHEN** the bridge receives an execute request whose resolved runtime is `codex`
- **THEN** the runtime registry SHALL return the dedicated Codex adapter for execution
- **THEN** the bridge SHALL keep using the same canonical `/bridge/execute` surface instead of requiring Go to call a Codex-specific route

#### Scenario: Legacy raw command assumption is not treated as readiness
- **WHEN** the registry can discover a `codex` executable but the bridge-owned Codex connector contract is not configured or supported
- **THEN** the registry SHALL NOT mark `codex` as ready solely because the command exists
- **THEN** the returned diagnostics SHALL identify the missing connector requirement before execution starts

### Requirement: Codex diagnostics validate connector and authentication prerequisites
The TypeScript bridge SHALL evaluate Codex readiness against the full connector contract, including supported authentication state and any required local prerequisites, and it MUST surface actionable blocking diagnostics without acquiring execution state.

#### Scenario: Codex authentication is unavailable during diagnostics
- **WHEN** the registry evaluates `codex` readiness and no supported Codex authentication source is configured
- **THEN** the diagnostics result SHALL mark `codex` as unavailable
- **THEN** the reported reason SHALL identify the missing authentication requirement before any execute request is attempted

#### Scenario: Codex connector prerequisites are satisfied
- **WHEN** the registry evaluates `codex` readiness and the dedicated connector plus its required prerequisites are available
- **THEN** the runtime catalog SHALL report `codex` as available with its canonical provider and model metadata
- **THEN** upstream consumers SHALL not need extra Codex-specific readiness checks outside the bridge catalog

### Requirement: Runtime catalog is queryable from Go API layer
Go backend SHALL expose `GET /api/v1/bridge/runtimes` endpoint that returns the Bridge runtime catalog. The endpoint SHALL cache the catalog for 60 seconds to avoid excessive Bridge calls.

#### Scenario: Cached catalog returned
- **WHEN** catalog was fetched 30 seconds ago and client requests again
- **THEN** cached catalog is returned without calling Bridge

#### Scenario: Cache expired
- **WHEN** catalog cache is older than 60 seconds
- **THEN** Go backend calls Bridge `/bridge/runtimes` and refreshes cache

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

