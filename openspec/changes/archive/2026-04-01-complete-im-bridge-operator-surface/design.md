## Context

当前仓库已经具备 IM Bridge 的三层基础：

- `src-im-bridge` 已经有 8 平台 provider、control-plane listener、typed delivery envelope、reply-target 恢复，以及现有 `/review`、`/task`、`/agent` 等命令面。
- Go 后端已经暴露 `GET /api/v1/im/channels`、`GET /api/v1/im/bridge/status`、`GET /api/v1/im/deliveries`、`POST /api/v1/im/deliveries/:id/retry` 和 `POST /api/v1/im/send` 等 IM operator API。
- 前端 `/im` 页面已经接上 store 和后端 route，并在 sidebar 中可达。

但当前 operator 面仍然不真实完整。`IMBridgeStatus` 只有注册、心跳、providers 和基础 health；`IMMessageHistory` 只覆盖单条 preview/retry；更关键的是，control-plane queue 成功后 backend 立即把 delivery 记成 `delivered`，而不是等待 bridge terminal settlement。这样会让队列深度、成功率、延迟和 test-send 结果都失真。与此同时，活跃 change `enhance-frontend-panel` 虽然定义了更完整的 IM status panel，但它覆盖面过宽，不适合作为这次 IMBridge focused completeness 的实现入口。

## Goals / Non-Goals

**Goals:**

- 让 `/im` 成为一个真实可用的 IM Bridge operator console，而不是只展示基础 tabs。
- 把 control-plane operator contract 扩展为可支撑 queue/backlog、provider diagnostics、aggregate metrics 和 test-send 的 truthful snapshot。
- 让 delivery history 的状态语义从“队列成功即 delivered”改成“queue -> pending -> terminal settlement”，从而支撑真实的 retry、summary 和 latency。
- 复用现有 channel config、delivery history 和 canonical send/retry pipeline，使现有 IMBridge 功能真正被前端和运维工作流使用起来。

**Non-Goals:**

- 不重做各平台 adapter、本地/远端 transport、rich rendering profile 或 command grammar。
- 不引入单独的第二个 IM 管理页面，也不把 IM 配置迁回一个新的 settings 子页面。
- 不把这次 change 扩展为 TS Bridge command surface、chatops orchestration、或更宽的 frontend mega-panel 收尾。
- 不把当前内存态 operator state 升级成长期审计仓；本次只要求当前 runtime/rolling-window 级别的 truthful observability。

## Decisions

### 1. 继续复用现有 `/im` 路由，升级为单一 operator console，而不是新增平行页面

现有 `app/(dashboard)/im/page.tsx` 已经在导航、store、i18n、测试中落位，说明正确 seam 不是再开一个 IM ops page，而是把当前 `/im` 页面升级为完整 console。channel configuration 仍保留在同一路由内；provider 卡片上的 configure affordance 直接切回 channels 视图并预选对应平台/通道上下文。

备选方案：

- 在 `/settings` 下再加一个 IM operations 区块。问题是会和现有 `/im` 路由重复，继续分裂 operator workflow。
- 新建独立 `/im/ops` 页面。问题是配置、状态、history 会再次拆开，和“功能都被使用”的目标相反。

### 2. 把 control-plane 状态从“基础 liveness”提升为 operator snapshot，但仍复用现有 `IMControlPlane` 内存态

`IMControlPlane` 已经跟踪 `instances`、`pending`、`history`、`channels`。这足以衍生出大部分 operator summary：pending backlog、每 provider 最近状态、失败/降级计数、最近 delivery 时间。设计上继续复用这套 state，并把 `GET /api/v1/im/bridge/status` 扩展为一个 operator snapshot，而不是再引入并行 metrics service 或新仓储。

新增/扩展的数据分为两类：

- **可从现有 state 推导的数据**：pending 数量、最近 24 小时 delivered/failed/timeout 数量、最近 fallback、last delivery timestamp。
- **必须由 bridge 上报的数据**：provider-specific diagnostics，例如 webhook health、quota 摘要、最近 transport error、配置警告。

这类 provider diagnostics 通过 bridge 注册/heartbeat 的扩展 metadata 刷新为 last-known snapshot；缺失时显示 `unavailable`，而不是伪造“健康”。

备选方案：

- 单独新增 IM metrics 聚合服务。问题是会复制 control-plane truth source，并增加无必要的系统复杂度。
- 只靠前端从 `deliveries` 数组做所有派生。问题是 queue depth、pending truth、diagnostics 和 latency 无法准确推导。

