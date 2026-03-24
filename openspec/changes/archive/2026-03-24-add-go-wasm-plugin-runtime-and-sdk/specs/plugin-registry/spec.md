## MODIFIED Requirements

### Requirement: Registry stores the authoritative plugin record
The system SHALL maintain a registry record for each installed plugin as the authoritative source of truth for plugin metadata and operator-visible state. Each record MUST include plugin identifier, plugin kind, version, install source, declared runtime, declared permissions, current lifecycle state, and manifest payload or its normalized equivalent. For plugins that declare `runtime: wasm`, the record MUST also preserve the resolved module source and SDK ABI compatibility metadata needed to audit which runtime artifact is expected to execute.

#### Scenario: New WASM plugin registration creates a registry record
- **WHEN** a plugin with `runtime: wasm` is installed from a supported source
- **THEN** the platform creates a registry record containing its metadata, source, runtime declaration, permissions, module source, ABI metadata, and initial lifecycle state

#### Scenario: Registry record survives runtime ownership boundaries
- **WHEN** a WASM plugin is managed by the Go orchestrator runtime and tool plugins are managed by the TS bridge runtime
- **THEN** operators still see one registry record for each plugin instead of separate per-runtime records

### Requirement: Registry synchronizes runtime state from plugin hosts
The registry SHALL accept runtime state updates from the plugin host that owns execution and reconcile them into the authoritative plugin record. Runtime updates for Go-hosted WASM plugins MUST reflect actual instantiation, initialization, health, and restart outcomes; runtime success MUST NOT be inferred before the Go host completes the SDK handshake. Runtime updates MUST NOT bypass the registry or create a second source of truth.

#### Scenario: Go runtime reports successful WASM activation
- **WHEN** the Go orchestrator runtime reports that a WASM plugin has finished activation successfully
- **THEN** the registry updates the corresponding plugin record with the new lifecycle state and operational details

#### Scenario: Go runtime reports a WASM instantiation failure
- **WHEN** the Go orchestrator runtime cannot instantiate or initialize a WASM plugin
- **THEN** the registry updates the corresponding plugin record with a degraded state and the reported error without requiring a separate manual sync step
