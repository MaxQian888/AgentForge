# additional-im-platform-support Specification

## Purpose
Define the live-transport and platform-capability contract for the AgentForge IM Bridge so Feishu, Slack, DingTalk, Telegram, Discord, WeCom, QQ, and QQ Bot can run the shared command and notification flows with explicit platform selection, accurate backend source metadata, provider-aware acknowledgement rules, and safe rich-message fallback behavior.
## Requirements
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

### Requirement: Core command handling remains platform-consistent across supported platforms
The system SHALL translate Feishu, Slack, DingTalk, Telegram, Discord, WeCom, QQ, and QQ Bot inbound events or interactions into core.Message values that preserve platform identity, user identity, chat identity, reply context, and message content so that the canonical operator command surface executes with consistent semantics across all supported platforms. This surface MUST include existing task, agent, review, sprint, cost, help, and @AgentForge flows plus newly approved operator commands such as agent runtime control, task workflow control, queue visibility, team summary, and memory access. Platform adapters MUST NOT special-case command names or silently drop these supported commands because the active platform entered through slash, mention, callback, or interaction normalization.

#### Scenario: Telegram slash-style task workflow command routes to a shared handler
- **WHEN** a Telegram inbound update is normalized into core.Message content containing /task move task-123 done
- **THEN** the engine invokes the registered /task command handler with the normalized subcommand and args
- **AND** the resulting response is sent back to the originating Telegram chat

#### Scenario: Discord interaction is normalized to an agent control command
- **WHEN** a Discord application command or interaction maps to the logical command /agent status run-123
- **THEN** the engine invokes the registered /agent command handler with the normalized args
- **AND** the response is delivered through the originating Discord interaction context

#### Scenario: WeCom callback event routes to the queue command
- **WHEN** a WeCom inbound callback or application message is normalized into core.Message content containing /queue list queued
- **THEN** the engine invokes the registered /queue command handler through the shared command path
- **AND** the resulting response is sent back to the originating WeCom conversation context

#### Scenario: QQ group command routes to the memory command
- **WHEN** a QQ inbound group or direct message is normalized into core.Message content containing /memory search release
- **THEN** the engine invokes the registered /memory command handler through the shared command path
- **AND** the resulting response is sent back to the originating QQ conversation context

#### Scenario: QQ Bot command routes to the team summary command
- **WHEN** a QQ Bot official inbound message or interaction is normalized into core.Message content containing /team list
- **THEN** the engine invokes the registered /team command handler through the shared command path
- **AND** the resulting response is sent back to the originating QQ Bot conversation context

#### Scenario: Feishu mention uses the existing fallback path
- **WHEN** a Feishu inbound message mentions @AgentForge without matching a registered slash command
- **THEN** the engine invokes the configured fallback handler
- **AND** any command guidance returned to the user references a command from the canonical operator catalog

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

### Requirement: Notifications respect platform matching and capability-aware rich-message fallback
The notification receiver SHALL only deliver a notification through the active platform instance when the notification platform matches the running bridge platform. If a notification includes structured content, update context, or interaction affordances, the Bridge MUST first choose the native renderer and update path declared by the active platform's capability matrix. When the active platform and preserved reply target support that native path, the Bridge SHALL send the structured or mutable response in the same platform-native context. Otherwise, it SHALL fall back to the supported plain-text or text-plus-link variant instead of emitting unsupported controls or invalid message mutations.

#### Scenario: Matching Slack delivery uses native threaded blocks
- **WHEN** the notification receiver receives a notification whose platform matches the active Slack bridge
- **AND** the notification contains structured content with a preserved thread-aware reply target
- **AND** the Slack capability matrix declares Block Kit rendering and threaded follow-up support
- **THEN** the Bridge sends the structured notification back into the same Slack thread using the native Slack renderer

#### Scenario: Matching Discord delivery uses interaction-aware update semantics
- **WHEN** the notification receiver receives a notification whose platform matches the active Discord bridge
- **AND** the notification contains a preserved interaction target that supports deferred follow-up or original-response editing
- **THEN** the Bridge delivers the update through the native Discord interaction path
- **AND** it does not fall back to an unrelated plain chat send unless the preserved target is unusable

