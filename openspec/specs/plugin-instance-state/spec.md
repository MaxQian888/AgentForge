# plugin-instance-state Specification

## Purpose
Define persistent plugin runtime instance tracking so AgentForge preserves current host ownership, lifecycle state, and runtime health details across control-plane reconciliation and server restarts.
## Requirements
### Requirement: Runtime instance snapshots are persisted
The system SHALL persist a runtime instance snapshot for each executable plugin activation managed by the Go control plane. Each snapshot MUST record the plugin identifier, runtime host, lifecycle state, resolved runtime artifact or process identity, restart count, last health timestamp, last error summary, and project scope when the caller provides one.

#### Scenario: Activation creates or refreshes a runtime instance snapshot
- **WHEN** an enabled executable plugin is activated successfully
- **THEN** the Go control plane creates or updates the current runtime instance snapshot for that plugin with the active lifecycle state and resolved runtime details

#### Scenario: Restart updates the current runtime instance snapshot
- **WHEN** the Go control plane restarts an executable plugin after a runtime failure or operator request
- **THEN** it updates the existing runtime instance snapshot instead of creating an unrelated duplicate current instance record

### Requirement: Runtime state reconciliation updates the current instance snapshot
The system SHALL reconcile runtime status updates from both the TS bridge and Go-hosted runtimes into the current runtime instance snapshot rather than mutating only the top-level plugin registry row. The reconciled snapshot MUST remain queryable after the Go server restarts.

#### Scenario: Runtime sync updates a tool plugin instance snapshot
- **WHEN** the TS bridge reports that a tool plugin has become `active` or `degraded`
- **THEN** the Go control plane updates the current instance snapshot for that plugin with the reported lifecycle state and operational details

#### Scenario: Disable clears the active instance state
- **WHEN** an operator disables an executable plugin
- **THEN** the Go control plane marks the current runtime instance snapshot as no longer active and keeps enough state to explain the last known lifecycle outcome
