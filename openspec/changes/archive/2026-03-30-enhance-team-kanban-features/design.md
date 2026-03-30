## Context

AgentForge's project workspace currently implements a Board (kanban) view for tasks, a team roster with member CRUD, and an agent team execution pipeline. The backend Go API has full CRUD for tasks, members, and teams, plus status transitions, assignment, decomposition, and agent dispatch. However, multiple spec-mandated capabilities are incomplete:

- The task workspace only renders Board view; List, Timeline, and Calendar views are spec'd but unbuilt
- `AgentTeamSummaryDTO` defines `taskTitle`, `coderRuns`, `plannerStatus`, `reviewerStatus` fields but the handler doesn't populate them (returns zero values)
- Dashboard widget workspace exists as a store but has no frontend grid or widget components
- Board drag-drop has no error rollback or user feedback on failure
- Member workload drill-down navigation is spec'd but not wired
- Agent readiness cues are partially implemented in `team-management.tsx` but incomplete profile indicators lack prominence

The frontend uses Next.js 16 (App Router), shadcn/ui, Zustand stores, and `@hello-pangea/dnd` for drag-and-drop. The backend is Go with PostgreSQL.

## Goals / Non-Goals

**Goals:**
- Bring task workspace to full multi-view compliance (Board + List + Timeline + Calendar)
- Populate `AgentTeamSummaryDTO` fully in the Go team handler
- Wire member workload drill-down from team roster to filtered task board
- Build configurable dashboard widget workspace with core widget types
- Add proper empty/loading/error states across all views
- Add custom field columns to list view with filter/sort/group support

**Non-Goals:**
- Real-time WebSocket push for dashboard widgets (polling/manual refresh is sufficient for now)
- Per-role runtime overrides in team execution (spec explicitly defers this)
- Saved view persistence (separate spec — this change only ensures view switching works)
- Document editor integration (linked docs display only, not authoring)
- Mobile/responsive layout optimization

## Decisions

### D1: Task List View — Use shadcn Table with virtual scrolling

**Decision**: Build the List view using shadcn/ui `Table` component with `@tanstack/react-table` for column management, sorting, and filtering. Use CSS-based virtual scrolling for performance with large task sets.

**Rationale**: `@tanstack/react-table` is headless and pairs well with shadcn's Table component. It natively supports column visibility toggles, sorting, filtering, and grouping — all needed for custom field columns. This avoids adding a heavy data grid library.

**Alternatives considered**:
- AG Grid: Too heavy for this use case, adds 200KB+ bundle
- Custom table: Would replicate what @tanstack/react-table already provides

### D2: Timeline View — Custom implementation with horizontal scroll

**Decision**: Build a Gantt-style timeline using a custom component backed by the existing `@hello-pangea/dnd` library for drag-to-reschedule. Render time slots as CSS grid columns with tasks as positioned bars.

**Rationale**: No well-maintained React Gantt library fits our shadcn/Tailwind design system without extensive customization. A CSS grid approach with our existing DnD library keeps the dependency footprint minimal and styling consistent.

**Alternatives considered**:
- dhtmlx-gantt: Not React-native, requires wrapper, breaks Tailwind theming
- react-big-calendar: Better for calendar view (see D3), not suited for Gantt

### D3: Calendar View — shadcn calendar + date grid

**Decision**: Build a month/week calendar grid using shadcn's date primitives and custom day cells showing task chips. Drag-to-reschedule updates `plannedStartAt`/`plannedEndAt`.

**Rationale**: Keeps design consistent with shadcn. Tasks without planning dates show in an "Unscheduled" sidebar section per spec requirement.

### D4: Team Summary DTO Population — JOIN in handler with optional agent-run lookup

**Decision**: In `team_handler.go`, when returning `AgentTeamSummaryDTO`, perform:
1. JOIN with `tasks` table to populate `taskTitle`
2. Query `agent_runs` table filtered by `team_id` and role=coder to populate `coderRuns`, `coderTotal`, `coderCompleted`
3. Query individual agent runs for `plannerRunId`/`reviewerRunId` to get their statuses

Do this in the repository layer with a dedicated `GetTeamSummary(id)` method that returns the enriched struct in one query (or minimal queries).

**Rationale**: The DTO already defines these fields. The data exists in the database. The only gap is the handler not querying it. Adding a repository method keeps the handler thin.

**Alternatives considered**:
- Frontend-side enrichment (extra API calls): Violates backend-for-frontend principle, adds waterfall requests
- Materialized view: Overkill for the query volume

### D5: Dashboard Widget Grid — react-grid-layout

**Decision**: Use `react-grid-layout` for the dashboard widget positioning and resizing. Each widget renders as a card with standard header (title, config button, refresh, remove) and type-specific content area.

**Rationale**: `react-grid-layout` is the de facto standard for draggable/resizable dashboard grids in React. It's lightweight (~15KB), well-maintained, and handles persistence of layout coordinates natively.

### D6: Shared Filter State Across Views

**Decision**: The existing `task-workspace-store` already holds filters (status, priority, assignee, sprint, search). All four views read from this same store. View switching only changes `viewMode` — filters persist.

**Rationale**: This is already the architecture. The key implementation detail is ensuring new view components consume the same store selectors rather than maintaining local filter state.

## Risks / Trade-offs

**[Risk] Timeline/Calendar views add significant frontend complexity** → Mitigated by implementing as separate lazy-loaded route segments; Board view remains the default and is unaffected.

**[Risk] `GetTeamSummary` adds N+1 query potential for team list** → Mitigated by using batch queries (IN clause for task titles, single query for all coder runs per team set) in the list endpoint.

**[Risk] react-grid-layout adds bundle size** → Mitigated by dynamic import; dashboard page is a separate route segment, not loaded with task workspace.

**[Risk] Custom field filter/sort/group adds complexity to list view** → Custom fields already have a backend API contract (`customFieldFilters`, `customFieldSort` query params). Frontend implementation follows existing patterns in `@tanstack/react-table` with dynamic column definitions.

**[Trade-off] Calendar drag-to-reschedule updates both start and end dates proportionally** — This may surprise users who want to change only the start date. Accept for now; add a resize handle for duration adjustment in a follow-up.
