## Why

当前项目任务页只有单一 Kanban Board 视图，虽然已经支持基于状态列的拖拽流转，但无法满足 PRD 中 P0 明确要求的多视图任务管理能力。技术负责人和开发成员缺少从列表、时间轴、日历等角度查看任务负载与排期的入口，也无法在不同视图里直接完成与计划相关的拖拽调整。

## What Changes

- 将项目任务页从单一 Board 扩展为 `Board / List / Timeline / Calendar` 四种可切换视图，并保留当前以项目为作用域的任务上下文。
- 为多视图补齐统一的任务筛选、排序和空状态体验，避免切换视图后出现信息缺失或交互断层。
- 在 Board 视图继续支持状态拖拽，在 Timeline / Calendar 视图支持与排期相关的拖拽调整，并让这些变更回写同一任务数据源。
- 为任务补齐支撑多视图展示所需的最小计划字段与前端状态契约，使列表、时间轴和日历不依赖临时计算或硬编码占位。

## Capabilities

### New Capabilities
- `task-multi-view-board`: 项目任务管理支持 Board、List、Timeline、Calendar 多视图切换，并在不同视图中提供一致的任务展示、筛选和拖拽更新体验。

### Modified Capabilities
- None.

## Impact

- Affected frontend routes: `app/(dashboard)/project/page.tsx` and the dashboard shell entry that navigates into project task management.
- Affected frontend UI: `components/kanban/*` plus new view-switching, list, timeline, and calendar task presentation components.
- Affected frontend state: `lib/stores/task-store.ts` and shared task view/filter state needed by all task views.
- Affected API/data contracts: project task read/update payloads may need minimal scheduling fields to support timeline/calendar placement and drag updates.
