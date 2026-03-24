## Context

AgentForge 当前已经具备插件、角色、工作流三条后端与基础前端能力，但前端页面仍然偏“脚手架态”：

- `app/(dashboard)/plugins/page.tsx` 只区分已安装插件和内建插件，未接入 `/api/v1/plugins/marketplace`，也没有把权限声明、runtime host、health/restart、ABI/source 等运行信息完整暴露给操作员。
- `app/(dashboard)/roles/page.tsx` 和 `components/roles/role-form-dialog.tsx` 只覆盖少量字段，无法编辑 `extends`、knowledge、allowed tools、permission mode、path 约束等 PRD/Role YAML 的核心内容。
- `app/(dashboard)/workflow/page.tsx` 和 `components/workflow/workflow-config-panel.tsx` 仅支持表格式状态与 trigger 编辑，没有“工作流可视化（只读，展示执行状态）”，也没有消费现有 `workflow.trigger_fired` 实时事件。

同时，产品文档已经把这些面板定义成正式功能，而非未来愿景：

- `docs/PRD.md` 与 `docs/part/PLUGIN_SYSTEM_DESIGN.md` 都把“角色配置面板 / 工作流可视化 / 插件市场 UI”列为 Layer 1 前端职责。
- 当前仓库已经具备可复用的 API 契约：`/api/v1/plugins`、`/api/v1/plugins/discover`、`/api/v1/plugins/marketplace`、`/api/v1/roles`、`/api/v1/projects/:projectId/workflow`。
- 当前仓库已经具备可复用的事件契约：`workflow.trigger_fired` WebSocket 事件类型已在 Go 侧声明。

这个 change 的目标不是扩展插件生态后端，而是把已有契约和 MVP 能力补齐成一套一致、真实、可操作的前端管理面板。

## Goals / Non-Goals

**Goals:**

- 让插件页成为可运维的插件控制台，而不是只显示卡片列表。
- 让角色页覆盖当前 Role YAML/Role API 的主要结构化字段，支持模板化起步和继承关系配置。
- 让工作流页同时具备配置入口和只读可视化观察能力，并消费现有 workflow 触发事件。
- 优先复用现有 API、现有 store 和现有实时通道，避免为了 UI 完整度再开新的长期架构分支。

**Non-Goals:**

- 不实现远程 Plugin Registry 安装协议、签名审核、评分评论或完整市场分发。
- 不实现完整 DAG/节点式工作流编辑器，也不把 workflow 页面升级成通用编排设计器。
- 不引入新的数据库表或新的插件生命周期状态。
- 不改动角色执行投影、插件运行时宿主映射或现有 Go/TS Bridge 架构。

## Decisions

### 1. 插件页采用“多分区控制台 + 详情面板”，不拆新路由

选择在现有 `Plugins` 页面内增强三类分区：

- 已安装插件
- 平台内建插件
- 市场插件目录

并加入统一筛选栏（kind / lifecycle / host / source / search）与插件详情面板，而不是拆成多级子页面。

原因：

- 当前操作流本质上是同一对象集合的不同来源和不同状态，不需要路由切换才能理解。
- 插件操作具有强上下文性，详情面板可让用户在不离开当前列表的情况下查看权限、运行态和动作限制。
- 现有 `PluginRecord` 已经携带 UI 所需的大多数字段，前端增强即可落地。

备选方案：

- 拆分成 `installed / marketplace / runtime` 三个独立页面。缺点是状态对比成本高，且会放大当前功能尚不完整的空页面感。

### 2. 插件市场先做“目录面板”，安装动作按真实可执行性降级

选择接入 `/api/v1/plugins/marketplace` 作为市场目录数据源，但安装动作以真实字段能力为准：

- 对已有本地路径或内建路径的条目，保留可执行安装。
- 对仅能浏览、尚无可执行安装源的市场条目，显示禁用安装按钮或“coming soon / browse-only”说明。

原因：

- 后端已经提供市场目录接口，但当前 `MarketplacePluginDTO` 并不保证有可直接安装的来源。
- 直接在前端伪造远程安装能力会制造错误预期。
- 目录先上线，既满足 PRD 中“基础插件市场 UI”，又不越过真实后端边界。

备选方案：

- 等远程 registry 完整实现后再做市场 UI。缺点是当前前端长期缺位，无法暴露现有市场目录能力。

### 3. 角色页采用结构化分区表单，不回退到 JSON-only 编辑

选择把角色编辑器拆成多个结构化 section：

- 基础信息
- 身份与提示词
- Capabilities
- Knowledge
- Security
- Inheritance / advanced

必要时保留少量高级字段入口，但不以整块 JSON 文本框作为主要编辑方式。

原因：

