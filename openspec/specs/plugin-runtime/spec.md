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
The system SHALL expose a unified lifecycle state model for executable plugins with the states `installed`, `enabled`, `activating`, `active`, `degraded`, and `disabled`. The Go control plane MUST transition a plugin to `activating` when runtime startup begins, to `active` only after the owning runtime handshake succeeds and the current runtime instance snapshot is updated, and to `degraded` when initialization, invocation, permission validation, or health checks fail. The system MUST keep registry state and current instance snapshots consistent after activation succeeds, activation fails, restart succeeds, restart fails, runtime reconciliation, disable, and uninstall operations.

#### Scenario: Successful activation moves a plugin to active
- **WHEN** an enabled plugin starts successfully and completes its host runtime handshake
- **THEN** the platform records the plugin state as `active` and updates the current runtime instance snapshot to the active state

#### Scenario: Failed module initialization degrades a WASM plugin
- **WHEN** the Go host cannot instantiate a WASM plugin or the SDK initialization handshake returns an error
- **THEN** the platform records the plugin state as `degraded`, preserves the last known error for operators, and updates the current runtime instance snapshot accordingly

#### Scenario: Disable clears executable lifecycle ownership
- **WHEN** an operator disables an executable plugin
- **THEN** the platform records the plugin state as `disabled` and marks the current runtime instance snapshot as no longer active

### Requirement: Runtime health details are published to the registry
The system SHALL publish runtime health details for executable plugins, including last health timestamp, restart count, and last error summary, so that the registry can present an authoritative operational view. For TS-hosted `ToolPlugin` instances, the published operational details MUST also include the MCP interaction snapshot metadata needed for operators to understand transport mode, discovery freshness, capability counts, and the latest interaction outcome summary. For Go-hosted WASM plugins, the reported operational details MUST come from the active WASM runtime instance rather than optimistic state transitions, and MUST identify the runtime artifact or instance being monitored when such data is available.

#### Scenario: Host runtime reports health information
- **WHEN** a host runtime reports plugin health for an executable plugin
- **THEN** the platform updates the plugin record with the reported health timestamp and operational details

#### Scenario: TS bridge reports MCP interaction metadata for a tool plugin
- **WHEN** the TS bridge reports health or runtime-state information for an active `ToolPlugin`
- **THEN** the platform includes the latest MCP interaction snapshot metadata in the operational details synchronized to the registry

#### Scenario: Plugin restart attempts are tracked after runtime failure
- **WHEN** the platform attempts to recover a failing executable plugin
- **THEN** it increments the recorded restart count and updates the plugin state according to the recovery outcome

### Requirement: Permission declarations are enforced before execution
The system SHALL validate plugin permission declarations and declared capabilities before activation or invocation reaches the owning runtime. If an executable plugin declares required network or filesystem permissions that the current server policy cannot satisfy, or if an invocation requests an operation outside the declared capability set, the Go control plane MUST reject the action before runtime execution begins.

#### Scenario: Activation rejects unsupported required permissions
- **WHEN** a plugin manifest declares required permissions that the Go server policy cannot satisfy for activation
- **THEN** the control plane refuses activation and records a control-plane error instead of attempting runtime startup

#### Scenario: Invocation rejects an undeclared capability
- **WHEN** an operator or internal caller requests a plugin operation that is not declared in the plugin manifest capability list
- **THEN** the control plane rejects the invocation before calling the runtime and preserves the error in plugin operational state

### Requirement: TS-hosted tool plugins maintain a refreshable MCP interaction snapshot
The system SHALL maintain a refreshable MCP interaction snapshot for each TS-hosted active `ToolPlugin`. The snapshot MUST capture the MCP transport, the latest successful discovery timestamp, tool count, resource count, prompt count, and the latest operator-triggered interaction summary available from the TS Bridge. The snapshot MUST be refreshable without reinstalling the plugin.

#### Scenario: Activation initializes an MCP interaction snapshot
- **WHEN** an enabled `ToolPlugin` is activated successfully through the TS Bridge
- **THEN** the runtime initializes an MCP interaction snapshot for that plugin using the discovered MCP capability surface and current transport details

#### Scenario: Refresh updates the MCP interaction snapshot in place
- **WHEN** an operator triggers a capability refresh for an already active `ToolPlugin`
- **THEN** the TS-hosted runtime updates the plugin's MCP interaction snapshot with the new discovery timestamp and latest capability counts instead of requiring a re-registration flow

