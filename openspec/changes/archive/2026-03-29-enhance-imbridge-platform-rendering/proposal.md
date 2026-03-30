## Why

The IMBridge currently supports 8 platforms but native rich-content rendering is Feishu-centric. `NativeMessage` only carries `FeishuCardPayload`; Slack Block Kit, Discord Embeds/Components, Telegram HTML/MarkdownV2 rich layouts, DingTalk ActionCard with multi-button layouts, WeCom template cards, and QQ/QQ Bot Markdown messages all lack first-class `NativeMessage` payloads. The shared `StructuredMessage` model (title + body + fields + actions) is too coarse to express platform-specific affordances like Slack section blocks with accessories, Discord embed fields with thumbnails, Telegram inline-keyboard grids, or DingTalk feed-card lists. This forces callers to either accept lowest-common-denominator output or bypass the delivery pipeline entirely.

## What Changes

- **Extend `NativeMessage`** with typed payload slots for Slack (Block Kit JSON), Discord (Embed + Components), Telegram (rich layout with inline keyboard grid), DingTalk (multi-button ActionCard, FeedCard), WeCom (template card, news article), and QQ Bot (Markdown + keyboard).
- **Add per-platform native payload builders** (constructors and validators) following the Feishu pattern: `NewSlackBlockKitMessage()`, `NewDiscordEmbedMessage()`, `NewTelegramRichMessage()`, `NewDingTalkFeedCardMessage()`, `NewWeComTemplateCardMessage()`, `NewQQBotMarkdownMessage()`.
- **Enrich `StructuredMessage`** with optional typed sections (image, divider, context line, multi-column fields, button grid layout hints) so the shared model can express richer intent without requiring native payloads.
- **Add per-platform structured renderers** that convert the enriched `StructuredMessage` into platform-native output: Slack Block Kit blocks, Discord embeds, Telegram formatted text + keyboard matrix, DingTalk ActionCard body, WeCom news articles.
- **Extend rendering profiles** with `NativeSurfaces []string` so platforms can declare which native payload types they accept, and the delivery pipeline can validate/fallback before transport.
- **Extend `TextFormatMode`** with `slack_mrkdwn`, `discord_md`, and `dingtalk_md` so formatted-text delivery can use each platform's native markdown dialect.
- **Wire native delivery paths** in `DeliverNative()` and `executeRenderingPlan()` to dispatch Slack/Discord/Telegram/DingTalk/WeCom/QQ Bot native payloads through each platform's `NativeMessageSender` implementation.
- **Add fallback-text generators** per native payload type so every rich native message degrades gracefully to readable plain text with metadata preserved for diagnostics.

## Capabilities

### New Capabilities
- `platform-native-payloads`: Typed native payload definitions, constructors, validators, and fallback-text generators for Slack Block Kit, Discord Embed+Components, Telegram rich layout, DingTalk FeedCard/ActionCard, WeCom template card/news, QQ Bot Markdown+keyboard.
- `enriched-structured-message`: Extended `StructuredMessage` model with typed sections (image, divider, context, column layout, button grid hints) and per-platform structured renderers.
- `platform-text-formats`: New `TextFormatMode` values (`slack_mrkdwn`, `discord_md`, `dingtalk_md`) with rendering-profile integration and formatted-text sender support.

### Modified Capabilities
- `im-rich-delivery`: Delivery pipeline changes to resolve and dispatch new native payload types through the rendering plan, with extended fallback semantics.
- `im-provider-rendering-profiles`: Rendering profiles gain `NativeSurfaces` field; per-platform defaults updated to declare accepted native payload types.
- `im-platform-native-interactions`: Platform capability matrix updated to reflect new structured surfaces and native payload support per provider.

## Impact

- **Core types**: `core/native_message.go` gains 6 new payload types; `core/structured_message.go` gains section types; `core/rendering_profile.go` gains format modes and native surface declarations.
- **Delivery pipeline**: `core/delivery.go` dispatch logic extended for multi-platform native paths; `ResolveRenderingPlan()` considers new native payload matching.
- **Platform adapters**: Each `platform/*/live.go` gains `NativeMessageSender` implementation for its native payloads; each `platform/*/stub.go` gains matching test endpoints.
- **Backend model**: `src-go/internal/model/im.go` `IMNativeMessage` and `IMSendRequest` updated to carry new payload types through HTTP/control-plane.
- **No breaking changes**: Existing `FeishuCardPayload` path unchanged; new payloads are additive; `StructuredMessage` extensions are backward-compatible (zero-value = current behavior).
