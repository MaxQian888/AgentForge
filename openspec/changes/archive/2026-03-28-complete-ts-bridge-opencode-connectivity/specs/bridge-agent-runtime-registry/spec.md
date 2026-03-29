## MODIFIED Requirements

### Requirement: Bridge validates runtime availability before acquiring execution state
The TypeScript bridge SHALL validate requested runtime keys and runtime-specific launch prerequisites before it acquires an active runtime from the pool, and it MUST reject requests with explicit errors when the runtime is unknown, disabled, or missing required integration prerequisites. For `opencode`, launch prerequisites MUST validate the configured OpenCode automation transport, authentication, and requested provider or model readiness rather than only checking whether a local executable exists.

#### Scenario: Request targets an unknown runtime
- **WHEN** the bridge receives an execute request with a `runtime` value that is not registered
- **THEN** it SHALL reject the request with a validation or configuration error that identifies the unknown runtime
- **THEN** it SHALL NOT acquire a runtime entry for that task

#### Scenario: OpenCode transport prerequisites are incomplete
- **WHEN** the bridge resolves `opencode` for execution but the configured OpenCode server transport is unreachable, authentication fails, or the requested upstream provider/model cannot be resolved
- **THEN** it SHALL reject the request with an explicit runtime-configuration error that identifies the missing prerequisite
- **THEN** it SHALL NOT acquire a runtime entry for that task or emit misleading running-state events

### Requirement: Runtime registry surfaces availability diagnostics without starting execution
The TypeScript bridge SHALL evaluate runtime readiness for catalog consumers without acquiring execution state, and it MUST surface actionable diagnostics when a runtime cannot currently start. For `opencode`, diagnostics MUST describe the official OpenCode transport being used and any blocking issue across server reachability, authentication, provider availability, or model selection.

#### Scenario: OpenCode server is unreachable during diagnostics
- **WHEN** the registry evaluates `opencode` readiness and the configured OpenCode server cannot respond to its health probe
- **THEN** the diagnostics result SHALL mark `opencode` as unavailable
- **THEN** the reported reason SHALL identify the server reachability issue without creating a runtime entry or emitting running-state events

#### Scenario: OpenCode provider or model is unavailable during diagnostics
- **WHEN** the OpenCode server is reachable but the configured provider or default model for `opencode` cannot be resolved upstream
- **THEN** the diagnostics result SHALL mark `opencode` as unavailable
- **THEN** the reported reason SHALL identify the specific provider or model mismatch before any execute request is attempted

#### Scenario: Claude credentials are unavailable during diagnostics
- **WHEN** the registry evaluates `claude_code` readiness and no valid credential source is configured
- **THEN** the diagnostics result SHALL mark `claude_code` as unavailable
- **THEN** the reported reason SHALL identify the credential requirement that is missing before any execute request is attempted
