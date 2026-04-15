## Context

AgentForge 当前已经具备比较完整的角色 authoring 体验：角色工作区可以编辑高级字段、显示 YAML、运行 preview/sandbox，并把结果保存回 `roles/<id>/role.yaml`。但角色在系统里的“消费侧”仍然分散：

- 角色删除目前主要只检查已安装插件/工作流的 role 绑定，无法覆盖 team member 的 `agentProfile.roleId`、排队中的执行请求或其他已持久化引用。
- team/member 侧把 `roleId` 当成普通字符串处理，roster readiness 只区分“空值”而不是“值存在但 role 已失效”。
- spawn / dispatch 对显式传入的无效 `roleId` 已经会失败，但从成员画像或其他上下文派生出的 role 绑定仍然缺少统一的 preflight 诊断。
- 历史 `agent_runs` 等记录已经保存 `role_id`，它们既不能被简单清空，也不应该继续被当成仍可执行的有效绑定。

这意味着当前仓库已经完成了“角色能被定义”，但还没完成“角色作为系统资产被安全引用、删除、修复和审计”。

## Goals / Non-Goals

**Goals:**

- 提供一个权威的角色引用治理 seam，统一枚举当前 role 的下游消费者，并对阻塞与提示性引用做稳定分类。
- 在角色删除前返回结构化、可解释的 blocker 信息，而不是只给出局部或模糊失败。
- 让 team/member 侧能明确区分未绑定角色、有效绑定和已失效绑定，并把修复入口直接放回现有工作流。
- 让 spawn / dispatch 在真正启动前拒绝失效角色绑定，不把问题拖到 runtime 启动或更晚的执行阶段。
- 保留历史运行与审计记录中的 `role_id` 上下文，不把“治理”实现成对历史数据的抹除。

**Non-Goals:**

- 不重做现有 role authoring workspace、preview/sandbox 或 YAML 编辑体验。
- 不在本 change 中支持 role id rename、批量重命名迁移或别名系统。
- 不重写 workflow/plugin 的既有 stale role 校验规则，只复用它们进入更统一的治理视图。
- 不清洗、重写或迁移历史 `agent_runs` / audit 记录中的既有 `role_id`。

## Decisions

### 1. 引入统一的 role reference aggregation seam，而不是让每个入口自己拼装引用判断

**Decision:** 在 Go 侧新增统一的角色引用聚合服务，负责枚举一个 role 当前被哪些系统对象引用，并产出稳定的结构化结果，供角色删除、角色详情/删除确认、team/member 诊断和 spawn preflight 共享。

聚合结果至少包含：

- consumer type，例如 `member-binding`、`plugin-binding`、`workflow-binding`、`queued-execution`、`historical-run`
- consumer identity，例如 member id/name、plugin id、queue entry id、run id
- lifecycle state，例如 `active`、`queued`、`historical`
- severity / blocking flag
- remediation hint

**Why:** 角色引用已经跨越多个模块；如果继续在 `role_handler`、`member_handler`、`agent_service`、前端 store 里各自判断，规则会继续漂移。统一 seam 可以保证删除 guard、stale 诊断和运行前阻塞共享同一套真相。

**Alternatives considered:**

- 在每个 handler/service 里分别查询各自关心的引用。放弃，因为规则会重复且难以保持 blocking 分类一致。
- 只在前端拼装多份 API 结果。放弃，因为删除与 spawn 拦截必须由后端权威判断，前端聚合无法替代服务端 guardrail。

### 2. 将 role 引用分成 blocking 与 advisory 两类，而不是“一律阻止删除”或“一律只提示”

**Decision:** 当前 change 采用显式分层：

- **Blocking references**
  - 已安装插件或工作流对该 role 的当前绑定
  - project team roster 中 agent member 的已绑定 `roleId`
  - 尚未物化 execution profile 的 queued / pending execution requests
- **Advisory references**
  - 已完成、失败或取消的历史 agent runs / 审计记录
  - 仅用于查询或历史上下文的其他 role_id 记录

对 blocking 引用，删除必须拒绝；对 advisory 引用，删除允许继续，但删除确认与引用视图里必须显式展示“历史上下文仍会保留该 role_id”。

**Why:** 如果把所有历史记录都当 blocker，角色几乎无法清理；如果忽略成员绑定和等待执行，则删除后只会制造更晚出现的 runtime 故障。分层后既能保护后续执行，又不会让历史审计成为永久锁。

**Alternatives considered:**

- 所有引用都阻塞删除。放弃，因为历史运行记录会把角色永久钉死。
- 只把插件/工作流当 blocker。放弃，因为 member 绑定和 queued execution 同样会造成后续失败，只是失败更晚、更难定位。

### 3. 角色删除走“按需查看引用 + DELETE 返回结构化冲突”的组合，而不是持续膨胀 roles list payload

**Decision:** 角色治理信息通过按需读取的 detail seam 暴露，而不是把完整 consumer 列表长期塞进 `GET /api/v1/roles` 的 catalog 响应。实现上采用：

