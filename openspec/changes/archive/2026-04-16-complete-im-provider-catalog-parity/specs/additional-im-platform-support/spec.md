## MODIFIED Requirements

### Requirement: Bridge runtime can start with a supported live platform as the active platform
The IM Bridge SHALL allow a deployment to select exactly one active IM platform provider per process. The runtime SHALL resolve the requested `IM_PLATFORM` through the provider contract so built-in providers such as `feishu`, `slack`, `dingtalk`, `telegram`, `discord`, `wecom`, `qq`, `qqbot`, `wechat`, and `email`, plus future plugin-backed providers, share the same startup path. The runtime SHALL validate the required credentials and transport-specific configuration for the selected provider before starting message handling or notification delivery, and SHALL fail with an actionable configuration error instead of silently falling back to another provider or a local stub when the runtime is configured for live transport.

#### Scenario: Feishu bridge starts with valid live configuration
- **WHEN** the bridge is configured with `IM_PLATFORM=feishu` and the required live transport credentials are present
- **THEN** the bridge resolves the Feishu provider through the shared provider contract
- **AND** the existing command engine is registered against the resulting live Feishu adapter

#### Scenario: Telegram bridge starts with valid live configuration
- **WHEN** the bridge is configured with `IM_PLATFORM=telegram` and the required Telegram bot credentials plus update intake configuration are present
- **THEN** the bridge resolves and starts a Telegram live platform provider through the same shared provider contract
- **AND** the bridge does not require another platform-specific adapter to be enabled in the same process

#### Scenario: WeCom bridge starts with valid live configuration
- **WHEN** the bridge is configured with `IM_PLATFORM=wecom` and the required WeCom application credentials plus callback configuration are present
- **THEN** the bridge resolves and starts a WeCom live platform provider through the same shared provider contract
- **AND** health and registration surfaces report WeCom as a supported active platform instead of a planned-only placeholder

#### Scenario: QQ bridge starts with valid live configuration
- **WHEN** the bridge is configured with `IM_PLATFORM=qq` and the required NapCat or OneBot live transport settings are present
- **THEN** the bridge resolves and starts a QQ live platform provider through the shared provider contract
- **AND** health and registration surfaces report QQ as a supported active platform instead of a documentation-only target

#### Scenario: QQ Bot bridge starts with valid live configuration
- **WHEN** the bridge is configured with `IM_PLATFORM=qqbot` and the required QQ Bot official credentials plus live transport settings are present
- **THEN** the bridge resolves and starts a QQ Bot live platform provider through the shared provider contract
- **AND** the runtime does not require a separate startup path outside the provider registry

#### Scenario: WeChat bridge starts with valid live configuration
- **WHEN** the bridge is configured with `IM_PLATFORM=wechat` and the required WeChat app credentials plus callback token are present
- **THEN** the bridge resolves and starts a WeChat live platform provider through the shared provider contract
- **AND** health and registration surfaces report WeChat as a supported active platform instead of an unexposed bridge-only implementation

#### Scenario: Email bridge starts with valid live configuration
- **WHEN** the bridge is configured with `IM_PLATFORM=email` and the required SMTP host plus from-address credentials are present
- **THEN** the bridge resolves and starts an Email live platform provider through the shared provider contract
- **AND** runtime metadata reports Email as a delivery-capable built-in provider without claiming interactive command parity

#### Scenario: Selected platform configuration is incomplete
- **WHEN** the bridge is configured for `slack`, `dingtalk`, `telegram`, `discord`, `wecom`, `qq`, `qqbot`, `wechat`, or `email` but a required credential or transport parameter is missing
- **THEN** startup fails with an actionable configuration error
- **AND** the bridge does not silently fall back to another platform implementation

#### Scenario: Provider id is recognized in models but not yet registered for runtime activation
- **WHEN** the bridge is configured with a normalized provider id that exists in roadmap or model enums but has no runnable provider descriptor
- **THEN** startup fails with an explicit unsupported-provider error
- **AND** operators can distinguish that explicit gap from a transient configuration failure

### Requirement: Core command handling remains platform-consistent across supported platforms
The system SHALL translate Feishu, Slack, DingTalk, Telegram, Discord, WeCom, QQ, QQ Bot, and WeChat inbound events or interactions into `core.Message` values that preserve platform identity, user identity, chat identity, reply context, and message content so that the canonical operator command surface executes with consistent semantics across all supported interactive platforms. This surface MUST include existing task, agent, review, sprint, cost, help, and `@AgentForge` flows plus newly approved operator commands such as agent runtime control, task workflow control, queue visibility, team summary, and memory access. Delivery-only providers such as Email MUST NOT advertise slash-command, mention, or callback parity that the active platform cannot honor.

#### Scenario: Telegram slash-style task workflow command routes to a shared handler
- **WHEN** a Telegram inbound update is normalized into `core.Message` content containing `/task move task-123 done`
- **THEN** the engine invokes the registered `/task` command handler with the normalized subcommand and args
- **AND** the resulting response is sent back to the originating Telegram chat

