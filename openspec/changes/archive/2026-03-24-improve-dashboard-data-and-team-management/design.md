## Context

当前前端已经具备 Dashboard Shell、项目看板、Agent 监控和若干 Zustand store，但 Dashboard 首页仍停留在基础摘要卡片和空白活动区，无法表达 PRD 中强调的任务风险、成本压力、审查负载和团队协作状态。与此同时，`docs/PRD.md` 与数据设计已经把 `members` 设定为统一的人机混合成员模型，并给出项目级成员接口，但仓库还没有对应的团队管理页面、状态层和导航入口。

这次变更会横跨 Dashboard 首页、导航、项目级团队视图、前端状态层以及与成员/概览相关的 API 合同，因此需要在实现前明确统一的数据边界和页面分工。

## Goals / Non-Goals

**Goals:**
- 让 Dashboard 首页提供可操作的任务、Agent、成本、审查和团队概览，而不是只展示静态数字。
- 新增项目级团队管理视图，统一展示 human 与 agent 成员，并体现角色、状态、技能和负载信息。
- 复用 PRD 里已有的 `members` 概念，使 Dashboard、团队管理和后续任务分配使用同一成员模型。
- 为空状态、加载状态和 drill-down 导航建立一致体验，减少“看得到入口但没有可用数据”的断层。

**Non-Goals:**
- 不在本次 change 中实现完整的组织架构、权限矩阵或跨项目成员目录。
- 不扩展 Role Plugin 编辑器、IM 人员同步或高级绩效评估功能。
- 不重做现有看板拖拽流程，只补齐它与 Dashboard / Team 之间的数据联动入口。
- 不要求一次性完成所有后端聚合分析；只定义支撑 Dashboard 与 Team 视图所需的最小合同。

## Decisions

### Decision: 将 Dashboard 洞察与 Team 管理拆成“首页概览 + 独立团队页”

Dashboard 首页保留为进入产品后的总览面，但新增团队快照、近期活动、风险/待办区块；详细的成员管理放到独立的 `team` 页面，通过侧边栏和 Dashboard 卡片进入。这样既能提升首页的信息密度，又不会把成员列表、编辑动作和筛选器全部挤进首页。

备选方案是把团队列表直接塞进 Dashboard 首页。这个方案实现更直接，但首页会同时承担概览、筛选、编辑和导航职责，信息层级混乱，也不利于后续继续扩展团队能力，因此不采用。

### Decision: Dashboard 使用后端聚合摘要合同，成员页使用项目级 members 合同

Dashboard 所需数据同时来自 task、agent、review、cost、member 多个域，并包含近期活动与风险信号。为了避免首页同时协调多个 store 的并发请求、空状态和局部失败，本次设计采用一个只读的 dashboard summary 合同来返回首页所需聚合数据。团队页则直接使用项目级 `members` 合同，因为它需要成员级明细、筛选和编辑能力。

备选方案是前端直接拼接 `task-store`、`agent-store`、`notification-store` 和未来的 `member-store`。这样虽然能少一个后端接口，但会让 Dashboard 首页承受过多组合逻辑，并且无法自然覆盖 review/activity 这类跨域信息，因此不采用。

### Decision: 以 `members` 作为 human/agent 统一实体，前端新增 team/member store

团队管理不复用 `agent-store` 作为主数据源，而是新增 `member-store`（或同等命名的 team store）来承接项目成员数据，核心字段与 PRD 的 `members` 模型对齐，显式保留 `type: human | agent`、角色、状态、技能、协作入口等信息。Agent 运行态仍由 `agent-store` 提供，但在团队页以成员视角被消费。

备选方案是把 human 成员临时塞进 `agent-store` 或在组件层拼装“伪成员”对象。这会混淆“团队成员”和“运行中的 agent run”两个不同层次的概念，后续也不利于任务分配与成员管理复用，因此不采用。

### Decision: 保持现有 UI 语言，优先增加信息组织和状态反馈

视觉实现沿用现有 shadcn/ui、Card、Table、Badge、Sidebar 和 Dashboard Shell，不做整体重构。重点放在更清晰的信息分组、零数据状态、局部加载骨架和从摘要到详情的导航路径上，保证这条 change 能平滑叠加到当前页面结构。

备选方案是同步重构首页视觉风格或整体导航。那会扩大范围、模糊用户原始需求，也会提高实现风险，因此不采用。

## Risks / Trade-offs

- [Dashboard 聚合接口容易演变成“大而全”接口] → 先限制为首页只读摘要，不承载成员编辑或任务写操作。
- [成员模型与运行中的 agent 数据可能出现状态不一致] → 团队页将成员静态属性与运行态字段分层展示，并明确缺省/未知状态文案。
- [项目级团队管理依赖后端 members 合同，当前仓库可能尚未完整实现] → 先在 spec 中锁定最小必需字段和场景，实施时允许用 mock/fixture 验证前端流转。
- [新导航入口可能让首页、Projects、Agents 之间职责重新分配] → 通过 Dashboard 卡片 drill-down 和 sidebar 明确“总览 vs 明细”的边界，避免重复信息块。

## Migration Plan

1. 补齐 capability specs，锁定 Dashboard summary 与 project members 的行为合同。
2. 在实现阶段先交付只读数据链路：Dashboard summary、team list、空状态与导航入口。
3. 再补成员管理动作（新增/更新）与成员工作负载展示，确保它们复用同一成员模型。
4. 如果发布后出现数据不完整或接口不稳定，可先隐藏 `team` 导航和 Dashboard 的新区块，保留原有首页卡片作为降级路径；新增接口保持向后兼容，不影响既有项目/agent 页面。

## Open Questions

- Dashboard 首页默认显示“全部项目总览”还是“当前选中项目总览”，是否需要显式项目切换器？
- 团队管理的首个版本是否允许直接创建 agent 类型成员，还是只展示已存在的 agent 成员并支持人工成员维护？
- 成员工作负载是否由后端直接返回聚合字段，还是前端基于 tasks/agent runs 再做二次汇总？
