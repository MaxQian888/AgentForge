# im-provider-rendering-profiles Specification

## Purpose
Define the provider-owned rendering profile contract for AgentForge IM Bridge so each active IM provider can declare how typed outbound delivery should be formatted, segmented, downgraded, and turned into final provider payloads.
## Requirements
### Requirement: Active IM provider SHALL publish a rendering profile
Each runnable IM provider SHALL define supported text formatting modes, structured rendering preferences, message length limits, mutable-update constraints, accepted native surfaces, and a readiness tier that matches the live adapter's real behavior. Chinese platform profiles MUST only advertise provider surfaces that can actually be produced by the current adapter and reply-target lifecycle, rather than inheriting Feishu's richest behavior by implication.

#### Scenario: Feishu provider publishes full native-card rendering profile
- **WHEN** the Feishu provider initializes
- **THEN** its rendering profile declares readiness tier `full_native_lifecycle`, supports Feishu-native card payloads, and reports delayed card update compatibility

#### Scenario: DingTalk provider publishes ActionCard-send profile without card-update support
- **WHEN** the DingTalk provider initializes
- **THEN** its rendering profile declares readiness tier `native_send_with_fallback`, includes `dingtalk_card` as an accepted native surface, and reports that mutable card updates are unavailable

#### Scenario: WeCom provider publishes template-card-aware fallback profile
- **WHEN** the WeCom provider initializes
- **THEN** its rendering profile declares readiness tier `native_send_with_fallback`, includes only the WeCom native surfaces the adapter can actually send, and reports richer update limits explicitly

#### Scenario: QQ Bot provider publishes markdown-first profile
- **WHEN** the QQ Bot provider initializes
- **THEN** its rendering profile declares readiness tier `markdown_first`, includes `qqbot_markdown` as an accepted native surface, and does not advertise card mutation support

#### Scenario: QQ provider publishes text-first profile
- **WHEN** the QQ provider initializes
- **THEN** its rendering profile declares readiness tier `text_first`, leaves native surfaces empty, and routes all richer requests to explicit text or link fallback

### Requirement: Rendering profiles SHALL declare accepted native payload surfaces
The `RenderingProfile` struct SHALL include a `NativeSurfaces []string` field listing the native payload types accepted by the platform (e.g., `["feishu_card"]`, `["slack_block_kit"]`, `["discord_embed"]`). The delivery pipeline SHALL check this field before attempting native delivery; if the payload type is not in the list, the pipeline SHALL fall back to structured or text delivery with explicit reason.

#### Scenario: Feishu rendering profile declares feishu_card native surface
- **WHEN** the active platform is Feishu
- **THEN** the rendering profile's `NativeSurfaces` includes `"feishu_card"`

#### Scenario: Slack rendering profile declares slack_block_kit native surface
- **WHEN** the active platform is Slack
- **THEN** the rendering profile's `NativeSurfaces` includes `"slack_block_kit"`

#### Scenario: Discord rendering profile declares discord_embed native surface
- **WHEN** the active platform is Discord
- **THEN** the rendering profile's `NativeSurfaces` includes `"discord_embed"`

#### Scenario: DingTalk rendering profile declares dingtalk_card native surface
- **WHEN** the active platform is DingTalk
- **THEN** the rendering profile's `NativeSurfaces` includes `"dingtalk_card"`

#### Scenario: WeCom rendering profile declares wecom_card native surface
- **WHEN** the active platform is WeCom
- **THEN** the rendering profile's `NativeSurfaces` includes `"wecom_card"`

#### Scenario: QQ Bot rendering profile declares qqbot_markdown native surface
- **WHEN** the active platform is QQ Bot
- **THEN** the rendering profile's `NativeSurfaces` includes `"qqbot_markdown"`

#### Scenario: QQ rendering profile declares no native surfaces
- **WHEN** the active platform is QQ
- **THEN** the rendering profile's `NativeSurfaces` is empty
- **AND** any native payload targeting QQ falls back to text

#### Scenario: Rendering profile with updated text format support
- **WHEN** the active platform is Slack
- **THEN** the rendering profile's `SupportedFormats` includes `slack_mrkdwn` alongside `plain_text`
- **AND** `DefaultTextFormat` remains `plain_text`

#### Scenario: Native payload type not in NativeSurfaces triggers fallback
- **WHEN** a `NativeMessage` with type `"slack_block_kit"` targets a platform whose `NativeSurfaces` does not include `"slack_block_kit"`
- **THEN** the delivery pipeline falls back to structured or text delivery
- **AND** the fallback reason includes `"native_surface_unsupported"`