#### Scenario: Discord interaction is normalized to an agent control command
- **WHEN** a Discord application command or interaction maps to the logical command `/agent status run-123`
- **THEN** the engine invokes the registered `/agent` command handler with the normalized args
- **AND** the response is delivered through the originating Discord interaction context

#### Scenario: WeCom callback event routes to the queue command
- **WHEN** a WeCom inbound callback or application message is normalized into `core.Message` content containing `/queue list queued`
- **THEN** the engine invokes the registered `/queue` command handler through the shared command path
- **AND** the resulting response is sent back to the originating WeCom conversation context

#### Scenario: QQ group command routes to the memory command
- **WHEN** a QQ inbound group or direct message is normalized into `core.Message` content containing `/memory search release`
- **THEN** the engine invokes the registered `/memory` command handler through the shared command path
- **AND** the resulting response is sent back to the originating QQ conversation context

#### Scenario: QQ Bot command routes to the team summary command
- **WHEN** a QQ Bot official inbound message or interaction is normalized into `core.Message` content containing `/team list`
- **THEN** the engine invokes the registered `/team` command handler through the shared command path
- **AND** the resulting response is sent back to the originating QQ Bot conversation context

#### Scenario: WeChat callback event routes to the task command
- **WHEN** a WeChat inbound callback or customer-service message is normalized into `core.Message` content containing `/task status task-123`
- **THEN** the engine invokes the registered `/task` command handler through the shared command path
- **AND** the resulting response is sent back to the originating WeChat conversation context

#### Scenario: Feishu mention uses the existing fallback path
- **WHEN** a Feishu inbound message mentions `@AgentForge` without matching a registered slash command
- **THEN** the engine invokes the configured fallback handler
- **AND** any command guidance returned to the user references a command from the canonical operator catalog

#### Scenario: Email does not claim interactive command parity
- **WHEN** the active provider is Email
- **THEN** the bridge does not register Email as a slash-command, mention, or callback-driven interactive command source
- **AND** operator metadata exposes Email as delivery-only instead of fabricating interactive command support

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
- **THEN** the bridge uses the documented WeCom callback or event intake and application-message delivery model rather than a synthetic polling loop
- **AND** the deployment documentation reflects the required callback exposure, token exchange, and supported update semantics for that path

#### Scenario: QQ live transport uses OneBot-compatible event and send semantics
- **WHEN** a QQ deployment is configured for live transport
- **THEN** the bridge uses the documented NapCat or OneBot-compatible event intake and message-send model rather than a synthetic polling loop
- **AND** the deployment documentation reflects the required socket or callback exposure, credential handling, and supported update semantics for that path

#### Scenario: QQ Bot live transport uses official event and reply semantics
- **WHEN** a QQ Bot official application is configured for live transport
- **THEN** the bridge uses the documented QQ Bot event intake and reply model rather than a synthetic polling loop
- **AND** the deployment documentation reflects the required app credentials, callback or websocket contract, and supported follow-up semantics for that path

#### Scenario: WeChat live transport uses callback-driven customer-service semantics
- **WHEN** a WeChat provider is configured for live transport
- **THEN** the bridge uses the documented WeChat callback intake plus customer-service or app-message delivery model rather than inventing a synthetic polling loop
- **AND** the deployment documentation reflects the required callback token, callback exposure, and reply constraints for that path

#### Scenario: Email live transport uses SMTP delivery semantics
- **WHEN** an Email provider is configured for live transport
- **THEN** the bridge sends outbound delivery through the configured SMTP transport rather than pretending webhook or callback-based interaction support
- **AND** the deployment documentation reflects SMTP credential requirements and the absence of inbound callback-driven command intake

#### Scenario: Feishu live transport prefers long connection where supported
- **WHEN** a Feishu enterprise self-built application is configured for live transport
- **THEN** the bridge uses Feishu long connection as the preferred event intake mode for supported event or callback types
- **AND** documents any callback types that still require an HTTP endpoint

### Requirement: Platform metadata exposes delivery-relevant runtime characteristics
The active IM platform runtime SHALL expose the delivery characteristics, command surface truth, and readiness tier needed by health, registration, control-plane routing, and operator documentation. Registration and health surfaces MUST distinguish whether a platform is `full_native_lifecycle`, `native_send_with_fallback`, `text_first`, `markdown_first`, or `delivery_only`, and MUST keep that truth aligned with the provider's actual callback, mutable-update, structured-rendering, and reply-target behavior instead of implying flat parity across all supported platforms.

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

#### Scenario: WeChat exposes text-first interactive runtime truth
- **WHEN** the active platform is WeChat
- **THEN** health and registration metadata report WeChat as an interactive platform with callback-driven reply semantics
- **AND** the capability matrix keeps its readiness truthful as text-first rather than implying richer mutable-card parity

#### Scenario: Email exposes delivery-only runtime truth
- **WHEN** the active platform is Email
- **THEN** health and registration metadata report Email as `delivery_only`
- **AND** the capability matrix exposes outbound delivery support without claiming slash-command, mention, or callback-driven interaction semantics
