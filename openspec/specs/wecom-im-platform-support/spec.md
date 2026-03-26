# wecom-im-platform-support Specification

## Purpose
Define the WeCom-specific runtime, message normalization, delivery fallback, and reply-target contract for the AgentForge IM Bridge.

## Requirements
### Requirement: WeCom provider SHALL be runnable through the shared IM Bridge platform contract
The IM Bridge SHALL expose WeCom as a runnable built-in provider through the same provider descriptor, transport selection, and configuration validation path used by the other supported IM platforms. The WeCom provider MUST support `stub` transport for local verification and MUST support a live transport path that validates the credentials and callback settings required by the documented WeCom application messaging model before startup succeeds.

#### Scenario: WeCom stub starts for local smoke verification
- **WHEN** the Bridge starts with `IM_PLATFORM=wecom` and `IM_TRANSPORT_MODE=stub`
- **THEN** the runtime resolves the WeCom provider through the shared provider registry
- **AND** local smoke and focused tests can exercise the WeCom command and notification paths without external network dependencies

#### Scenario: WeCom live startup fails fast on incomplete configuration
- **WHEN** the Bridge starts with `IM_PLATFORM=wecom` and `IM_TRANSPORT_MODE=live` but required WeCom credentials or callback parameters are missing
- **THEN** startup fails with an actionable configuration error
- **AND** the runtime does not fall back to another provider or silently degrade to stub mode

### Requirement: WeCom inbound events SHALL normalize into the shared command surface
The system SHALL normalize supported WeCom inbound messages or callback events into `core.Message` values that preserve the WeCom platform identity, sender identity, conversation identity, reply-target context, and command text needed by the existing `/task`, `/agent`, `/cost`, `/help`, and fallback command flows. The normalized message MUST be sufficient for the shared command engine and backend source propagation to treat WeCom traffic as a first-class IM source.

#### Scenario: WeCom command message invokes a registered shared command
- **WHEN** a WeCom inbound message is normalized into `core.Message` content containing `/task list`
- **THEN** the shared command engine invokes the registered `/task` handler
- **AND** any backend request triggered by that handler carries `wecom` as the normalized source platform

#### Scenario: WeCom mention or plain text uses the fallback path
- **WHEN** a WeCom inbound message does not match a registered slash command but is still eligible for the fallback handler
- **THEN** the engine invokes the configured fallback path through the same shared command surface
- **AND** the resulting reply is routed back to the originating WeCom conversation context

### Requirement: WeCom outbound delivery SHALL support explicit structured downgrade semantics
The system SHALL resolve WeCom-targeted typed deliveries through a WeCom rendering profile that can choose a supported application-message representation for plain text, structured content, or template-card-oriented output. When the requested richer path cannot be honored by the current WeCom reply target or payload shape, the Bridge MUST explicitly fall back to a supported WeCom text delivery and preserve fallback metadata rather than pretending richer delivery succeeded.

#### Scenario: Structured WeCom notification uses the provider rendering profile
- **WHEN** the backend submits a typed notification for platform `wecom` containing structured content
- **THEN** the Bridge resolves that delivery through the active WeCom rendering profile before transport execution
- **AND** the final transport path uses a WeCom-supported representation instead of leaking cross-platform structured payload assumptions into the transport layer

#### Scenario: Unsupported WeCom richer payload degrades explicitly
- **WHEN** a WeCom-targeted typed delivery requests a richer card or mutable update path that the current reply target does not support
- **THEN** the Bridge sends the supported WeCom text fallback instead
- **AND** the resulting delivery metadata records the downgrade reason for operators and replay diagnostics

### Requirement: WeCom reply targets SHALL preserve enough context for asynchronous updates
When a WeCom message or callback starts a backend-backed action that may emit later progress or terminal updates, the Bridge SHALL preserve enough WeCom reply-target context to route those later deliveries back to the same user-visible conversation or message thread when the platform supports it. That preserved reply target MUST survive control-plane queueing and replay, and any missing update affordance MUST trigger a truthful WeCom fallback path rather than a silent drop.

#### Scenario: WeCom action binding survives control-plane replay
- **WHEN** a WeCom-originated action stores a reply target for later progress delivery
- **THEN** the backend control plane persists that reply target with the typed delivery envelope
- **AND** reconnect replay can continue delivering later updates to the same WeCom conversation context

#### Scenario: Missing WeCom update context falls back truthfully
- **WHEN** a replayed or direct WeCom delivery requires richer update context that was not preserved or is no longer valid
- **THEN** the Bridge uses the documented WeCom fallback delivery path instead of attempting an invalid update
- **AND** operators can see from the delivery metadata that the original WeCom update target was unusable
