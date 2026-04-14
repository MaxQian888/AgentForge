## Context

`app/(dashboard)/memory/page.tsx` 已经提供了项目选择门槛，并在选中项目后挂载 `components/memory/memory-panel.tsx`。但当前 panel 仍是最小实现：搜索框、角色下拉、分类 tabs、单条删除和一组截断卡片；它没有消费后端刚刚补齐的 detail / stats / export / bulk-delete / cleanup 能力，也没有把 memory explorer 做成一个可持续运维的工作区。

另一方面，`complete-memory-explorer-backend-surface` 已经把 `/api/v1/projects/:pid/memory*` 扩成 explorer-ready contract：`src-go/internal/handler/memory_handler.go` 和 `src-go/internal/service/memory_api_service.go` 现在已经支持 list、detail、stats、JSON export、bulk delete 与 age-based cleanup。当前前端如果继续停留在最小列表，就会让真实后端能力长期闲置，也会让 `enhance-frontend-panel` 中的 memory explorer 方向一直卡在未接线状态。

约束条件：
- 优先复用当前 shadcn/ui、Zustand、`PageHeader` / `EmptyState` / `ErrorBanner` 等既有前端 seam，不引入新的数据网格或状态库。
- 以后端现有 `/api/v1/projects/:pid/memory*` contract 为准，不为这次 workspace 重新发明一套 memory truth。
- 范围聚焦在 operator-facing workspace；不顺手扩成 memory tagging、memory editing、CSV export 或新的 long-term learning 架构。

## Goals / Non-Goals

**Goals:**
- 把 `/memory` 升级为一个真正可操作的 memory explorer workspace，而不是继续停留在单列摘要列表。
- 让列表、筛选、详情、统计、导出和清理都直接消费现有 memory explorer API，并共享同一组前端 query state。
- 在桌面宽度下提供 master-detail 体验，在窄屏下保持可访问的 detail / confirm 流，不牺牲现有项目选择门槛。
- 为 destructive actions（单删、批量删、清理）提供清晰确认、结果反馈和列表/统计刷新语义。
- 用 focused store / component tests 锁定 explorer workspace 的状态流与关键交互。

**Non-Goals:**
- 不新增 memory tagging、memory editing、semantic vector search 或 procedural learning automation。
- 不改造 Go 后端为 cursor pagination 或 CSV export；本次基于现有 result window + JSON export contract 工作。
- 不把 memory explorer 扩展到跨项目全局工作区；当前仍以已选项目为边界。
- 不重做 `app/(dashboard)/memory/page.tsx` 的上层导航、breadcrumb 或 dashboard project selection 机制。

## Decisions

### 1. 采用“workspace shell + detail surface + action dialogs”的分层，而不是继续在单个卡片列表里堆功能

**Decision:** 保留 `app/(dashboard)/memory/page.tsx` 只负责项目级 gate，把真正的 explorer 逻辑下沉到 `components/memory/*`：拆出 summary/filter toolbar、result list、detail surface、bulk-action bar 和确认/结果 dialogs。

**Rationale:**
- 当前 `MemoryPanel` 已经承担搜索、筛选、展示和删除，继续往里堆 detail/export/cleanup 会快速失焦。
- workspace shell 分层更适合后续补更多 memory 交互，也方便针对 list/detail/action 做 focused tests。
- 页面级 gate（未选项目时显示 empty state）已经存在，没有必要把这层判断和 explorer 细节耦在一起。

**Alternatives considered:**
- 继续扩写单个 `MemoryPanel`：实现快，但会把查询、选择、详情、导出和危险操作混成一个大组件。
- 新开 `/memory/[id]` 子路由：能复用 URL，但会把目前的工作区体验拆散，也会放大 App Router 复杂度。

### 2. 让 `lib/stores/memory-store.ts` 成为 explorer query truth，而不是把请求参数散落在多个组件 state 中

**Decision:** 扩展 memory store，使其同时持有 query/filter state、列表结果、详情、统计、selection、action loading 与导出/清理结果；组件只表达布局和交互，不重复拼接 API 查询参数。

**Rationale:**
- 当前 store 已经负责 list query 和 delete；沿着这一 seam 扩展 detail/stats/export/cleanup 最自然。
- 列表、统计与批量操作都依赖同一组过滤条件，把 query truth 保持在 store 里可以避免多个组件各自维护 `query/scope/date` 的漂移。
- 一旦 destructive action 成功，store 可以集中触发列表与统计刷新，减少页面级手动同步。

**Alternatives considered:**
- 让每个子组件各自请求数据：会造成 query drift 和重复 loading/error 逻辑。
- 把所有状态塞回 page 组件：对当前单页面可行，但不利于复用测试和后续 workspace 扩展。

### 3. 使用“列表即摘要、详情懒加载”的 API 消费模式，而不是首次加载拉全量 detail

