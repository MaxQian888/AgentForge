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
The system SHALL route executable plugins to the correct host runtime based on plugin kind and declared runtime. `ToolPlugin` and `ReviewPlugin` instances that declare `runtime: mcp` MUST be activated through the TypeScript bridge runtime. `IntegrationPlugin` instances and any Go-hosted `WorkflowPlugin` instances that declare `runtime: wasm` MUST be activated through the Go orchestrator runtime. `RolePlugin` instances MUST remain declarative registry or configuration assets instead of executable runtimes. The registry MUST NOT mark any executable plugin as `active` until its owning runtime has instantiated the referenced module or transport and completed the required initialization handshake.

#### Scenario: Tool plugin activates through the TS bridge
- **WHEN** an enabled tool plugin is activated for first use
- **THEN** the platform starts or connects the plugin through the TS bridge runtime instead of the Go orchestrator runtime

#### Scenario: Review plugin activates through the TS bridge
- **WHEN** an enabled review plugin that declares `runtime: mcp` is activated for a matching review run
- **THEN** the platform starts or connects the plugin through the TS bridge runtime and does not route it to the Go orchestrator runtime

#### Scenario: Workflow WASM plugin activates through the Go runtime
- **WHEN** an enabled workflow plugin that declares `runtime: wasm` is activated for execution
- **THEN** the platform instantiates the referenced module through the Go runtime instead of the TS bridge runtime

#### Scenario: Role plugin never enters an executable runtime
- **WHEN** the platform loads a role plugin definition
- **THEN** it projects the role into registry and execution-profile records without attempting executable plugin activation

### Requirement: Plugin lifecycle state is tracked consistently
The system SHALL expose a unified lifecycle state model for executable plugins with the states `installed`, `enabled`, `activating`, `active`, `degraded`, and `disabled`. The platform MUST drive those states through the lifecycle operations install, enable, activate, deactivate, disable, uninstall, and update. Deactivation from idle timeout or operator action MUST return a plugin to `enabled` without losing installed metadata. Update MUST preserve plugin identity while replacing version or source metadata and re-entering the enable or activate flow. Runtime failures during activation, execution, health checks, deactivation, or update MUST transition the plugin to `degraded` or keep it `disabled` with the last known error preserved.

#### Scenario: Idle plugin deactivates back to enabled
- **WHEN** an active plugin is deactivated because of idle timeout or an operator deactivation request
- **THEN** the platform stops the owning runtime instance and records the plugin lifecycle state as `enabled`

#### Scenario: Plugin update replaces the artifact and re-enters activation flow
- **WHEN** an installed plugin is updated to a newer validated artifact
- **THEN** the platform preserves the plugin identity, replaces the stored version or source metadata, and drives the plugin back through enable or activate according to the update policy

#### Scenario: Failed activation degrades the plugin
- **WHEN** an executable plugin fails activation, runtime initialization, or post-update health checks
- **THEN** the platform records the plugin state as `degraded` and preserves the reported runtime error for operators

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