#### Scenario: Matching WeCom delivery uses supported app-message rendering
- **WHEN** the notification receiver receives a notification whose platform matches the active WeCom bridge
- **AND** the notification contains structured or card-oriented content with a preserved WeCom reply target
- **THEN** the Bridge resolves the delivery through the WeCom rendering profile into a supported app message or template-card path
- **AND** it falls back to WeCom-supported plain text with explicit fallback metadata when the richer path cannot be honored

#### Scenario: Matching QQ delivery uses declared QQ rendering profile
- **WHEN** the notification receiver receives a notification whose platform matches the active QQ bridge
- **AND** the notification contains structured or richer content with a preserved QQ reply target
- **THEN** the Bridge resolves the delivery through the QQ rendering profile into a QQ-supported send or reply path
- **AND** it falls back to QQ-supported text or link output with explicit fallback metadata when the richer path cannot be honored

#### Scenario: Matching QQ Bot delivery uses declared QQ Bot rendering profile
- **WHEN** the notification receiver receives a notification whose platform matches the active QQ Bot bridge
- **AND** the notification contains structured or richer content with a preserved QQ Bot reply target
- **THEN** the Bridge resolves the delivery through the QQ Bot rendering profile into a QQ Bot-supported send or reply path
- **AND** it falls back to QQ Bot-supported text or link output with explicit fallback metadata when the richer path cannot be honored

#### Scenario: Matching platform without the required native capability falls back cleanly
- **WHEN** the notification receiver receives a notification whose platform matches the active bridge platform
- **AND** the notification requests structured or mutable behavior that the active platform or preserved reply target does not support
- **THEN** the Bridge sends the supported plain-text or minimally interactive fallback instead
- **AND** it does not emit buttons, cards, or edit attempts that the active platform cannot honor

#### Scenario: Mismatched platform notification is rejected
- **WHEN** the notification receiver receives a notification whose platform does not match the active bridge platform
- **THEN** the bridge rejects the delivery request with an explicit error
- **AND** the notification is not sent to the wrong IM platform

### Requirement: Live transports honor the official delivery model of the selected platform
The bridge SHALL implement the live transport of each supported platform according to that platform's official delivery contract so that events, commands, and replies remain reliable under reconnect, retry, acknowledgement, and callback timing constraints.

#### Scenario: Slack Socket Mode payload is acknowledged before command completion
- **WHEN** a Slack Socket Mode envelope containing a command or interaction is received
- **THEN** the bridge acknowledges the Slack envelope according to the Socket Mode contract
- **AND** command execution may continue after the acknowledgement is sent

#### Scenario: DingTalk live transport uses Stream mode by default
- **WHEN** a DingTalk live deployment is created without an explicit override
- **THEN** the bridge uses DingTalk Stream mode as the primary event intake mechanism
- **AND** the deployment documentation reflects Stream mode as the default path

#### Scenario: Telegram update intake chooses exactly one official model
- **WHEN** a Telegram live deployment is configured
- **THEN** the bridge uses exactly one update intake model from `getUpdates` long polling or `setWebhook`
- **AND** the bridge rejects configurations that attempt to enable both models at the same time

#### Scenario: Discord interaction meets provider response deadlines
- **WHEN** a Discord interaction triggers a bridge command that cannot finish immediately
- **THEN** the bridge sends the required initial interaction acknowledgement within the provider deadline
- **AND** completes the user-visible response through the permitted follow-up interaction path

#### Scenario: WeCom live transport uses callback and application-message semantics
- **WHEN** a WeCom enterprise application is configured for live transport
- **THEN** the bridge uses the documented WeCom callback/event intake and application-message delivery model rather than a synthetic polling loop
- **AND** the deployment documentation reflects the required callback exposure, token exchange, and supported update semantics for that path

#### Scenario: QQ live transport uses OneBot-compatible event and send semantics
- **WHEN** a QQ deployment is configured for live transport
- **THEN** the bridge uses the documented NapCat or OneBot-compatible event intake and message-send model rather than a synthetic polling loop
- **AND** the deployment documentation reflects the required socket or callback exposure, credential handling, and supported update semantics for that path

