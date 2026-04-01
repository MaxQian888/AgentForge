# im-bridge-control-plane Specification (Delta)

## Purpose
Enhance the IM Bridge control plane to intelligently route commands through the appropriate backend layer (Bridge vs Go API) based on capability location, ensuring feature completeness and architectural clarity.

## MODIFIED Requirements

### Requirement: Outbound control-plane deliveries are authenticated and instance-targeted
The system SHALL authenticate every backend-to-Bridge control-plane delivery and SHALL route each delivery to either an explicitly targeted `bridge_id` or a backend-selected live instance that matches the requested platform and project binding. A Bridge MUST reject unsigned or invalidly signed deliveries, and it MUST NOT deliver a message that targets another instance.

**Changes**:
- **ADDED**: When IM commands are processed, the system now determines whether to route through Bridge or Go API based on capability type
- **ADDED**: Capability routing logic that checks if Bridge is available and has the requested capability before deciding routing path
- **ADDED**: Fallback behavior when Bridge is unavailable for Bridge-specific capabilities

#### Scenario: Natural language command routes through Bridge classify-intent
- **WHEN** user sends `@AgentForge show me the sprint status` and Bridge is available
- **THEN** IM Bridge checks if Bridge supports `classify-intent` capability
- **THEN** IM Bridge calls `POST /api/v1/ai/classify-intent` with `{ text: "show me the sprint status", candidates: [...] }`
- **THEN** Bridge returns `{ intent: "sprint_status", confidence: 0.95 }`
- **THEN** IM Bridge routes to `/sprint status` command handler
- **AND** IM Bridge executes command and displays result

#### Scenario: Bridge-specific command routes directly to Bridge
- **WHEN** user sends `/agent runtimes` command
- **THEN** IM Bridge identifies this as a Bridge-specific capability
- **THEN** IM Bridge checks Bridge availability
- **THEN** IM Bridge calls `GET /api/v1/bridge/runtimes`
- **THEN** Go backend proxies to `GET http://localhost:7778/bridge/runtimes`
- **AND** IM Bridge displays runtime list from Bridge response

#### Scenario: Legacy command routes to Go API directly
- **WHEN** user sends `/task create Fix the bug` command
- **THEN** IM Bridge identifies this as a Go API capability (task persistence)
- **THEN** IM Bridge calls `POST /api/v1/tasks` directly (not through Bridge proxy)
- **AND** IM Bridge displays task creation result

#### Scenario: Bridge capability with fallback
- **WHEN** user sends `/task decompose task-123` and Bridge is unavailable
- **THEN** IM Bridge detects Bridge is unavailable
- **THEN** IM Bridge attempts fallback to Go API decompose endpoint
- **THEN** IM Bridge displays result with note "Using fallback (Bridge unavailable)"
 if fallback succeeds
 - **OR** IM Bridge displays error "Bridge required for this operation" if no fallback available

#### Scenario: Valid signed delivery reaches the targeted instance
- **WHEN** the backend sends a notification or progress delivery with a valid signature and a target `bridge_id`
- **THEN** the matching Bridge instance accepts the delivery
- **AND** it delivers the payload through the active IM platform

#### Scenario: Invalid signature is rejected
- **WHEN** a control-plane delivery arrives without the required signature or with an invalid signature
- **THEN** the Bridge rejects the request with an authentication error
- **AND** it does not forward any content to the external IM platform

#### Scenario: Delivery targets a different bridge instance
- **WHEN** a Bridge receives a control-plane delivery whose target `bridge_id` does not match the local instance
- **THEN** the Bridge rejects or ignores that delivery as not-for-this-instance
- **AND** no user-visible IM message is sent from the wrong Bridge

### Requirement: Control-plane delivery resumes safely after reconnect
The system SHALL preserve pending outbound deliveries across transient Bridge disconnects and SHALL resume from the last acknowledged delivery cursor when the Bridge reconnects. Replayed deliveries MUST remain idempotent so the same logical notification or progress update is not sent more than once to the user-visible IM target.

**Changes**: No changes to this requirement (preserved as-is)

#### Scenario: Reconnect resumes from last acknowledged delivery
- **WHEN** a Bridge loses connection after acknowledging delivery cursor `N`
- **AND** pending deliveries `N+1` and later were queued while it is offline
- **THEN** the Bridge reconnects and requests replay beginning after cursor `N`
- **AND** the backend resends only the pending deliveries that had not yet acknowledged

#### Scenario: Duplicate delivery id is suppressed
- **WHEN** a Bridge receives the same logical delivery more than once during replay or retry
- **THEN** it recognizes the duplicate using the delivery identifier
- **AND** it does not send a second copy of that message to the IM conversation

### Requirement: Control-plane deliveries SHALL preserve typed outbound payloads across queue and replay
The system SHALL preserve the canonical typed outbound delivery envelope when a message is queued for a Bridge instance, replayed after reconnect, and acknowledged through the control-plane cursor. Control-plane routing and replay MUST retain rich payload shape, reply-target context, and fallback metadata instead of collapsing the delivery to a text-only `content` field.

**Changes**: No changes to this requirement (preserved as-is)

#### Scenario: Targeted delivery reaches the Bridge with typed payload intact
- **WHEN** the backend queues a signed delivery containing structured or provider-native payload for a specific `bridge_id`
- **THEN** the control plane routes that typed delivery to the targeted Bridge instance without flattening it to text
- **AND** the Bridge applies the same payload shape during delivery resolution as the backend originally queued

#### Scenario: Reconnect replay preserves rich payload fidelity
- **WHEN** a Bridge reconnects after rich or mutable deliveries were queued while it was offline
- **THEN** replay resumes from the last acknowledged cursor using the same typed delivery envelope
- **AND** the replayed delivery still contains the structured/native payload, reply target, and fallback metadata needed for the correct provider-native update path

#### Scenario: Duplicate ack suppresses the same typed delivery
- **WHEN** a Bridge acknowledges a typed delivery cursor and later reconnect logic encounters the same delivery again
- **THEN** the control plane suppresses the duplicate replay using the delivery cursor and identifier
- **AND** users do not receive a second copy of the same rich or terminal delivery
