## Why

`project-dashboard` 这条能力的后端合同、widget 聚合接口和基础前端骨架已经落地，但当前 `app/(dashboard)/project/dashboard` 仍停留在“能创建一个默认 dashboard、能渲染第一块网格”的半成品阶段。它还没有兑现主 spec 里要求的可配置看板工作区体验：前端默认只显示第一个 dashboard、不支持真实的布局持久化交互、缺少 widget 配置和移除入口，也没有把加载/空态/失败态组织成一个可运营的看板页面。

现在单独补齐这个前端 seam，能把仓库里已经存在的 dashboard contract 转成真正可用的项目看板，而不是再开一个新的 dashboard 体系或回头扩大成 task workspace、首页总览或后端聚合重构。

## What Changes

- 重构 `app/(dashboard)/project/dashboard` 为真实的项目看板工作区，补齐 dashboard 列表与切换、创建后的默认选中、基础重命名与删除入口，以及与当前 project scope 对齐的空状态和错误状态。
- 将 `components/dashboard/dashboard-grid.tsx` 从静态卡片网格升级为消费 `layout` / `position` 的前端工作区，支持 widget 排布调整、布局保存反馈，以及 dashboard 为空或 widget 尚未配置时的明确状态表达。
- 补齐 widget 生命周期操作面，包括新增时的可读选择信息、已存在 widget 的刷新/配置/移除入口、数据加载和局部失败反馈，以及对尚未支持的配置或数据条件给出解释而不是静默降级。
- 收敛 `lib/stores/dashboard-store.ts` 的前端状态语义，让 dashboard CRUD、widget CRUD、widget 数据刷新和布局提交具备可消费的 pending/error 状态，便于项目看板页面实现完整交互闭环。

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `project-dashboard`: 细化项目看板前端工作区要求，明确 dashboard 选择与管理、widget 布局持久化交互、widget 操作入口，以及页面级和局部级状态反馈都必须在现有合同上形成完整体验。

## Impact

- Affected frontend routes: `app/(dashboard)/project/dashboard/page.tsx`.
- Affected frontend UI: `components/dashboard/dashboard-grid.tsx`, `components/dashboard/add-widget-dialog.tsx`, `components/dashboard/widget-wrapper.tsx`, `components/dashboard/widgets.tsx`, and likely new dashboard management/configuration helper components.
- Affected frontend state: `lib/stores/dashboard-store.ts` and related tests for dashboard selection, widget actions, and widget data loading.
- Affected specs: `openspec/specs/project-dashboard/spec.md` via a delta spec for the front-end workspace behavior.
- Existing backend/API contracts are reused: `GET/POST/PUT/DELETE /api/v1/projects/:pid/dashboards`, widget CRUD endpoints, and `GET /api/v1/projects/:pid/dashboard/widgets/:type`.
