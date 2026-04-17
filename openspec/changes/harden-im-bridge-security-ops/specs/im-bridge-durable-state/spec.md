## ADDED Requirements

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
