## Context

`OverviewLayout` 的 children 包裹在 `grid lg:grid-cols-2` 中，设计意图是接受两个并排 section（左/右）。但 `DashboardPage` 传入了 3 个 children：project filter div（条件渲染）、4-widget 嵌套 grid、QuickActionShortcuts——导致 grid 分配错位，出现大片空白列。

`RoleWorkspaceCatalog` 的 catalog panel 固定宽度 260px，header 使用 `flex items-center justify-between` 单行排布，容纳了标题文字、长副标题、以及两个 `size="sm"` 按钮（共约 176px），可用于标题的空间只剩约 52px，内容不可避免地溢出或换行挤压。

## Goals / Non-Goals

**Goals:**
- Dashboard widget 区域 4 张 card 在全宽可用空间内左右对称显示，无空白列
- Project filter 和 Quick actions 全宽独立显示
- Roles catalog header 在 260px 内清晰分层：第一行标题+副标题，第二行按钮
- 修复不引入新依赖，不改变任何 store 或 API

**Non-Goals:**
- 重新设计 OverviewLayout 组件本身的 API（留给后续）
- 改变 widget 数量或内容
- 修改 project page 布局（另立 change）

## Decisions

### D1: Dashboard — 拆分 children 而非修改 OverviewLayout

**选择**：在 `DashboardPage` 中将 4 个 widget 分为两组（左列：ActivityFeed + TeamHealthWidget，右列：AgentFleetWidget + BudgetWidget），作为两个独立的 `<div className="space-y-[var(--space-grid-gap)]">` 传入 OverviewLayout；project filter 和 QuickActionShortcuts 移到 OverviewLayout 之外，作为独立全宽行渲染。

**为何不改 OverviewLayout**：OverviewLayout 的 2 列 grid 用于其他页面（overview 类型），改变其结构会影响其他消费者；只改 DashboardPage 影响范围最小，且测试更易覆盖。

**左右分配逻辑**：
- 左：ActivityFeed（高度可变）+ TeamHealthWidget（高度可变，依数据多少）
- 右：AgentFleetWidget（表格，高度依 agent 数）+ BudgetWidget（固定高）
- 两侧高度相对均衡，数据为空时各列都呈现意义完整的 card

### D2: Roles catalog header — 改为 flex-col 两行

**选择**：将 header 内层由 `flex items-center justify-between` 改为 `flex flex-col gap-2`，第一行放 title+desc，第二行放 `flex gap-2` 的两个按钮。

**为何不用 icon-only Marketplace 按钮**：icon-only 降低了 Marketplace 入口的发现性（用户可能不知道 Store 图标代表市场），260px 的两行布局完全可以容纳两个完整 button，无需牺牲文字。

## Risks / Trade-offs

- **Dashboard 左右高度不对称**：当 agents 多时右列 AgentFleetWidget 变高而左列较短，出现视觉差距——但这是数据驱动的正常情况，比空白列更合理。
- **OverviewLayout children 语义**：将 project filter 和 quick actions 移到 OverviewLayout 之外，意味着 DashboardPage 的 JSX 结构变化较大，需要更新对应的单元测试（mock）。
