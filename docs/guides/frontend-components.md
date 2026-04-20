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

## Knowledge Components

| Component | Path | Key props | Notes |
| --- | --- | --- | --- |
| `IngestedFilesPane` | `components/knowledge/IngestedFilesPane.tsx` | `projectId`, asset list props | browse ingested knowledge assets with status |
| `KnowledgeSearch` | `components/knowledge/KnowledgeSearch.tsx` | `projectId`, search query props | semantic search over knowledge assets |
| `MaterializedFromPill` | `components/knowledge/MaterializedFromPill.tsx` | `sourceId`, `sourceType` | provenance badge linking a materialized artifact to its source |
| `SourceUpdatedBanner` | `components/knowledge/SourceUpdatedBanner.tsx` | `assetId`, `updatedAt` | banner prompting re-materialization when source has changed |

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

## Shared Page Primitives (use these first)

The following primitives under `components/shared/` are the mandatory first-pass
targets for every page under `app/(dashboard)/**`. Hand-rolled replacements of
these roles are not accepted under the `ui-design-consistency` capability.

| Primitive | Path | Role | Key props | Use instead of |
| --- | --- | --- | --- | --- |
| `PageHeader` | `components/shared/page-header.tsx` | page-level title, breadcrumbs, description, actions, optional status ribbon, optional filter slot, optional sticky | `title`, `breadcrumbs`, `description`, `actions`, `status`, `filters`, `sticky`, `className` | raw `<h1>` + flex-row header blocks |
| `FilterBar` | `components/shared/filter-bar.tsx` | search + filter chips with automatic mobile overflow `Sheet` below `md` | `searchValue`, `onSearch`, `filters`, `onReset`, `moreFiltersLabel`, `resetLabel`, `children` | hand-rolled `Input` + `Select` toolbars |
| `MetricCard` | `components/shared/metric-card.tsx` | KPI tile with optional trend/sparkline; `compact` and `loading` variants | `label`, `value`, `icon`, `trend`, `sparkline`, `href`, `compact`, `loading` | raw `<Card><CardContent>…</CardContent></Card>` KPI tiles |
| `SectionCard` | `components/shared/section-card.tsx` | section framing with title/description/actions/body/footer slots on a shadcn Card | `title`, `description`, `actions`, `footer`, `as`, `className`, `bodyClassName` | raw `<Card><CardHeader><CardTitle>...</CardTitle></CardHeader><CardContent>` trios |
| `ResponsiveTabs` | `components/shared/responsive-tabs.tsx` | tab bar that degrades to a `Select` fallback below `md` | `value`, `onValueChange`, `items`, `collapseAt`, `ariaLabel` | raw `<Tabs>`/`<TabsList>` without a mobile fallback |
| `EmptyState` | `components/shared/empty-state.tsx` | empty-collection state with icon + title + optional CTA | `icon`, `title`, `description`, `action` | ad-hoc "no data" copy |
| `ErrorBanner` | `components/shared/error-banner.tsx` | recoverable-error state above the affected section | `title`, `message`, `onRetry` | `toast.error` for persistent surface errors |
| `skeleton-layouts/*` | `components/shared/skeleton-layouts/` | matching skeleton footprints for `MetricCard`, list items, workspace | component-specific | per-page hand-rolled `<Skeleton>` grids |

### When NOT to use a raw shadcn Card

Direct imports of `Card`, `CardHeader`, `CardTitle`, `CardDescription`,
`CardContent` should be rare on page code. The raw Card primitives are correct
when building a *new* shared primitive (e.g., a new chart panel). Page code
should reach for `SectionCard` or `MetricCard` instead.

### When NOT to use raw shadcn Tabs

Raw `Tabs` from `@/components/ui/tabs` are acceptable when the tab set is
desktop-only (workspace viewer, editor modes) with ≤3 tabs and explicit
breakpoint coverage. For any tab bar that must work on viewports below `md`,
use `ResponsiveTabs`.

## Layout Templates

The four approved layout templates live under `components/layout/templates/`.
Every page under `app/(dashboard)/**/page.tsx` must wrap its content in one of
these (or propose a new template via the `ui-design-consistency` spec process
if none fit).