### Requirement: Typed outbound delivery SHALL resolve through a provider rendering plan
Before a typed outbound IM delivery is executed, the Bridge SHALL resolve the delivery through the active provider's rendering profile and produce a provider-aware rendering plan. The rendering plan MUST decide whether the delivery uses native payloads, structured output, formatted text, segmented text, editable updates, or explicit downgrade to plain text.

#### Scenario: Control-plane replay preserves provider-aware rendering
- **WHEN** a queued delivery is replayed for the active provider
- **THEN** the Bridge resolves the same provider-aware rendering plan that direct `/im/send` or `/im/notify` would use for the same envelope
- **AND** replay does not silently flatten the delivery into a different provider representation

#### Scenario: Unsupported rich request degrades through the rendering plan
- **WHEN** a typed delivery requests rendering behavior the active provider profile or reply target cannot honor
- **THEN** the Bridge resolves an explicit downgrade plan instead of attempting an invalid native or mutable update
- **AND** the resulting delivery metadata preserves the fallback reason

### Requirement: Provider rendering profiles SHALL enforce provider-safe formatting
If a provider supports formatted text modes such as Markdown, HTML, or provider-specific rich text, its rendering profile MUST enforce the provider's escaping, segmentation, and update-safety constraints before transport execution. If the requested formatted output cannot be rendered safely, the Bridge MUST fall back to a supported plain-text representation.

#### Scenario: Unsafe formatted text falls back cleanly
- **WHEN** the Bridge receives a formatted-text intent that cannot be rendered safely under the active provider's escaping or update rules
- **THEN** the provider rendering profile degrades the delivery to a supported plain-text representation
- **AND** the transport executor does not send malformed formatted text

### Requirement: Feishu rendering plans SHALL keep operator cards on provider-safe interactive structures
When a structured operator response targets Feishu, the rendering profile SHALL resolve it through a provider-safe interactive card plan instead of assuming that any shared markdown fragment can be embedded into any Feishu card field. The Feishu rendering plan MUST choose only documented card elements and markdown-bearing fields that are valid for the selected interactive message shape, and MUST fall back explicitly when the requested richer output cannot be rendered safely.

#### Scenario: Feishu help response resolves through a provider-safe card plan
- **WHEN** `/help` or another structured command-catalog response targets the Feishu provider
- **THEN** the rendering plan produces a documented interactive card structure that the Feishu send or reply API accepts
- **AND** the final card body uses only provider-safe markdown or text slots for the content it carries

#### Scenario: Unsafe or unsupported richer formatting downgrades explicitly
- **WHEN** a structured Feishu response would require markdown or card elements that are incompatible with the selected send, reply, or update path
- **THEN** the rendering plan downgrades to a simpler supported representation such as plain text or a reduced card body
- **AND** the delivery metadata preserves an explicit fallback reason instead of silently emitting an invalid Feishu payload

### Requirement: Feishu update rendering SHALL remain compatible with delayed-update constraints
If a Feishu reply target carries delayed-update context, the rendering profile SHALL choose an update-compatible card representation before attempting in-place mutation. The Bridge MUST NOT select a rendering mode that only works for fresh sends or replies when the preserved callback token can only be used for supported delayed card updates.

#### Scenario: Delayed-update target chooses an update-compatible card body
- **WHEN** a Feishu completion flow prefers delayed card update for the originating interaction
- **THEN** the rendering plan selects a card body that is valid for the delayed-update path and current token state
- **AND** the provider does not attempt to mutate the card with a send-only or reply-only representation

### Requirement: Provider capability matrix SHALL declare attachment, reaction, and thread capabilities

`PlatformCapabilities` SHALL expose `SupportsAttachments`, `MaxAttachmentSize`, `AllowedAttachmentKinds`, `SupportsReactions`, `ReactionEmojiSet`, `SupportsThreads`, `ThreadPolicySupport`, and `MutableUpdateMethod`. Every provider's `MetadataForPlatform` MUST populate these fields truthfully — missing support MUST be `false` / zero-value, not silently inherited from richer providers. `/im/health` MUST surface the new fields under `capability_matrix` so operators and the backend provider catalog can observe truth.

#### Scenario: Supported provider advertises truthful matrix
- **WHEN** Bridge is running Slack in live mode
- **THEN** `/im/health` reports `supportsAttachments=true`, `supportsReactions=true`, `supportsThreads=true` with non-empty `allowedAttachmentKinds`, `reactionEmojiSet`, `threadPolicySupport`

#### Scenario: Unsupported provider advertises zero values
- **WHEN** Bridge is running WeCom in live mode
- **THEN** `/im/health` reports `supportsAttachments=false`, `supportsReactions=false`, `supportsThreads=false`
- **AND** `mutableUpdateMethod=template_card_update` so operators know how mutable updates are implemented

