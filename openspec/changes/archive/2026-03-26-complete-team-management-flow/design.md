## Context

AgentForge 当前已经有两组与团队管理相关的前端表面：

- `/team` 负责项目级成员 roster、成员编辑和 workload 概览。
- `/teams` / `/teams/detail` 负责 agent team run 列表与详情。

但这两组表面还没有形成真实可用的管理流。当前仓库里至少存在四类断点：

1. `/team` 的读路径仍依赖 `dashboard-store.fetchSummary()`，导致成员页被 Dashboard 聚合加载状态绑住，而不是以项目级成员契约为中心。
2. 成员 drill-down 只生成 `member` / `focus` 查询参数，但 `/project`、`/agents` 等目标页并没有消费这些参数，导致导航不能保留上下文。
3. `/teams` 首屏直接调用 `fetchTeams()`，但后端 `GET /api/v1/teams` 明确要求 `projectId`；team detail 也会在异步拉取完成前直接渲染 “Team not found”。
4. team startup 与 team run 展示的 `strategy` 标识发生漂移：前端和旧 migration 仍使用 `planner_coder_reviewer`，而后端服务实际以 `plan-code-review` 作为 canonical 标识，并支持其他 strategy。

这次设计是一次 continuation change，不新增新的组织能力，而是把现有成员管理和 team run 管理补成一个项目作用域内闭环可用的工作流。

## Goals / Non-Goals

**Goals:**
- 让 `/team` 成为真正项目作用域的成员管理页，成员列表、工作负载、最近活动和写操作刷新路径保持一致。
- 让成员 drill-down 跳转在 `/project`、`/agents`、`/teams` 等目标页保留并消费相同的项目/成员上下文。
- 让 `/teams` 与 `/teams/detail` 在真实 API 合同下稳定工作，补齐加载、错误、重试和项目切换体验。
- 对齐前端 team startup / team run 展示与后端 canonical `strategy`、`runtime`、`provider`、`model` 合同，同时兼容已落库的 legacy strategy 值。
- 通过 focused tests 锁定团队管理关键断点，而不是只覆盖表单 happy path。

**Non-Goals:**
- 不在本次 change 中新增组织架构、权限矩阵、邀请审批或跨项目成员目录。
- 不在本次 change 中设计新的 team strategy 算法，只对齐现有后端已支持的策略标识和展示。
- 不引入新的 dashboard 聚合接口，也不把 `/team` 重做成一个新的总览页面。
- 不扩展 agent run 输出存储模型；如果 detail 页历史输出恢复能力不足，只在当前 contract 下保证状态与导航正确。

## Decisions

### Decision: 用显式项目/成员查询参数定义团队管理跨页上下文

团队管理相关页统一使用可消费的查询参数来携带上下文，而不是依赖“某页生成了链接，另一页碰巧知道怎么解释”。本次 change 中，`project` / `id` / `member` / `focus` 的消费规则需要在目标页显式实现，并在 source 页只生成目标页真正支持的参数集合。

这样做的原因是当前断点不是“缺少链接”，而是“链接生成的上下文没有被消费”。如果继续依赖隐式全局状态，成员从 `/team` 跳到 `/project` 或 `/agents` 时仍然会丢失筛选条件，用户必须重新搜索成员上下文。

备选方案是只保留 source 页链接，不要求目标页消费对应参数。这个方案实现更少，但无法满足现有 spec 对“preserve enough context”的要求，因此不采用。

### Decision: `/team` 以项目级成员 contract 为读写中心，Dashboard 聚合只保留摘要角色

`/team` 页不再以 `dashboard-store` 作为事实上的读源。成员静态属性与 CRUD 以项目级 members contract 为中心，工作负载与最近活动则从项目作用域下的 tasks / agents / activity 数据做 enrichment，继续复用现有 `summarizeMemberRoster()` 逻辑或等价 helper。

这样做的原因是 archived design 已经明确区分“Dashboard 聚合摘要”和“Team 页成员明细”；当前实现把 team 页重新绑回 dashboard 聚合读链路，导致 team 页既拿不到独立的加载/错误边界，也放大了 Dashboard 聚合变化对成员页的耦合。

备选方案是继续使用 `dashboard-store.fetchSummary()` 作为 `/team` 的唯一读路径。这个方案短期最省事，但会继续把成员页的状态和 dashboard 首页耦合在一起，无法干净补齐项目级成员页行为，因此不采用。

### Decision: 保持 `GET /api/v1/teams?projectId=` 的项目作用域 contract，由前端负责显式传参与切换

