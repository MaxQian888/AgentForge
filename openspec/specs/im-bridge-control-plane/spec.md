# im-bridge-control-plane Specification

## Purpose
Define the runtime control-plane contract for AgentForge IM Bridge instances, including stable bridge registration, liveness reporting, authenticated instance-targeted delivery, and replay-safe recovery after reconnect.
## Requirements
### Requirement: Bridge instances register and report liveness to the backend
The system SHALL treat each IM Bridge process as a registered runtime instance identified by a stable `bridge_id`. On startup the Bridge MUST register the set of live providers it hosts and the set of tenants it serves through a single control-plane connection. The registration payload MUST include a `providers[]` array where each entry declares `platform`, `transportMode`, `capabilities`, and the `tenants[]` it can dispatch for, and a top-level `tenants[]` array mapping each `tenantId` to its backend `projectId`. While running, the Bridge MUST refresh its liveness on a bounded interval, and the backend MUST stop routing new deliveries to any `(bridgeId, providerId, tenantId)` triple whose registration has expired or been explicitly revoked. The backend MUST continue to accept the legacy single-provider payload (with a top-level `platform` + `projectId`) by wrapping it as `providers=[{platform, tenants:[<default>]}]` until the next migration phase.

#### Scenario: Multi-provider registration exposes all triples
- **WHEN** a Bridge starts with Feishu and DingTalk active and tenants `acme` and `beta`
- **THEN** its registration payload carries `providers=[{platform:feishu,...,tenants:[acme,beta]},{platform:dingtalk,...,tenants:[acme]}]` and `tenants=[{id:acme,projectId:...},{id:beta,projectId:...}]`
- **AND** the backend marks `(bridgeId, feishu, acme)`, `(bridgeId, feishu, beta)`, and `(bridgeId, dingtalk, acme)` as available for outbound delivery

#### Scenario: Legacy single-provider registration remains compatible
- **WHEN** an older Bridge sends a payload with only top-level `platform=feishu` and `projectId=<id>`
- **THEN** the backend wraps it as a single-provider / single-tenant entry internally
- **AND** later multi-tenant deliveries that do not identify a tenant still resolve to the default tenant derived from `projectId`

#### Scenario: Heartbeat expires one triple at a time
- **WHEN** the Bridge's overall liveness is healthy but the `dingtalk` provider transport has failed and reported itself unavailable
- **THEN** the backend marks only `(bridgeId, dingtalk, *)` triples as unavailable
- **AND** `(bridgeId, feishu, *)` triples remain eligible for routing

#### Scenario: Graceful shutdown unregisters all triples
- **WHEN** a running Bridge shuts down cleanly
- **THEN** it sends an unregister or terminal heartbeat signal covering every `(providerId, tenantId)` it previously registered
- **AND** the backend removes or deactivates those triples without waiting for liveness expiry

### Requirement: Outbound control-plane deliveries are authenticated and instance-targeted

The system SHALL authenticate every backend-to-Bridge control-plane delivery and SHALL route each delivery to a specific `(bridgeId, providerId, tenantId)` triple. A Bridge MUST reject unsigned or invalidly signed deliveries, and it MUST NOT deliver a message that targets another instance, another provider on the same instance, or a tenant that the targeted provider does not serve. HMAC verification MUST use the `IM_SECRET_<PROVIDER>` override when present and fall back to `IM_CONTROL_SHARED_SECRET` otherwise.

**Changes**:
- **ADDED**: Delivery routing key is `(bridgeId, providerId, tenantId)` instead of just `bridgeId`. A delivery whose `tenantId` is not served by `providerId` on this bridge MUST be rejected with `409 tenant_provider_mismatch`.
- **ADDED**: HMAC secret resolution prefers `IM_SECRET_<PROVIDER>` (for example `IM_SECRET_FEISHU`); falls back to `IM_CONTROL_SHARED_SECRET`.
- **ADDED**: 入站签名校验现在 MUST 同时校验 `X-AgentForge-Delivery-Timestamp` 的偏差不超过 `IM_SIGNATURE_SKEW_SECONDS`（默认 300s），偏差过大的请求以 `408 timestamp_out_of_window` 拒绝，即使 HMAC 合法。
- **ADDED**: 幂等 dedupe 状态 MUST 由持久化状态存储（`im-bridge-durable-state`）承担，重启和多副本场景下 `deliveryId` 仍然一次性有效；重复请求以 `200 {"status":"duplicate"}` 响应。
- **ADDED**: 拒绝响应 MUST 按原因分类为 `401 invalid_signature`、`408 timestamp_out_of_window`、`409 duplicate_delivery`、`409 tenant_provider_mismatch`，响应 body 含 `{"error":"<code>","retryable":<bool>}`，以便后端按重试策略区分处理。
- **ADDED**: 每次被接受或被拒绝的入站控制面请求 MUST 触发一条结构化审计 event（见 `im-bridge-audit-trail`）。

#### Scenario: Valid signed delivery reaches the targeted triple
- **WHEN** the backend sends a notification with a valid signature targeting `(bridgeId=X, provider=feishu, tenant=acme)`
- **THEN** the matching Bridge instance accepts the delivery on its Feishu provider under tenant `acme`'s credentials
- **AND** it delivers the payload through the Feishu platform using `acme`'s reply target

#### Scenario: Invalid signature is rejected
- **WHEN** a control-plane delivery arrives without the required signature or with an invalid signature
- **THEN** the Bridge rejects the request with `401 invalid_signature`
- **AND** it does not forward any content to the external IM platform
- **AND** an audit event with `status=rejected metadata.reason=invalid_signature` is emitted

