## MODIFIED Requirements

### Requirement: Bridge runtime can start with a supported live platform as the active platform
The IM Bridge SHALL allow a deployment to select one or more active IM platform providers per process via the comma-separated `IM_PLATFORMS` env (legacy single-value `IM_PLATFORM` remains accepted as an alias until a subsequent migration). The runtime SHALL resolve each requested provider id through the provider contract so built-in providers such as `feishu`, `slack`, `dingtalk`, `telegram`, `discord`, `wecom`, `qq`, and `qqbot`, plus future plugin-backed providers, share the same startup path. The runtime SHALL validate the required credentials and transport-specific configuration for every selected provider before starting message handling or notification delivery, and SHALL fail with an actionable configuration error identifying the offending provider instead of silently falling back to another provider or a local stub when the runtime is configured for live transport.

#### Scenario: Single Feishu deployment still boots via IM_PLATFORM alias
- **WHEN** the bridge is configured with `IM_PLATFORM=feishu` and the required live transport credentials are present
- **THEN** the bridge resolves the Feishu provider through the shared provider contract and treats the configuration as `IM_PLATFORMS=feishu`
- **AND** the existing command engine is registered against the resulting live Feishu adapter

#### Scenario: Feishu + DingTalk coexist in one process
- **WHEN** the bridge is configured with `IM_PLATFORMS=feishu,dingtalk` and both providers' credentials are present
- **THEN** the bridge starts independent live transports for Feishu and DingTalk
- **AND** each provider's capability matrix, reply plan, rate limiter, and callback receiver are isolated

#### Scenario: Telegram bridge starts with valid live configuration
- **WHEN** the bridge is configured with `IM_PLATFORMS=telegram` and the required Telegram bot credentials plus update intake configuration are present
- **THEN** the bridge resolves and starts a Telegram live platform provider through the same shared provider contract
- **AND** the bridge does not require another platform-specific adapter to be enabled in the same process

#### Scenario: WeCom bridge starts with valid live configuration
- **WHEN** the bridge is configured with `IM_PLATFORMS=wecom` and the required WeCom application credentials plus callback configuration are present
- **THEN** the bridge resolves and starts a WeCom live platform provider through the same shared provider contract
- **AND** health and registration surfaces report WeCom as a supported active platform instead of a planned-only placeholder

#### Scenario: QQ bridge starts with valid live configuration
- **WHEN** the bridge is configured with `IM_PLATFORMS=qq` and the required NapCat or OneBot live transport settings are present
- **THEN** the bridge resolves and starts a QQ live platform provider through the shared provider contract
- **AND** health and registration surfaces report QQ as a supported active platform instead of a documentation-only target

#### Scenario: QQ Bot bridge starts with valid live configuration
- **WHEN** the bridge is configured with `IM_PLATFORMS=qqbot` and the required QQ Bot official credentials plus live transport settings are present
- **THEN** the bridge resolves and starts a QQ Bot live platform provider through the shared provider contract
- **AND** the runtime does not require a separate startup path outside the provider registry

#### Scenario: One misconfigured provider in the set fails the whole process
- **WHEN** the bridge is configured with `IM_PLATFORMS=feishu,dingtalk` but DingTalk is missing a required credential
- **THEN** startup fails with an actionable configuration error naming `dingtalk` and the missing field
- **AND** the bridge does not silently drop DingTalk while keeping Feishu healthy, nor fall back to a stub for DingTalk

#### Scenario: Provider id is recognized in models but not yet registered for runtime activation
- **WHEN** the bridge is configured with a normalized provider id that exists in roadmap or model enums but has no runnable provider descriptor
- **THEN** startup fails with an explicit unsupported-provider error
- **AND** operators can distinguish that explicit gap from a transient configuration failure

### Requirement: Platform source metadata is propagated to backend API calls
IM Bridge requests to the AgentForge backend SHALL identify the actual source platform and the resolved tenant so that backend audit, routing, notification policy, and downstream analytics can distinguish Feishu, Slack, DingTalk, Telegram, Discord, WeCom, QQ, and QQ Bot traffic per tenant. Every backend-bound request originating from an inbound IM message MUST carry both the normalized source platform and the tenant id resolved by `im-bridge-tenant-routing`, and MUST route through the tenant-aware client factory so the request inherits the tenant's `projectId` and credential.

#### Scenario: Telegram command call includes Telegram as source and its tenant
- **WHEN** a user in tenant `acme` triggers a backend-backed command from Telegram
- **THEN** the bridge sends the backend request with source metadata identifying `telegram` and tenant metadata identifying `acme`
- **AND** the request carries `acme`'s `projectId` and credential through the client factory

#### Scenario: Discord command call includes Discord as source
- **WHEN** a user triggers a backend-backed command from Discord
- **THEN** the bridge sends the backend request with source metadata identifying `discord` and the resolved tenant id

#### Scenario: QQ command call includes QQ as source
- **WHEN** a user triggers a backend-backed command from QQ
- **THEN** the bridge sends the backend request with source metadata identifying `qq` and the resolved tenant id

#### Scenario: QQ Bot command call includes QQ Bot as source
- **WHEN** a user triggers a backend-backed command from QQ Bot
- **THEN** the bridge sends the backend request with source metadata identifying `qqbot` and the resolved tenant id

#### Scenario: Active platform source remains stable outside inbound message context
- **WHEN** the bridge issues a backend-backed request from logic that is scoped to an active provider instance but not a specific inbound message (for example a periodic health push)
- **THEN** the request still carries the normalized active platform source value and the bridge-level tenant scope (or an explicit `tenantId` omission flag for bridge-global operations)

## ADDED Requirements

### Requirement: Provider and tenant binding SHALL be declared together in registration payloads

Each provider descriptor published to the control plane SHALL carry the list of tenants it serves so the backend can index the bridge by the full `(bridgeId, providerId, tenantId)` triple described in `im-bridge-control-plane`. The provider registration payload MUST include a `tenants` array referencing the top-level tenant ids declared in the same registration, and MUST NOT advertise a tenant id that is not present at the top level.

#### Scenario: Feishu provider serves only ACME
- **WHEN** the bridge registers Feishu with `tenants=["acme"]` and top-level tenant array `[{id:acme,...},{id:beta,...}]`
- **THEN** the backend records `(bridgeId, feishu, acme)` as valid and does not create a `(bridgeId, feishu, beta)` entry
- **AND** a later Feishu-targeted delivery scoped to `beta` is rejected at the backend selector

#### Scenario: Provider lists an unknown tenant id
- **WHEN** the bridge registers DingTalk with `tenants=["gamma"]` but omits `gamma` from the top-level tenant array
- **THEN** the backend rejects the registration with a validation error
- **AND** no `(bridgeId, dingtalk, gamma)` entry is created
