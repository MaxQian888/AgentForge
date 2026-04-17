## MODIFIED Requirements

### Requirement: Outbound control-plane deliveries are authenticated and instance-targeted

The system SHALL authenticate every backend-to-Bridge control-plane delivery and SHALL route each delivery to either an explicitly targeted `bridge_id` or a backend-selected live instance that matches the requested platform and project binding. A Bridge MUST reject unsigned or invalidly signed deliveries, and it MUST NOT deliver a message that targets another instance.

**Changes**:
- **ADDED**: 入站签名校验现在 MUST 同时校验 `X-AgentForge-Delivery-Timestamp` 的偏差不超过 `IM_SIGNATURE_SKEW_SECONDS`（默认 300s），偏差过大的请求以 `408 timestamp_out_of_window` 拒绝，即使 HMAC 合法。
- **ADDED**: 幂等 dedupe 状态 MUST 由持久化状态存储（`im-bridge-durable-state`）承担，重启和多副本场景下 `deliveryId` 仍然一次性有效；重复请求以 `200 {"status":"duplicate"}` 响应。
- **ADDED**: 拒绝响应 MUST 按原因分类为 `401 invalid_signature`、`408 timestamp_out_of_window`、`409 duplicate_delivery`，响应 body 含 `{"error":"<code>","retryable":<bool>}`，以便后端按重试策略区分处理。
- **ADDED**: 每次被接受或被拒绝的入站控制面请求 MUST 触发一条结构化审计 event（见 `im-bridge-audit-trail`）。

#### Scenario: Valid signed delivery reaches the targeted instance
- **WHEN** the backend sends a notification or progress delivery with a valid signature and a target `bridge_id`
- **THEN** the matching Bridge instance accepts the delivery
- **AND** it delivers the payload through the active IM platform

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

#### Scenario: Delivery targets a different bridge instance
- **WHEN** a Bridge receives a control-plane delivery whose target `bridge_id` does not match the local instance
- **THEN** the Bridge rejects or ignores that delivery as not-for-this-instance
- **AND** no user-visible IM message is sent from the wrong Bridge

### Requirement: Bridge rate limiting SHALL operate on multi-dimensional policies

IM Bridge SHALL 以 policy 列表（每 policy 定义 `KeyDimensions ⊂ {tenant, chat, user, command, action_class, bridge}` + `Rate` + `Window`）驱动限速，取代早期仅以 `platform:chat:user` 单一 key 的全局 20/min 限流。命令执行前 Bridge MUST 按每条 policy 构建 composite key 并判定，一旦被任一 policy 拦截即返回 `{Allowed:false, Policy:<id>, RetryAfterSec:<n>}`，同时发送审计 event `status=rate_limited metadata.rate_policy=<id>`。

#### Scenario: Default policies preserve legacy 20/min per session behavior
- **WHEN** 未显式设置 `IM_RATE_POLICY`
- **THEN** Bridge 加载 default policy 集合，其中包含 `session-default` policy（维度 `chat+user`，20/min），保持已有 SLA
- **AND** 一名用户在同一群聊内 60s 内第 21 次触发命令时被该 policy 拒绝，响应带 `retry_after_sec > 0`

#### Scenario: Write-action policy bounds command-class throughput
- **WHEN** 用户在 60s 内连续触发 10 次 `/task create`，第 11 次再触发
- **THEN** `write-action`（维度 `user+action_class=write`，10/min）在 `session-default` 之前命中
- **AND** 审计 event `metadata.rate_policy=write-action`

#### Scenario: Custom policy set via IM_RATE_POLICY replaces defaults cleanly
- **WHEN** 运营通过 `IM_RATE_POLICY` 覆盖 policy 列表，移除 `write-action`，加入新的 `destructive-hourly`
- **THEN** Bridge 重启或 SIGHUP 之后 `write-action` 不再生效，`destructive-hourly` 生效
- **AND** 持久化存储里旧 policy id 的历史计数在下次清理周期被回收
