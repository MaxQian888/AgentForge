## MODIFIED Requirements

### Requirement: Bridge runtime can start with a supported live platform as the active platform
The IM Bridge SHALL allow a deployment to select exactly one active IM platform provider per process. The runtime SHALL resolve the requested `IM_PLATFORM` through the provider contract so built-in providers such as `feishu`, `slack`, `dingtalk`, `telegram`, and `discord`, plus future plugin-backed providers, share the same startup path. The runtime SHALL validate the required credentials and transport-specific configuration for the selected provider before starting message handling or notification delivery, and SHALL fail with an actionable configuration error instead of silently falling back to another provider or a local stub when the runtime is configured for live transport.

#### Scenario: Feishu bridge starts with valid live configuration
- **WHEN** the bridge is configured with `IM_PLATFORM=feishu` and the required live transport credentials are present
- **THEN** the bridge resolves the Feishu provider through the shared provider contract
- **AND** the existing command engine is registered against the resulting live Feishu adapter

#### Scenario: Telegram bridge starts with valid live configuration
- **WHEN** the bridge is configured with `IM_PLATFORM=telegram` and the required Telegram bot credentials plus update intake configuration are present
- **THEN** the bridge resolves and starts a Telegram live platform provider through the same shared provider contract
- **AND** the bridge does not require another platform-specific adapter to be enabled in the same process

#### Scenario: Selected platform configuration is incomplete
- **WHEN** the bridge is configured for `slack`, `dingtalk`, `telegram`, or `discord` but a required credential or transport parameter is missing
- **THEN** startup fails with an actionable configuration error
- **AND** the bridge does not silently fall back to another platform implementation

#### Scenario: Provider id is recognized in models but not yet registered for runtime activation
- **WHEN** the bridge is configured with a normalized provider id that exists in roadmap or model enums but has no runnable provider descriptor
- **THEN** startup fails with an explicit unsupported-provider error
- **AND** operators can distinguish that explicit gap from a transient configuration failure
