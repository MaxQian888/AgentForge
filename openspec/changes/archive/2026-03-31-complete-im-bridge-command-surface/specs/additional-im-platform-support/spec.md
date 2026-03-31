## MODIFIED Requirements

### Requirement: Core command handling remains platform-consistent across supported platforms
The system SHALL translate Feishu, Slack, DingTalk, Telegram, Discord, WeCom, QQ, and QQ Bot inbound events or interactions into core.Message values that preserve platform identity, user identity, chat identity, reply context, and message content so that the canonical operator command surface executes with consistent semantics across all supported platforms. This surface MUST include existing task, agent, review, sprint, cost, help, and @AgentForge flows plus newly approved operator commands such as agent runtime control, task workflow control, queue visibility, team summary, and memory access. Platform adapters MUST NOT special-case command names or silently drop these supported commands because the active platform entered through slash, mention, callback, or interaction normalization.

#### Scenario: Telegram slash-style task workflow command routes to a shared handler
- **WHEN** a Telegram inbound update is normalized into core.Message content containing /task move task-123 done
- **THEN** the engine invokes the registered /task command handler with the normalized subcommand and args
- **AND** the resulting response is sent back to the originating Telegram chat

#### Scenario: Discord interaction is normalized to an agent control command
- **WHEN** a Discord application command or interaction maps to the logical command /agent status run-123
- **THEN** the engine invokes the registered /agent command handler with the normalized args
- **AND** the response is delivered through the originating Discord interaction context

#### Scenario: WeCom callback event routes to the queue command
- **WHEN** a WeCom inbound callback or application message is normalized into core.Message content containing /queue list queued
- **THEN** the engine invokes the registered /queue command handler through the shared command path
- **AND** the resulting response is sent back to the originating WeCom conversation context

#### Scenario: QQ group command routes to the memory command
- **WHEN** a QQ inbound group or direct message is normalized into core.Message content containing /memory search release
- **THEN** the engine invokes the registered /memory command handler through the shared command path
- **AND** the resulting response is sent back to the originating QQ conversation context

#### Scenario: QQ Bot command routes to the team summary command
- **WHEN** a QQ Bot official inbound message or interaction is normalized into core.Message content containing /team list
- **THEN** the engine invokes the registered /team command handler through the shared command path
- **AND** the resulting response is sent back to the originating QQ Bot conversation context

#### Scenario: Feishu mention uses the existing fallback path
- **WHEN** a Feishu inbound message mentions @AgentForge without matching a registered slash command
- **THEN** the engine invokes the configured fallback handler
- **AND** any command guidance returned to the user references a command from the canonical operator catalog
