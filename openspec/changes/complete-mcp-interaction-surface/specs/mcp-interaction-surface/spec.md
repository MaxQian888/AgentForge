## ADDED Requirements

### Requirement: Operator-facing MCP capability discovery is available for tool plugins
The system SHALL expose a typed control-plane interface for `ToolPlugin` MCP capability discovery that can return and refresh the plugin's tools, resources, and prompts without requiring direct access to the TS Bridge process. The discovery response MUST identify the plugin, the MCP transport in use, whether the response came from a fresh refresh or the latest cached snapshot, and the returned capability lists or summaries for each supported primitive.

#### Scenario: Successful capability refresh returns MCP primitives
- **WHEN** an operator requests a refresh of an active `ToolPlugin` capability surface
- **THEN** the system returns the refreshed tools, resources, and prompts discovered from the plugin's MCP server together with transport and freshness metadata

#### Scenario: Non-tool plugins are rejected from MCP discovery
- **WHEN** an operator requests MCP capability discovery for a plugin that is not a `ToolPlugin`
- **THEN** the system rejects the request with a typed error indicating that the selected plugin does not expose MCP interaction primitives

### Requirement: Typed MCP interaction APIs proxy through the Go control plane
The system SHALL provide typed operator-facing APIs for MCP interactions that proxy through the Go control plane to the TS Bridge. The supported operations MUST include tool invocation, resource reading, and prompt retrieval or preview for an active `ToolPlugin`. The control plane MUST validate the plugin state, the requested operation type, and required request fields before the TS Bridge executes the interaction.

#### Scenario: Tool invocation succeeds through the control plane
- **WHEN** an operator invokes a discovered MCP tool on an active `ToolPlugin` with valid arguments
- **THEN** the Go control plane forwards the request to the TS Bridge, the MCP server executes the tool, and the system returns the structured MCP result to the caller

#### Scenario: Invalid interaction input is rejected before execution
- **WHEN** an operator submits a tool invocation, resource read, or prompt retrieval request that omits required identifiers or fails schema validation
- **THEN** the control plane rejects the request before calling the TS Bridge and returns a validation error that identifies the missing or invalid input

### Requirement: MCP interaction outcomes are auditable and diagnostic-friendly
The system SHALL record operator-triggered MCP discovery and interaction outcomes as structured plugin events and runtime summaries. Each recorded outcome MUST include the plugin identifier, operation type, success or failure state, timestamp, and a bounded result or error summary. A failed discovery or interaction MUST update the plugin's latest MCP diagnostic state so operators can distinguish connection failures, validation failures, and execution failures.

#### Scenario: Failed capability refresh updates diagnostics
- **WHEN** a refresh of a plugin's MCP capability surface fails because the MCP server cannot be reached or queried
- **THEN** the system records a failed plugin event and updates the latest MCP diagnostic summary with the failure category and message

#### Scenario: Successful resource read is audited without persisting the full payload to registry state
- **WHEN** an operator successfully reads an MCP resource through the control plane
- **THEN** the system returns the resource contents to the caller, records a success event with a bounded summary, and does not require the full resource body to be stored in the registry snapshot
