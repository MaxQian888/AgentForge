# Plugin Runtime Specification

## ADDED Requirements

### Requirement: WASM plugins activate through the Go runtime manager with manifest and ABI validation
The system SHALL activate WASM plugins through the Go runtime manager using the plugin manifest as the source of runtime configuration. Activation MUST validate the plugin manifest fields required by the runtime, execute the plugin `describe` and `init` operations, and reject incompatible ABI versions.

#### Scenario: Activate valid WASM plugin
- **WHEN** a plugin record declares runtime `wasm`, a valid module path, ABI version `v1`, and required lifecycle operations
- **THEN** the runtime manager activates the plugin successfully
- **THEN** the returned runtime status is marked active

#### Scenario: Reject plugin with ABI mismatch
- **WHEN** the plugin manifest declares an ABI version that does not match the module-reported ABI version
- **THEN** activation fails with an ABI compatibility error

#### Scenario: Reject plugin missing required lifecycle exports
- **WHEN** the plugin module does not expose the required autorun lifecycle operations consumed by the runtime manager
- **THEN** activation fails with a missing export error

### Requirement: WASM plugins run inside the current wazero and WASI execution envelope
The system SHALL execute WASM plugins through the current Go runtime envelope backed by `wazero` and `wasi_snapshot_preview1`, passing plugin config, capabilities, and operation payload through the module environment for each invocation.

#### Scenario: Instantiate plugin module with WASI support
- **WHEN** the runtime manager executes a WASM plugin operation
- **THEN** it instantiates a fresh `wazero` runtime with WASI enabled for that execution

#### Scenario: Pass plugin runtime context through environment variables
- **WHEN** the runtime manager runs a plugin operation
- **THEN** the plugin receives the requested operation, config, payload, and declared capabilities through the execution envelope environment

### Requirement: Declared plugin capabilities gate runtime invocation
The system SHALL allow plugin invocation only for operations declared in the plugin manifest capability list.

#### Scenario: Invoke declared plugin capability
- **WHEN** a plugin declares `send_message` in its capability list and the caller invokes `send_message`
- **THEN** the runtime manager executes the operation and returns the plugin payload

#### Scenario: Reject undeclared plugin capability
- **WHEN** the caller invokes an operation that is not declared in the plugin manifest capability list
- **THEN** the runtime manager rejects the invocation before executing the module

### Requirement: Plugin runtime exposes health, restart, and debug execution helpers
The system SHALL expose plugin health checks, restart handling, and debug execution using the same runtime envelope and lifecycle contract as activation and invoke.

#### Scenario: Check plugin health through runtime operation
- **WHEN** the runtime manager performs a health check for an activated plugin
- **THEN** it executes the plugin health operation and returns a runtime status reflecting the plugin lifecycle state

#### Scenario: Restart plugin increments restart count
- **WHEN** the runtime manager restarts an already activated plugin
- **THEN** it reruns the plugin lifecycle activation flow
- **THEN** the returned runtime status increments the plugin restart count

#### Scenario: Debug execution returns runtime diagnostics
- **WHEN** the runtime manager performs a debug execution for a declared plugin capability
- **THEN** the response includes execution diagnostics such as resolved module path and captured stdio alongside the plugin result
