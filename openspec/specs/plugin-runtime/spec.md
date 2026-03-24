# plugin-runtime Specification

## Purpose
Define the executable plugin runtime contract for AgentForge, including manifest validation, host-runtime routing, lifecycle state semantics, and runtime health reporting across Go and TS hosts.

## Requirements
### Requirement: Unified plugin manifest validation
The system SHALL accept plugins through a unified manifest contract that includes plugin identity, plugin kind, runtime declaration, source metadata, and permission declarations. Go-hosted executable plugins that declare `runtime: wasm` MUST also declare the module artifact location, the SDK ABI version, and any required host capability declarations needed for activation. The system MUST reject manifests that omit required identity fields, omit the WASM runtime metadata, or declare a runtime that is incompatible with the plugin kind.

#### Scenario: Valid manifest is accepted for registration
- **WHEN** a plugin manifest includes a supported `kind`, a valid plugin identifier, a declared runtime, and required metadata
- **THEN** the platform accepts the manifest for registration and records the declared runtime and permissions

#### Scenario: WASM manifest missing runtime metadata is rejected
- **WHEN** a Go-hosted plugin manifest declares `runtime: wasm` but omits the module artifact location or ABI version
- **THEN** the platform rejects the plugin before activation and returns a validation error describing the missing runtime metadata

#### Scenario: Unsupported runtime-kind combination is rejected
- **WHEN** a manifest declares a runtime that is not allowed for its plugin kind
- **THEN** the platform rejects the plugin before activation and returns a validation error describing the incompatible combination

### Requirement: Runtime ownership is routed by plugin kind
The system SHALL route executable plugins to the correct host runtime based on plugin kind. `ToolPlugin` instances MUST be activated through the TypeScript bridge runtime, and `IntegrationPlugin` instances that declare `runtime: wasm` MUST be activated through the Go orchestrator runtime. The registry MUST NOT mark a Go-hosted WASM plugin as `active` until the Go runtime has instantiated the referenced module and completed the SDK initialization handshake.

#### Scenario: Tool plugin activates through the TS bridge
- **WHEN** an enabled tool plugin is activated for first use
- **THEN** the platform starts or connects the plugin through the TS bridge runtime instead of the Go orchestrator runtime

#### Scenario: Integration WASM plugin activates through the Go runtime
- **WHEN** an enabled integration plugin that declares `runtime: wasm` is activated
- **THEN** the platform instantiates the referenced module through the Go WASM runtime instead of the TS bridge runtime

### Requirement: Plugin lifecycle state is tracked consistently
The system SHALL expose a unified lifecycle state model for executable plugins with the states `installed`, `enabled`, `activating`, `active`, `degraded`, and `disabled`. The system MUST transition a Go-hosted WASM plugin to `activating` when module startup begins, to `active` only after module instantiation and SDK initialization succeed, and to `degraded` when initialization, invocation, or health checks fail. The system MUST update plugin state transitions when activation succeeds, activation fails, health checks fail, restart succeeds, restart fails, or an operator disables the plugin.

#### Scenario: Successful activation moves a plugin to active
- **WHEN** an enabled plugin starts successfully and completes its host runtime handshake
- **THEN** the platform records the plugin state as `active`

#### Scenario: Failed module initialization degrades a WASM plugin
- **WHEN** the Go host cannot instantiate a WASM plugin or the SDK initialization handshake returns an error
- **THEN** the platform records the plugin state as `degraded` and preserves the last known error for operators

### Requirement: Runtime health details are published to the registry
The system SHALL publish runtime health details for executable plugins, including last health timestamp, restart count, and last error summary, so that the registry can present an authoritative operational view. For Go-hosted WASM plugins, the reported operational details MUST come from the active WASM runtime instance rather than optimistic state transitions, and MUST identify the runtime artifact or instance being monitored when such data is available.

#### Scenario: Host runtime reports health information
- **WHEN** a host runtime reports plugin health for an executable plugin
- **THEN** the platform updates the plugin record with the reported health timestamp and operational details

#### Scenario: Plugin restart attempts are tracked after WASM runtime failure
- **WHEN** the platform attempts to recover a failing Go-hosted WASM plugin
- **THEN** it increments the recorded restart count and updates the plugin state according to the recovery outcome
