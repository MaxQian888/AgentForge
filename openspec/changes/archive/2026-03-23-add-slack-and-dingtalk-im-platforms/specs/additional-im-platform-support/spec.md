## ADDED Requirements

### Requirement: Bridge runtime can start with Slack or DingTalk as the active platform
The IM Bridge SHALL allow a deployment to select exactly one active IM platform per process, and that platform MAY be `feishu`, `slack`, or `dingtalk`. The runtime SHALL validate the required credentials for the selected platform before starting message handling or notification delivery.

#### Scenario: Slack bridge starts with valid configuration
- **WHEN** the bridge is configured with `IM_PLATFORM=slack` and all required Slack credentials are present
- **THEN** the bridge starts a Slack platform adapter and registers the existing command engine against that adapter

#### Scenario: DingTalk bridge starts with valid configuration
- **WHEN** the bridge is configured with `IM_PLATFORM=dingtalk` and all required DingTalk credentials are present
- **THEN** the bridge starts a DingTalk platform adapter and registers the existing command engine against that adapter

#### Scenario: Selected platform configuration is incomplete
- **WHEN** the bridge is configured for Slack or DingTalk but a required credential is missing
- **THEN** startup fails with an actionable configuration error
- **AND** the bridge does not silently fall back to another platform implementation

### Requirement: Core command handling remains platform-consistent across Slack and DingTalk
The system SHALL translate Slack and DingTalk inbound events into `core.Message` values that preserve platform identity, user identity, chat identity, and message content so that the existing `/task`, `/agent`, `/cost`, `/help`, and `@AgentForge` flows execute with the same command semantics as the current bridge.

#### Scenario: Slack slash-style message routes to an existing command handler
- **WHEN** a Slack message mapped into `core.Message` contains `/task list`
- **THEN** the engine invokes the registered `/task` command handler
- **AND** the platform sends the resulting reply back to the originating Slack conversation

#### Scenario: DingTalk mention uses the existing fallback path
- **WHEN** a DingTalk message mapped into `core.Message` mentions `@AgentForge` without matching a registered slash command
- **THEN** the engine invokes the configured fallback handler
- **AND** the platform returns the fallback response to the originating DingTalk conversation

### Requirement: Platform source metadata is propagated to backend API calls
IM Bridge requests to the AgentForge backend SHALL identify the actual source platform instead of hardcoding Feishu so that backend audit, routing, and downstream policy can distinguish Slack, DingTalk, and Feishu traffic.

#### Scenario: Slack command call includes Slack as source
- **WHEN** a user triggers a backend-backed command from Slack
- **THEN** the bridge sends the backend request with source metadata identifying `slack`

#### Scenario: DingTalk command call includes DingTalk as source
- **WHEN** a user triggers a backend-backed command from DingTalk
- **THEN** the bridge sends the backend request with source metadata identifying `dingtalk`

### Requirement: Notifications respect platform matching and rich-message fallback
The notification receiver SHALL only deliver a notification through the active platform instance when the notification platform matches the running bridge platform. If a notification includes structured card content and the active platform supports rich card delivery, the bridge SHALL send the card; otherwise it SHALL fall back to the plain-text notification content.

#### Scenario: Matching platform with card support sends structured content
- **WHEN** the notification receiver receives a notification whose platform matches the active bridge platform
- **AND** the notification contains card content
- **AND** the active platform implements `core.CardSender`
- **THEN** the bridge sends the structured card message

#### Scenario: Matching platform without card support falls back to plain text
- **WHEN** the notification receiver receives a notification whose platform matches the active bridge platform
- **AND** the notification contains card content
- **AND** the active platform does not implement `core.CardSender`
- **THEN** the bridge sends the plain-text notification content instead

#### Scenario: Mismatched platform notification is rejected
- **WHEN** the notification receiver receives a notification whose platform does not match the active bridge platform
- **THEN** the bridge rejects the delivery request with an explicit error
- **AND** the notification is not sent to the wrong IM platform