#### Scenario: QQ Bot live transport uses official event and reply semantics
- **WHEN** a QQ Bot official application is configured for live transport
- **THEN** the bridge uses the documented QQ Bot event intake and reply model rather than a synthetic polling loop
- **AND** the deployment documentation reflects the required app credentials, callback or websocket contract, and supported follow-up semantics for that path

#### Scenario: Feishu live transport prefers long connection where supported
- **WHEN** a Feishu enterprise self-built application is configured for live transport
- **THEN** the bridge uses Feishu long connection as the preferred event intake mode for supported event or callback types
- **AND** documents any callback types that still require an HTTP endpoint

### Requirement: Platform adapters preserve deferred reply targets for later updates
The system SHALL preserve enough platform-specific reply target data when normalizing an inbound message or interaction so later progress and terminal updates can be delivered back to the same user-visible context. This preserved target MUST be serializable, MUST survive handoff to the backend, and MUST distinguish between plain chat replies, threaded replies, and provider-specific deferred follow-up contexts where applicable.

#### Scenario: Slack command preserves threaded reply target
- **WHEN** a Slack command or mention is normalized into a `core.Message`
- **THEN** the Bridge captures the channel and thread-aware reply target needed for later updates
- **AND** that target can be serialized and reused for asynchronous progress delivery

#### Scenario: Discord interaction preserves follow-up context
- **WHEN** a Discord interaction is normalized for shared command handling
- **THEN** the Bridge captures the interaction follow-up context required after the initial acknowledgement
- **AND** that context remains available for later completion messages

#### Scenario: QQ command preserves conversation target
- **WHEN** a QQ message starts a long-running action
- **THEN** the Bridge captures the QQ conversation-specific reply target needed for later updates
- **AND** later updates use the preserved target instead of inferring a destination only from task metadata

#### Scenario: QQ Bot command preserves conversation target
- **WHEN** a QQ Bot message starts a long-running action
- **THEN** the Bridge captures the QQ Bot conversation-specific reply target needed for later updates
- **AND** later updates use the preserved target instead of inferring a destination only from task metadata

#### Scenario: Feishu or Telegram command preserves conversation target
- **WHEN** a Feishu or Telegram message starts a long-running action
- **THEN** the Bridge captures the conversation-specific reply target for that action
- **AND** later updates use the preserved target instead of inferring a destination only from task metadata

### Requirement: Platform metadata exposes delivery-relevant runtime characteristics
The active IM platform runtime SHALL expose the delivery characteristics and readiness tier needed by health, registration, control-plane routing, and operator documentation. Registration and health surfaces MUST distinguish whether a platform is `full_native_lifecycle`, `native_send_with_fallback`, `text_first`, or `markdown_first`, and MUST keep that tier aligned with the provider's actual callback, mutable-update, structured-rendering, and reply-target behavior instead of implying flat parity across all supported Chinese platforms.

#### Scenario: Feishu exposes full native lifecycle metadata
- **WHEN** the active platform is Feishu
- **THEN** health and registration metadata report readiness tier `full_native_lifecycle`
- **AND** the capability matrix indicates native callback response plus delayed card update support

#### Scenario: DingTalk and WeCom expose native send without mutable card parity
- **WHEN** the active platform is DingTalk or WeCom
- **THEN** health and registration metadata report readiness tier `native_send_with_fallback`
- **AND** the capability matrix indicates provider-native send and callback semantics without claiming Feishu-style mutable card updates

#### Scenario: QQ exposes text-first runtime truth
- **WHEN** the active platform is QQ
- **THEN** health and registration metadata report readiness tier `text_first`
- **AND** the capability matrix does not advertise native payload surfaces or mutable update support

#### Scenario: QQ Bot exposes markdown-first runtime truth
- **WHEN** the active platform is QQ Bot
- **THEN** health and registration metadata report readiness tier `markdown_first`
- **AND** the capability matrix indicates markdown or keyboard send support without claiming full rich-card lifecycle parity

