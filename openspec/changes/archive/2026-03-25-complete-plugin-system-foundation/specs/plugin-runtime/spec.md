## MODIFIED Requirements

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