`/teams` 集合页继续遵循后端已存在的项目作用域约束，前端必须总是传入明确的 `projectId`。页面需要显式持有当前项目选择、加载、错误和重试状态，而不是在缺少项目参数时向后端发出无效请求。

这样做的原因是后端 handler 已经把 `projectId` 作为 list contract 的一部分，强行放宽为“无参数返回全部 team”会扩大 API 语义并引入新的权限/性能问题；当前真正的问题是前端没有尊重已有 contract。

备选方案是修改后端让 `/api/v1/teams` 在无 `projectId` 时返回所有项目 team。这个方案会弱化“project-scoped management flow”的边界，也会让列表页的数据量和权限语义变得不稳定，因此不采用。

### Decision: 以后端 strategy 标识为 canonical contract，前端做 label 映射并兼容 legacy alias

前后端统一以后端当前支持的 strategy 标识作为 API contract，例如 `plan-code-review`、`wave-based`、`pipeline`、`swarm`。前端展示层负责将 canonical ID 映射为可读标签。对于历史数据里的 `planner_coder_reviewer`，本次 change 先做读时 alias 归一化，使现有 team 仍能正确显示和继续操作；是否做数据库回填放到迁移步骤中作为可选补充。

这样做的原因是当前 drift 已经发生在 migration、前端 state 和后端 service 之间。如果先做数据库回填再修前端，期间用户依然会遇到错误展示和不一致请求。先统一 API 与 UI 层语义，能最快把真实用户流修正到可用状态。

备选方案是保留 `planner_coder_reviewer` 作为前端专用值，并让后端继续做隐式 fallback。这个方案会持续制造“UI 能显示但 contract 不可推理”的问题，因此不采用。

### Decision: Team detail 采用显式 loading / loaded-not-found 分层，而不是请求前直接渲染 not-found

`/teams/detail` 在拉取 team summary 前必须区分“尚未加载”和“已加载但不存在”两种状态。只有当请求明确返回 not-found 后，页面才显示缺失文案；在请求进行中必须保持 loading 状态。

这样做的原因是当前 detail 页在 `fetchTeam()` 完成前直接判断 `team` 是否存在，导致首次直达时很容易闪出错误反馈。这个问题不是视觉瑕疵，而是把一个可恢复的异步过程错误表达成 terminal state。

备选方案是继续用 store 中是否已有 team 作为唯一判断条件。这个方案在慢请求和直达详情页场景下都会误导用户，因此不采用。

## Risks / Trade-offs

- [跨页查询参数 contract 增加了多个页面的耦合面] → 通过集中定义允许的参数集合和对应消费逻辑来降低分散实现风险，并用路由级测试锁定行为。
- [`/team` 读路径从 Dashboard 聚合拆开后，短期内会出现部分重复请求] → 接受 scoped duplication，优先保证项目级成员页的正确性；缓存或请求合并留到后续优化。
- [legacy `planner_coder_reviewer` 兼容可能掩盖脏数据] → 仅对已知 alias 做显式映射，对未知 strategy 保留错误/unknown 提示并记录测试样例。
- [保持 `projectId` 必填会要求 `/teams` 新增项目选择与空状态管理] → 这是成本，但它直接换来与现有后端 contract 一致的稳定行为，优于继续发无效请求。

## Migration Plan

1. 为 `team-management` capability 增加 delta spec，锁定项目作用域、成员 drill-down、team list/detail、strategy canonicalization 行为。
2. 在前端先实现 query-param consumption、`/team` 读链路收敛、`/teams` 项目作用域和 detail loading/error 状态。
3. 在 team store / startup dialog / team card/detail 中引入 canonical strategy label mapping，并对 legacy alias 做读时归一化。
4. 补齐 team-related tests，覆盖目标页参数消费、`/teams` 请求参数、detail loading/not-found 分层、以及 startup strategy/runtime 合同。
5. 如验证后仍需要清理存量数据，再评估一次性回填历史 `planner_coder_reviewer` 记录到 canonical 值；该步骤可独立发布，不阻塞当前 change。

回滚策略：如果上线后发现跨页 filter 行为有回归，可以先回退目标页的参数消费与新入口链接，同时保留 strategy alias 兼容和 team list projectId 约束，不影响已有后端数据。

## Open Questions

- `/teams` 首屏在没有显式 `project` 参数时，是优先使用当前选中项目，还是必须要求用户先选项目再加载列表？本设计默认前者，但实现前需要统一交互细节。
- 成员 drill-down 到 `/project` 后，应当是“预填筛选器并保留完整列表”，还是“直接切成仅该成员视图”？本 change 只要求上下文可消费，具体默认视图需要在实现时定稿。