### Requirement: Chinese-platform registration and health SHALL expose async completion truth
The active IM platform runtime SHALL expose enough metadata for operators and downstream routing to distinguish whether DingTalk, WeCom, QQ Bot, or QQ can complete long-running work through provider-native reply or update paths, reply-only paths, or text-first fallback. Health, registration, and capability matrix payloads MUST keep this async completion truth aligned with the provider's actual reply target restoration and replay behavior instead of collapsing those platforms into one generic `native_send_with_fallback` label.

#### Scenario: DingTalk publishes session-webhook completion truth
- **WHEN** the active platform is DingTalk
- **THEN** health and registration metadata expose that asynchronous completion prefers session webhook or conversation-scoped reply
- **AND** the capability matrix does not imply delayed mutable-card update parity with Feishu

#### Scenario: WeCom publishes reply-first completion truth
- **WHEN** the active platform is WeCom
- **THEN** health and registration metadata expose that asynchronous completion prefers preserved `response_url` and may fall back to direct app send
- **AND** the capability matrix makes that fallback boundary visible to operators

#### Scenario: QQ Bot publishes msg-id-aware completion truth
- **WHEN** the active platform is QQ Bot
- **THEN** health and registration metadata expose that asynchronous completion may reuse preserved `msg_id` or conversation context for markdown or text follow-up
- **AND** the capability matrix does not claim mutable native update support unless the adapter truly implements it

#### Scenario: QQ remains text-first in completion metadata
- **WHEN** the active platform is QQ
- **THEN** health and registration metadata continue to report QQ as `text_first`
- **AND** the capability matrix describes reply reuse and text fallback without advertising richer update surfaces

### Requirement: DingTalk and WeCom SHALL declare full_native_lifecycle with explicit mutable update methods

The DingTalk provider SHALL advertise `ReadinessTier=full_native_lifecycle` with `MutableUpdateMethod=openapi_only` so operators know mutable updates apply only to cards originally sent via DingTalk OpenAPI (webhook-origin cards are not mutable). The WeCom provider SHALL advertise `ReadinessTier=full_native_lifecycle` with `MutableUpdateMethod=template_card_update` so operators know mutable updates flow through the template-card API.

#### Scenario: DingTalk reports openapi_only mutable update method
- **WHEN** Bridge registers with the control plane using the DingTalk live transport
- **THEN** the registration payload carries `readiness_tier=full_native_lifecycle` and `capability_matrix.mutableUpdateMethod=openapi_only`

#### Scenario: WeCom reports template_card_update mutable update method
- **WHEN** Bridge registers with the control plane using the WeCom live transport
- **THEN** the registration payload carries `readiness_tier=full_native_lifecycle` and `capability_matrix.mutableUpdateMethod=template_card_update`

### Requirement: QQ Bot SHALL declare native_send_with_fallback tier with OpenAPI PATCH mutability

The QQ Bot provider SHALL advertise `ReadinessTier=native_send_with_fallback` with `MutableUpdateMethod=openapi_patch` to reflect that markdown messages dispatched via OpenAPI can be updated through the `PATCH /messages/{id}` endpoint. Mutable update requests targeting webhook-origin messages MUST degrade with `fallback_reason`.

#### Scenario: QQ Bot registration advertises openapi_patch mutability
- **WHEN** Bridge registers with the QQ Bot live transport
- **THEN** the registration payload carries `readiness_tier=native_send_with_fallback` and `capability_matrix.mutableUpdateMethod=openapi_patch`

### Requirement: QQ (OneBot) SHALL declare simulated mutable update truthfully

The QQ provider SHALL advertise `MutableUpdateMethod=simulated` so operators and backend consumers know that mutable updates are implemented as "delete old + send new + preserve thread context" rather than a native edit API. The readiness tier SHALL remain `text_first` so no richer-delivery pretence enters the catalog.

#### Scenario: QQ registration advertises simulated mutable update
- **WHEN** Bridge registers with the QQ OneBot live transport
- **THEN** the registration payload carries `capability_matrix.mutableUpdateMethod=simulated`

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

