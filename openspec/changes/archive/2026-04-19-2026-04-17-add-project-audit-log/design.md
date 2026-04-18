## Context

`add-project-rbac` 把"谁能做什么"做完之后，留下的真实基座欠账是"谁实际做了什么"。两者不可分开：

- RBAC 没有审计：无法证明权限策略被正确执行，也无法在事后判定某次事故中涉事人员。
- 审计没有 RBAC：事件里缺少"发起时的 role"上下文，排障只能看到动作但看不到"按当时策略应不应发生"。

当前仓库已有两类"日志"，但语义都不对：

- `log_handler` + `log_repo`：业务/执行日志，focus on agent/scheduler 执行过程与排错。无 actor/action 枚举，payload 非结构化。
- `plugin_event_audit`：只覆盖插件 enable/disable/install 事件。

把审计做在这两个里都会让语义混淆，也让查询无法通过 actor+action 索引化。所以必须独立表 + 独立服务。

审计还有一个必须早期决定的工程面问题：写入是不是主流程的 critical path？答案是"不是"——审计丢一条不应该阻塞业务；但"丢了没人知道"又不可接受。这需要一套**best-effort + 补偿**的写路径。

## Goals / Non-Goals

**Goals:**
- 给每个 `ActionID` 发生的写动作发射一条可查询的结构化审计事件。
- 审计事件表独立于 log_handler，schema 面向 actor/action/resource 查询，而非面向执行排障。
- 与 RBAC 的 ActionID 枚举完全对齐，不定义第二套 action 命名。
- 查询 API 默认只对 `admin+` 开放；特定场景允许 `editor` 查看自己发起的事件（本 change 先定 spec，允许实现上先做 `admin+`-only 并在 follow-up 放宽）。
- 写路径 best-effort：主流程不因审计表故障失败，但必须有重试与 observability。
- 内测期永久保留，不做 TTL。

**Non-Goals:**
- 不做审计数据的导出、图表化、仪表盘（留给 Wave 2 或运维工具）。
- 不做跨项目或组织级审计视图。
- 不做"reverted/undo" 系统（审计是追溯，不是回放）。
- 不做业务/执行日志（`log_handler`）与审计事件的合并或改名。
- 不做事件 hash 链 / 不可篡改签名（未来合规需要可加，不是内测期基础）。
- 不加入"查看者（viewer）自己触发了哪些读动作"——只审计 write 类 ActionID。

## Decisions

### 1. 事件表独立于 `log` 和 `plugin_event`

新建 `project_audit_events`，schema 与索引为审计查询而设计：

```
project_audit_events
├─ id                          UUID PRIMARY KEY
├─ project_id                  UUID NOT NULL              (index: project_id, occurred_at DESC)
├─ occurred_at                 TIMESTAMPTZ NOT NULL       (same composite index)
├─ actor_user_id               UUID                       (index: project_id, actor_user_id)
│                                                         (nullable only for system_initiated=true)
├─ actor_project_role_at_time  VARCHAR(16)                (owner/admin/editor/viewer, snapshot at event time)
├─ action_id                   VARCHAR(64) NOT NULL       (index: project_id, action_id)
├─ resource_type               VARCHAR(32) NOT NULL       (project|member|task|team_run|workflow|wiki|settings|automation|dashboard)
├─ resource_id                 VARCHAR(64)                (nullable for collection-level actions)
├─ payload_snapshot_json       JSONB                      (bounded size, see Decision 4)
├─ system_initiated            BOOLEAN NOT NULL
├─ configured_by_user_id       UUID                       (non-null when system_initiated=true; the human who authorized the automation)
├─ request_id                  VARCHAR(64)                (trace correlation)
├─ ip                          VARCHAR(64)                (nullable)
├─ user_agent                  TEXT                       (nullable, truncated)
└─ created_at                  TIMESTAMPTZ DEFAULT now()  (for debug; occurred_at is the canonical time)
```

**Why this**：查询是 `WHERE project_id=? AND occurred_at BETWEEN ? ORDER BY occurred_at DESC`，外加 `action_id` / `actor_user_id` 过滤；此索引布局直接对上。JSONB payload 留给事件细节扩展不改 schema。

**Alternative rejected – 复用 `logs` 表**：logs 面向执行可观测，结构是 stream/level/message；硬塞 actor/action 会让两边都难用。

**Alternative rejected – event sourcing/事件流作为唯一真相**：当前数据模型是 state-based，不是 event-sourced；为审计单独引入 event sourcing 过度设计。

### 2. ActionID 由 RBAC change 定义，audit 不另起一套

审计事件的 `action_id` 字段取值必须完全来自 `add-project-rbac` 引入的 `ActionID` 枚举。如果某个写动作想审计但尚未在 RBAC 矩阵中声明，**先补到 RBAC 矩阵**，再发事件。

**Why this**：action 命名漂移是最常见的治理问题；两个系统用同一套枚举能让"能做的事"和"做过的事"一一对应，任何新增写动作都会同时获得 gate 和 audit。

### 3. 审计事件发射走 eventbus + 独立 `AuditSink`

- 每个 ActionID 对应的 write handler 在 RBAC allow 后、成功返回前，通过 `eventbus.Publish` 发射一条 `AuditableEvent` 消息。
- `AuditSink` 是一个独立消费者，订阅 `AuditableEvent`，写入 `project_audit_events` 表。
- RBAC middleware 在 **deny** 时也发射一条 `action_id=<denied_action>` + `resource_type=auth` 的 `rbac_denied` 事件——这是 tamper-evidence 的基础（有人试过但没权限）。
- 主业务路径与审计 sink 解耦：sink DB 不可达时事件进入内存退避队列，超过阈值时 metric 报警。

