## Context

AgentForge 已经具备 `project-dashboard` 的基础后端合同与前端入口：`app/(dashboard)/project/dashboard/page.tsx` 能加载某个项目下的 dashboard 列表，`lib/stores/dashboard-store.ts` 已有 dashboard CRUD、widget CRUD 和 widget 数据拉取接口，Go 侧 `SaveWidget` 也支持通过可选 `id` 做 upsert，说明前端并不缺“能不能存”的基础能力。

真正缺的是前端工作区本身。当前页面默认只展示 `dashboards[0]`，没有 dashboard 级切换、重命名或删除体验；`DashboardGrid` 只是固定三列 CSS grid，并没有消费 `position` / `layout` 做真实排布；`AddWidgetDialog` 只暴露一个原始类型单选列表；`WidgetWrapper` 只有刷新按钮，没有配置、删除、局部加载失败或空数据语义。结果是主 spec 里已经写下的“可配置 dashboard”和“可定位 widget”在前端仍然不可操作。

这次设计只补齐这个前端 seam，严格复用现有 dashboard/widget REST 合同，不把范围扩大成新的 dashboard 聚合接口、首页总览重做、或 task workspace 再次重构。

## Goals / Non-Goals

**Goals:**
- 把 `/project/dashboard` 做成一个真实可用的项目看板工作区，而不是永远展示第一个 dashboard 的骨架页。
- 让 dashboard 选择、创建、重命名、删除和空状态都形成完整页面级交互。
- 让 widget 新增、刷新、配置、删除、布局调整和局部失败反馈在同一工作区里闭环，并复用现有 API 合同持久化。
- 为页面级和 widget 级请求增加细粒度 pending/error 状态，保证局部失败不会拖垮整个工作区。
- 用现有测试风格为 page、grid、dialog、store 补齐可回归的前端验证。

**Non-Goals:**
- 不新增 widget 类型，不改 server-side aggregation 算法，也不重开 dashboard 后端 capability。
- 不把 dashboard 首页 `app/(dashboard)/page.tsx` 的 summary/insights 与项目 dashboard 混成一个页面。
- 不引入组织级全局 dashboard、跨项目共享中心或权限矩阵扩展。
- 不要求本次把 widget 配置能力做成无限扩展的 schema-driven 表单系统；只覆盖当前 widget 类型真正需要的最小配置面。

## Decisions

### Decision: 以项目级工作区壳层承载 dashboard 选择和管理，而不是继续默认 `dashboards[0]`

`app/(dashboard)/project/dashboard/page.tsx` 将改成一个工作区壳层，负责：
- 加载当前项目 dashboards
- 维护明确的选中 dashboard
- 在无 dashboard、dashboard 删除后回退、或 query 参数指定 dashboard 不存在时给出可理解的页面状态
- 暴露创建、重命名、删除入口

选中 dashboard 使用路由 query 参数（例如 `?dashboard=<id>`）作为可分享、可刷新恢复的状态来源，页面在缺省情况下回退到当前项目的第一个可访问 dashboard。这样既能保持项目上下文，又不会把“当前看的是哪个 dashboard”藏在一次性的本地 state 里。

备选方案是继续使用 `dashboards[0]` 或仅用组件内部 state 记录选择。前者无法形成真实多 dashboard 工作区；后者刷新后会丢上下文，也不利于后续从别的页面 drill-down 到特定 dashboard，因此不采用。

### Decision: 布局编辑采用“本地草稿 + debounce 持久化 + 明确保存反馈”，并复用现有 dashboard/widget upsert 合同

布局调整会连续产生多个坐标变化，前端不适合在每次拖拽帧都直接写后端。因此工作区维护一个本地 layout 草稿，拖拽或 resize 时先即时更新 UI，再以 debounce 方式提交：
- widget 的位置与尺寸通过 `saveWidget({ id, widgetType, config, position })` 持久化
- dashboard 级 `layout` 作为工作区布局快照一并通过 `updateDashboard` 保持同步

页面同时展示保存中、保存成功、保存失败和重试入口，避免“拖完了但不知道是否落盘”。这样做能保持交互顺滑，又不需要新建专门的布局 API。