- 新增角色引用详情读取接口，例如 `GET /api/v1/roles/:id/references`
- `DELETE /api/v1/roles/:id` 在命中 blocking 引用时返回结构化 409 payload，复用同一份聚合结果
- 角色页在删除确认、详情或审查上下文需要时按需请求该引用详情

**Why:** roles list 是高频 catalog 请求，而引用治理是相对低频、面向操作决策的详细信息。按需查询可以控制载荷和查询成本，同时让 UI 与 DELETE guard 共享同一个结构化响应模型。

**Alternatives considered:**

- 永远把完整引用列表塞入 `GET /api/v1/roles`。放弃，因为 catalog 请求会变重，而且大多数时候用户只需要角色概览。
- DELETE 只返回字符串错误。放弃，因为这会延续今天“失败了但不知道还引用在哪”的问题。

### 4. member 绑定校验采用“写时权威校验 + 读时 stale 显示”双轨策略

**Decision:** team/member 侧在 create/update agent member 时，后端对 `agentProfile.roleId` 做权威 role existence 校验；但对已经存在的历史 stale 绑定，不做静默修正，而是在读取和汇总时显式标记为 `stale-role-binding` 并把修复入口指向现有编辑流。

对应行为：

- 新建 agent member 时提交不存在的 `roleId` -> 请求被拒绝并返回字段级错误
- 编辑 agent member 时保留或提交不存在的 `roleId` -> 保存失败并高亮 role 字段
- roster / summary / attention workflow 读取到当前 registry 中不存在的已绑定 `roleId` -> 显示 “Stale role binding” 类状态，而不是继续显示 Ready 或只显示 “Needs role binding”

**Why:** 只做前端校验并不可靠，但只在写时校验又不足以解释现有历史 drift。双轨策略既能阻止继续写入坏引用，也能让已有坏数据以可修复状态显性化。

**Alternatives considered:**

- 仅前端根据当前 role list 做校验。放弃，因为 API 仍会接受坏数据。
- 删除角色时自动把所有 member 绑定清空。放弃，因为这是隐式破坏性修改，不符合当前仓库的显式治理风格。

### 5. spawn / dispatch preflight 解析“有效角色绑定”，并在 admission 前失败

**Decision:** spawn 与 dispatch 在 admission 前统一解析 effective role binding：

- 如果请求显式带 `roleId`，先校验该 role 是否存在
- 如果请求未显式带 `roleId` 但目标 member 的 agent profile 已绑定 role，则用该绑定做同样校验
- 一旦发现 stale role binding，返回结构化 blocked / validation 结果，不创建 queue entry，也不启动 runtime

**Why:** 当前显式 roleId 已有一定保护，但从成员画像等路径派生出的 role 绑定仍可能晚失败。把所有入口统一收口到 preflight，可以保证“无效角色不入队、不启动、不留下模糊状态”。

**Alternatives considered:**

- 继续依赖 runtime 启动前的深层报错。放弃，因为 queued flow 和 UI 入口会把这类错误包装得太晚、太模糊。
- 只校验显式 roleId，不校验派生绑定。放弃，因为对用户来说两者都属于“当前要执行的有效角色”，不应该因为来源不同而得到不同保护。

## Risks / Trade-offs

- **更多历史 drift 会被显性暴露** → 通过明确的 stale 状态、字段高亮和修复入口降低操作成本，而不是继续静默容忍。
- **删除角色会比现在更容易被阻止** → 删除确认和 409 payload 必须提供分组后的 blocker 清单与 remediation hint，避免“更严格但更难懂”。
- **引用聚合查询可能增加角色删除/详情成本** → 采用按需查询、分层 consumer summary 和 scoped repository lookups，避免把重查询塞到 catalog 列表。
- **member 更新的 role 校验可能让部分已有坏数据不能继续原样保存** → 保持 quick lifecycle action 不依赖 role 编辑，同时在 edit flow 中直接引导修复 stale binding。

## Migration Plan

1. 在 Go 侧新增 role reference aggregation seam 和统一 DTO，但先只接入查询与测试，不改变现有删除行为。
2. 让 `DELETE /api/v1/roles/:id` 复用该 seam 返回结构化 blocker 信息，并在角色页接入删除前引用查看。
3. 为 member create/update 增加 role existence 校验，同时在 roster summary / attention workflow 中接入 stale role 诊断。
4. 把 spawn / dispatch preflight 切到统一的 effective role 校验路径，保证 stale 绑定不会入队或启动。
5. 通过 targeted tests 验证 blocking/advisory 分类、member stale 诊断和 spawn blocked outcome；如需回退，可先停用新查询/UI，再回退写时校验与 DELETE guard 逻辑。

## Open Questions

- 当前 paused / resumable agent runs 是否应继续视为 advisory，而不是 blocking？按现有 runtime snapshot 语义推断它们通常已经持有物化后的 `role_config`，但实现阶段需要再核对 resume 路径是否会重新依赖 role store。