| Template | Path | Shape | Slots | Use when |
| --- | --- | --- | --- | --- |
| `OverviewLayout` | `components/layout/templates/overview-layout.tsx` | page header + KPI strip + 2-column widget grid | `title`, `breadcrumbs`, `description`, `actions`, `metrics`, `children` | dashboard-style pages (root, project dashboard, cost) |
| `ListLayout` | `components/layout/templates/list-layout.tsx` | page header + filter bar + list/grid/table body | per template API | filterable record collections (projects, teams, plugins, marketplace, skills, roles) |
| `SettingsLayout` | `components/layout/templates/settings-layout.tsx` | section navigator + detail pane | per template API | settings surfaces (project settings) |
| `WorkspaceLayout` | `components/layout/templates/workspace-layout.tsx` | primary content + detail/inspector pane, collapses to `Sheet` below `md` | per template API | workbench surfaces (agents, workflow, scheduler, reviews, im) |

### Template Decision Tree

1. Does the page show a single KPI/overview screen? → `OverviewLayout`.
2. Is it a filterable list of records? → `ListLayout`.
3. Is it a navigable set of configuration sections? → `SettingsLayout`.
4. Does it need a primary pane plus a detail/inspector pane? → `WorkspaceLayout`.
5. None fit? → open an issue to propose a new template; do not hand-roll a
   page-root shell.

## Breakpoint Matrix

Every grid and flex container on a dashboard page must declare behavior at
`sm` (≥640px), `md` (≥768px), `lg` (≥1024px), and `xl` (≥1280px). When a
breakpoint intentionally keeps the previous layout, note it in the audit
checklist row.

| Viewport | Width | Sidebar | Filter bar | Side pane |
| --- | --- | --- | --- | --- |
| mobile | <768px | hidden, hamburger | inline + overflow `Sheet` | collapses into `Sheet` |
| tablet | 768–1023 | icon-only | inline | visible, narrower |
| desktop | 1024–1279 | expanded | inline | visible |
| wide | ≥1280 | expanded | inline | visible |

## Spacing and Typography Tokens

Page roots must use CSS-variable spacing tokens from `app/globals.css`:

| Token | Usage |
| --- | --- |
| `var(--space-section-gap)` | vertical rhythm between page sections |
| `var(--space-grid-gap)` | gap inside responsive grids |
| `var(--space-page-inline)` | horizontal page padding |
| `var(--space-card-padding)` | `SectionCard` / `MetricCard` padding |
| `var(--space-stack-xs|sm|md)` | inline stacking |

Typography utilities:

| Utility | Usage |
| --- | --- |
| `text-fluid-title` | page title |
| `text-fluid-body` | description paragraph |
| `text-fluid-metric` | `MetricCard` value |
| `text-fluid-caption` | `MetricCard` label, breadcrumb |

Raw Tailwind spacing classes (`gap-6`, `p-6`, `space-y-6`, `px-6`, `py-6`,
`gap-4`, `space-y-4`) are flagged by ESLint on JSX roots under
`app/(dashboard)/**/page.tsx`. The rule currently warns; phase 7 of the
`refine-ui-design-cohesion` change escalates it to error.

## shadcn/ui Coverage Rules

| Role | shadcn primitive | Notes |
| --- | --- | --- |
| icon-only button | `Tooltip` wrapper | translated label via i18n |
| mobile side panel | `Sheet` | `side="right"` for side flows, `side="bottom"` for filters |
| collapsible config section | `Accordion` | not a hand-rolled toggle |
| desktop split view | `ResizablePanelGroup` | stack below `md` |
| scroll region in fixed-height panel | `ScrollArea` | matches global scrollbar style |
| quick switcher / command surface | reuse `CommandPalette` via `useLayoutStore` | do not install a second `cmdk` surface |

### shadcn Drawer decision

We evaluated installing `@/components/ui/drawer` for bottom-sheet flows. Since
the shadcn `Sheet` primitive already supports `side="bottom"` with identical
semantics (Radix Dialog-backed, focus-trapped, swipe-to-dismiss on touch
devices), we do not install `drawer` at this time. Revisit if/when a flow
needs a drag-to-resize snap-point experience that `Sheet` cannot express.

## Page Audit Checklist

