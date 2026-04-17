## ADDED Requirements

### Requirement: Bridge SHALL expose reactions as a first-class message primitive

IM Bridge SHALL model emoji reactions as a first-class primitive distinct from text. `core.Reaction` MUST carry `UserID`, `EmojiCode` (unified cross-provider code), `RawEmoji` (provider-native representation), `MessageID`, `ReactedAt`, and `Removed`. `Message.Kind` MUST distinguish `text | reaction | attachment | system` so inbound reaction events dispatch through the engine without string-sniffing payload.

#### Scenario: Inbound reaction event flows through the engine
- **WHEN** a provider adapter captures a reaction callback
- **THEN** the adapter constructs a `Message` with `Kind=MessageKindReaction` and populated `Reactions[]`
- **AND** the engine routes the event to the configured reaction sink without treating it as a command

### Requirement: Bridge SHALL define a unified emoji code table

IM Bridge SHALL define a unified emoji code table (`ack`, `running`, `done`, `failed`, `thumbs_up`, `thumbs_down`, `eyes`, `question`) and map each unified code to the provider-native representation each platform expects (Slack shortcode, Telegram emoji, Feishu reaction_type, etc.). Callers SHALL emit unified codes; Bridge SHALL translate on the wire through `NativeEmojiForCode(platform, code)` and `ResolveReactionCode(platform, raw)`.

#### Scenario: Slack receives the provider-native shortcode
- **WHEN** a caller passes the unified code `done` with an outbound envelope targeting Slack
- **THEN** Bridge calls `SendReaction(ctx, replyCtx, "white_check_mark")`

#### Scenario: Unknown platform falls back to the unified code
- **WHEN** the platform has no mapping defined
- **THEN** Bridge passes the unified code through unchanged so the provider either accepts or rejects it

### Requirement: Bridge SHALL dispatch ack reactions automatically from envelope metadata

When `DeliveryEnvelope.Metadata["ack_reaction"]` is set and the primary delivery succeeds, Bridge SHALL call `ReactionSender.SendReaction` with the mapped provider-native emoji. Providers that do not implement `ReactionSender` or advertise `SupportsReactions=false` MUST silently skip the reaction. Reaction dispatch errors MUST NOT fail the primary delivery.

#### Scenario: Successful delivery fires ack emoji
- **WHEN** `ack_reaction=running` is set on an envelope delivered to a reaction-capable provider
- **THEN** after the primary reply succeeds, Bridge fires the provider-native running emoji
- **AND** the receipt still reports the primary method (not a reaction)

#### Scenario: Unsupported provider skips silently
- **WHEN** the active provider lacks `ReactionSender`
- **THEN** Bridge does not synthesize a text reply to compensate

### Requirement: Bridge SHALL forward inbound reactions to the Go backend

IM Bridge SHALL expose a `ReactionSink` seam that forwards inbound reaction events to the backend via `POST /api/v1/im/reactions`. The bridge MUST include platform, chat id, message id, user id, unified emoji code, raw emoji, reply target, bridge id, and timestamp. Backend `im_reaction_event_repo.Record` MUST persist the event for audit and review-shortcut resolution.

#### Scenario: Reaction on a review card posts to the backend
- **WHEN** a user reacts with 👍 to a review card on Slack
- **THEN** Bridge resolves the unified code (`thumbs_up`), constructs a `ReactionEvent`, and calls `AgentForgeClient.PostReaction`
- **AND** the backend persists the event with `event_type=created`

#### Scenario: Reaction removal produces a deletion record
- **WHEN** the user later removes the reaction
- **THEN** Bridge posts a removal event with `removed=true`
- **AND** the backend writes an `event_type=deleted` row
