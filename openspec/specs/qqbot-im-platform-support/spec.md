# qqbot-im-platform-support Specification

## Purpose
Define the runnable QQ Bot provider contract for the AgentForge IM Bridge, including startup validation, shared command normalization, provider-aware delivery downgrade, and reply-target preservation for asynchronous updates.
## Requirements
### Requirement: QQ Bot provider SHALL be runnable through the shared IM Bridge platform contract
The IM Bridge SHALL expose QQ Bot 官方 as a runnable built-in provider through the same provider descriptor, transport selection, and configuration validation path used by the other supported IM platforms. The QQ Bot provider MUST support `stub` transport for local verification and MUST support a live transport path that validates the credentials and connection settings required by the documented QQ Bot event and message delivery model before startup succeeds.

#### Scenario: QQ Bot stub starts for local smoke verification
- **WHEN** the Bridge starts with `IM_PLATFORM=qqbot` and `IM_TRANSPORT_MODE=stub`
- **THEN** the runtime resolves the QQ Bot provider through the shared provider registry
- **AND** local smoke and focused tests can exercise the QQ Bot command and notification paths without external network dependencies

#### Scenario: QQ Bot live startup fails fast on incomplete configuration
- **WHEN** the Bridge starts with `IM_PLATFORM=qqbot` and `IM_TRANSPORT_MODE=live` but required QQ Bot credentials or transport parameters are missing
- **THEN** startup fails with an actionable QQ Bot configuration error
- **AND** the runtime does not silently fall back to stub mode or another provider

### Requirement: QQ Bot inbound events SHALL normalize into the shared command surface
The system SHALL normalize supported QQ Bot inbound messages or events into `core.Message` values that preserve QQ Bot platform identity, sender identity, conversation identity, reply-target context, and command text needed by the existing `/task`, `/agent`, `/cost`, `/help`, and fallback command flows. The normalized message MUST be sufficient for the shared command engine and backend source propagation to treat QQ Bot traffic as a first-class IM source.

#### Scenario: QQ Bot command message invokes a registered shared command
- **WHEN** a QQ Bot inbound message is normalized into `core.Message` content containing `/task list`
- **THEN** the engine invokes the registered `/task` command handler
- **AND** the resulting response is routed back to the originating QQ Bot conversation context

#### Scenario: QQ Bot plain text uses the fallback path
- **WHEN** a QQ Bot inbound message does not match a registered slash command but is still eligible for the fallback handler
- **THEN** the engine invokes the configured fallback handler
- **AND** the resulting reply is routed back to the originating QQ Bot conversation context

### Requirement: QQ Bot outbound delivery SHALL support explicit structured downgrade semantics
The system SHALL resolve QQ Bot-targeted typed deliveries through a markdown-first QQ Bot rendering profile that may include keyboard buttons when the current scene and payload support them. QQ Bot MUST preserve the truthful boundary between markdown or keyboard send, reply-target reuse via preserved conversation metadata, and the absence of Feishu-style mutable card lifecycle. When the requested richer path cannot be honored, the Bridge SHALL fall back explicitly to supported QQ Bot text output and preserve fallback metadata.

#### Scenario: Markdown QQ Bot notification uses the provider rendering profile
- **WHEN** the notification receiver handles a QQ Bot-targeted delivery with markdown-compatible richer content
- **THEN** the Bridge resolves that delivery through the active QQ Bot rendering profile and uses the QQ Bot markdown path
- **AND** the resulting delivery metadata preserves that QQ Bot-native markdown rendering was chosen

#### Scenario: QQ Bot keyboard or mutable update request degrades explicitly when context is incompatible
- **WHEN** a QQ Bot-targeted delivery requests keyboard-assisted completion or mutable richer update behavior that the current reply target cannot honor
- **THEN** the Bridge falls back to a supported QQ Bot text follow-up path
- **AND** the resulting delivery metadata records that the original richer update plan was unavailable

#### Scenario: QQ Bot reply-target reuse remains truthful
- **WHEN** a QQ Bot-originated action preserves conversation metadata that supports replying in place to the same chat context
- **THEN** the Bridge reuses that preserved reply target for the follow-up delivery path it actually supports
- **AND** it does not claim full rich-card lifecycle parity when only markdown or keyboard send is available

### Requirement: QQ Bot reply targets SHALL preserve enough context for asynchronous updates
When a QQ Bot message starts a backend-backed action that may emit later progress or terminal updates, the Bridge SHALL preserve enough QQ Bot reply-target context to route those later deliveries back to the same user-visible conversation when the platform supports it. That preserved reply target MUST survive control-plane queueing and replay, and any missing update affordance MUST trigger a truthful QQ Bot fallback path rather than a silent drop.

#### Scenario: QQ Bot action binding survives control-plane replay
- **WHEN** a QQ Bot-originated action stores a reply target for later progress delivery
- **THEN** that reply target survives backend persistence and control-plane replay
- **AND** reconnect replay can continue delivering later updates to the same QQ Bot conversation context

#### Scenario: Missing QQ Bot update context falls back truthfully
- **WHEN** a replayed or direct QQ Bot delivery requires richer update context that was not preserved or is no longer valid
- **THEN** the Bridge uses the documented QQ Bot fallback delivery path instead of attempting an invalid update
- **AND** operators can see from the delivery metadata that the original QQ Bot update target was unusable

### Requirement: QQ Bot asynchronous completion SHALL prefer msg-id-aware reply before generic follow-up
When a QQ Bot inbound message or interaction starts long-running work, asynchronous progress and terminal completion SHALL first use preserved `msg_id` and conversation context for the provider-supported reply path. If the requested richer behavior cannot be honored in that context, the Bridge SHALL fall back to supported markdown or text follow-up and preserve explicit downgrade metadata.

#### Scenario: QQ Bot completion uses preserved msg_id reply context
- **WHEN** a QQ Bot-originated long-running action finishes while preserved `msg_id` and conversation context are still usable
- **THEN** the Bridge delivers the completion through the provider-supported reply path tied to that context
- **AND** the completion remains visible in the same user-facing conversation

#### Scenario: QQ Bot mutable-update request degrades explicitly
- **WHEN** a QQ Bot progress or terminal update requests mutable richer behavior that the preserved reply context cannot honor
- **THEN** the Bridge falls back to supported markdown or text follow-up delivery
- **AND** the resulting metadata records that the original mutable-update plan was unavailable

