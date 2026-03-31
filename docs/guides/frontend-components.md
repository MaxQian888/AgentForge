# Frontend Component Catalog / 前端关键组件目录

This guide maps the major reusable components that back the current dashboard
surfaces.

## Layout Components

| Component | Path | Key contract | Use when |
| --- | --- | --- | --- |
| `DashboardShell` | `components/layout/dashboard-shell.tsx` | dashboard route shell | every dashboard page |
| `Sidebar` | `components/layout/sidebar.tsx` | route/navigation driven | main navigation |
| `Header` | `components/layout/header.tsx` | page title and action seam | page top bar |
| `DesktopWindowFrame` | `components/layout/desktop-window-frame.tsx` | frameless desktop chrome wrapper | Tauri mode titlebar and window actions |

## Task Workspace Components

| Component | Path | Key props | Notes |
| --- | --- | --- | --- |
| `ProjectTaskWorkspace` | `components/tasks/project-task-workspace.tsx` | `projectId`, `projectName`, `tasks`, `loading`, `error`, `realtimeConnected`, `notifications`, `members` | orchestrates board/list/timeline/calendar workspace |
| `TaskDetailContent` | `components/tasks/task-detail-content.tsx` | `task`, `tasks`, `members`, `agents`, `sprints`, `onTaskSave`, `onTaskAssign` | editable task detail panel |
| `TaskContextRail` | `components/tasks/task-context-rail.tsx` | `selectionState`, `selectedTask`, `counts`, `dependencySummary`, `costSummary`, `alerts`, `realtimeState`, `tasks` | right-side context rail |
| `SpawnAgentDialog` | `components/tasks/spawn-agent-dialog.tsx` | task/member/runtime selection props | manual agent launch |
| `DispatchPreflightDialog` | `components/tasks/dispatch-preflight-dialog.tsx` | preflight result and confirmation props | guarded dispatch UX |

## Review Components

| Component | Path | Key props | Notes |
| --- | --- | --- | --- |
| `ReviewWorkspace` | `components/review/review-workspace.tsx` | `reviews`, `loading`, `error`, `selectedReviewId`, `onTriggerReview` | review backlog + detail split view |
| `ReviewDetailPanel` | `components/review/review-detail-panel.tsx` | `review`, `onApprove`, `onRequestChanges` | single review detail |
| `ReviewDecisionActions` | `components/review/review-decision-actions.tsx` | `reviewId`, `onApprove`, `onRequestChanges`, `compact` | inline decision controls |
| `ReviewFindingsTable` | `components/review/review-findings-table.tsx` | findings list props | structured finding display |

## Form Components

| Component | Path | Key contract | Notes |
| --- | --- | --- | --- |
| `FormBuilder` | `components/forms/form-builder.tsx` | form-definition editing surface | build project forms |
| `FormRenderer` | `components/forms/form-renderer.tsx` | runtime submission surface | render saved forms for users |

## Design Constraints

- prefer existing workspace components before inventing page-local clones
- keep task and review interactions flowing through the existing store-backed seams
- use `components/ui/*` for primitives and `components/<domain>/*` for composites
- keep desktop-only affordances behind the platform/runtime facade
