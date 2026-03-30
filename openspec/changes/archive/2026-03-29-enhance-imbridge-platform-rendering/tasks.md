## 1. Core Type Extensions

- [x] 1.1 Add `TextFormatSlackMrkdwn`, `TextFormatDiscordMD`, `TextFormatDingTalkMD` constants to `core/rendering_profile.go`
- [x] 1.2 Add `NativeSurfaces []string` field to `RenderingProfile` struct and update `normalizeRenderingProfile()` and `defaultRenderingProfileForSource()` with per-platform native surface declarations
- [x] 1.3 Update `defaultRenderingProfileForSource()` to include new text formats in `SupportedFormats` for Slack (`slack_mrkdwn`), Discord (`discord_md`), and DingTalk (`dingtalk_md`)
- [x] 1.4 Define `SlackBlockKitPayload` struct with `Blocks json.RawMessage`, constructor `NewSlackBlockKitMessage()`, `Validate()`, and `FallbackText()` in `core/native_message.go`
- [x] 1.5 Define `DiscordEmbedPayload` struct with title, description, fields, color, components; constructor `NewDiscordEmbedMessage()`, `Validate()`, and `FallbackText()` in `core/native_message.go`
- [x] 1.6 Define `TelegramRichPayload` struct with text, parse mode, inline keyboard grid; constructor `NewTelegramRichMessage()`, `Validate()`, and `FallbackText()` in `core/native_message.go`
- [x] 1.7 Define `DingTalkCardPayload` struct with title, markdown body, buttons, card type; constructor `NewDingTalkCardMessage()`, `Validate()`, and `FallbackText()` in `core/native_message.go`
- [x] 1.8 Define `WeComCardPayload` struct with card type (news/template), articles, template fields; constructor `NewWeComCardMessage()`, `Validate()`, and `FallbackText()` in `core/native_message.go`
- [x] 1.9 Define `QQBotMarkdownPayload` struct with markdown content, optional keyboard; constructor `NewQQBotMarkdownMessage()`, `Validate()`, and `FallbackText()` in `core/native_message.go`
- [x] 1.10 Add typed payload pointer fields (`SlackBlockKit`, `DiscordEmbed`, `TelegramRich`, `DingTalkCard`, `WeComCard`, `QQBotMarkdown`) to `NativeMessage` struct; update `NormalizedPlatform()` and `Validate()` to dispatch across all payload types and reject multi-payload messages

## 2. Enriched StructuredMessage

- [x] 2.1 Define `StructuredSection` struct with `Type string` and typed optional pointers: `TextSection`, `ImageSection`, `DividerSection`, `ContextSection`, `FieldsSection`, `ActionsSection` in `core/structured_message.go`
- [x] 2.2 Define section content structs: `TextSection{Body}`, `ImageSection{URL, AltText}`, `DividerSection{}`, `ContextSection{Elements []string}`, `FieldsSection{Fields []StructuredField}`, `ActionsSection{Actions []StructuredAction, ButtonsPerRow int}`
- [x] 2.3 Add `Sections []StructuredSection` field to `StructuredMessage`
- [x] 2.4 Implement `FallbackText()` on each section type and update `StructuredMessage.FallbackText()` to prefer `Sections` when non-empty

## 3. Delivery Pipeline Updates

- [x] 3.1 Update `ResolveRenderingPlan()` in `core/delivery.go` to check `NativeSurfaces` against native payload type before choosing native path; set fallback reason `"native_surface_unsupported"` or `"native_platform_mismatch"` on mismatch
- [x] 3.2 Update `DeliverNativePlan()` / `DeliverNative()` to dispatch new payload types (Slack, Discord, Telegram, DingTalk, WeCom, QQ Bot) through `NativeMessageSender` on matching platforms
- [x] 3.3 Update `executeRenderingPlan()` structured path to detect `Sections` and route to per-platform section renderers before legacy card/text fallback

## 4. Platform Adapter: Slack