Every PR that modifies a page under `app/(dashboard)/**/page.tsx` must update
the corresponding row. Columns: `Tmpl` (uses approved layout template),
`Hdr` (uses `PageHeader`), `Mtr` (uses `MetricCard` if metrics present),
`Flt` (uses `FilterBar` if filters present), `Emp` (uses `EmptyState`/`ErrorBanner`
where applicable), `Spc` (design-token spacing on page root), `Bp` (breakpoint
coverage at `sm/md/lg/xl`), `A11y` (density / contrast / reduced-motion verified),
`Shd` (shadcn primitive coverage per rules above). Status: `✅` compliant,
`⚠️` gap tracked, `—` not applicable, blank = unaudited.

| Page | Tmpl | Hdr | Mtr | Flt | Emp | Spc | Bp | A11y | Shd |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `/` (root dashboard) | ✅ | ✅ | ✅ | — | — | ✅ | ✅ |  | ✅ |
| `/agent` | — | — | — | — | — | ✅ | — | — | — |
| `/agents` | ⚠️ | — | — | — | — | ✅ | — |  | — |
| `/cost` | — | ✅ | ✅ | — | ✅ | ✅ | ✅ |  | ✅ |
| `/docs` | — | ✅ | — | — | ✅ | ✅ | ✅ |  | ✅ |
| `/documents` | — | ✅ | — | — | ✅ | ✅ | — | — | — |
| `/im` | — | ✅ | — | — | ✅ | ✅ | — | — | ✅ |
| `/marketplace` | ⚠️ | ⚠️ | — | ✅ | ✅ | ✅ | ✅ |  | ✅ |
| `/memory` | — | ✅ | — | — | ✅ | ✅ | — | — | — |
| `/plugins` | — | — | — | ✅ | — | ✅ | ✅ |  | ✅ |
| `/project` | — | — | — | — | — | ✅ | — | — | — |
| `/project/dashboard` | — | ✅ | — | — | ✅ | ✅ | — |  | ✅ |
| `/project/templates` | — | — | — | — | — | ✅ | — | — | — |
| `/projects` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |  | ✅ |
| `/reviews` | — | ✅ | — | ✅ | ✅ | ✅ | — | — | ✅ |
| `/roles` | — | — | — | — | — | ✅ | — | — | — |
| `/scheduler` | — | ✅ | ✅ | — | — | ✅ | ✅ |  | ✅ |
| `/settings` | ✅ | — | — | — | — | ✅ | — | — | ✅ |
| `/skills` | — | — | — | — | — | ✅ | — | — | — |
| `/sprints` | — | ✅ | — | — | ✅ | ✅ | ✅ |  | ✅ |
| `/team` | — | — | — | — | — | ✅ | — | — | — |
| `/teams` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |  | ✅ |
| `/teams/detail` | — | ✅ | — | — | — | ✅ | — | — | — |
| `/workflow` | — | ✅ | — | — | ✅ | ✅ | — | — | — |

Legend: `✅` compliant, `⚠️` gap tracked (see notes below), `—` not applicable
(e.g., redirect pages, fullscreen workspace containers), blank = unaudited.

### Phase 2 gap notes

- `/agents` uses `WorkspaceLayout`-equivalent structure through the domain-owned
  `AgentWorkspace` component; root spacing uses design tokens but the page does
  not wrap in one of the four approved templates because `AgentWorkspace`
  manages its own split view. This is an accepted gap recorded in the
  `ui-design-consistency` audit; a follow-up would migrate `AgentWorkspace` to
  compose `WorkspaceLayout`.

### Phase 5 gap notes

- `/marketplace` ships a custom 3-pane layout (filter sidebar + main list +
  detail panel) instead of wrapping in one of the four templates, and renders
  its title via a plain `<h1>` inside the top toolbar rather than `PageHeader`.
  Root spacing/grid breakpoints and primitive adoption (FilterPanel,
  EmptyState) are compliant. This is an accepted gap with a follow-up to
  migrate the whole page into `WorkspaceLayout` + `PageHeader` once the filter
  sidebar collapses into a mobile `Sheet`.

Rows are updated per-phase as the `refine-ui-design-cohesion` change lands.
