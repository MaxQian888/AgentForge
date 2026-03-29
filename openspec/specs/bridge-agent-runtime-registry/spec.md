# bridge-agent-runtime-registry Specification

## Purpose
Define the canonical TypeScript bridge runtime registry for coding-agent execution, including supported runtime keys, default runtime selection, centralized runtime availability validation, and normalization expectations for Claude Code, Codex, and OpenCode behind one `/bridge/execute` surface.
## Requirements
### Requirement: Bridge resolves coding-agent execution through one runtime registry
The TypeScript bridge SHALL maintain one canonical runtime registry for coding-agent execution, and that registry MUST define the supported runtime keys, default runtime selection, and runtime metadata needed to launch Claude Code, Codex, and OpenCode behind the `/bridge/execute` endpoint.

#### Scenario: Execute request omits runtime
- **WHEN** the bridge receives a valid execute request without an explicit `runtime`
- **THEN** it SHALL resolve the runtime from the configured default in the runtime registry
- **THEN** downstream execution code SHALL consume the resolved runtime metadata instead of assuming a Claude-only path

#### Scenario: Execute request targets a supported runtime
- **WHEN** the bridge receives an execute request with `runtime` set to `codex`
- **THEN** it SHALL resolve the Codex runtime adapter from the registry before execution starts
- **THEN** the bridge SHALL use the same canonical execute endpoint and response shape as every other supported runtime

### Requirement: Bridge validates runtime availability before acquiring execution state
The TypeScript bridge SHALL validate requested runtime keys and runtime-specific launch prerequisites before it acquires an active runtime from the pool, and it MUST reject requests with explicit errors when the runtime is unknown, disabled, or missing required integration prerequisites. For `opencode`, launch prerequisites MUST validate the configured OpenCode automation transport, authentication, and requested provider or model readiness rather than only checking whether a local executable exists.

#### Scenario: Request targets an unknown runtime
- **WHEN** the bridge receives an execute request with a `runtime` value that is not registered
- **THEN** it SHALL reject the request with a validation or configuration error that identifies the unknown runtime
- **THEN** it SHALL NOT acquire a runtime entry for that task

#### Scenario: OpenCode transport prerequisites are incomplete
- **WHEN** the bridge resolves `opencode` for execution but the configured OpenCode server transport is unreachable, authentication fails, or the requested upstream provider/model cannot be resolved
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
The TypeScript bridge SHALL evaluate runtime readiness for catalog consumers without acquiring execution state, and it MUST surface actionable diagnostics when a runtime cannot currently start. For `opencode`, diagnostics MUST describe the official OpenCode transport being used and any blocking issue across server reachability, authentication, provider availability, or model selection.

#### Scenario: OpenCode server is unreachable during diagnostics
- **WHEN** the registry evaluates `opencode` readiness and the configured OpenCode server cannot respond to its health probe
- **THEN** the diagnostics result SHALL mark `opencode` as unavailable
- **THEN** the reported reason SHALL identify the server reachability issue without creating a runtime entry or emitting running-state events

#### Scenario: OpenCode provider or model is unavailable during diagnostics
- **WHEN** the OpenCode server is reachable but the configured provider or default model for `opencode` cannot be resolved upstream
- **THEN** the diagnostics result SHALL mark `opencode` as unavailable
- **THEN** the reported reason SHALL identify the specific provider or model mismatch before any execute request is attempted

#### Scenario: Claude credentials are unavailable during diagnostics
- **WHEN** the registry evaluates `claude_code` readiness and no valid credential source is configured
- **THEN** the diagnostics result SHALL mark `claude_code` as unavailable
- **THEN** the reported reason SHALL identify the credential requirement that is missing before any execute request is attempted

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
Frontend agent store SHALL fetch and cache the runtime catalog from `GET /api/v1/bridge/runtimes`. The `RuntimeSelector` component SHALL use this data to populate runtime, provider, and model dropdowns.

#### Scenario: Agent store loads catalog on first access
- **WHEN** `RuntimeSelector` renders and catalog is not yet loaded
- **THEN** agent store fetches catalog from API and populates the selector options

#### Scenario: Catalog shows runtime diagnostics
- **WHEN** a runtime has `available: false` with diagnostics `["API key not configured"]`
- **THEN** RuntimeSelector shows runtime as disabled with tooltip showing the diagnostic messages

