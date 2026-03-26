## ADDED Requirements

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
