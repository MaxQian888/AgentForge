## MODIFIED Requirements

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
The system SHALL publish runtime health details for executable plugins, including last health timestamp, restart count, and last error summary, so that the registry can present an authoritative operational view. For Go-hosted WASM plugins, the reported operational details MUST come from the active WASM runtime instance rather than optimistic state transitions, and MUST identify the runtime artifact or instance being monitored when such data is available. Health reconciliation MUST update the authoritative registry record and the current runtime instance snapshot using one control-plane path.

#### Scenario: Host runtime reports health information
- **WHEN** a host runtime reports plugin health for an executable plugin
- **THEN** the platform updates the plugin record and current runtime instance snapshot with the reported health timestamp and operational details

#### Scenario: Plugin restart attempts are tracked after WASM runtime failure
- **WHEN** the platform attempts to recover a failing Go-hosted WASM plugin
- **THEN** it increments the recorded restart count in the registry and current runtime instance snapshot and updates the plugin state according to the recovery outcome

## ADDED Requirements

### Requirement: Permission declarations are enforced before execution
The system SHALL validate plugin permission declarations and declared capabilities before activation or invocation reaches the owning runtime. If an executable plugin declares required network or filesystem permissions that the current server policy cannot satisfy, or if an invocation requests an operation outside the declared capability set, the Go control plane MUST reject the action before runtime execution begins.

#### Scenario: Activation rejects unsupported required permissions
- **WHEN** a plugin manifest declares required permissions that the Go server policy cannot satisfy for activation
- **THEN** the control plane refuses activation and records a control-plane error instead of attempting runtime startup

#### Scenario: Invocation rejects an undeclared capability
- **WHEN** an operator or internal caller requests a plugin operation that is not declared in the plugin manifest capability list
- **THEN** the control plane rejects the invocation before calling the runtime and preserves the error in plugin operational state
