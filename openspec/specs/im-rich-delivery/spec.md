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
The canonical typed outbound IM envelope SHALL be resolved through the active provider's rendering profile before the Bridge executes transport delivery. That rendering step MUST choose the final provider-facing representation for native payloads, structured payloads, formatted text, segmented text, or explicit downgrade so the same typed envelope produces the same provider-aware outcome across direct notify, compatibility HTTP, queueing, and replay.

#### Scenario: Telegram structured delivery becomes text plus inline keyboard
- **WHEN** a typed delivery containing structured content targets Telegram
- **THEN** the rendering step resolves the delivery into Telegram-supported text plus inline-keyboard output instead of an unsupported card payload
- **AND** the resulting delivery receipt records that Telegram-native structured rendering was chosen through the provider profile

#### Scenario: Feishu typed delivery becomes builder-owned native content
- **WHEN** a typed delivery containing richer card intent targets Feishu
- **THEN** the rendering step resolves the delivery through Feishu's provider-owned builders into JSON-card, template-card, or `lark_md`-backed output as appropriate
- **AND** transport execution does not require shared layers to assemble raw Feishu payload fragments directly

### Requirement: Delivery fallback metadata SHALL reflect rendering-profile decisions
If the active provider profile changes the final delivery method by downgrading formatted text, splitting oversized text, avoiding an unsafe edit, or abandoning a native update path, the delivery result SHALL preserve provider-aware fallback metadata that explains the rendering decision.

#### Scenario: Unsafe Telegram markdown falls back to plain text
- **WHEN** a Telegram-targeted delivery requests formatted text but the provider renderer cannot produce safe Markdown-aware output
- **THEN** the Bridge falls back to Telegram plain text before sending the message
- **AND** the delivery result records that the formatted path was skipped because the renderer selected a safe fallback

#### Scenario: Incompatible mutable update becomes provider-aware follow-up
- **WHEN** a reply target requests an in-place update that the active provider profile considers invalid for the current content or target
- **THEN** the Bridge chooses a supported provider-aware follow-up delivery path instead of forcing the invalid update
- **AND** the fallback metadata explains that the original mutable update plan was not usable
