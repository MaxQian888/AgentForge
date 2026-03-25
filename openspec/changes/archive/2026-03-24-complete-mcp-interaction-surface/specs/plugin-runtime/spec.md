## ADDED Requirements

### Requirement: TS-hosted tool plugins maintain a refreshable MCP interaction snapshot
The system SHALL maintain a refreshable MCP interaction snapshot for each TS-hosted active `ToolPlugin`. The snapshot MUST capture the MCP transport, the latest successful discovery timestamp, tool count, resource count, prompt count, and the latest operator-triggered interaction summary available from the TS Bridge. The snapshot MUST be refreshable without reinstalling the plugin.

#### Scenario: Activation initializes an MCP interaction snapshot
- **WHEN** an enabled `ToolPlugin` is activated successfully through the TS Bridge
- **THEN** the runtime initializes an MCP interaction snapshot for that plugin using the discovered MCP capability surface and current transport details

#### Scenario: Refresh updates the MCP interaction snapshot in place
- **WHEN** an operator triggers a capability refresh for an already active `ToolPlugin`
- **THEN** the TS-hosted runtime updates the plugin's MCP interaction snapshot with the new discovery timestamp and latest capability counts instead of requiring a re-registration flow

## MODIFIED Requirements

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
