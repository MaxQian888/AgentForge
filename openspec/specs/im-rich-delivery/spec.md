# im-rich-delivery Specification

## Purpose
Define the canonical typed outbound IM delivery contract so text, structured, and provider-native payloads survive compatibility HTTP, control-plane queueing, replay, and explicit fallback reporting consistently.
## Requirements
### Requirement: Outbound IM deliveries SHALL use a canonical typed envelope
The system SHALL represent outbound IM delivery through a canonical typed envelope instead of a text-only payload. That envelope MUST be able to carry plain text, structured content, provider-native content, reply-target information, delivery kind, and operator-visible fallback metadata so the same outbound message semantics survive direct notify, compatibility HTTP, queueing, and replay.

#### Scenario: Control-plane delivery preserves provider-native payload
- **WHEN** the backend queues a delivery that contains provider-native content and a matching reply target
- **THEN** the queued delivery retains that native payload as typed data instead of flattening it into plain text
- **AND** the Bridge can choose the provider-native delivery path when the active platform supports it

#### Scenario: Structured delivery survives queueing and replay
- **WHEN** the backend queues a structured notification or terminal update for later replay
- **THEN** the canonical envelope retains the structured payload, reply target, and delivery metadata intact
- **AND** reconnect replay does not reduce the payload to a plain-text surrogate unless an explicit fallback decision is made at delivery time

### Requirement: Rich delivery fallback SHALL be explicit and transport-consistent
If a structured or provider-native payload cannot be delivered through the requested platform or preserved reply target, the system SHALL apply the same fallback semantics regardless of whether the message is sent through control plane replay or compatibility HTTP. The resulting delivery metadata MUST make the fallback reason visible to operators and downstream diagnostics.

#### Scenario: Compatibility HTTP uses the same fallback semantics as control-plane replay
- **WHEN** a caller sends a rich IM delivery through compatibility `/im/notify` or `/im/send`
- **THEN** the backend and Bridge resolve the payload through the same canonical delivery rules used by queued control-plane delivery
- **AND** the resulting fallback reason matches the control-plane path for the same payload and reply target

#### Scenario: Unsupported native update reports a truthful fallback reason
- **WHEN** a typed delivery requests a native or mutable update path that the active platform or restored reply context cannot honor
- **THEN** the Bridge falls back to the supported structured or text path
- **AND** the canonical delivery result exposes a fallback reason instead of silently pretending native delivery succeeded

### Requirement: Canonical rich delivery SHALL be rendered through the active provider profile
The canonical typed outbound IM envelope SHALL be resolved through the active provider's rendering profile before the Bridge executes transport delivery. That rendering step MUST choose the final provider-facing representation for native payloads (including new platform-specific native payloads beyond Feishu), structured payloads (including section-based structured messages), formatted text (including new platform markdown formats), segmented text, or explicit downgrade so the same typed envelope produces the same provider-aware outcome across direct notify, compatibility HTTP, queueing, and replay.

#### Scenario: Telegram structured delivery becomes text plus inline keyboard
- **WHEN** a typed delivery containing structured content targets Telegram
- **THEN** the rendering step resolves the delivery into Telegram-supported text plus inline-keyboard output instead of an unsupported card payload
- **AND** the resulting delivery receipt records that Telegram-native structured rendering was chosen through the provider profile

#### Scenario: WeCom typed delivery becomes supported app-message content
- **WHEN** a typed delivery containing structured or richer card intent targets WeCom
- **THEN** the rendering step resolves the delivery into a WeCom-supported text, news-style, or template-card-compatible representation according to the active WeCom provider profile
- **AND** transport execution does not require shared layers to special-case WeCom outside the provider contract

#### Scenario: Slack native Block Kit delivery dispatched through rendering plan
- **WHEN** a typed delivery containing a Slack Block Kit native payload targets Slack
- **THEN** the rendering step selects the native delivery path and dispatches through the Slack adapter's `NativeMessageSender`
- **AND** the delivery receipt records `type: "native"` without fallback