- 当前 RoleManifest 字段结构已经稳定，适合映射为具名表单。
- 结构化编辑更符合“角色配置面板”的产品预期，也更利于校验和摘要展示。
- 仅靠 JSON 编辑会让“模板化起步”和“继承关系”价值被掩盖。

备选方案：

- 只扩大现有表单并把剩余字段塞进高级 JSON。缺点是用户仍需理解完整 schema，且容易写出无效结构。

### 4. 角色模板复用现有角色源，而不是额外维护硬编码模板副本

选择以当前 `roles/` 目录经 `/api/v1/roles` 暴露出来的角色清单作为模板/复制来源，在 UI 中提供：

- 从现有角色复制
- 从现有角色继承（设置 `extends`）

而不是在前端单独维护第二份模板常量。

原因：

- 仓库当前已经有一批真实角色 YAML，且这些角色就是系统最终执行来源。
- 避免前端模板与后端角色定义漂移。
- 允许用户既把平台角色当模板，也能从团队自定义角色继续派生。

备选方案：

- 前端内置独立模板列表。缺点是会制造第二份 source of truth。

### 5. 工作流页面升级为“编辑器 + 只读可视化 + 最近活动”

选择保留现有 transition/trigger 编辑能力，并新增两块只读视图：

- 状态流转图
- 最近触发活动列表

最近触发活动优先消费 `workflow.trigger_fired` WebSocket 事件，并在断连时显示 degraded 提示。

原因：

- PRD要求的是“工作流可视化（只读，展示执行状态）”，不是完整可编辑 DAG。
- 当前后端已经有工作流配置接口和触发事件类型，前端缺的是消费与展示。
- 可视化和活动流能让用户理解配置的运行结果，而不只是编辑静态规则。

备选方案：

- 直接引入节点编排器或画布编辑器。缺点是明显超出当前后端能力，也会把 change 扩成新的产品线。

### 6. 复用现有 Zustand store，并只补必要的 UI 派生状态

选择在现有 `plugin-store` / `role-store` / `workflow-store` 基础上扩展：

- `plugin-store`：增加 marketplace 数据、筛选参数、选中详情对象或本地派生 helpers
- `role-store`：保留 CRUD 数据源，额外在页面层维护编辑草稿与模板选择
- `workflow-store` / `ws-store`：新增最近 workflow 触发活动的前端状态，并消费 `workflow.trigger_fired`

原因：

- 这些页面目前都已是 client page + Zustand 模式，延续现有架构最稳妥。
- 当前 change 重点是产品完整性，不是状态管理范式重构。

备选方案：

- 引入新的 query/cache 层或全局 UI store。缺点是改动面过大，且与本次面板补全目标不成正比。

## Risks / Trade-offs

- [市场接口目前是目录级 MVP，安装源未必可执行] → UI 明确区分“可安装”和“仅浏览”，避免假动作。
- [角色结构化表单字段较多，首屏可能显得重] → 使用分组区块、摘要卡片和模板起步流，降低首次编辑门槛。
- [workflow 实时事件当前前端未消费，事件 payload 细节可能不够稳定] → 先做容错解析和最近活动上限缓存，解析失败只影响活动流，不阻塞配置编辑。
- [插件详情字段多，卡片若承载过多会破坏信息密度] → 关键信息留在列表卡片，长尾运行态与权限细节下沉到详情面板。
- [角色模板复用 `/api/v1/roles` 会混入团队自定义角色] → UI 将“当前角色复制”和“继承自现有角色”视为能力而非缺陷；后续若后端区分 builtin，可再增加来源标识。

## Migration Plan

1. 先扩展 stores 和页面数据装载能力，确保插件市场、workflow 活动流和角色模板源可被读取。
2. 分别增强 `plugins / roles / workflow` 页面和对应组件，优先保持现有 CRUD/动作链路不回归。
3. 为新增面板状态和动作限制补齐前端单元测试，覆盖筛选、禁用动作、模板预填、workflow 实时活动、降级提示等关键行为。
4. 通过 `pnpm lint`、相关 Jest 测试和必要的 typecheck 进行验证。

回滚策略：

- 该 change 以前端增量和 store 扩展为主，无数据迁移。
- 如需回退，可直接回退本次前端改动，不需要数据库恢复步骤。

## Open Questions

- 当前 `/api/v1/plugins/marketplace` 是否会在短期内补充真正的 `installUrl` 或其他远程安装字段？在没有之前，前端默认按目录浏览处理。
- `workflow.trigger_fired` 的最终 payload 结构是否需要在前端做更细粒度展示（如 task title、trigger action label）？本次先按兼容解析和最小展示设计。
- 角色来源是否会在后端增加 builtin/project 标记？若没有，本次前端不阻塞，继续以“复制/继承现有角色”方式交付。
