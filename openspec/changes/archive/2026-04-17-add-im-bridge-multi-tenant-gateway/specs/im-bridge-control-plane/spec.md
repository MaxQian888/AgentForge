## MODIFIED Requirements

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

## ADDED Requirements

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
