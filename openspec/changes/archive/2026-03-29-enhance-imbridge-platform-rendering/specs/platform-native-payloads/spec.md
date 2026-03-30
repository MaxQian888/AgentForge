## ADDED Requirements

### Requirement: NativeMessage SHALL carry typed payloads for each supported platform
The `NativeMessage` struct SHALL include optional typed payload slots for Slack Block Kit, Discord Embed+Components, Telegram rich layout, DingTalk card, WeCom card, and QQ Bot markdown. Each payload slot SHALL be a pointer to a platform-specific struct with constructor, `Validate()`, and `FallbackText()` methods. `NativeMessage.NormalizedPlatform()` and `Validate()` SHALL dispatch correctly when any single platform payload is set.

#### Scenario: Slack Block Kit native message is constructed and validated
- **WHEN** a caller creates a `NativeMessage` via `NewSlackBlockKitMessage()` with section blocks and action blocks
- **THEN** the message has `Platform: "slack"` and a populated `SlackBlockKit` payload
- **AND** `Validate()` succeeds when blocks are well-formed and fails when the blocks array is empty

#### Scenario: Discord embed native message is constructed and validated
- **WHEN** a caller creates a `NativeMessage` via `NewDiscordEmbedMessage()` with title, description, fields, color, and optional components
- **THEN** the message has `Platform: "discord"` and a populated `DiscordEmbed` payload
- **AND** `Validate()` succeeds when at least title or description is present and fails when both are empty

#### Scenario: Telegram rich native message is constructed and validated
- **WHEN** a caller creates a `NativeMessage` via `NewTelegramRichMessage()` with formatted text and an inline keyboard grid
- **THEN** the message has `Platform: "telegram"` and a populated `TelegramRich` payload
- **AND** `Validate()` succeeds when text content is non-empty and fails when text is empty

#### Scenario: DingTalk card native message is constructed and validated
- **WHEN** a caller creates a `NativeMessage` via `NewDingTalkCardMessage()` with title, markdown body, and action buttons
- **THEN** the message has `Platform: "dingtalk"` and a populated `DingTalkCard` payload
- **AND** `Validate()` succeeds when title and body are present and fails when title is empty

#### Scenario: WeCom card native message is constructed and validated
- **WHEN** a caller creates a `NativeMessage` via `NewWeComCardMessage()` with card type (news or template), articles or template fields
- **THEN** the message has `Platform: "wecom"` and a populated `WeComCard` payload
- **AND** `Validate()` succeeds for well-formed news articles or template payloads

#### Scenario: QQ Bot markdown native message is constructed and validated
- **WHEN** a caller creates a `NativeMessage` via `NewQQBotMarkdownMessage()` with markdown content and optional keyboard buttons
- **THEN** the message has `Platform: "qqbot"` and a populated `QQBotMarkdown` payload
- **AND** `Validate()` succeeds when markdown content is non-empty

#### Scenario: Only one platform payload is active per NativeMessage
- **WHEN** a `NativeMessage` has multiple platform payload pointers set simultaneously
- **THEN** `Validate()` returns an error indicating that exactly one platform payload must be set

### Requirement: Each native payload type SHALL provide a FallbackText method
Every platform-specific payload struct SHALL implement a `FallbackText() string` method that produces a human-readable plain-text degradation of the rich content. This fallback text SHALL be used by the delivery pipeline when the target platform or reply context cannot deliver the native payload.

#### Scenario: Slack Block Kit falls back to readable text
- **WHEN** a `SlackBlockKitPayload` contains section blocks with text and action buttons
- **THEN** `FallbackText()` returns a string with section text and button labels, suitable for plain-text delivery

#### Scenario: Discord embed falls back to title and description
- **WHEN** a `DiscordEmbedPayload` has title, description, and fields
- **THEN** `FallbackText()` returns title, description, and "label: value" lines for fields

#### Scenario: DingTalk card falls back to title and body text
- **WHEN** a `DingTalkCardPayload` has title, markdown body, and buttons
- **THEN** `FallbackText()` returns title, body stripped of markdown, and button labels

### Requirement: Native payload constructors SHALL enforce platform-specific constraints
Each `New*Message()` constructor SHALL validate platform-specific constraints at construction time. Constraints include maximum text lengths, required fields, and structural rules imposed by each platform's API.

#### Scenario: Slack Block Kit rejects payload exceeding 50 blocks
- **WHEN** a caller attempts `NewSlackBlockKitMessage()` with more than 50 blocks
- **THEN** the constructor returns an error citing the Slack API block limit

#### Scenario: Telegram rich message rejects text exceeding 4096 characters
- **WHEN** a caller attempts `NewTelegramRichMessage()` with text content exceeding 4096 characters without segmentation
- **THEN** the constructor returns an error citing the Telegram message length limit

#### Scenario: Discord embed rejects description exceeding 4096 characters
- **WHEN** a caller attempts `NewDiscordEmbedMessage()` with description exceeding 4096 characters
- **THEN** the constructor returns an error citing the Discord embed description limit

### Requirement: Native delivery pipeline SHALL dispatch per-platform payloads through NativeMessageSender
The `DeliverNative()` path SHALL route `NativeMessage` payloads to the active platform's `NativeMessageSender` implementation based on the payload's `NormalizedPlatform()`. If the active platform does not match the payload's target platform, or does not implement `NativeMessageSender`, the pipeline SHALL fall back to structured or text delivery with an explicit fallback reason.

#### Scenario: Slack native payload dispatched to Slack adapter
- **WHEN** a `NativeMessage` with `SlackBlockKit` payload targets a Slack-active bridge
- **THEN** the delivery pipeline calls the Slack adapter's `SendNative`/`ReplyNative` with the block kit payload
- **AND** the delivery receipt records `type: "native"` with no fallback

#### Scenario: Slack native payload targeting non-Slack bridge falls back
- **WHEN** a `NativeMessage` with `SlackBlockKit` payload targets a Telegram-active bridge
- **THEN** the delivery pipeline falls back to text using the payload's `FallbackText()`
- **AND** the delivery receipt records `fallback_reason: "native_platform_mismatch"`

#### Scenario: Platform without NativeMessageSender falls back from native payload
- **WHEN** a `NativeMessage` targets a platform whose adapter does not implement `NativeMessageSender`
- **THEN** the delivery pipeline falls back to structured or text delivery
- **AND** the delivery receipt records `fallback_reason: "native_sender_unavailable"`
