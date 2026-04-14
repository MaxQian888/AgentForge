## ADDED Requirements

### Requirement: Backend-mediated IM delivery SHALL remain bound to the originating bridge instance
When a backend workflow, Bridge runtime event, or bound IM action produces progress or terminal IM output, the Go backend SHALL route that output through the IM control plane to the originating or explicitly targeted live bridge instance. The system MUST NOT bypass the control plane or retarget a bound delivery to an unrelated bridge instance simply because another instance is online.

#### Scenario: Bound task progress returns to the originating IM conversation
- **WHEN** an IM-originated task workflow binds a reply target to a live `bridge_id`
- **THEN** later backend progress deliveries for that workflow are queued to that same bridge instance through the control plane
- **THEN** the IM Bridge can render the progress update into the original conversation without guessing a new destination

#### Scenario: No live bound instance exists for follow-up delivery
- **WHEN** the backend needs to deliver a bound progress or terminal update but the bound bridge instance is no longer live
- **THEN** the control plane reports the delivery as blocked, stale, or retryable according to the failure reason
- **THEN** the backend does not silently reroute the delivery to another unrelated instance

### Requirement: Control-plane delivery sources SHALL stay explicit for diagnostics
The IM control plane SHALL preserve whether a queued outbound delivery originated from backend compatibility send/notify, bound action completion, or progress streaming so operator-facing diagnostics can explain which backend seam produced the message.

#### Scenario: Operator views a queued delivery
- **WHEN** an operator inspects delivery history or bridge status after a queued outbound message
- **THEN** the delivery record identifies its source category and target bridge binding
- **THEN** the operator can distinguish a progress-streaming issue from a generic compatibility send failure
