## MODIFIED Requirements

### Requirement: Bridge resolves provider and model defaults from one registry

The canonical provider registry SHALL define supported provider names, AI capabilities, default models, and IM platform capability descriptors. For IM providers, the registry entry SHALL include the full capability matrix: command surface, structured-message surface, callback mode, async update mode, message scope, mutability, and card support.

WeCom, QQ, and QQ Bot providers SHALL have complete capability matrix entries in the registry, matching the level of detail already present for Feishu, Slack, Telegram, Discord, and DingTalk.

#### Scenario: WeCom provider registered with complete capabilities
- **WHEN** the provider registry initializes
- **THEN** WeCom entry declares `{commandSurface: "callback", structuredMessage: "text", callback: "callback", asyncUpdate: "appMessage", card: false, cardUpdate: false}`

#### Scenario: QQ provider registered with complete capabilities
- **WHEN** the provider registry initializes
- **THEN** QQ entry declares `{commandSurface: "websocket", structuredMessage: "text", callback: "websocket", asyncUpdate: "reply", card: false, cardUpdate: false}`

#### Scenario: QQ Bot provider registered with complete capabilities
- **WHEN** the provider registry initializes
- **THEN** QQ Bot entry declares `{commandSurface: "webhook", structuredMessage: "text", callback: "webhook", asyncUpdate: "openapi", card: false, cardUpdate: false}`
