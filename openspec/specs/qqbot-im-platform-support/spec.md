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
The system SHALL resolve QQ Bot-targeted typed deliveries through a QQ Bot rendering profile that can choose a QQ Bot-supported representation for plain text, structured content, or provider-supported richer output. When the requested richer path cannot be honored by the current QQ Bot reply target or payload shape, the Bridge MUST explicitly fall back to a supported QQ Bot text or link delivery and preserve fallback metadata rather than pretending richer delivery succeeded.

#### Scenario: Structured QQ Bot notification uses the provider rendering profile
- **WHEN** the notification receiver handles a QQ Bot-targeted delivery with structured or richer content
- **THEN** the Bridge resolves that delivery through the active QQ Bot rendering profile before transport execution
- **AND** the final transport path uses a QQ Bot-supported representation instead of leaking cross-platform structured payload assumptions into the transport layer

#### Scenario: Unsupported QQ Bot richer payload degrades explicitly
- **WHEN** a QQ Bot-targeted typed delivery requests a richer card or mutable update path that the current reply target does not support
- **THEN** the Bridge sends the supported QQ Bot text or link fallback instead
- **AND** operators can see from the delivery metadata that the original QQ Bot update plan was unusable

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
