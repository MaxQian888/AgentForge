## MODIFIED Requirements

### Requirement: Registry stores the authoritative plugin record
The system SHALL maintain a persistent registry record for each installed plugin as the authoritative source of truth for plugin metadata and operator-visible state. Each record MUST include plugin identifier, plugin kind, version, install source, declared runtime, declared permissions, current lifecycle state, and manifest payload or its normalized equivalent. Each record MUST survive Go server restarts and remain available for control-plane queries before any runtime activation occurs. The registry MUST also preserve the latest linked runtime instance summary and, for plugins that declare `runtime: wasm`, the resolved module source and SDK ABI compatibility metadata needed to audit which runtime artifact is expected to execute.

#### Scenario: New plugin registration creates a registry record
- **WHEN** a plugin is installed from a supported source
- **THEN** the platform creates a persistent registry record containing its metadata, source, runtime declaration, permissions, and initial lifecycle state

#### Scenario: Registry record survives a Go server restart
- **WHEN** the Go server restarts after plugins were previously installed
- **THEN** operators can still query those plugin records without re-running plugin installation

#### Scenario: New WASM plugin registration creates a registry record
- **WHEN** a plugin with `runtime: wasm` is installed from a supported source
- **THEN** the platform creates a registry record containing its metadata, source, runtime declaration, permissions, module source, ABI metadata, and initial lifecycle state

### Requirement: Registry synchronizes runtime state from plugin hosts
The registry SHALL accept runtime state updates from the plugin host that owns execution and reconcile them into the authoritative plugin record. Runtime updates for Go-hosted WASM plugins MUST reflect actual instantiation, initialization, health, restart, and invocation outcomes; runtime success MUST NOT be inferred before the Go host completes the runtime handshake. Runtime reconciliation MUST update the registry record, refresh the current runtime instance snapshot, and append an audit event without creating a second source of truth outside the registry-controlled flow.

#### Scenario: TS bridge reports a tool plugin runtime state change
- **WHEN** the TS bridge reports that a tool plugin has become `active` or `degraded`
- **THEN** the registry updates the corresponding plugin record and current instance snapshot with the new lifecycle state and operational details

#### Scenario: Go runtime reports successful WASM activation
- **WHEN** the Go orchestrator runtime reports that a WASM plugin has finished activation successfully
- **THEN** the registry updates the corresponding plugin record and current instance snapshot with the new lifecycle state and operational details

#### Scenario: Go runtime reports a WASM instantiation failure
- **WHEN** the Go orchestrator runtime cannot instantiate or initialize a WASM plugin
- **THEN** the registry updates the corresponding plugin record with a degraded state, updates the current instance snapshot, and appends the reported error without requiring a separate manual sync step