#### Scenario: Discord native embed delivery dispatched through rendering plan
- **WHEN** a typed delivery containing a Discord embed native payload targets Discord
- **THEN** the rendering step selects the native delivery path and dispatches through the Discord adapter's `NativeMessageSender`
- **AND** the delivery receipt records `type: "native"` without fallback

#### Scenario: DingTalk native card delivery dispatched through rendering plan
- **WHEN** a typed delivery containing a DingTalk card native payload targets DingTalk
- **THEN** the rendering step selects the native delivery path and dispatches through the DingTalk adapter's `NativeMessageSender`

#### Scenario: Native payload targeting wrong platform uses fallback text
- **WHEN** a typed delivery containing a Slack Block Kit native payload targets a Telegram bridge
- **THEN** the rendering step falls back to text delivery using the payload's `FallbackText()`
- **AND** the delivery receipt records `fallback_reason: "native_platform_mismatch"`

#### Scenario: Section-based structured message renders richer platform output
- **WHEN** a typed delivery containing a `StructuredMessage` with non-empty `Sections` targets Slack
- **THEN** the rendering step converts sections to Slack Block Kit blocks (section, image, divider, context, actions)
- **AND** the result is richer than the legacy title+body+fields rendering

#### Scenario: Feishu typed delivery becomes builder-owned native content
- **WHEN** a typed delivery containing richer card intent targets Feishu
- **THEN** the rendering step resolves the delivery through Feishu's provider-owned builders into JSON-card, template-card, or `lark_md`-backed output as appropriate
- **AND** transport execution does not require shared layers to assemble raw Feishu payload fragments directly

#### Scenario: QQ typed delivery becomes provider-supported QQ content
- **WHEN** a typed delivery containing structured or richer intent targets QQ
- **THEN** the rendering step resolves the delivery into a QQ-supported text, link, or provider-supported structured representation according to the active QQ provider profile
- **AND** transport execution does not require shared layers to special-case QQ outside the provider contract

#### Scenario: QQ Bot typed delivery becomes provider-supported QQ Bot content
- **WHEN** a typed delivery containing structured or richer intent targets QQ Bot
- **THEN** the rendering step resolves the delivery into a QQ Bot-supported text, link, or provider-supported structured representation according to the active QQ Bot provider profile
- **AND** transport execution does not require shared layers to special-case QQ Bot outside the provider contract

### Requirement: Delivery fallback metadata SHALL reflect rendering-profile decisions

When the rendering profile changes the delivery method (e.g., card → text), the delivery record SHALL preserve provider-aware fallback metadata. The metadata SHALL include: original intended format, actual delivered format, reason for fallback, and provider name.

The delivery record persisted by the backend SHALL include a `downgrade_reason` field populated from the bridge ack. The `ListDeliveries` API response SHALL expose this field. The bridge control-plane ack message SHALL accept an optional `downgrade_reason` string.

#### Scenario: Unsafe markdown falls back to plain text with reason
- **WHEN** rendering profile determines markdown is unsafe for the target provider
- **THEN** delivery executes as plain text and fallback metadata records `"markdown_unsafe → plain_text"`

#### Scenario: Unsupported card delivery falls back with explicit reason
- **WHEN** a card-typed delivery targets a provider with `card: false`
- **THEN** delivery executes as structured text and fallback metadata records `"card_unsupported → structured_text"`

#### Scenario: Bridge ack carries downgrade reason to backend
- **WHEN** bridge sends delivery ack with `downgradeReason: "actioncard_send_failed"`
- **THEN** backend persists `downgrade_reason` on the delivery record and returns it in subsequent `ListDeliveries` responses

### Requirement: Backend SHALL expose event types endpoint

`GET /im/event-types` SHALL return the canonical list of subscribable event types. This endpoint SHALL be used by the frontend to dynamically render event subscription checkboxes instead of hardcoding.

#### Scenario: Fetching event types
- **WHEN** frontend calls `GET /api/v1/im/event-types`
- **THEN** the response includes `["task.created", "task.completed", "review.completed", "agent.started", "agent.completed", "budget.warning", "sprint.started", "sprint.completed", "review.requested", "workflow.failed"]`

