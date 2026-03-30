## Why

The team management and project kanban features have been scaffolded with stores, components, types, and backend endpoints, but several spec-mandated capabilities remain incomplete or disconnected. Specifically: (1) the task workspace only implements Board view while the spec requires List, Timeline, and Calendar views with shared filters; (2) team detail surfaces expect `taskTitle`, `coderRuns`, `plannerStatus`, `reviewerStatus` from the backend but the Go handler doesn't populate `AgentTeamSummaryDTO` fully; (3) member workload drill-down navigation is spec'd but not wired; (4) project dashboard with configurable widgets is spec'd but the frontend workspace is minimal; (5) several UX polish items like empty states, error feedback on failed drag, and agent readiness cues are incomplete. This change closes these gaps to bring existing features to spec compliance.

## What Changes

- **Task workspace multi-view**: Implement List view, Timeline view, and Calendar view components alongside the existing Board view, all sharing `task-workspace-store` filters
- **Board drag error handling**: Add rollback and inline error feedback when a Board drag-drop status transition fails to persist
- **Team summary DTO population**: Update Go `team_handler.go` to populate `taskTitle`, `coderRuns`, `plannerStatus`, `reviewerStatus` in `AgentTeamSummaryDTO` by joining task/agent-run data
- **Member workload drill-down**: Wire team roster member click to navigate to filtered project task view scoped to that member
- **Agent readiness cues**: Surface incomplete profile indicator and direct edit path on agent member rows in team roster
- **Dashboard widget workspace**: Build the configurable dashboard grid with add/remove/configure widget flows, connecting to the existing dashboard store and backend widget data API
- **Empty and error states**: Ensure all views (team list, team detail, board, list, timeline, calendar, dashboard) have explicit loading, empty, not-found, and error states per spec
- **Custom field columns in list/table views**: Render custom field values, support filter/sort/group by custom fields

## Capabilities

### New Capabilities
- `task-list-view`: Dense list/table view for project tasks with sortable columns, custom field columns, and scheduling state indicators
- `task-timeline-view`: Gantt-style timeline view with drag-to-reschedule, unscheduled task prompts, and planning field updates
- `task-calendar-view`: Calendar grid view showing tasks by planned dates with drag-to-reschedule support
- `dashboard-widget-workspace`: Configurable dashboard grid with widget catalog (throughput, burndown, blockers, budget, agent cost, review backlog, task aging, SLA), add/remove/configure/refresh flows, and server-side data aggregation

### Modified Capabilities
- `team-management`: Populate `AgentTeamSummaryDTO` fully in backend; add member workload drill-down navigation; surface agent readiness cues and incomplete profile indicators
- `task-multi-view-board`: Add drag error rollback with inline feedback; add shared filter persistence across views; add custom field filter/sort/group; add linked docs column and doc preview popover

## Impact

- **Frontend**: New view components in `components/tasks/` (list, timeline, calendar), new `components/dashboard/` widget workspace, updates to `components/team/team-management.tsx` and `components/kanban/board.tsx`
- **Backend (Go)**: Updates to `src-go/internal/handler/team_handler.go` to populate summary DTO; new/updated widget data aggregation endpoints in `src-go/internal/handler/dashboard_handler.go`
- **Stores**: Extend `task-workspace-store` for view-mode switching state; extend `dashboard-store` for widget CRUD
- **Dependencies**: May need a timeline/gantt library (e.g., `@hello-pangea/dnd` already present for drag); calendar component from shadcn/ui or similar
- **i18n**: New translation keys in `messages/` for list/timeline/calendar/dashboard labels
- **Tests**: New test files for each view component and updated tests for modified components
