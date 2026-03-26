# im-bridge-control-plane Specification

## Purpose
Define the runtime control-plane contract for AgentForge IM Bridge instances, including stable bridge registration, liveness reporting, authenticated instance-targeted delivery, and replay-safe recovery after reconnect.
## Requirements
### Requirement: Bridge instances register and report liveness to the backend
The system SHALL treat each IM Bridge process as a registered runtime instance identified by a stable `bridge_id`. On startup the Bridge MUST register its active platform, transport mode, delivery capabilities, callback exposure, and project bindings with the backend before it is considered eligible for targeted outbound delivery. While running, the Bridge MUST refresh its liveness on a bounded interval, and the backend MUST stop routing new deliveries to instances whose registration has expired or been explicitly revoked.

#### Scenario: Startup registration succeeds
- **WHEN** an IM Bridge starts with valid configuration and backend credentials
- **THEN** it registers a `bridge_id` together with its normalized platform source, transport mode, callback metadata, and bound project identifiers
- **AND** the backend marks that instance as available for outbound delivery only after registration succeeds

#### Scenario: Heartbeat expires
- **WHEN** a registered Bridge stops refreshing its liveness before the configured expiry window
- **THEN** the backend marks the instance as unavailable
- **AND** new notifications or progress deliveries are not routed to that stale instance

#### Scenario: Graceful shutdown unregisters the instance
- **WHEN** a running Bridge shuts down cleanly
- **THEN** it sends an unregister or terminal heartbeat signal for its `bridge_id`
- **AND** the backend removes or deactivates the instance without waiting for liveness expiry

### Requirement: Outbound control-plane deliveries are authenticated and instance-targeted
The system SHALL authenticate every backend-to-Bridge control-plane delivery and SHALL route each delivery to either an explicitly targeted `bridge_id` or a backend-selected live instance that matches the requested platform and project binding. A Bridge MUST reject unsigned or invalidly signed deliveries, and it MUST NOT deliver a message that targets another instance.

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
The system SHALL preserve pending outbound deliveries across transient Bridge disconnects and SHALL resume from an acknowledged delivery cursor when the Bridge reconnects. Replayed deliveries MUST remain idempotent so the same logical notification or progress update is not sent more than once to the user-visible IM target.

#### Scenario: Reconnect resumes from last acknowledged delivery
- **WHEN** a Bridge loses its persistent control-plane connection after acknowledging delivery cursor `N`
- **AND** pending deliveries `N+1` and later are queued while it is offline
- **THEN** the Bridge reconnects and requests replay beginning after cursor `N`
- **AND** the backend resends only the pending deliveries that were not yet acknowledged

#### Scenario: Duplicate delivery id is suppressed
- **WHEN** a Bridge receives the same logical delivery more than once during replay or retry
- **THEN** it recognizes the duplicate using the delivery identifier
- **AND** it does not send a second copy of that message to the IM conversation

### Requirement: Control-plane deliveries SHALL preserve typed outbound payloads across queue and replay
The system SHALL preserve the canonical typed outbound delivery envelope when a message is queued for a Bridge instance, replayed after reconnect, or acknowledged through the control-plane cursor. Control-plane routing and replay MUST retain rich payload shape, reply-target context, and fallback metadata instead of collapsing the delivery to a text-only `content` field.

#### Scenario: Targeted delivery reaches the Bridge with typed payload intact
- **WHEN** the backend queues a signed delivery containing structured or provider-native payload for a specific `bridge_id`
- **THEN** the control plane routes that typed delivery to the targeted Bridge instance without flattening it to text
- **AND** the Bridge applies the same payload shape during delivery resolution that the backend originally queued

#### Scenario: Reconnect replay preserves rich payload fidelity
- **WHEN** a Bridge reconnects after rich or mutable deliveries were queued while it was offline
- **THEN** replay resumes from the last acknowledged cursor using the same typed delivery envelope
- **AND** the replayed delivery still contains the structured/native payload, reply target, and fallback metadata needed for the correct provider-native update path

#### Scenario: Duplicate ack suppresses the same typed delivery
- **WHEN** a Bridge acknowledges a typed delivery cursor and later reconnect logic encounters the same delivery again
- **THEN** the control plane suppresses the duplicate replay using the delivery cursor and identifier
- **AND** users do not receive a second copy of the same rich or terminal delivery

