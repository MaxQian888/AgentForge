## MODIFIED Requirements

### Requirement: Bridge validates runtime availability before acquiring execution state
The TypeScript bridge SHALL validate requested runtime keys, runtime-specific launch prerequisites, and parity-sensitive execute inputs before it acquires an active runtime from the pool, and it MUST reject requests with explicit errors when the runtime is unknown, disabled, missing required integration prerequisites, or asked to honor an input it cannot represent truthfully. For additional CLI-backed runtimes, launch prerequisites MUST validate executable discovery, authentication or login state, provider-profile setup, and model compatibility according to the selected runtime profile. For runtimes with provider/config-mediated features such as OpenCode, validation MUST also account for selected provider auth, config readiness, and any requested `attachments`, `env`, `web_search`, or rollback prerequisites that the runtime cannot truthfully honor.

#### Scenario: Request targets an unknown runtime
- **WHEN** the bridge receives an execute request with a `runtime` value that is not registered
- **THEN** it SHALL reject the request with a validation or configuration error that identifies the unknown runtime
- **THEN** it SHALL NOT acquire a runtime entry for that task

#### Scenario: CLI-backed runtime prerequisites are incomplete
- **WHEN** the bridge resolves `gemini` for execution but the configured executable, authentication profile, or requested provider or model combination is incomplete for that runtime
- **THEN** it SHALL reject the request with an explicit runtime-configuration error that identifies the missing prerequisite
- **THEN** it SHALL NOT start execution or emit misleading running-state events

#### Scenario: Requested execute input exceeds runtime parity support
- **WHEN** the bridge receives an execute request whose selected runtime or provider cannot truthfully honor a requested input such as `attachments`, `env`, or `web_search`
- **THEN** it SHALL reject the request with an explicit validation or configuration error that identifies the runtime and rejected field
- **THEN** it SHALL NOT acquire a runtime entry or send the request into runtime execution

### Requirement: Runtime catalog publishes structured interaction capability metadata
The TypeScript bridge SHALL publish runtime catalog entries with a structured interaction capability matrix in addition to any legacy flat feature list. Each entry MUST describe the runtime's supported input surfaces, lifecycle controls, approval and permission pathways, MCP integration surface, and diagnostics state so upstream consumers can make runtime-aware decisions without inferring behavior from the runtime key alone. The published support state MUST be derived from the same preflight and control-path rules used by execute validation and route handlers, including provider-specific auth/config prerequisites and continuity-dependent controls such as rollback.

#### Scenario: Catalog entry includes grouped interaction capabilities
- **WHEN** the backend or an equivalent upstream consumer requests Bridge runtime metadata
- **THEN** each runtime entry SHALL include machine-readable capability groups for `inputs`, `lifecycle`, `approval`, `mcp`, and `diagnostics`
- **THEN** the existing `supported_features` field MAY remain for compatibility, but it SHALL NOT be the only published interaction contract

#### Scenario: Capability is currently unavailable because prerequisites are missing
- **WHEN** a runtime capability such as Codex rollback, Claude callback hooks, or OpenCode provider auth cannot currently run because required credentials, callback prerequisites, config state, or continuity are absent
- **THEN** the catalog SHALL publish that capability as degraded or unavailable together with actionable diagnostics
- **THEN** upstream consumers SHALL be able to distinguish missing prerequisites from permanent unsupported behavior

#### Scenario: Input surface support is provider-aware
- **WHEN** a runtime only supports a parity-sensitive input for specific provider or config states
- **THEN** the catalog publishes that input as degraded or unsupported with a reason code and actionable guidance
- **THEN** upstream consumers can distinguish “requires provider auth/config” from “permanently unsupported”

### Requirement: OpenCode provider-auth state is published separately from execution reachability
The runtime registry SHALL publish OpenCode provider-auth readiness as its own catalog concern. A provider that is discoverable but disconnected and auth-capable MUST be represented as auth-required rather than collapsed into a generic unavailable runtime state, and the related interaction capability metadata MUST explain whether auth can be started from the Bridge control plane. Any execute inputs or live controls that depend on that provider or config state MUST reference the same auth-required or config-required reason codes in the capability matrix and execute preflight.

#### Scenario: Discoverable provider requires auth before execution
- **WHEN** the OpenCode provider catalog reports a provider that is available, disconnected, and exposes auth methods
- **THEN** the provider entry in the runtime catalog includes `connected=false`, `auth_required=true`, and the published auth methods
- **THEN** the OpenCode interaction capability metadata reports provider auth as degraded with a reason that tells callers authentication is required before execution

#### Scenario: Provider-gated input surface is not silently advertised
- **WHEN** an OpenCode execute input or live control depends on provider auth or config that is not yet ready
- **THEN** the corresponding capability is published as degraded with guidance to start Bridge-managed provider auth or config preparation
- **THEN** execute preflight returns the same reason if a caller still requests that capability
