# qq-im-platform-support Specification

## Purpose
Define the runnable QQ (NapCat or OneBot) provider contract for the AgentForge IM Bridge, including startup validation, shared command normalization, provider-aware delivery downgrade, and reply-target preservation for asynchronous updates.
## Requirements
### Requirement: QQ provider SHALL be runnable through the shared IM Bridge platform contract
The IM Bridge SHALL expose QQ (NapCat or OneBot) as a runnable built-in provider through the same provider descriptor, transport selection, and configuration validation path used by the other supported IM platforms. The QQ provider MUST support `stub` transport for local verification and MUST support a live transport path that validates the credentials and connection settings required by the documented QQ event and message delivery model before startup succeeds.

#### Scenario: QQ stub starts for local smoke verification
- **WHEN** the Bridge starts with `IM_PLATFORM=qq` and `IM_TRANSPORT_MODE=stub`
- **THEN** the runtime resolves the QQ provider through the shared provider registry
- **AND** local smoke and focused tests can exercise the QQ command and notification paths without external network dependencies

#### Scenario: QQ live startup fails fast on incomplete configuration
- **WHEN** the Bridge starts with `IM_PLATFORM=qq` and `IM_TRANSPORT_MODE=live` but required QQ credentials or transport parameters are missing
- **THEN** startup fails with an actionable QQ configuration error
- **AND** the runtime does not silently fall back to stub mode or another provider

### Requirement: QQ inbound events SHALL normalize into the shared command surface
The system SHALL normalize supported QQ inbound messages or events into `core.Message` values that preserve QQ platform identity, sender identity, conversation identity, reply-target context, and command text needed by the existing `/task`, `/agent`, `/cost`, `/help`, and fallback command flows. The normalized message MUST be sufficient for the shared command engine and backend source propagation to treat QQ traffic as a first-class IM source.

#### Scenario: QQ command message invokes a registered shared command
- **WHEN** a QQ inbound message is normalized into `core.Message` content containing `/task list`
- **THEN** the engine invokes the registered `/task` command handler
- **AND** the resulting response is routed back to the originating QQ conversation context

#### Scenario: QQ plain text uses the fallback path
- **WHEN** a QQ inbound message does not match a registered slash command but is still eligible for the fallback handler
- **THEN** the engine invokes the configured fallback handler
- **AND** the resulting reply is routed back to the originating QQ conversation context

### Requirement: QQ outbound delivery SHALL support explicit structured downgrade semantics
The system SHALL resolve QQ-targeted typed deliveries through a text-first QQ rendering profile. QQ MUST not advertise provider-native payload surfaces or mutable-update lifecycle it does not support. When the requested richer path cannot be honored, the Bridge SHALL convert the delivery into QQ-supported reply-segment-aware text or link output for the originating conversation and preserve explicit fallback metadata instead of pretending richer delivery succeeded.

#### Scenario: Structured QQ notification becomes text-first output
- **WHEN** the notification receiver handles a QQ-targeted delivery with structured or richer content
- **THEN** the Bridge resolves that delivery into QQ-supported text or link output for the active conversation
- **AND** the delivery metadata records that QQ remained on its text-first path

#### Scenario: Native or mutable QQ request degrades explicitly
- **WHEN** a QQ-targeted typed delivery requests native payload or mutable update behavior
- **THEN** the Bridge falls back to supported QQ text delivery
- **AND** operators can see from the delivery metadata that the original richer request was unsupported for QQ

### Requirement: QQ reply targets SHALL preserve enough context for asynchronous updates
When a QQ message starts a backend-backed action that may emit later progress or terminal updates, the Bridge SHALL preserve enough QQ reply-target context to route those later deliveries back to the same user-visible conversation when the platform supports it. That preserved reply target MUST survive control-plane queueing and replay, and any missing update affordance MUST trigger a truthful QQ fallback path rather than a silent drop.

#### Scenario: QQ action binding survives control-plane replay
- **WHEN** a QQ-originated action stores a reply target for later progress delivery
- **THEN** that reply target survives backend persistence and control-plane replay
- **AND** reconnect replay can continue delivering later updates to the same QQ conversation context

#### Scenario: Missing QQ update context falls back truthfully
- **WHEN** a replayed or direct QQ delivery requires richer update context that was not preserved or is no longer valid
- **THEN** the Bridge uses the documented QQ fallback delivery path instead of attempting an invalid update
- **AND** operators can see from the delivery metadata that the original QQ update target was unusable

### Requirement: QQ asynchronous completion SHALL remain text-first and conversation-scoped
When a QQ command or action starts long-running work, asynchronous progress and terminal completion SHALL reuse preserved group or direct-message context through supported text delivery before considering a new unrelated send path. QQ MUST remain text-first: the Bridge SHALL not advertise or attempt provider-native payload or mutable-update semantics that QQ does not actually implement.

#### Scenario: QQ terminal completion reuses reply-aware text delivery
- **WHEN** a QQ-originated long-running action finishes and the preserved conversation or message context is still available
- **THEN** the Bridge delivers the terminal completion back into that same QQ conversation through supported text delivery
- **AND** users do not receive an invented richer payload type

#### Scenario: QQ replay falls back explicitly when reply context is stale
- **WHEN** a replayed QQ completion no longer has a usable reply-aware context
- **THEN** the Bridge emits the documented text fallback
- **AND** delivery metadata records that the original QQ completion context was unusable

