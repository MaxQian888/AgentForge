# platform-text-formats Specification

## Purpose
Define platform-specific formatted-text modes for AgentForge IM Bridge so native markdown dialects can be requested and delivered safely across providers.

## Requirements
### Requirement: TextFormatMode SHALL include platform-specific markdown dialects
The `TextFormatMode` type SHALL include `slack_mrkdwn`, `discord_md`, and `dingtalk_md` constants alongside the existing `plain_text`, `markdown_v2`, `lark_md`, and `html` values. Each platform's rendering profile SHALL declare its native markdown format in `SupportedFormats`.

#### Scenario: Slack rendering profile declares mrkdwn support
- **WHEN** the active platform is Slack
- **THEN** the rendering profile's `SupportedFormats` includes `slack_mrkdwn` and `plain_text`
- **AND** `DefaultTextFormat` remains `plain_text` for backward compatibility

#### Scenario: Discord rendering profile declares discord_md support
- **WHEN** the active platform is Discord
- **THEN** the rendering profile's `SupportedFormats` includes `discord_md` and `plain_text`

#### Scenario: DingTalk rendering profile declares dingtalk_md support
- **WHEN** the active platform is DingTalk
- **THEN** the rendering profile's `SupportedFormats` includes `dingtalk_md` and `plain_text`

### Requirement: Formatted text delivery SHALL route platform markdown through FormattedTextSender
When a delivery requests a platform-specific text format via metadata `text_format`, the delivery pipeline SHALL use the `FormattedTextSender` interface if the active platform implements it and the requested format is in `SupportedFormats`. Platforms that do not implement `FormattedTextSender` SHALL receive plain text with the format stripped.

#### Scenario: Slack mrkdwn formatted text delivered through FormattedTextSender
- **WHEN** a text delivery targets Slack with metadata `text_format: "slack_mrkdwn"` and Slack implements `FormattedTextSender`
- **THEN** the pipeline delivers through `SendFormattedText`/`ReplyFormattedText` with format `slack_mrkdwn`
- **AND** the Slack adapter renders the text using Slack's native mrkdwn formatting

#### Scenario: Unsupported format falls back to plain text
- **WHEN** a text delivery targets a platform with metadata `text_format: "discord_md"` but the active platform is Telegram
- **THEN** the pipeline strips the format and delivers as `plain_text`
- **AND** no error is raised; the delivery succeeds with plain text

### Requirement: Slack adapter SHALL implement FormattedTextSender for mrkdwn
The Slack live adapter SHALL implement the `FormattedTextSender` interface with `SendFormattedText`, `ReplyFormattedText`, and `UpdateFormattedText` methods. When the format is `slack_mrkdwn`, the adapter SHALL send the text using Slack's mrkdwn-capable message API. When the format is `plain_text`, the adapter SHALL send without mrkdwn parsing.

#### Scenario: Slack sends mrkdwn formatted message
- **WHEN** `SendFormattedText` is called with format `slack_mrkdwn` and content `"*bold* and ~strike~"`
- **THEN** the Slack adapter sends the message with `mrkdwn: true` so Slack renders formatting

#### Scenario: Slack sends plain text without mrkdwn parsing
- **WHEN** `SendFormattedText` is called with format `plain_text` and content containing `*asterisks*`
- **THEN** the Slack adapter sends without mrkdwn so asterisks appear literally

### Requirement: Discord adapter SHALL implement FormattedTextSender for discord_md
The Discord live adapter SHALL implement `FormattedTextSender`. When the format is `discord_md`, the adapter SHALL send the text as-is since Discord auto-renders markdown. When the format is `plain_text`, the adapter SHALL escape markdown characters.

#### Scenario: Discord sends markdown formatted message
- **WHEN** `SendFormattedText` is called with format `discord_md` and content `"**bold** and ~~strike~~"`
- **THEN** the Discord adapter sends the content directly and Discord renders formatting

### Requirement: DingTalk adapter SHALL implement FormattedTextSender for dingtalk_md
The DingTalk live adapter SHALL implement `FormattedTextSender`. When the format is `dingtalk_md`, the adapter SHALL send the text as a markdown-type message through the DingTalk Robot API.

#### Scenario: DingTalk sends markdown formatted message
- **WHEN** `SendFormattedText` is called with format `dingtalk_md` and content with markdown syntax
- **THEN** the DingTalk adapter sends via the markdown message type endpoint
