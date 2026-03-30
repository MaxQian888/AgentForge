## MODIFIED Requirements

### Requirement: Platform capability matrix SHALL describe native interaction strategy, not just transport availability
The system SHALL publish a capability matrix for each active IM platform that describes the platform's native command surface, structured-message surface, callback mode, asynchronous update mode, message scope, mutability semantics, and accepted native payload surfaces. The Bridge, control-plane registration payload, and health surfaces MUST use this matrix to choose delivery behavior instead of inferring behavior from platform names or from a single rich-message boolean.

#### Scenario: Slack declares threaded block-capable interactions with native Block Kit payloads
- **WHEN** the active platform is Slack
- **THEN** the Bridge publishes capability metadata that identifies Socket Mode command intake, Block Kit structured output, threaded reply scope, response-driven follow-up behavior, and `slack_block_kit` as an accepted native payload surface
- **AND** downstream delivery code can choose thread-aware block responses or dispatch native Block Kit payloads without hard-coding `"slack"`

#### Scenario: Discord declares deferred interaction lifecycle with native embed payloads
- **WHEN** the active platform is Discord
- **THEN** the Bridge publishes capability metadata that identifies public interaction callback requirements, deferred acknowledgement support, follow-up or original-response mutation behavior, and `discord_embed` as an accepted native payload surface
- **AND** asynchronous work can be routed through the correct interaction lifecycle including native embed delivery

#### Scenario: Telegram declares mutable text-first updates with inline keyboard
- **WHEN** the active platform is Telegram
- **THEN** the Bridge publishes capability metadata that identifies inline-keyboard callbacks, mutable message updates, text-first rendering constraints, and `telegram_rich` as an accepted native payload surface
- **AND** progress delivery can prefer low-noise message edits with rich keyboard layouts

#### Scenario: DingTalk declares ActionCard and native card support
- **WHEN** the active platform is DingTalk
- **THEN** the Bridge publishes capability metadata that identifies stream mode command intake, ActionCard structured output, session webhook async updates, and `dingtalk_card` as an accepted native payload surface

#### Scenario: WeCom declares native card support
- **WHEN** the active platform is WeCom
- **THEN** the Bridge publishes capability metadata that identifies callback-driven command intake, and `wecom_card` as an accepted native payload surface for news articles and template cards

#### Scenario: QQ Bot declares markdown native support
- **WHEN** the active platform is QQ Bot
- **THEN** the Bridge publishes capability metadata that identifies webhook command intake and `qqbot_markdown` as an accepted native payload surface

#### Scenario: QQ declares text-only capabilities
- **WHEN** the active platform is QQ
- **THEN** the Bridge publishes capability metadata with no native payload surfaces and no structured surface
- **AND** all deliveries resolve to plain text
