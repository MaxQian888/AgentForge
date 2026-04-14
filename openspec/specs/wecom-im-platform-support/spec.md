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
The system SHALL resolve WeCom-targeted typed deliveries through a WeCom rendering profile that can choose a WeCom-supported template-card, markdown, or text representation according to the active reply target and payload shape. The Bridge MUST preserve the distinction between WeCom's richer send-time surfaces and Feishu-style mutable card lifecycle: WeCom MAY send richer payloads when the provider contract supports them, but it MUST fall back explicitly when the request requires unsupported in-place update or callback-dependent richer behavior.

#### Scenario: Template-card-capable WeCom notification uses the provider rendering profile
- **WHEN** the backend submits a WeCom-targeted delivery with template-card-compatible content
- **THEN** the Bridge resolves that delivery through the active WeCom rendering profile and sends a supported WeCom richer payload
- **AND** the transport layer does not require shared code to assemble provider-specific WeCom payloads directly

#### Scenario: WeCom richer update request falls back truthfully
- **WHEN** a WeCom-targeted delivery requests mutable richer update behavior that the current reply target or provider contract cannot honor
- **THEN** the Bridge sends a supported WeCom markdown or text fallback instead
- **AND** the resulting delivery metadata records that the richer update path was unavailable

#### Scenario: WeCom reply-first path remains explicit
- **WHEN** a WeCom-originated action has a preserved response context that supports a direct reply
- **THEN** the Bridge prefers that WeCom reply path first
- **AND** any fallback to direct app-message send remains explicit in the delivery metadata rather than being treated as invisible parity

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

### Requirement: WeCom asynchronous completion SHALL prefer callback reply and explicit direct-send fallback
When a WeCom message or callback starts long-running work, asynchronous progress and terminal completion SHALL first use the preserved WeCom callback reply context when it is still valid. If callback reply is unavailable, the Bridge SHALL fall back to the documented direct app-message send path and preserve metadata that the completion left the original reply context.

#### Scenario: WeCom terminal completion uses preserved response_url
- **WHEN** a WeCom callback-triggered action finishes while the preserved `response_url` is still valid
- **THEN** the Bridge delivers the terminal completion through that callback reply path
- **AND** the completion remains tied to the original WeCom conversation context

#### Scenario: WeCom progress falls back to direct send when callback context is unavailable
- **WHEN** a queued or replayed WeCom progress update no longer has a usable callback reply context
- **THEN** the Bridge falls back to direct app-message send using the preserved chat or user target
- **AND** delivery metadata records that callback reply context was unavailable

