## ADDED Requirements

### Requirement: OpenCode runtime catalog includes server-backed provider and session control metadata
The Bridge SHALL publish OpenCode's server-backed control-plane metadata in the runtime catalog, including discovered agents, skills, provider availability, provider-auth readiness, and session control surfaces such as messages, command execution, shell execution, share or summarize support, and permission-response loops whenever the official OpenCode server makes them available.

#### Scenario: OpenCode server is healthy and exposes discovery surfaces
- **WHEN** the Bridge refreshes runtime catalog metadata while the OpenCode server is reachable
- **THEN** the OpenCode catalog entry SHALL include discovered agents, skills, provider metadata, and the published support state of server-backed session controls
- **THEN** upstream consumers SHALL be able to distinguish "server reachable but auth required" from "control not supported by the current Bridge contract"

#### Scenario: OpenCode provider auth or config update is required before execution
- **WHEN** the selected OpenCode provider requires OAuth or equivalent configuration changes before a run can start
- **THEN** the runtime diagnostics and capability metadata SHALL identify the auth or config prerequisite explicitly
- **THEN** the Bridge SHALL NOT report the runtime as simply unavailable without indicating the missing provider-auth handshake

### Requirement: OpenCode shell execution is exposed through the canonical Bridge control plane
The Bridge SHALL expose shell execution for OpenCode through a canonical Bridge route and SHALL proxy that control through the official `POST /session/:id/shell` server API when the selected runtime publishes shell support.

#### Scenario: Shell execution requested for an active OpenCode session
- **WHEN** a caller invokes the canonical Bridge shell route for a task backed by an active OpenCode session
- **THEN** the Bridge SHALL call the official OpenCode shell endpoint for the bound session and return the resulting assistant message payload
- **THEN** the runtime capability metadata SHALL publish shell execution as supported for that runtime

#### Scenario: Shell execution requested for a runtime without shell support
- **WHEN** a caller invokes the canonical Bridge shell route for a runtime that does not publish shell control support
- **THEN** the Bridge SHALL reject the request with a structured unsupported response
- **THEN** it SHALL NOT emulate shell execution through a different non-canonical route
