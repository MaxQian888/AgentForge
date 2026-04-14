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

### Requirement: Bridge status snapshot SHALL expose operator-facing runtime summary

`GET /api/v1/im/bridge/status` SHALL return an operator-oriented IM Bridge snapshot in addition to basic liveness. The snapshot MUST include overall health, per-provider transport and capability data, pending delivery counts, recent delivery summary, rolling aggregate counters, and last-known provider diagnostics metadata when available.

#### Scenario: Status snapshot includes backlog and recent delivery health
- **WHEN** one Feishu bridge has pending deliveries and recent fallback or failure activity
- **THEN** `GET /api/v1/im/bridge/status` includes that provider's pending count, last settled delivery timestamp, recent failure or fallback summary, and the aggregate pending/error counters for the operator console

#### Scenario: Status snapshot tolerates missing diagnostics metadata
- **WHEN** a registered provider has not reported optional diagnostics metadata
- **THEN** the status endpoint still returns the provider entry successfully
- **THEN** the diagnostics field is marked unavailable instead of failing the entire snapshot

### Requirement: Control-plane delivery settlement SHALL be operator-truthful

A delivery queued for a live IM Bridge SHALL be recorded as `pending` until the bridge reports a terminal settlement. The bridge settlement payload MUST carry terminal status, processed timestamp, and optional failure or downgrade reason so operator history, queue depth, and latency metrics reflect actual delivery outcomes instead of optimistic queue acceptance.

#### Scenario: Successful settlement updates a pending delivery
- **WHEN** the backend queues delivery `d1` and the bridge later settles `d1` with status `delivered`
- **THEN** `d1` is removed from the pending backlog, marked `delivered` in history, and assigned a processed timestamp and latency derived from queue time to settlement time

#### Scenario: Failed settlement remains visible in operator history
- **WHEN** the bridge settles delivery `d2` with status `failed` and failure reason `rate_limit`
- **THEN** the history record for `d2` is marked `failed`
- **THEN** the failure reason is persisted and included in subsequent operator snapshot and history responses

#### Scenario: Unsettled delivery stays pending
- **WHEN** the backend queues delivery `d3` and no terminal settlement has been reported yet
- **THEN** `d3` remains `pending` in the operator snapshot and history
- **THEN** `d3` is not counted as delivered for success-rate or latency metrics

### Requirement: Bridge registration and heartbeat SHALL support optional diagnostics refresh

Bridge registration and heartbeat flows SHALL allow an instance to refresh optional operator diagnostics metadata, including transport warnings, callback health, quota summaries, or last transport error snapshots. The backend MUST store the latest diagnostics per bridge instance and expose them through the operator status snapshot.

#### Scenario: Heartbeat refreshes diagnostics metadata
- **WHEN** a bridge heartbeat reports webhook health `healthy` and quota summary metadata
- **THEN** the backend stores the latest diagnostics for that bridge instance
- **THEN** the next operator status snapshot exposes those diagnostics on the matching provider card

### Requirement: IM /review command supports deep, approve, and request-changes subcommands
The system SHALL extend the IM `/review` command handler to accept three additional subcommands: `deep <pr-url>`, `approve <review-id>`, and `request-changes <review-id> [comment]`. Each subcommand SHALL call the corresponding backend API and reply with a structured result card. The existing `/review <pr-url>` and `/review status <id>` commands SHALL remain unchanged.

#### Scenario: /review deep <pr-url> creates a standalone deep review
- **WHEN** an IM user sends `/review deep <pr-url>`
- **THEN** the bridge calls the standalone deep review creation API
- **THEN** the bridge replies with a card showing review ID, initial pending status, and a "View Review" link

#### Scenario: /review approve <review-id> approves a pending_human review
- **WHEN** an IM user sends `/review approve <review-id>`
- **THEN** the bridge calls `ApproveReview` for the specified review ID with the IM user's identity as the actor
- **THEN** the bridge replies with a confirmation card showing the updated review status

#### Scenario: /review approve on a non-pending_human review returns an error card
- **WHEN** an IM user sends `/review approve <review-id>` for a review not in `pending_human` state
- **THEN** the bridge receives a backend error and replies with an error card describing the invalid transition

#### Scenario: /review request-changes <review-id> <comment> records a changes request
- **WHEN** an IM user sends `/review request-changes <review-id> <comment>`
- **THEN** the bridge calls `RequestChangesReview` with the review ID and the supplied comment
- **THEN** the bridge replies with a confirmation card showing the review ID and new state

### Requirement: Review result cards include approve and request-changes action buttons
The system SHALL include inline action buttons on review result cards delivered to IM platforms when the review is in `pending_human` state. The card SHALL contain at minimum an "Approve" button and a "Request Changes" button that trigger the corresponding `/review approve` and `/review request-changes` flows via the existing IM action execution infrastructure.

#### Scenario: pending_human review card includes action buttons
- **WHEN** the bridge delivers a review card for a review in `pending_human` state
- **THEN** the card includes interactive "Approve" and "Request Changes" buttons
- **THEN** pressing "Approve" triggers the approve flow for the review ID embedded in the card
- **THEN** pressing "Request Changes" prompts the user for a comment and triggers the request-changes flow

#### Scenario: Completed review card does not include action buttons
- **WHEN** the bridge delivers a review card for a review in a terminal completed state
- **THEN** the card does not include Approve or Request Changes buttons
- **THEN** the card may include a "View Details" link only

### Requirement: IM Bridge 端到端测试流程文档
TESTING.md SHALL 新增 IM Bridge 端到端测试流程章节，包含：测试环境准备（Mock 平台 webhook）、消息收发验证、平台配置测试、Payload 格式验证、错误场景覆盖。

#### Scenario: 验证飞书消息收发
- **WHEN** 开发者需要测试飞书集成
- **THEN** 文档提供 Mock webhook 配置方法和消息发送验证步骤

#### Scenario: 验证多平台 Payload 格式
- **WHEN** 开发者修改了 IM Bridge 的消息格式
- **THEN** 文档提供各平台 Payload 格式的测试用例和验证方法

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

