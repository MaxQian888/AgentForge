## ADDED Requirements

### Requirement: Bridge resolves coding-agent execution through one runtime registry
The TypeScript bridge SHALL maintain one canonical runtime registry for coding-agent execution, and that registry MUST define the supported runtime keys, default runtime selection, and runtime metadata needed to launch Claude Code, Codex, and OpenCode behind the `/bridge/execute` endpoint.

#### Scenario: Execute request omits runtime
- **WHEN** the bridge receives a valid execute request without an explicit `runtime`
- **THEN** it SHALL resolve the runtime from the configured default in the runtime registry
- **THEN** downstream execution code SHALL consume the resolved runtime metadata instead of assuming a Claude-only path

#### Scenario: Execute request targets a supported runtime
- **WHEN** the bridge receives an execute request with `runtime` set to `codex`
- **THEN** it SHALL resolve the Codex runtime adapter from the registry before execution starts
- **THEN** the bridge SHALL use the same canonical execute endpoint and response shape as every other supported runtime

### Requirement: Bridge validates runtime availability before acquiring execution state
The TypeScript bridge SHALL validate requested runtime keys and runtime-specific launch prerequisites before it acquires an active runtime from the pool, and it MUST reject requests with explicit errors when the runtime is unknown, disabled, or missing required local configuration.

#### Scenario: Request targets an unknown runtime
- **WHEN** the bridge receives an execute request with a `runtime` value that is not registered
- **THEN** it SHALL reject the request with a validation or configuration error that identifies the unknown runtime
- **THEN** it SHALL NOT acquire a runtime entry for that task

#### Scenario: Runtime configuration is incomplete
- **WHEN** the bridge resolves `opencode` for execution but the required executable path or credential is unavailable
- **THEN** it SHALL reject the request with an explicit runtime-configuration error
- **THEN** it SHALL NOT start execution or emit misleading running-state events

### Requirement: Runtime adapters normalize native execution into the canonical bridge event model
The TypeScript bridge SHALL require every registered coding runtime adapter to translate its native execution stream into the canonical AgentForge runtime lifecycle contract, including normalized status, output, tool activity, cost, error, cancellation, and snapshot semantics.

#### Scenario: Non-Claude runtime emits native execution output
- **WHEN** a Codex or OpenCode adapter emits backend-native progress or tool activity during execution
- **THEN** the bridge SHALL normalize that activity into the existing `AgentEvent` categories consumed by Go
- **THEN** the Go orchestrator SHALL NOT need runtime-specific event parsing to observe the run

#### Scenario: Runtime adapter cannot support a canonical signal
- **WHEN** a registered runtime lacks a native equivalent for a canonical signal such as cost or tool-call detail
- **THEN** the adapter SHALL provide the closest truthful normalized data permitted by the bridge contract
- **THEN** the bridge SHALL preserve consistent lifecycle completion and error semantics instead of omitting required terminal behavior
