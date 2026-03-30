## MODIFIED Requirements

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
