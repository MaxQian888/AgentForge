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
The canonical typed outbound IM envelope SHALL be resolved through the active provider profile and readiness tier before the Bridge executes transport delivery. Chinese platform deliveries MUST preserve identical provider-aware outcomes and fallback metadata across direct notify, compatibility HTTP, control-plane replay, and action completion. The rendered outcome MUST honor provider-specific limits: Feishu may use native card send or delayed update, DingTalk may use ActionCard send or truthful text fallback, WeCom may use template-card or markdown or text without mutable-update pretence, QQ SHALL resolve to text or link output, and QQ Bot SHALL resolve to markdown or keyboard-first output or explicit text fallback.

#### Scenario: Feishu delivery keeps native card or delayed-update semantics across transports
- **WHEN** a Feishu-targeted typed delivery crosses direct notify, replay, or action-completion paths
- **THEN** the rendering step preserves the same Feishu-native card or delayed-update plan whenever the preserved reply target permits it
- **AND** fallback metadata is emitted only when that native lifecycle cannot be honored

#### Scenario: DingTalk delivery uses ActionCard or explicit fallback consistently
- **WHEN** a DingTalk-targeted typed delivery requests card-like richer output
- **THEN** the rendering step chooses ActionCard delivery when the provider profile and reply target allow it
- **AND** otherwise falls back to text with the same machine-readable downgrade reason regardless of transport path

#### Scenario: WeCom delivery resolves through template-card-aware profile
- **WHEN** a WeCom-targeted typed delivery requests structured or richer content
- **THEN** the rendering step chooses a WeCom-supported template-card, markdown, or text representation according to the active WeCom profile
- **AND** it does not pretend that mutable richer updates are available when only send-time richer payloads are supported

#### Scenario: QQ delivery remains text-first with explicit richer fallback
- **WHEN** a QQ-targeted typed delivery requests structured, native, or mutable-update behavior
- **THEN** the rendering step resolves the delivery into QQ-supported text or link output
- **AND** the delivery receipt records that the richer request degraded because QQ is text-first

#### Scenario: QQ Bot delivery remains markdown-first with explicit update limits
- **WHEN** a QQ Bot-targeted typed delivery requests markdown, keyboard, or richer completion output
- **THEN** the rendering step uses the QQ Bot markdown or keyboard path when the current reply target supports it
- **AND** falls back explicitly when the request requires unsupported mutable-update behavior

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

