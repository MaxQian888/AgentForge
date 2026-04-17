# im-bridge-durable-state Specification

## Purpose
Define the durable local state contract for AgentForge IM Bridge, covering persistent delivery dedupe across restarts, durable rate-limit counter storage with per-policy cleanup, and operator-configurable state directory with explicit failure modes instead of silent in-memory fallback.
## Requirements
### Requirement: Bridge SHALL persist delivery dedupe state across restarts

IM Bridge SHALL 将已处理的 `deliveryId` 及其入口类别持久化到本地存储（默认 `${IM_BRIDGE_STATE_DIR}/state.db`），使得进程重启或多副本部署场景下同一 `deliveryId` 不会被第二次投递到外部 IM 平台。持久化记录 MUST 带显式 TTL（默认与 `IM_SIGNATURE_SKEW_SECONDS + 60s` 对齐），并 MUST 通过后台清理周期性回收过期记录以防止存储无界增长。

#### Scenario: Duplicate deliveryId survives bridge restart
- **WHEN** Bridge 在 T1 收到 `deliveryId=D` 并成功处理，随后进程在 T1+30s 重启
- **THEN** Bridge 重启完成后，若再次收到同一 `deliveryId=D` 的签名请求，响应 `200 {"status":"duplicate"}` 且不触发新的外部 IM 投递
- **AND** 审计流记录该事件 `status=duplicate`，`surface` 与首次保持一致

#### Scenario: Dedupe records expire within skew window + grace period
- **WHEN** `IM_SIGNATURE_SKEW_SECONDS=300`，某 `deliveryId=D` 在 T1 被记录
- **THEN** 后台清理在 `T1 + 300 + 60s` 之后删除该记录
- **AND** 之后若同 `deliveryId=D` 再次到达，会被时间戳窗口校验以 408 拦截而不再依赖 dedupe

#### Scenario: State store remains functional under concurrent writes
- **WHEN** Bridge 同一瞬间处理多个不同 `deliveryId` 的 `/im/notify` 请求
- **THEN** 所有请求都能在 5 秒内完成 dedupe 检查，未阻塞关键路径超过 `busy_timeout`
- **AND** 无一 `deliveryId` 被跳过记录或错误地报告为 duplicate

### Requirement: Bridge SHALL persist rate limit state across restarts

IM Bridge SHALL 将限速滑窗计数持久化到本地状态存储，使进程重启或横向扩副本场景下限速阈值不会因进程生命周期而重置。限速数据 MUST 按 policy id + composite scope key 组织，MUST 有独立于 dedupe 的清理路径，且 MUST 在 policy 配置（`IM_RATE_POLICY`）变化时能丢弃失效 policy 的旧记录。

#### Scenario: Rate counter survives restart within policy window
- **WHEN** 用户 U 在 policy `write-action`（10/min）内已触发 9 次，bridge 随即重启
- **THEN** 重启后 U 在 policy window 内的下一次写操作会被观察为第 10 次
- **AND** 第 11 次请求被拒绝，响应含 `retry_after_sec` 指向 window 剩余时间

#### Scenario: Stale policy records are cleaned up on reconfiguration
- **WHEN** 运营通过 `IM_RATE_POLICY` 删除 policy id `destructive-action` 并重启 bridge
- **THEN** 旧 `destructive-action` 的记录在下一次清理周期被回收
- **AND** 不会对新 policy 集合的计数产生干扰

### Requirement: State store location SHALL be operator-configurable and self-healing

IM Bridge SHALL 通过 `IM_BRIDGE_STATE_DIR` 接受状态目录覆盖，若目录不存在或不可写 MUST 在启动时明确失败并给出可操作的错误，而不是静默降级回内存模式。运营如需临时禁用持久化 MUST 通过显式 env（`IM_DISABLE_DURABLE_STATE=true`）回退到内存行为，且回退路径 MUST 在启动日志和审计流里打出 warning 以避免意外生效。

#### Scenario: Startup fails fast on unwritable state directory
- **WHEN** `IM_BRIDGE_STATE_DIR=/nonwritable/path`
- **THEN** Bridge 启动以非零状态退出，错误信息包含目录路径和失败原因
- **AND** 运营得以立刻纠正部署配置而不是在运行时遭遇幂等失效

#### Scenario: Explicit in-memory fallback is clearly signaled
- **WHEN** `IM_DISABLE_DURABLE_STATE=true` 被设置
- **THEN** Bridge 启动 log 打印 `durable_state=disabled fallback=memory` 级别 warning
- **AND** 审计流首条 event 为 `direction=internal, action=durable_state_disabled`

### Requirement: Durable state schema SHALL include session, intent, and reply-binding tables

The Bridge's durable state store SHALL provision three additional tables alongside the existing dedupe and rate-limit tables: `session_history`, `intent_cache`, and `reply_target_binding`. These tables MUST live in the same SQLite database file referenced by `IM_BRIDGE_STATE_DIR/state.db`, MUST use `tenant_id` as a primary-key prefix on every row, and MUST be created or migrated idempotently on bridge startup so operators do not have to manage schema versions by hand.

#### Scenario: Fresh state.db is provisioned with all new tables
- **WHEN** the bridge starts against an empty `state.db`
- **THEN** it creates `session_history`, `intent_cache`, and `reply_target_binding` tables with `tenant_id` as the primary-key prefix
- **AND** subsequent startups are no-ops against those tables

#### Scenario: Existing state.db is upgraded in place
- **WHEN** the bridge starts against a `state.db` created by a prior version that only had dedupe and rate tables
- **THEN** the missing session tables are created without touching the existing data
- **AND** startup logs the migration outcome with a clear "added N tables" summary

#### Scenario: Migration failure keeps the bridge from accepting traffic
- **WHEN** the schema migration fails (for example due to a corrupt SQLite file)
- **THEN** the bridge exits non-zero with an actionable error and does not enter the ready state
- **AND** no session write path is exposed to handlers

### Requirement: Session persistence workload SHALL share the state store's operational rules

Writes and reads against the session tables SHALL use the same SQLite connection pool, busy-timeout, and background cleanup scheduler as dedupe and rate-limit tables. Cleanup of session tables MUST follow the same general rule: bounded periodic sweeps, independent per-table TTL configuration, and explicit operator signaling when a table grows beyond the configured soft cap. Session cleanup MUST NOT block dedupe or rate-limit writes for more than the shared busy-timeout, and the cleanup worker MUST surface per-table timing through the existing state-store diagnostics.

#### Scenario: Cleanup sweeps sessions alongside dedupe without starving writes
- **WHEN** the cleanup worker runs a periodic sweep that touches all five tables
- **THEN** session cleanup completes within the shared busy-timeout budget and yields back to the pool
- **AND** concurrent dedupe writes remain unblocked beyond `busy_timeout`

#### Scenario: Soft cap on a session table produces the same warning surface
- **WHEN** `session_history` exceeds its configured soft cap
- **THEN** the cleanup worker emits the same warning path used by dedupe growth warnings
- **AND** the operator diagnostics surface the session-table warning alongside any existing dedupe or rate warnings

#### Scenario: Explicit in-memory fallback disables session persistence too
- **WHEN** `IM_DISABLE_DURABLE_STATE=true`
- **THEN** session history, intent cache, and reply-target bindings also fall back to in-memory storage with the same warning signaling as the existing dedupe fallback
- **AND** the warning names every subsystem that is running in memory, not just dedupe

