## ADDED Requirements

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