**Why this**：同步写会让审计故障成为业务故障；异步发射 + 独立消费 + 退避队列是经典 pattern，和现有 `eventbus` 已有消费端一致。

**Alternative rejected – 同步写入审计表**：业务延迟受审计表健康状态影响，违反"审计不阻塞业务"。

**Alternative rejected – 完全 fire-and-forget 无补偿**：事件静默丢失不可接受。

### 4. Payload snapshot 有边界

- `payload_snapshot_json` 存事件相关"关键字段"的镜像：比如 member role 变更就存 `{before: {role:'editor'}, after: {role:'admin'}}`；task 更新存改动字段；workflow execute 存启动参数但不存整个 DAG。
- 硬上限每条 **64 KB**（JSONB 存入前校验）；超出则截断并在 payload 里加 `"_truncated": true`。
- 禁止写入敏感字段的明文（token、secret、API key 等），由 `AuditSink` 的 sanitization 函数兜底遮蔽。

**Why this**：审计要"回答是什么变了"够用即可；把业务数据完整搬进审计表会让审计表膨胀、恢复变慢、也是合规隐患。

### 5. 查询 API 默认 `admin+`；UI 放在项目工作区下

- `GET /projects/:pid/audit-events` + `GET /projects/:pid/audit-events/:id` 默认要求 `projectRole ≥ admin`。
- 前端入口挂到 `/project/audit` 或 `/settings` 下的 Audit Log 子页。两种方案的差异只在导航位置；实现上挂到 `/settings` 更贴近"治理"语义，**默认选 `/settings` 子页**。
- 列表默认按 `occurred_at DESC`，分页 cursor 形式。
- 过滤字段：actor、action、resource_type、resource_id、时间段。

**Why this**：审计读是 governance 行为，放在 admin 手里；入口和 project settings 相邻与运营者心智一致。

### 6. 写失败的重试与降级

- `AuditSink` 在写 DB 失败时：内存队列最多 1000 条，指数退避重试；超过 **5 分钟仍无法持久化**，事件落到 `logs/audit_backlog.jsonl`（本地文件）+ 紧急 metric + alert。
- 这些落地文件由运维脚本手动 replay 回 DB；本 change 实现 sink 的基础能力，replay 脚本可留待 follow-up。

**Why this**：让"写入永远不丢"要可用性保证成本极高；给出明确的"退化到磁盘"方案，比悄悄丢事件要可观测得多。

## Risks / Trade-offs

- **[Risk] 写放大带来的 DB 压力**：高频动作（如 task transition）会直接把事件量推到业务表同数量级。→ **Mitigation**：索引只建查询路径上必要的三组；payload 64KB 上限；真需要可将 audit 表迁到独立 schema/实例。
- **[Risk] ActionID 漂移**：新增写动作忘了加 ActionID 导致审计缺失。→ **Mitigation**：RBAC change 已经要求 "write-capable 路由必须有 ActionID"，wire test 兜底；审计事件发射点和 RBAC gate 放在同一 middleware 调用链里一起发射。
- **[Risk] payload sanitization 漏洞**：某字段被当作非敏感但实际是密钥。→ **Mitigation**：列黑名单（`secret`, `token`, `api_key`, `password`, `access_token`, `refresh_token`）+ 反射遍历；对被遮蔽字段打 `"_redacted"` 标记便于排查。
- **[Risk] 永久保留导致表体积失控**：内测期先不做 TTL，但要为后续做好迁移面。→ **Mitigation**：schema 独立表 + 时间索引，未来可直接按 `occurred_at` 分区或归档。
- **[Risk] `rbac_denied` 事件可能被用来故意压垮表（恶意刷 401/403）**：→ **Mitigation**：按 `actor_user_id + action_id + resource_id` 做 60 秒内重复去重（在 sink 层，非表级约束），避免重复审计事件写盘。

## Migration Plan

1. 建表 + 索引 + model/repo/service 骨架，复用 `add-project-rbac` 合入的 ActionID 枚举。
2. 在 `eventbus` 定义 `AuditableEvent` 类型；实现 `AuditSink` 消费端 + 内存退避队列 + 磁盘降级。
3. 在 RBAC middleware 的 allow 与 deny 两条路径上发射事件（deny→`rbac_denied`）。
4. 在每个 write-capable handler 的成功路径上发射"业务层"事件（RBAC 层只覆盖"尝试"；业务层覆盖"实际变更"+ payload snapshot）。这两层事件对应同一条用户操作。
5. 新增查询 handler + 挂到 routes.go。
6. 前端 store + 审计页面，接入 `/settings/audit`。
7. 端到端测试：覆盖典型 action（member role 变更、task dispatch、project settings 更新）各发了至少一条事件；viewer 被拒的动作也产生了 `rbac_denied` 事件。
8. 回滚：审计表不会阻塞业务，发射点如有问题可用 feature flag 跳过发射，但表与索引保留；不要回退到"日志表里混着"的旧状态。

## Open Questions

- 是否给事件加"对象链接字段"（`resource_url`）方便 UI 跳转？当前设计先留 `resource_type + resource_id`，URL 由前端拼接；保持 schema 稳定。
- `rbac_denied` 的 rate-limit 是否要进一步收紧（如从 60s 到 300s）？内测期保守 60s 足够；后续按实际噪声调。
- 是否允许 `editor` 查看"自己触发的审计事件"？心理学上这增加了信任感，但也是条可被滥用的侧信道（通过反复试探可以推断权限矩阵）。本 change 不放行，留 Open 待多人反馈。
- 审计表是否走主业务 DB 还是独立 schema？当前主库足够，等数据量压力显现再迁。
