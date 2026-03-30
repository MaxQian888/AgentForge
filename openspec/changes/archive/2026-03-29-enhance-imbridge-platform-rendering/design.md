## Context

The IMBridge currently supports 8 IM platforms through a provider-adapter pattern. Rich content delivery follows a three-tier model: `NativeMessage` (provider-owned payloads) > `StructuredMessage` (shared model) > plain text. However:

- `NativeMessage` only carries `FeishuCardPayload`; other platforms cannot receive provider-native rich payloads through the delivery pipeline.
- Slack, Discord, and Telegram have working `StructuredSender` / `CardSender` implementations that convert `StructuredMessage` to platform-native output, but the shared model is too coarse (title + body + flat fields + flat actions) to express platform-specific affordances.
- DingTalk, WeCom, QQ, and QQ Bot fall back to plain text for all structured deliveries.
- `TextFormatMode` only covers `plain_text`, `markdown_v2`, `lark_md`, and `html` — missing Slack mrkdwn, Discord markdown, and DingTalk markdown dialects.

The rendering pipeline (`ResolveRenderingPlan` → `executeRenderingPlan`) already handles native/structured/text tiers with fallback. The change extends each tier without breaking existing paths.

## Goals / Non-Goals

**Goals:**
- Every platform with a rich-content API gets a typed `NativeMessage` payload slot so callers can send platform-optimized content when they know the target.
- The shared `StructuredMessage` gains enough expressiveness (sections, images, dividers, button grids) that platform renderers produce meaningfully richer output than flat text.
- Each platform's `TextFormatMode` is available for formatted-text delivery, so markdown-capable platforms render text natively instead of stripping formatting.
- The delivery pipeline validates native payloads against rendering-profile declarations before attempting transport, with explicit fallback.
- All new payloads degrade to readable plain text when the target platform or reply context cannot render them.

**Non-Goals:**
- Interactive callback handling changes (already covered by `im-platform-native-interactions` spec).
- New platform adapter additions (only enhancing existing 8 platforms).
- UI/frontend changes for composing rich messages (backend/bridge only).
- Feishu card lifecycle changes (already covered by `feishu-rich-card-lifecycle` spec).
- Message template management or storage (callers construct payloads directly).

## Decisions

### D1: Extend `NativeMessage` with platform-specific payload slots (not a generic `json.RawMessage`)

Each platform gets a typed Go struct: `SlackBlockKitPayload`, `DiscordEmbedPayload`, `TelegramRichPayload`, `DingTalkCardPayload`, `WeComCardPayload`, `QQBotMarkdownPayload`. These are optional pointer fields on `NativeMessage`, mirroring the existing `FeishuCard *FeishuCardPayload` pattern.

**Why not generic JSON?** Typed payloads enable compile-time validation, constructor helpers with defaults, and per-payload `Validate()` + `FallbackText()` methods. The Feishu pattern has proven this works well. Generic JSON would push validation to runtime and lose IDE support.

**Why not one struct per platform in separate packages?** Keeping them in `core/` maintains the single-import pattern and allows `NativeMessage.Validate()` to dispatch by platform without circular imports.

### D2: Enrich `StructuredMessage` with typed sections instead of replacing it

Add `Sections []StructuredSection` where `StructuredSection` is a tagged union (type + typed content). Section types: `text`, `image`, `divider`, `context`, `fields` (multi-column), `actions` (button grid with layout hints). Existing `Title`, `Body`, `Fields`, `Actions` remain for backward compatibility; when `Sections` is non-empty, renderers prefer it.

**Why tagged union?** Go doesn't have sum types, but a struct with a `Type` string and typed optional pointers (`*TextSection`, `*ImageSection`, etc.) gives clear dispatch. Each platform renderer switches on section type and skips unsupported sections with fallback text.

**Why keep legacy fields?** Existing callers and specs use `Title`/`Body`/`Fields`/`Actions`. Removing them would be a breaking change for zero benefit. When `Sections` is empty, legacy fields drive rendering as before.

### D3: Add platform markdown format modes to `TextFormatMode`

Add `slack_mrkdwn`, `discord_md`, `dingtalk_md`. Each platform's rendering profile adds these to `SupportedFormats` and the formatted-text delivery path routes through `FormattedTextSender` when available.

**Why not auto-convert from a universal markdown?** Each platform's markdown dialect has incompatible escaping rules (Slack uses `*bold*` and `~strike~`, Discord uses `**bold**` and `~~strike~~`, Telegram requires escaping `.`, `-`, `(`, etc.). Auto-conversion is error-prone and loses platform-specific features. Callers that want cross-platform delivery use `StructuredMessage`; callers targeting a specific platform can use the native format mode.

### D4: Extend `RenderingProfile` with `NativeSurfaces` declaration

Add `NativeSurfaces []string` listing accepted native payload types (e.g., `["feishu_card"]`, `["slack_block_kit"]`). `ResolveRenderingPlan()` checks this before choosing the native delivery path; mismatches trigger structured/text fallback with explicit reason.

**Why a string list?** Native payload types are open-ended as platforms evolve. A string identifier is extensible without enum changes. Validation happens in `NativeMessage.NormalizedPlatform()` + `Validate()`.

### D5: Per-platform native renderers live in `platform/{name}/native_payload.go`

Following Feishu's pattern (`platform/feishu/native_payload.go`), each platform gets its own native payload rendering file. The live adapter implements `NativeMessageSender` when native payloads are available. Stub adapters store native payloads in the test reply log for verification.

### D6: Structured section renderers are per-platform functions in `platform/{name}/renderer.go`

Each platform that supports structured surfaces gets a `renderStructuredSections()` function that converts `[]StructuredSection` to platform-native output. Telegram already has `renderer.go`; Slack, Discord, and DingTalk get similar files. WeCom, QQ, and QQ Bot continue to use fallback text unless their APIs support richer content.

## Risks / Trade-offs

**[Payload sprawl]** → 6 new payload types in `core/native_message.go` increases package size. Mitigation: each payload is a focused struct with constructor + validator + fallback. The alternative (external packages) would complicate the delivery pipeline with imports.

**[Platform API drift]** → Native payload structs may lag behind platform API changes. Mitigation: payloads model the stable subset of each API (Block Kit blocks, Discord embeds, Telegram inline keyboards are all stable APIs). Unstable features are left out.

**[Section rendering gaps]** → Not every platform can render every section type (e.g., QQ has no image block). Mitigation: renderers skip unsupported sections and include fallback text. The `FallbackText()` method on each section ensures text degradation is always available.

**[Backward compatibility]** → Callers using `StructuredMessage` without `Sections` see no change. Callers sending `NativeMessage` with `FeishuCard` see no change. New payloads are purely additive. Risk is minimal.

**[WeCom/QQ/QQ Bot limited APIs]** → These platforms have minimal rich-content APIs. WeCom supports news articles and template cards; QQ Bot supports markdown in some contexts. QQ (OneBot) is text-only. Mitigation: implement what each API supports; don't force parity. The rendering profile declares what's available.
