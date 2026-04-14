## ADDED Requirements

### Requirement: OpenCode execute inputs are handled through official transport surfaces
The Bridge SHALL handle parity-sensitive ExecuteRequest inputs for OpenCode through official transport surfaces before `prompt_async` begins. Supported attachments MUST be encoded as official OpenCode prompt parts. Provider or session config such as `env` or `web_search` MUST be applied through official session bootstrap or config update surfaces when the selected OpenCode server exposes them. Inputs that cannot be represented truthfully MUST be rejected before prompt submission rather than silently dropped.

#### Scenario: OpenCode attachment is forwarded as a prompt part
- **WHEN** ExecuteRequest includes a supported attachment for an OpenCode run
- **THEN** the Bridge encodes that attachment as an official OpenCode prompt part for the bound session
- **THEN** the agent receives the attachment within the same run

#### Scenario: Unsupported OpenCode execute input is rejected before prompt submission
- **WHEN** ExecuteRequest asks OpenCode to use an input that the selected server or provider does not expose truthfully
- **THEN** the Bridge returns an explicit validation or configuration error before `prompt_async`
- **THEN** no OpenCode prompt is sent for that task

### Requirement: OpenCode rollback uses continuity-backed revert targets
The Bridge SHALL resolve canonical `/bridge/rollback` for OpenCode by translating `checkpoint_id` or `turns` into revertable message targets stored in OpenCode continuity or recovered from the bound session history. The Bridge MUST call the official OpenCode revert or unrevert endpoints and MUST preserve enough continuity metadata to explain rollback failures truthfully.

#### Scenario: Rollback to explicit message checkpoint
- **WHEN** `/bridge/rollback` is called for an OpenCode task with a message checkpoint that maps to the bound upstream session
- **THEN** the Bridge calls the official OpenCode revert endpoint for that session and message
- **THEN** the rollback request returns success without degrading to a blanket unsupported error

#### Scenario: Rollback target cannot be resolved
- **WHEN** `/bridge/rollback` is called for an OpenCode task whose continuity lacks a resolvable message target
- **THEN** the Bridge returns a structured runtime-specific rollback error
- **THEN** the response identifies the missing continuity or history prerequisite

## MODIFIED Requirements

### Requirement: OpenCode runtime configuration updates
The Bridge SHALL support updating OpenCode runtime configuration by calling `PATCH /config` when execution parameters change before or during a session. Model switches MUST use this path for active sessions. Execute-time provider or capability settings such as provider selection, environment/config overlays, or web-search intent MUST also flow through official session or config surfaces before prompt submission whenever the server exposes them.

#### Scenario: Update OpenCode model during session
- **WHEN** the Go orchestrator sends a model change request for an active OpenCode session
- **THEN** the Bridge calls `PATCH /config` with the updated provider or model configuration

#### Scenario: Apply execute-time config before prompt submission
- **WHEN** an OpenCode execute request includes provider or capability settings that require server-side configuration before prompt submission
- **THEN** the Bridge patches or initializes the official OpenCode config for that session before calling `prompt_async`
- **THEN** the resulting session state remains aligned with the runtime catalog and execute preflight outcome

### Requirement: OpenCode runtime catalog includes server-backed provider and session control metadata
The Bridge SHALL publish OpenCode's server-backed control-plane metadata in the runtime catalog, including discovered agents, skills, provider availability, provider-auth readiness, parity-sensitive execute input support, and session control surfaces such as rollback, messages, command execution, shell execution, and permission-response loops whenever the official OpenCode server makes them available.

#### Scenario: OpenCode server is healthy and exposes discovery surfaces
- **WHEN** the Bridge refreshes runtime catalog metadata while the OpenCode server is reachable
- **THEN** the OpenCode catalog entry SHALL include discovered agents, skills, provider metadata, and the published support state of server-backed session controls and execute inputs
- **THEN** upstream consumers SHALL be able to distinguish “server reachable but auth or config required” from “control not supported by the current Bridge contract”

#### Scenario: OpenCode provider auth or config update is required before execution
- **WHEN** the selected OpenCode provider requires OAuth or equivalent configuration changes before a run can start
- **THEN** the runtime diagnostics and capability metadata SHALL identify the auth or config prerequisite explicitly
- **THEN** the Bridge SHALL NOT report the runtime as simply unavailable without indicating the missing provider-auth handshake or config step
