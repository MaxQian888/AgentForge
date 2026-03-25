# plugin-event-audit Specification

## Purpose
Define the plugin lifecycle audit and live event contract so AgentForge operators can observe control-plane actions and runtime failures without relying on polling-only status checks.
## Requirements
### Requirement: Plugin lifecycle actions are written to an audit log
The system SHALL append a structured plugin event audit record for each install, enable, disable, activate, runtime-sync, health, restart, invoke, and uninstall action processed by the Go control plane. Each event record MUST include the plugin identifier, event type, event source, resulting lifecycle state when applicable, timestamp, and an operator-meaningful summary of any runtime error.

#### Scenario: Install action creates an audit event
- **WHEN** an operator installs a plugin from a built-in or local source
- **THEN** the Go control plane appends an audit event that records the install action, plugin identity, and source metadata

#### Scenario: Runtime failure creates an audit event
- **WHEN** activation, health checking, restart, or invocation fails for an executable plugin
- **THEN** the Go control plane appends an audit event containing the resulting degraded state and summarized error details

### Requirement: Operator-relevant plugin events are broadcast live
The system SHALL broadcast operator-relevant plugin lifecycle events through the Go-side plugin event hub so connected dashboards can observe plugin state changes without polling-only behavior. Broadcast payloads MUST use the same plugin identity and lifecycle semantics as the persisted audit record.

#### Scenario: Successful activation is broadcast to subscribers
- **WHEN** a plugin transitions to `active`
- **THEN** the Go control plane emits a live plugin event that subscribers can use to update operator-facing plugin status

#### Scenario: Runtime sync degradation is broadcast to subscribers
- **WHEN** the Go control plane reconciles a runtime update that marks a plugin `degraded`
- **THEN** it emits a live plugin event reflecting the degraded lifecycle state and error summary
