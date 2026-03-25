# im-platform-native-interactions Specification

## Purpose
Define the platform-native interaction contract for AgentForge IM Bridge so capability matrices, interactive callbacks, mutable reply targets, and explicit downgrade paths remain consistent across Slack, Discord, Telegram, Feishu, and DingTalk.
## Requirements
### Requirement: Platform capability matrix SHALL describe native interaction strategy, not just transport availability
The system SHALL publish a capability matrix for each active IM platform that describes the platform's native command surface, structured-message surface, callback mode, asynchronous update mode, message scope, and mutability semantics. The Bridge, control-plane registration payload, and health surfaces MUST use this matrix to choose delivery behavior instead of inferring behavior from platform names or from a single rich-message boolean.

#### Scenario: Slack declares threaded block-capable interactions
- **WHEN** the active platform is Slack
- **THEN** the Bridge publishes capability metadata that identifies Socket Mode command intake, Block Kit structured output, threaded reply scope, and response-driven follow-up behavior
- **AND** downstream delivery code can choose thread-aware block responses without hard-coding `"slack"`

#### Scenario: Discord declares deferred interaction lifecycle
- **WHEN** the active platform is Discord
- **THEN** the Bridge publishes capability metadata that identifies public interaction callback requirements, deferred acknowledgement support, and follow-up or original-response mutation behavior
- **AND** asynchronous work can be routed through the correct interaction lifecycle without treating Discord like a generic chat reply target

#### Scenario: Telegram declares mutable text-first updates
- **WHEN** the active platform is Telegram
- **THEN** the Bridge publishes capability metadata that identifies inline-keyboard callbacks, mutable message updates, and text-first rendering constraints
- **AND** progress delivery can prefer low-noise message edits over repeated new messages when the reply target supports it

### Requirement: Asynchronous progress and completion updates SHALL use the platform-native update path before fallback
For IM-initiated long-running actions, the system SHALL prefer the update path that is native to the originating platform and reply target. If a platform supports message edits, follow-ups, thread replies, session webhooks, or delayed card updates, the Bridge MUST use that strategy first. Only when the originating reply target or capability matrix does not support the preferred strategy MAY the system fall back to a new plain-text message.

#### Scenario: Slack progress stays inside the originating thread
- **WHEN** a Slack command or mention starts a long-running action with a preserved `thread_ts` or `response_url`
- **THEN** progress and terminal updates are delivered inside the same thread or response context
- **AND** the Bridge does not create a new top-level channel message unless the preserved target is unavailable

#### Scenario: Discord deferred response becomes follow-up or edit
- **WHEN** a Discord interaction triggers work that cannot complete in the initial response window
- **THEN** the Bridge first satisfies the required deferred acknowledgement
- **AND** later progress or completion uses the interaction follow-up or original-response edit path associated with the preserved interaction token

#### Scenario: Feishu card action prefers immediate or delayed card mutation
- **WHEN** a Feishu card interaction starts a long-running action and the preserved reply target includes card update context
- **THEN** the Bridge uses immediate card response or delayed card update within the provider-supported window before considering a plain-text fallback
- **AND** users remain in the same card conversation context rather than receiving an unrelated duplicate notification

#### Scenario: DingTalk uses session webhook when conversational reply is available
- **WHEN** a DingTalk action or command preserves a session webhook or conversation-scoped reply target
- **THEN** the Bridge uses that session-aware outbound path for progress or completion
- **AND** it falls back to direct-send text only when the session-aware path is unavailable

### Requirement: Platform-native interactive callbacks SHALL normalize into one backend action contract
The system SHALL normalize platform-native interactive inputs into one backend action contract that preserves action identity, entity identity, reply target, bridge identity, and provider-specific metadata. Buttons, modal submissions, select menus, inline keyboard callbacks, and card actions MUST all be convertible into the same action envelope so backend workflows can respond consistently regardless of provider.

#### Scenario: Slack block action preserves response context
- **WHEN** a Slack Block Kit button or modal submission triggers an action
- **THEN** the Bridge normalizes it into the shared action contract with preserved `response_url`, channel, thread, and user context
- **AND** the backend can issue a follow-up update without receiving a Slack-specific payload shape

#### Scenario: Telegram callback query becomes a shared action
- **WHEN** a Telegram inline-keyboard callback query is received
- **THEN** the Bridge normalizes the callback data, chat identity, message identity, and user context into the shared action envelope
- **AND** later completion can edit or reply to the same Telegram message using the preserved target

#### Scenario: Feishu or DingTalk card action carries provider metadata without leaking provider-specific APIs upstream
- **WHEN** a Feishu or DingTalk interactive card action is received
- **THEN** the Bridge normalizes it into the shared backend action contract while preserving provider-specific metadata in the reply target or metadata bag
- **AND** upstream handlers do not need to parse Feishu- or DingTalk-specific callback payloads directly

### Requirement: Unsupported native features SHALL degrade explicitly and truthfully
If a requested structured or interactive behavior is unsupported by the active platform or by the current preserved reply target, the system SHALL degrade to a supported experience explicitly. It MUST avoid emitting unusable buttons, invalid edit attempts, or fake parity claims, and it MUST preserve enough metadata for operators to understand why fallback occurred.

#### Scenario: Telegram avoids unsupported card semantics
- **WHEN** a notification requests a card-style rich message on Telegram
- **THEN** the Bridge renders the supported Telegram text or inline-keyboard variant instead of attempting an unsupported card payload
- **AND** the delivery path records that the fallback occurred because Telegram does not advertise card-style structured output

#### Scenario: Future provider remains unavailable until matrix and adapter are complete
- **WHEN** a provider such as `wecom` is present in roadmap or model-level enums but does not yet have a live adapter and declared capability matrix
- **THEN** the system marks that provider as not yet supported for runtime activation
- **AND** operators can see that the absence is an explicit gap rather than an implicit silent fallback
