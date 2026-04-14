## 1. Roles Catalog Header Fix

- [x] 1.1 修改 `components/roles/role-workspace-catalog.tsx`：将 header 内层由单行 `flex items-center justify-between` 改为两行 `flex flex-col gap-2`，第一行放 title+desc，第二行放 `flex items-center gap-2` 的两个按钮
- [x] 1.2 更新 `components/roles/role-workspace-catalog.test.tsx`：验证 header 中标题和按钮分布在两个独立行

## 2. Dashboard Widget Layout Fix

- [x] 2.1 修改 `app/(dashboard)/page.tsx`：将 4 个 widget 拆为两个 `<div className="flex flex-col gap-[var(--space-grid-gap)]">`，左列放 ActivityFeed + TeamHealthWidget，右列放 AgentFleetWidget + BudgetWidget，作为两个并排 children 传给 OverviewLayout
- [x] 2.2 将 project filter `<div>` 和 `<QuickActionShortcuts>` 移到 OverviewLayout 之外，在外层 wrapper 中作为独立全宽行渲染
- [x] 2.3 更新 `app/(dashboard)/page.test.tsx`：验证 widget 区域无空列，project filter 和 quick actions 各自独立全宽渲染

## 3. Test & Verify

- [x] 3.1 运行 `pnpm test` 确保所有测试通过
