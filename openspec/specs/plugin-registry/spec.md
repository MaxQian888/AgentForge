# plugin-registry Specification

## Purpose
Define AgentForge's authoritative plugin registry model for installed plugins, including source tracking, operator-visible lifecycle state, filtering, enablement controls, and runtime-state reconciliation from plugin hosts.
## Requirements
### Requirement: Registry stores the authoritative plugin record
The system SHALL maintain a registry record for each installed plugin as the authoritative source of truth for plugin metadata and operator-visible state. Each record MUST include plugin identifier, plugin kind, version, install source, declared runtime, declared permissions, current lifecycle state, and manifest payload or its normalized equivalent. For plugins that declare `runtime: wasm`, the record MUST also preserve the resolved module source and SDK ABI compatibility metadata needed to audit which runtime artifact is expected to execute. For `ToolPlugin` instances hosted by the TS bridge, the record MUST additionally preserve the latest synchronized MCP interaction snapshot summary, including capability counts, discovery freshness, and the latest interaction diagnostic summary needed by operators.

#### Scenario: New plugin registration creates a registry record
- **WHEN** a plugin is installed from a supported source
- **THEN** the platform creates a registry record containing its metadata, source, runtime declaration, permissions, and initial lifecycle state

#### Scenario: New WASM plugin registration creates a registry record
- **WHEN** a plugin with `runtime: wasm` is installed from a supported source
- **THEN** the platform creates a registry record containing its metadata, source, runtime declaration, permissions, module source, ABI metadata, and initial lifecycle state

#### Scenario: Tool plugin registry record includes synchronized MCP summary
- **WHEN** a TS-hosted `ToolPlugin` has reported MCP capability or interaction metadata
- **THEN** operators see that synchronized MCP summary in the single authoritative registry record for the plugin instead of querying bridge-only state

#### Scenario: Registry record survives runtime ownership boundaries
- **WHEN** a plugin is managed by the TS bridge runtime or the Go orchestrator runtime
- **THEN** operators still see one registry record for that plugin instead of separate per-runtime records

### Requirement: Registry supports local and built-in plugin sources
The system SHALL support registering plugins from built-in sources and local installation sources in the first phase. The system MUST preserve source details so future remote registry or marketplace integrations can be added without changing plugin identity semantics.

#### Scenario: Built-in plugin is discovered
- **WHEN** the platform scans built-in plugin definitions
- **THEN** it registers each built-in plugin with a source type that identifies it as platform-provided

#### Scenario: Local plugin is registered from path-based installation
- **WHEN** an operator installs a plugin from a local path
- **THEN** the registry stores the local source details alongside the plugin metadata and lifecycle state

### Requirement: Registry supports operational plugin management
The system SHALL allow operators and internal services to query plugins by kind and lifecycle state, and to change whether a plugin is enabled or disabled. A plugin MUST NOT be activated while its registry state is `disabled`.

#### Scenario: Operator filters plugins by type and state
- **WHEN** an operator requests the plugin list with kind or lifecycle filters
- **THEN** the platform returns only the plugins matching those registry filters

#### Scenario: Disabled plugin cannot be activated
- **WHEN** a plugin record is marked `disabled`
- **THEN** the platform refuses activation requests until the plugin is re-enabled

### Requirement: Registry synchronizes runtime state from plugin hosts
The registry SHALL accept runtime state updates from the plugin host that owns execution and reconcile them into the authoritative plugin record. Runtime updates for Go-hosted WASM plugins MUST reflect actual instantiation, initialization, health, and restart outcomes; runtime success MUST NOT be inferred before the Go host completes the SDK handshake. Runtime updates for TS-hosted `ToolPlugin` instances MUST include the latest MCP interaction snapshot summary when that data changes. Runtime updates MUST NOT bypass the registry or create a second source of truth.

#### Scenario: TS bridge reports a tool plugin runtime state change
- **WHEN** the TS bridge reports that a tool plugin has become `active` or `degraded`
- **THEN** the registry updates the corresponding plugin record with the new lifecycle state and operational details

#### Scenario: TS bridge reports refreshed MCP interaction metadata
- **WHEN** the TS bridge reports that a `ToolPlugin` has refreshed its MCP capability surface or completed an operator-triggered MCP interaction
- **THEN** the registry reconciles the latest MCP interaction snapshot summary into the same authoritative plugin record

#### Scenario: Go runtime reports successful WASM activation
- **WHEN** the Go orchestrator runtime reports that a WASM plugin has finished activation successfully
- **THEN** the registry updates the corresponding plugin record with the new lifecycle state and operational details

#### Scenario: Go runtime reports a WASM instantiation failure
- **WHEN** the Go orchestrator runtime cannot instantiate or initialize a WASM plugin
- **THEN** the registry updates the corresponding plugin record with a degraded state and the reported error without requiring a separate manual sync step