### 3. delivery history 的状态必须改为 settlement-truthful，而不是 queue 成功即 `delivered`

当前 `IMService.Send/Notify` 在 `QueueDelivery()` 成功后立刻 `RecordDeliveryResult(..., delivered)`，这会把“已入队”误报为“已送达”。本次改成：

- queue 成功时写入/更新 delivery 为 `pending`，记录 `queuedAt`；
- bridge terminal settlement 时通过扩展 ack/update 契约上报 `status`、`processedAt`、`failureReason`、`downgradeReason`；
- backend 以 settlement 更新 history 记录并从 pending queue 中移除；
- 未 settlement 的 delivery 继续保持 `pending`，并参与 backlog 指标；
- latency 只对已 settlement delivery 计算。

这一步是 operator console 成立的前提；否则 summary、history、test-send、retry 全部是假完整。

备选方案：

- 保持 optimistic `delivered`，前端自行解释“其实只是 queued”。问题是契约不真实，且无法支撑失败率和延迟。
- 只在前端隐藏这些数字。问题是用户明确要求“接口接入完整、功能完整、功能都被使用”，不能靠回避指标来绕过 truthfulness。

### 4. operator actions 复用 canonical delivery pipeline，但提供 operator-friendly endpoint

测试发送和批量 retry 都应复用现有 delivery seam，而不是另写 debug transport：

- `POST /api/v1/im/test-send` 复用 canonical send path，写入 test metadata，并在 bounded wait 内等待对应 delivery settlement；若未及时 settlement，则返回 `pending + deliveryId`，由 operator 在 history 中继续观察。
- `POST /api/v1/im/deliveries/retry-batch` 接收显式 `deliveryIds`，逐条复用现有 retry 逻辑并返回 per-item outcome，避免前端循环多次请求造成局部失败难以解释。

这样 operator action 和真实 production path 共用同一套 delivery semantics、fallback metadata 和 history settlement。

备选方案：

- 直接让前端循环调 `/api/v1/im/deliveries/:id/retry`。问题是批量操作的错误收敛、部分成功语义和刷新边界都不清晰。
- 直接复用 `POST /api/v1/im/send` 做 test-send。问题是现有接口缺少 bounded settlement 结果与 operator 反馈结构。

### 5. history 继续走 canonical list endpoint，但增加 filter params 与 richer detail，而不是改成全新 response envelope

为了保持兼容性，`GET /api/v1/im/deliveries` 继续返回 delivery 列表，只增加 query filters，例如 `status`、`platform`、`eventType`、`kind`、`since`。聚合 counters 与 provider summary 放在扩展后的 bridge status snapshot 中，避免强行把 list endpoint 改成 `{items, summary}` 这类 breaking-ish shape。

delivery detail drawer 则在现有 payload preview 上继续补齐 queued/processed timestamps、latency、failure reason、downgrade reason、rendered outcome metadata。

## Risks / Trade-offs

- [Operator state 主要在内存中，进程重启后 rolling metrics 会归零] -> Mitigation: 明确把这些 summary 定义为当前 runtime / recent-window truth，不包装成长期审计；文档与 UI 文案写清语义。
- [Bridge settlement 扩展后，会暴露此前被 optimistic delivered 掩盖的真实失败] -> Mitigation: 增加 focused tests 覆盖 queue -> pending -> delivered/failed/timeout 迁移，并在 UI 中用 pending/degraded 状态如实呈现。
- [不同 provider 的 diagnostics 质量不一致] -> Mitigation: 采用标准化 optional diagnostics contract；缺失时显示 unavailable，而不是把 provider 直接判成 unhealthy。
- [test-send 等待 settlement 可能超时，用户体验变慢] -> Mitigation: 采用 bounded wait；超过阈值返回 pending + deliveryId，并自动刷新 history/status。

## Migration Plan

1. 扩展 Go model / control-plane contract，先让 delivery settlement 和 operator snapshot 变得 truthful，并补上 operator action endpoints。
2. 更新 bridge control-plane ack / diagnostics metadata 上报路径，使 backend 能接到 terminal status 与 provider diagnostics。
3. 更新前端 store 和 `/im` 页面，消费新的 snapshot、filters、batch retry 和 test-send 结果，同时保留现有 channel config / history 交互。
4. scoped verification 以 Go handler/service tests、IM store/component tests 和 targeted page tests 为主；需要 live transport 的平台诊断路径则以 graceful fallback 方式处理。

## Open Questions

- 如果后续需要多日趋势或审计级 IM observability，是否应把 operator summary 从内存态迁到持久化仓储；本次先明确不做，以保持 focused delivery。