- [x] 4.1 Implement `NativeMessageSender` (`SendNative`, `ReplyNative`) on Slack live adapter to dispatch `SlackBlockKitPayload` through Slack API
- [x] 4.2 Implement `FormattedTextSender` (`SendFormattedText`, `ReplyFormattedText`, `UpdateFormattedText`) on Slack live adapter with `slack_mrkdwn` support
- [x] 4.3 Create `platform/slack/renderer.go` with `renderStructuredSections()` converting `[]StructuredSection` to `[]slack.Block` (section, image, divider, context, actions blocks)
- [x] 4.4 Update Slack stub adapter to log native payloads and formatted text in test reply store

## 5. Platform Adapter: Discord

- [x] 5.1 Implement `NativeMessageSender` on Discord live adapter to dispatch `DiscordEmbedPayload` through Discord interaction responses
- [x] 5.2 Implement `FormattedTextSender` on Discord live adapter with `discord_md` support
- [x] 5.3 Create `platform/discord/renderer.go` with `renderStructuredSections()` converting sections to Discord embed fields and component action rows
- [x] 5.4 Update Discord stub adapter to log native payloads and formatted text

## 6. Platform Adapter: Telegram

- [x] 6.1 Implement `NativeMessageSender` on Telegram live adapter to dispatch `TelegramRichPayload` with text + inline keyboard grid
- [x] 6.2 Update `platform/telegram/renderer.go` `renderStructuredSections()` to convert `[]StructuredSection` to formatted text + inline keyboard rows
- [x] 6.3 Update Telegram stub adapter to log native payloads

## 7. Platform Adapter: DingTalk

- [x] 7.1 Implement `NativeMessageSender` on DingTalk live adapter to dispatch `DingTalkCardPayload` through DingTalk card API
- [x] 7.2 Implement `FormattedTextSender` on DingTalk live adapter with `dingtalk_md` support
- [x] 7.3 Create `platform/dingtalk/renderer.go` with `renderStructuredSections()` converting sections to ActionCard markdown body + buttons
- [x] 7.4 Update DingTalk stub adapter to log native payloads and formatted text

## 8. Platform Adapter: WeCom

- [x] 8.1 Implement `NativeMessageSender` on WeCom live adapter to dispatch `WeComCardPayload` (news articles and template card messages) through WeCom API
- [x] 8.2 Create `platform/wecom/renderer.go` with `renderStructuredSections()` converting sections to WeCom news article format where supported
- [x] 8.3 Update WeCom stub adapter to log native payloads

## 9. Platform Adapter: QQ Bot

- [x] 9.1 Implement `NativeMessageSender` on QQ Bot live adapter to dispatch `QQBotMarkdownPayload` with markdown content and optional keyboard
- [x] 9.2 Update QQ Bot stub adapter to log native payloads

## 10. Backend Model Updates

- [x] 10.1 Update `IMNativeMessage` in `src-go/internal/model/im.go` to carry new payload types (Slack, Discord, Telegram, DingTalk, WeCom, QQ Bot) through HTTP and control-plane delivery
- [x] 10.2 Update `IMSendRequest` serialization/deserialization to round-trip new native payload types

## 11. Platform Metadata Updates

- [x] 11.1 Update `PlatformCapabilities` or registration metadata for each platform to include native surface declarations in bridge registration payload
- [x] 11.2 Update platform metadata provider implementations to expose new native surfaces and text format capabilities

## 12. Tests

- [x] 12.1 Add unit tests for all new `NativeMessage` payload constructors, validators, and `FallbackText()` methods
- [x] 12.2 Add unit tests for `StructuredSection` types, section `FallbackText()`, and `StructuredMessage.FallbackText()` with sections
- [x] 12.3 Add unit tests for `ResolveRenderingPlan()` with new native payload types, native surface validation, and platform mismatch fallback
- [x] 12.4 Add unit tests for each platform's `renderStructuredSections()` function
- [x] 12.5 Add integration tests for end-to-end delivery through stub adapters: native payloads, section-based structured messages, and formatted text for each platform
