## ADDED Requirements

### Requirement: Cross-runtime interaction controls publish truthful support state
The Bridge SHALL publish every advanced interaction control with an explicit support state that matches the runtime catalog and route behavior. For each runtime-specific control, the published state MUST distinguish `supported`, `degraded`, and `unsupported`, and the Bridge MUST return a structured unsupported or degraded error instead of silently ignoring the request.

#### Scenario: Route is invoked for a supported interaction control
- **WHEN** a caller invokes a lifecycle or interaction control that the selected runtime publishes as supported
- **THEN** the Bridge SHALL execute the canonical control path for that runtime
- **THEN** the returned status and diagnostics SHALL remain consistent with the capability metadata published in the runtime catalog

#### Scenario: Route is invoked for an unsupported interaction control
- **WHEN** a caller invokes a lifecycle or interaction control that the selected runtime publishes as unsupported
- **THEN** the Bridge SHALL reject the request with a structured error that identifies the runtime, operation, support state, and reason code
- **THEN** it SHALL NOT silently drop the control or pretend the request completed successfully

### Requirement: Callback-dependent interaction inputs validate prerequisites before execution
The Bridge SHALL validate callback and approval prerequisites before execution whenever a request enables hook callbacks, tool permission callbacks, provider-auth handshakes, or equivalent runtime-mediated user interaction. Requests missing a required callback surface or required runtime prerequisite MUST fail as validation or configuration errors before execution begins.

#### Scenario: Callback-dependent request omits callback surface
- **WHEN** an execute request enables a callback-dependent interaction such as Claude hook forwarding or tool permission callbacks but does not provide the required callback target
- **THEN** the Bridge SHALL reject the request before runtime execution starts
- **THEN** the returned error SHALL identify the missing callback prerequisite instead of falling back to a misleading best-effort mode