#### Scenario: Delivery timestamp outside skew window is rejected
- **WHEN** a delivery arrives with a valid HMAC signature but `|now − X-AgentForge-Delivery-Timestamp| > IM_SIGNATURE_SKEW_SECONDS`
- **THEN** the Bridge rejects the request with `408 timestamp_out_of_window`
- **AND** the response body indicates `retryable=false` because a retry with the same timestamp cannot succeed
- **AND** an audit event with `status=rejected metadata.reason=timestamp_out_of_window` is emitted

#### Scenario: Duplicate delivery across restart is still suppressed
- **WHEN** the backend retries a `/im/notify` with the same `deliveryId` after the Bridge has restarted
- **THEN** the Bridge responds `200 {"status":"duplicate"}` using durable dedupe state
- **AND** no second message reaches the external IM conversation
- **AND** an audit event with `status=duplicate` is emitted

#### Scenario: Delivery targets an unknown tenant on a known provider
- **WHEN** a Bridge receives a signed delivery targeting `(bridgeId=X, provider=feishu, tenant=gamma)` but `gamma` is not registered on Feishu for this bridge
- **THEN** the Bridge rejects the delivery with `409 tenant_provider_mismatch`
- **AND** an audit event with `status=rejected metadata.reason=tenant_provider_mismatch` is emitted

#### Scenario: Delivery targets a different bridge instance
- **WHEN** a Bridge receives a control-plane delivery whose target `bridge_id` does not match the local instance
- **THEN** the Bridge rejects or ignores that delivery as not-for-this-instance
- **AND** no user-visible IM message is sent from the wrong Bridge

### Requirement: Bridge rate limiting SHALL operate on multi-dimensional policies

IM Bridge SHALL 以 policy 列表（每 policy 定义 `KeyDimensions ⊂ {tenant, chat, user, command, action_class, bridge, provider}` + `Rate` + `Window`）驱动限速。`tenant` 维度 MUST 使用 `im-bridge-tenant-routing` 解析出的 `TenantID` 作为真实键值，替代此前的空/占位值。命令执行前 Bridge MUST 按每条 policy 构建 composite key 并判定，一旦被任一 policy 拦截即返回 `{Allowed:false, Policy:<id>, RetryAfterSec:<n>}`，同时发送审计 event `status=rate_limited metadata.rate_policy=<id> metadata.tenant_id=<id>`。

#### Scenario: Tenant-scoped policy isolates two tenants
- **WHEN** policy `tenant-write`（维度 `tenant+action_class=write`，60/min）生效，tenant `acme` 已在窗口内消耗 60 次写操作，tenant `beta` 刚开始写操作
- **THEN** tenant `acme` 的第 61 次请求被该 policy 拒绝
- **AND** tenant `beta` 在同一窗口内继续被允许
- **AND** 审计事件 `metadata.tenant_id` 分别指向各自 tenant

#### Scenario: Default policies preserve legacy 20/min per session behavior
- **WHEN** 未显式设置 `IM_RATE_POLICY`
- **THEN** Bridge 加载 default policy 集合，其中包含 `session-default` policy（维度 `tenant+chat+user`，20/min），保持已有 SLA
- **AND** 一名用户在同一群聊内 60s 内第 21 次触发命令时被该 policy 拒绝，响应带 `retry_after_sec > 0`

#### Scenario: Write-action policy bounds command-class throughput
- **WHEN** 用户在 60s 内连续触发 10 次 `/task create`，第 11 次再触发
- **THEN** `write-action`（维度 `tenant+user+action_class=write`，10/min）在 `session-default` 之前命中
- **AND** 审计 event `metadata.rate_policy=write-action`

#### Scenario: Custom policy set via IM_RATE_POLICY replaces defaults cleanly
- **WHEN** 运营通过 `IM_RATE_POLICY` 覆盖 policy 列表，移除 `write-action`，加入新的 `destructive-hourly`
- **THEN** Bridge 重启或 SIGHUP 之后 `write-action` 不再生效，`destructive-hourly` 生效
- **AND** 持久化存储里旧 policy id 的历史计数在下次清理周期被回收

### Requirement: Backend SHALL route outbound deliveries by the full triple

The Go backend's IM control plane SHALL index registered bridges by `(bridgeId, providerId, tenantId)` and SHALL select a delivery target using all three keys. When a backend workflow produces an outbound message bound to a specific tenant and provider, the selector MUST prefer an exact triple match on the originating bridge, MUST only fall back to a peer bridge when the originating bridge is no longer live, and MUST refuse to reroute the delivery to a bridge that does not advertise that `(providerId, tenantId)` pair.

#### Scenario: Exact triple match wins over peer liveness
- **WHEN** bridges `A` and `B` both advertise `(feishu, acme)` but the delivery was bound to `A`
- **THEN** the backend routes the delivery to `A` while `A` remains live
- **AND** the backend does not reroute to `B` on the grounds that `B` is healthier

#### Scenario: No peer supports the required triple
- **WHEN** the originating bridge is down and no other live bridge advertises `(dingtalk, beta)`
- **THEN** the backend marks the delivery as `blocked_no_target` rather than routing it to an unrelated triple
- **AND** operator diagnostics expose `blocked_no_target` with the missing `(provider, tenant)` pair

#### Scenario: Peer failover is allowed within the same triple
- **WHEN** bridge `A` goes offline and bridge `B` advertises the same `(feishu, acme)` triple without an origin-bound binding
- **THEN** the backend may fail a non-bound delivery over to `B`
- **AND** the operator snapshot records the failover as a peer reroute with the triple preserved

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