**Decision:** 首屏使用 list + stats 拉起摘要视图；当用户选择一条记忆时，再通过 detail endpoint 拉完整内容、structured metadata 与 related context。导出、bulk delete、cleanup 也都以当前 query state 为输入单独触发。

**Rationale:**
- 当前 list DTO 已足够支撑筛选后的结果面板，而 detail endpoint 更适合承载长文本和元数据。
- 详情懒加载能避免初始加载被大体量 content/metadata 拖慢，并让后续 detail UI 更可控。
- 各 action endpoint 已经是后端真实 contract，前端只需围绕当前 query state 组织触发与刷新。

**Alternatives considered:**
- 首次请求直接拉全部 detail：实现简单，但会在长内容或 metadata 较大时放大首屏负担。
- 只靠列表数据做详情：会迫使前端继续解析 raw metadata 字符串，违背刚完成的 backend surface 初衷。

### 4. 桌面端采用 master-detail，窄屏端采用 sheet/dialog detail，不强推复杂表格或三栏布局

**Decision:** 在宽屏上使用左侧结果列表 + 右侧详情面板/卡片的双栏布局；在较窄视口上，详情通过 sheet/dialog 打开。批量操作和清理/导出则通过 toolbar + confirm dialog 呈现。

**Rationale:**
- memory explorer 的核心动作是“筛选后浏览摘要，再进入单条详情”，天然适合 master-detail。
- 当前项目已有 shadcn dialog/sheet 模式，可以低风险复用。
- 三栏布局或重型 data grid 对当前 `memory` 页面来说过度，且会推高移动端复杂度。

**Alternatives considered:**
- 一律用表格：统计与摘要会被压平，不利于展示 content preview 和 scope/category badge。
- 一律使用新页面跳转详情：会降低 operator 的对比/回看效率。

### 5. 这次把“更完善”定义为真实运营闭环，而不是预支未来标签系统

**Decision:** 当前 workspace 的完成标准聚焦在五条闭环：filtered list、summary stats、detail inspection、filtered export、safe cleanup/delete。tagging、editing、CSV、saved views 暂不纳入本 change。

**Rationale:**
- PRD 和现有 main specs 更强调 project memory 的检索、诊断、导出和清理；这些已经有真实后端基础，收益最高。
- `memory-system` 当前也明确没有承诺 tagging/editing 等长期能力，本次不应为了“看起来完整”而过度扩 scope。
- 先把 operator 工作区做真，比继续堆未来概念更符合当前 repo truth。

**Alternatives considered:**
- 一次做 tags/edit/export/edit all-in-one：范围失控，并会把刚稳定的 backend surface 再次拉回 schema 讨论。
- 只补 detail 不补管理流：仍无法称为完整 workspace，导出/清理能力会继续闲置。

## Risks / Trade-offs

- **[workspace 状态变多，store 复杂度上升]** → 通过拆分 store selectors、把 query/detail/action 分成清晰字段，并为关键状态流补测试来控制复杂度。
- **[列表、统计、详情存在刷新不同步风险]** → 统一由 store action 在成功后触发 refresh，避免组件各自手动同步。
- **[长内容或大 metadata 让详情面板过重]** → 首屏只加载摘要，详情懒加载，并对 metadata 使用折叠/格式化显示。
- **[批量删除和 cleanup 属于高风险操作]** → 必须加入显式确认、受影响数量提示和执行后反馈，且默认不隐藏当前过滤条件。
- **[现有 API 没有完整分页协议]** → 首版使用 limit/result window 和 stats 总量提示，不把这次 change 扩成新的分页后端改造。

## Migration Plan

1. 先扩展 `lib/stores/memory-store.ts`，补齐 explorer query state、stats/detail/action methods，并保持与现有 API 路径对齐。
2. 重构 `components/memory/memory-panel.tsx`，拆出 workspace shell、toolbar、list、detail 与 action dialogs，同时保留当前页面入口和 project gate。
3. 接入 summary stats、detail lazy-load、JSON export、bulk delete 与 episodic cleanup，并补齐对应多语言文案。
4. 最后补 focused 前端 tests（store + component flows），确认空态、筛选、选择、导出、删除/清理等关键路径可验证。

**Rollback strategy:**
- 若新 workspace 交互出现回归，可回退到当前最小 `MemoryPanel` 列表实现，同时保留已经完成的 backend memory explorer API，不需要回滚 Go 侧能力。
- 由于本次不引入新依赖或 schema 迁移，回滚主要是前端组件和 store 的代码回退，不涉及数据迁移。

## Open Questions

- 当前先不做 URL query 同步；如果后续 operator 明确需要可分享的 memory filter 视图，再单独提出 follow-up change。
- 如果验证中发现 `limit` result window 无法支撑真实使用，再考虑下一步为 `memory-explorer-api` 增加明确分页协议，而不是在本 change 中预支。 
