## 1. Backend: Team Summary DTO Population

- [x] 1.1 Add `GetTeamSummary(id)` repository method that JOINs `agent_teams` with `tasks` to populate `taskTitle`, and queries `agent_runs` for coder/planner/reviewer statuses
- [x] 1.2 Add `ListTeamSummaries(projectId)` repository method with batch task title lookup (IN clause) to avoid N+1 queries
- [x] 1.3 Update `team_handler.go` GetTeam to call `GetTeamSummary` and return fully populated `AgentTeamSummaryDTO`
- [x] 1.4 Update `team_handler.go` ListTeams to call `ListTeamSummaries` and return summaries with `taskTitle` populated
- [x] 1.5 Write Go tests for team summary population (verify taskTitle, coderRuns, plannerStatus, reviewerStatus fields)

## 2. Backend: Dashboard Widget Data Aggregation

- [x] 2.1 Create `dashboard_handler.go` with `GET /api/v1/projects/:pid/dashboard/widgets/:type` endpoint
- [x] 2.2 Implement `throughput_chart` aggregation (tasks completed per time period)
- [x] 2.3 Implement `blocker_count` aggregation (current blocked tasks + trend)
- [x] 2.4 Implement `budget_consumption` aggregation (spent vs allocated)
- [x] 2.5 Implement `task_aging` aggregation (days since creation grouped by status)
- [x] 2.6 Implement `agent_cost`, `review_backlog`, `burndown`, `sla_compliance` aggregations
- [x] 2.7 Register dashboard widget route in `routes.go`
- [x] 2.8 Write Go tests for widget data aggregation endpoints

## 3. Board View: Drag Error Handling & Polish

- [x] 3.1 Add optimistic update with rollback in `board.tsx` — show loading state on card during transition API call, snap back on failure
- [x] 3.2 Add inline toast/error feedback when drag status transition fails, including reason from API response
- [x] 3.3 Add Board loading skeleton (column placeholders) during initial task fetch
- [x] 3.4 Add Board empty state with "Create Task" action when no tasks exist or all are filtered out
- [x] 3.5 Update board tests for drag error rollback behavior and loading/empty states

## 4. Task List View

- [x] 4.1 Install `@tanstack/react-table` dependency
- [x] 4.2 Create `components/tasks/task-list-view.tsx` — dense table with columns: title, status, priority, assignee, planned dates, labels
- [x] 4.3 Add column sorting (click header to sort asc/desc with indicator)
- [x] 4.4 Add inline status and priority quick-change dropdowns in table rows
- [x] 4.5 Add custom field dynamic columns with column visibility toggle
- [x] 4.6 Add filter/sort/group by custom field values
- [x] 4.7 Add Linked Docs column with clickable chips
- [x] 4.8 Add scheduled vs unscheduled indicator in date columns
- [x] 4.9 Add empty state for List view
- [x] 4.10 Wire List view into `project-task-workspace.tsx` view switcher
- [x] 4.11 Write tests for task-list-view (sorting, filtering, inline edits, empty state)

## 5. Task Timeline View

- [x] 5.1 Create `components/tasks/task-timeline-view.tsx` — CSS grid-based Gantt with day/week/month granularity toggle
- [x] 5.2 Render scheduled tasks as horizontal bars positioned by `plannedStartAt`/`plannedEndAt`
- [x] 5.3 Add drag-to-reschedule using `@hello-pangea/dnd` — update planning dates on drop, preserve duration
- [x] 5.4 Add drag failure rollback with inline feedback
- [x] 5.5 Add "Unscheduled" sidebar section listing tasks without planning dates
- [x] 5.6 Support drag from unscheduled section onto timeline to assign dates
- [x] 5.7 Add empty state for Timeline view
- [x] 5.8 Wire Timeline view into workspace view switcher
- [x] 5.9 Write tests for task-timeline-view (rendering, drag reschedule, unscheduled section)

## 6. Task Calendar View

- [x] 6.1 Create `components/tasks/task-calendar-view.tsx` — month/week date grid with task chips on date cells
- [x] 6.2 Render multi-day tasks spanning across date cells
- [x] 6.3 Add month/week mode toggle
- [x] 6.4 Add drag-to-reschedule between date cells with proportional date shift
- [x] 6.5 Add drag failure rollback with feedback
- [x] 6.6 Add "Unscheduled" section for tasks without planning dates
- [x] 6.7 Add empty state for Calendar view
- [x] 6.8 Wire Calendar view into workspace view switcher
- [x] 6.9 Write tests for task-calendar-view (rendering, drag, month/week toggle)

## 7. Shared View Infrastructure

- [x] 7.1 Verify `task-workspace-store` filter state persists across Board/List/Timeline/Calendar switches — fix if not
- [x] 7.2 Ensure selected task persists across view switches (detail panel stays open)
- [x] 7.3 Add doc preview popover on task card hover (linked-doc indicator) — show title + first 3 lines + "View" link
- [x] 7.4 Add i18n translation keys for new view labels and UI strings in `messages/`

## 8. Team Management: Workload Drill-Down & Readiness

- [x] 8.1 Wire member row click / workload indicator click to navigate to project task workspace filtered by that member as assignee
- [x] 8.2 Wire agent member active-run count click to navigate to team runs list filtered by that agent
- [x] 8.3 Add prominent "Setup Required" badge on agent rows with incomplete profiles (missing runtime/provider/model)
- [x] 8.4 Make "Setup Required" badge clickable — opens agent profile editor with missing fields highlighted
- [x] 8.5 Add team list error state with retry button
- [x] 8.6 Add team detail loading skeleton (not premature not-found)
- [x] 8.7 Ensure team detail shows `taskTitle` from populated DTO instead of raw task ID
- [x] 8.8 Update team management tests for drill-down navigation, readiness badge, and error states

## 9. Dashboard Widget Workspace

- [x] 9.1 Install `react-grid-layout` dependency
- [x] 9.2 Create `components/dashboard/dashboard-workspace.tsx` — dashboard selector, create/rename/delete actions
- [x] 9.3 Create `components/dashboard/widget-grid.tsx` — react-grid-layout based draggable/resizable grid
- [x] 9.4 Create `components/dashboard/widget-card.tsx` — standard card with header (title, configure, refresh, remove)
- [x] 9.5 Create `components/dashboard/widget-catalog.tsx` — dialog showing available widget types to add
- [x] 9.6 Create widget type components: `throughput-chart.tsx`, `burndown.tsx`, `blocker-count.tsx`, `budget-consumption.tsx`, `agent-cost.tsx`, `review-backlog.tsx`, `task-aging.tsx`, `sla-compliance.tsx`
- [x] 9.7 Wire widget data fetching to `GET /api/v1/projects/:pid/dashboard/widgets/:type`
- [x] 9.8 Add widget empty states and error states per widget type
- [x] 9.9 Add dashboard loading skeleton, empty state (no dashboards), and no-project-selected state
- [x] 9.10 Persist layout changes through dashboard store save flow with save feedback
- [x] 9.11 Wire dashboard workspace into project dashboard page route
- [x] 9.12 Write tests for dashboard workspace (CRUD, widget add/remove/configure, layout persistence, error states)

## 10. Board Multi-Select & Bulk Operations

- [x] 10.1 Implement Ctrl/Cmd+click multi-select on task cards in Board view
- [x] 10.2 Add bulk action toolbar (appears on multi-select) with bulk status change, bulk assign, bulk delete
- [x] 10.3 Handle partial failures in bulk operations (report individually failed tasks)
- [x] 10.4 Write tests for bulk operations
