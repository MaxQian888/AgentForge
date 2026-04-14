## ADDED Requirements

### Requirement: Runtime interaction routes return capability-aware responses
The Bridge SHALL keep runtime interaction routes canonical and capability-aware. For advanced runtime controls such as messages, command execution, model changes, shell execution, and permission callbacks, the route response MUST align with the runtime catalog support state and MUST identify unsupported or degraded operations explicitly.

#### Scenario: Canonical interaction route succeeds for a supported runtime
- **WHEN** a caller invokes a canonical Bridge interaction route whose selected runtime publishes the operation as supported
- **THEN** the Bridge SHALL delegate to the runtime-specific control path and return the runtime's normalized result
- **THEN** the route response SHALL remain consistent with the support state advertised in `/bridge/runtimes`

#### Scenario: Canonical interaction route is unsupported for the selected runtime
- **WHEN** a caller invokes a canonical Bridge interaction route whose selected runtime publishes the operation as unsupported or degraded
- **THEN** the Bridge SHALL return a structured error that includes the runtime key, requested operation, support state, and reason code
- **THEN** the response SHALL NOT collapse that outcome into an unqualified generic 500 error

### Requirement: Bridge exposes a canonical shell control route
The Bridge SHALL expose `POST /bridge/shell` for runtimes that publish shell execution support. The request MUST include the task identity plus the shell command payload, and runtimes that do not publish shell execution support MUST return an explicit unsupported response.

#### Scenario: OpenCode-backed task uses the canonical shell route
- **WHEN** a caller posts a shell command to `/bridge/shell` for an active OpenCode-backed task
- **THEN** the Bridge SHALL resolve the task runtime, verify shell support from the runtime contract, and proxy the request through the OpenCode session shell API
- **THEN** the normalized response SHALL identify the originating task and runtime session
