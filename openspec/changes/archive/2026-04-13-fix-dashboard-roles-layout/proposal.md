## Why

Dashboard 首页的 widget 区域因 `OverviewLayout` 的 2 列 grid 与传入的 children 结构不匹配，导致左列出现大片空白。Roles 页面左侧 catalog panel（260px）的 header 在单行内塞入标题、描述和两个按钮，内容挤成一坨。两处均为纯布局缺陷，影响日常使用体验。

## What Changes

- **Dashboard (`app/(dashboard)/page.tsx`)**: 重构传给 `OverviewLayout` 的 children 结构，将 project filter 和 quick actions 从 grid cell 中剥离，4 个 widget card 按左/右两列对称分布，消除空白列。
- **Roles catalog header (`components/roles/role-workspace-catalog.tsx`)**: 将 header 改为两行布局——第一行显示标题和副标题，第二行显示操作按钮（Marketplace + New Role），消除 260px 容器内的内容溢出。

## Capabilities

### New Capabilities
<!-- none -->

### Modified Capabilities
- `dashboard-insights`: Dashboard 首页 widget 布局行为变更——4 个 widget 现在分左右两列对称排布，quick actions 和 project filter 独立全宽渲染。
- `role-management-panel`: Roles catalog header 布局变更——按钮区域移至标题下方独立行，解决 260px 容器内的溢出问题。

## Impact

- `app/(dashboard)/page.tsx` — children 重组，widget grid 拆成两组分列传入
- `components/roles/role-workspace-catalog.tsx` — header 内部 flex 方向由单行改为两行
- 无 API 变更，无 store 变更，无新依赖