备选方案一是每次布局变更都立即调用后端，会产生过高请求噪音且很难提供清晰的反馈。备选方案二是改成显式“保存布局”按钮，虽然实现简单，但会让看板拖拽后的心智模型变差，也容易出现离开页面后改动丢失，因此不采用。

### Decision: widget 操作面收敛到统一卡片容器，并为不同 widget 类型提供最小真实配置

`WidgetWrapper` 扩展为统一的 widget chrome，承载标题、说明、刷新、配置、删除、局部错误、空数据说明和最近刷新状态。`AddWidgetDialog` 不再只显示原始类型字符串，而是展示 widget 元信息、用途说明和默认配置。已有 widget 的配置通过一个统一 dialog/sheet 打开，但每种 widget 只支持当前后端已能消费的最小配置字段，例如：
- throughput / burndown: 时间范围、按天或周聚合
- blocker_count / review_backlog: 项目级或 sprint 级过滤
- budget_consumption / sla_compliance: 时间窗口或阈值展示参数

这样既能让用户真正操作 dashboard，也不会一步把配置系统做成另一个复杂产品。

备选方案是继续保持“添加后直接用空配置渲染”，把配置能力完全延后。这个方案会让 spec 中“configure widget”继续停留在名义上，因此不采用。

### Decision: dashboard-store 增加细粒度工作区状态，而不是复用一个全局 `loading/error`

当前 store 只有全局 `loading` / `error`，足够支撑 dashboard 首页 summary，但不足以表达项目 dashboard 工作区的局部行为。本次会为 dashboard workspace 补齐细粒度状态，例如：
- `dashboardsLoadingByProject` / `dashboardsErrorByProject`
- `activeDashboardIdByProject`
- `widgetRequestStateByKey`
- `dashboardMutationState` 或等价的 CRUD / layout save 状态

这些状态仍留在现有 `dashboard-store` 内，不再新增平行 store。原因是项目 dashboard 与 dashboard summary 已共享项目选择语义，拆出第二个 store 只会增加同步成本。

备选方案是让页面组件自己堆一层本地 pending/error 状态。短期可行，但会把请求真相散在 page、grid、dialog 多处，测试也更脆弱，因此不采用。

## Risks / Trade-offs

- [布局字段双写带来一致性风险] → 保存时由同一条前端提交流程同时派发 `saveWidget` 与 `updateDashboard`，并在失败时保留本地草稿与重试入口，不静默覆盖。
- [widget 配置面容易范围失控] → 只支持当前 8 种 widget 的最小真实配置，不引入 schema-driven 通用配置系统。
- [局部请求状态变多后 store 复杂度提升] → 把状态按 projectId / widgetKey 归档，避免和 dashboard 首页 summary 状态混写。
- [拖拽/resize 依赖选择不当会带来交互抖动] → 优先选轻量且与现有 React 19/Next 16 前端兼容的 grid 布局方案，必要时先以可验证的排序/尺寸编辑落地，再扩成完整拖拽。

## Migration Plan

1. 先补 page/store 测试，锁定 dashboard 选择、空状态、创建后默认选中、删除后回退等工作区行为。
2. 再补 grid/widget 组件测试，覆盖布局草稿、局部加载/失败状态、widget 配置和删除。
3. 在实现中优先复用现有 `updateDashboard` 与 `saveWidget` 合同；如果发现前端无法用现有 contract 表达某个必要操作，只允许补最小兼容，不改 capability 范围。
4. 回归验证时以项目 dashboard 相关页面和 store 测试为主，再补一次受影响的 dashboard summary / team 页面 smoke，确保没有把共享 `dashboard-store` 弄坏。

## Open Questions

- widget 布局第一版是否直接引入拖拽 resize 库，还是先以“移动顺序 + 宽度档位”这种更轻量的布局编辑落地；实现前需要结合现有依赖和测试成本再定。
- 当前 dashboard sharing requirement 只停留在主 spec，前端是否需要在这次 change 里显式展示“共享给项目成员”的只读提示；如果现有后端没有额外共享字段，本次先保留只读说明而不扩权限 UI。
