# additional-im-platform-support Specification

## Purpose
Define the live-transport and platform-capability contract for the AgentForge IM Bridge so Feishu, Slack, DingTalk, Telegram, and Discord can run the shared command and notification flows with explicit platform selection, accurate backend source metadata, provider-aware acknowledgement rules, and safe rich-message fallback behavior.
## Requirements
### Requirement: Bridge runtime can start with a supported live platform as the active platform
The IM Bridge SHALL allow a deployment to select exactly one active IM platform per process, and that platform MAY be `feishu`, `slack`, `dingtalk`, `telegram`, or `discord`. The runtime SHALL validate the required credentials and transport-specific configuration for the selected platform before starting message handling or notification delivery, and SHALL fail with an actionable configuration error instead of silently falling back to another platform or a local stub when the runtime is configured for live transport.

#### Scenario: Feishu bridge starts with valid live configuration
- **WHEN** the bridge is configured with `IM_PLATFORM=feishu` and the required live transport credentials are present
- **THEN** the bridge starts a Feishu live platform adapter
- **AND** the existing command engine is registered against that adapter

#### Scenario: Telegram bridge starts with valid live configuration
- **WHEN** the bridge is configured with `IM_PLATFORM=telegram` and the required Telegram bot credentials plus update intake configuration are present
- **THEN** the bridge starts a Telegram live platform adapter
- **AND** the bridge does not require another platform-specific adapter to be enabled in the same process

#### Scenario: Selected platform configuration is incomplete
- **WHEN** the bridge is configured for `slack`, `dingtalk`, `telegram`, or `discord` but a required credential or transport parameter is missing
- **THEN** startup fails with an actionable configuration error
- **AND** the bridge does not silently fall back to another platform implementation

### Requirement: Core command handling remains platform-consistent across supported platforms
The system SHALL translate Feishu, Slack, DingTalk, Telegram, and Discord inbound events or interactions into `core.Message` values that preserve platform identity, user identity, chat identity, reply context, and message content so that the existing `/task`, `/agent`, `/cost`, `/help`, and `@AgentForge` fallback flows execute with consistent command semantics across all supported platforms.

#### Scenario: Telegram slash-style command routes to an existing handler
- **WHEN** a Telegram inbound update is normalized into `core.Message` content containing `/task list`
- **THEN** the engine invokes the registered `/task` command handler
- **AND** the platform sends the resulting reply back to the originating Telegram chat

#### Scenario: Discord interaction is normalized to an existing command
- **WHEN** a Discord application command or interaction maps to the logical command `/agent spawn`
- **THEN** the engine invokes the registered `/agent` command handler with the normalized arguments
- **AND** the resulting response is delivered back through the originating Discord interaction context

#### Scenario: Feishu mention uses the existing fallback path
- **WHEN** a Feishu inbound message mentions `@AgentForge` without matching a registered slash command
- **THEN** the engine invokes the configured fallback handler
- **AND** the platform returns the fallback response to the originating Feishu conversation

### Requirement: Platform source metadata is propagated to backend API calls
IM Bridge requests to the AgentForge backend SHALL identify the actual source platform instead of hardcoding Feishu so that backend audit, routing, notification policy, and downstream analytics can distinguish Feishu, Slack, DingTalk, Telegram, and Discord traffic.

#### Scenario: Telegram command call includes Telegram as source
- **WHEN** a user triggers a backend-backed command from Telegram
- **THEN** the bridge sends the backend request with source metadata identifying `telegram`

#### Scenario: Discord command call includes Discord as source
- **WHEN** a user triggers a backend-backed command from Discord
- **THEN** the bridge sends the backend request with source metadata identifying `discord`

#### Scenario: Active platform source remains stable outside inbound message context
- **WHEN** the bridge issues a backend-backed request from logic that is scoped to the active platform instance but not a specific inbound message
- **THEN** the request still carries the normalized active platform source value

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

#### Scenario: Feishu or Telegram command preserves conversation target
- **WHEN** a Feishu or Telegram message starts a long-running action
- **THEN** the Bridge captures the conversation-specific reply target for that action
- **AND** later updates use the preserved target instead of inferring a destination only from task metadata

### Requirement: Platform metadata exposes delivery-relevant runtime characteristics
The active IM platform runtime SHALL expose the delivery characteristics needed by the control plane, including whether it requires a public callback endpoint, whether it supports deferred follow-up delivery, and whether it supports rich or editable messages for progress updates. Registration and health surfaces MUST reflect those characteristics so the backend can route deliveries and choose the correct update strategy.

#### Scenario: Health and registration reflect callback requirements
- **WHEN** a platform such as Discord requires a public interactions endpoint for live delivery
- **THEN** the Bridge health or registration payload identifies that callback exposure requirement
- **AND** the backend can distinguish it from platforms that do not require a public callback

#### Scenario: Health and registration reflect deferred update capabilities
- **WHEN** a platform supports deferred replies or editable progress updates
- **THEN** the Bridge health or registration payload reports those capabilities
- **AND** the backend can choose a compatible progress delivery strategy for that platform

