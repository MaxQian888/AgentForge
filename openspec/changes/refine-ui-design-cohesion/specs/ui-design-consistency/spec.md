## ADDED Requirements

### Requirement: Every dashboard page uses an approved layout template

The system SHALL require every page file under `app/(dashboard)/**/page.tsx` to wrap its content in one of the approved layout templates: `OverviewLayout`, `ListLayout`, `SettingsLayout`, or `WorkspaceLayout` from `components/layout/templates/`. Hand-rolled `<div className="flex flex-col gap-*">` shells at the page root SHALL NOT be accepted.

#### Scenario: Overview-style page is refactored

- **WHEN** a contributor builds a page that shows a headline and KPI grid above secondary widgets
- **THEN** the page wraps its content in `OverviewLayout`
- **AND** the layout supplies breadcrumbs, title, and optional description/actions via props
- **AND** metrics are rendered inside the template's `metrics` slot, not in a hand-rolled grid

#### Scenario: List-style page is refactored

- **WHEN** a contributor builds a page that shows a filterable collection of records
- **THEN** the page wraps its content in `ListLayout`
- **AND** the layout renders the shared `PageHeader` and `FilterBar` in the correct slots
- **AND** the content slot receives the resulting list/grid/table

#### Scenario: Settings-style page is refactored

- **WHEN** a contributor builds a page that shows a navigable set of configuration sections
- **THEN** the page uses `SettingsLayout` (or `SettingsShell`) to render the section navigator and detail pane
- **AND** each section is framed by `SectionCard`

#### Scenario: Workspace-style page is refactored

- **WHEN** a contributor builds a page with primary content plus a detail pane or side inspector
- **THEN** the page uses `WorkspaceLayout` and renders the split view through the template's slots
- **AND** on viewports below `md` (768px), the side pane moves into a `Sheet` controlled by the template

#### Scenario: Reviewer audits a non-compliant page

- **WHEN** a PR adds a page that renders `<div className="flex flex-col gap-6">...</div>` as its root with no template wrapper
- **THEN** the reviewer MUST block the PR citing this requirement
- **AND** the audit checklist for that page is marked non-compliant

### Requirement: Pages render header chrome through the upgraded PageHeader primitive

The system SHALL render page title, breadcrumbs, description, actions, and (optionally) status ribbon through the shared `PageHeader` component. Hand-rolled `<h1>` + flex-row replacements SHALL NOT be accepted.

#### Scenario: Page declares a title

- **WHEN** a page needs a visible title
- **THEN** it passes the translated string to `PageHeader`'s `title` prop
- **AND** no raw `<h1>` is used for the page title elsewhere

#### Scenario: Page declares breadcrumbs

- **WHEN** a page has a breadcrumb trail
- **THEN** the page calls `useBreadcrumbs(...)` AND passes the same entries to `PageHeader`'s `breadcrumbs` prop (or lets the layout template do it)
- **AND** the last breadcrumb is not a link

#### Scenario: Page declares actions

- **WHEN** a page exposes primary actions (new/create/refresh)
- **THEN** actions are rendered via the `actions` slot of `PageHeader`
- **AND** icon-only actions are wrapped in `Tooltip`

#### Scenario: Header sticks on scroll when requested

- **WHEN** a page passes `sticky` to `PageHeader`
- **THEN** the header remains visible at the top of the scroll container
- **AND** it uses a translucent backdrop (`bg-background/80 backdrop-blur-sm`) so content shows through

### Requirement: Metric grids use MetricCard, not raw Card primitives

The system SHALL render numeric summaries via the shared `MetricCard` component. Raw `<Card>` + `<CardContent>` compositions that replicate the metric pattern SHALL NOT be accepted.

#### Scenario: Page shows a KPI row

- **WHEN** a page displays a row of numeric summaries
- **THEN** each cell is a `MetricCard` with `label`, `value`, optional `icon`, and optional `trend`/`sparkline`
- **AND** the row is placed inside the `metrics` slot of the enclosing layout template

#### Scenario: Metric is loading

- **WHEN** the data behind a metric is still loading
- **THEN** the page renders the shared metric skeleton (`components/shared/skeleton-layouts/metric-card-skeleton.tsx`) in place of the `MetricCard`

### Requirement: Filter chrome uses the upgraded FilterBar primitive

The system SHALL render search and filter chrome via the shared `FilterBar` component. Hand-rolled search inputs + filter chips combinations SHALL NOT be accepted.

#### Scenario: Page needs search and filter

- **WHEN** a page needs text search plus optional filter chips
- **THEN** it uses `FilterBar` with `searchValue`, `onSearch`, and whatever filter slots are defined by the upgraded component
- **AND** the reset button clears all filters at once

#### Scenario: Filters collapse on mobile

- **WHEN** the viewport is below `md` (768px) and more than three filters are active
- **THEN** `FilterBar` collapses overflow filters into a "More filters" trigger that opens a `Sheet`
- **AND** focus is moved into the sheet for keyboard users

### Requirement: Empty, error, and loading states use shared primitives

The system SHALL render empty, error, and loading states via the shared primitives `EmptyState`, `ErrorBanner`, and the `skeleton-layouts` family.

#### Scenario: Page has no data

- **WHEN** a collection is empty and the user has not filtered it out
- **THEN** the page renders `EmptyState` with an icon, title, optional description, and optional call-to-action

#### Scenario: Page has filtered results but none match

- **WHEN** a collection is non-empty but the current filters yield zero matches
- **THEN** the page renders `EmptyState` with the search icon and a "no matching results" title

#### Scenario: Page encounters a recoverable error

- **WHEN** an API call fails in a way the user can retry
- **THEN** the page renders `ErrorBanner` above the affected section with a retry action
- **AND** the rest of the page remains interactive

#### Scenario: Page is loading

- **WHEN** page data is loading for the first time
- **THEN** the page renders a matching skeleton from `components/shared/skeleton-layouts/`
- **AND** the skeleton reserves the same layout footprint as the loaded content to prevent cumulative layout shift

### Requirement: Pages use design-token spacing and typography

The system SHALL use CSS variable spacing tokens and fluid typography utilities defined in `app/globals.css`. Raw Tailwind spacing classes (`gap-6`, `p-6`, `space-y-6`, `px-4`) SHALL NOT appear at page-root level under `app/(dashboard)/**`.

#### Scenario: Page declares section spacing

- **WHEN** a page needs vertical rhythm between sections
- **THEN** it uses `gap-[var(--space-section-gap)]` or `space-y-[var(--space-section-gap)]`
- **AND** CI lint flags the use of `gap-6` / `space-y-6` as an error on the page root

#### Scenario: Page declares inline padding

- **WHEN** a page needs horizontal padding at its root
- **THEN** it uses `p-[var(--space-page-inline)]` or the template equivalent
- **AND** it does not hardcode `p-4` / `p-6`

#### Scenario: Page declares title typography

- **WHEN** a page renders a page-level heading
- **THEN** it uses the `text-fluid-title` utility (not a hardcoded `text-3xl`)
- **AND** descriptions use `text-fluid-body`, metric values use `text-fluid-metric`, and captions use `text-fluid-caption`

### Requirement: Pages declare breakpoint behavior at sm, md, lg, xl

The system SHALL require every grid and flex container on a dashboard page to declare breakpoint behavior at `sm` (≥640px), `md` (≥768px), `lg` (≥1024px), and `xl` (≥1280px). When a breakpoint intentionally does not change the layout, the reviewer confirms this explicitly in the audit checklist.

#### Scenario: Grid adapts across viewports

- **WHEN** a grid shows items in columns
- **THEN** it declares `grid-cols-1 sm:grid-cols-2 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4` (or the page-specific equivalent)
- **AND** there is no tablet dead zone at 768px–1023px

#### Scenario: Side pane behavior at mobile

- **WHEN** a page uses a side pane via `WorkspaceLayout` on viewports below `md`
- **THEN** the side pane moves into a `Sheet` triggered by a visible button
- **AND** the main content takes the full width

#### Scenario: Data table on mobile

- **WHEN** a page renders a table that would overflow on viewports below `md`
- **THEN** it uses `ResponsiveTable` which transforms rows into stacked cards
- **AND** each card preserves every field the desktop row exposed

### Requirement: Pages use shadcn/ui primitives for their intended roles

The system SHALL adopt shadcn/ui components for the roles they were designed to serve. Hand-rolled replacements for these roles SHALL NOT be accepted.

#### Scenario: Icon-only button

- **WHEN** a button shows only an icon
- **THEN** it is wrapped in `Tooltip` exposing the action label
- **AND** the tooltip content is translated via i18n

#### Scenario: Collapsible configuration section

- **WHEN** a configuration panel has optional advanced sections
- **THEN** it uses `Accordion` (not a hand-rolled toggle)

#### Scenario: Split view on desktop

- **WHEN** a page shows a resizable two-pane view on viewports at or above `md`
- **THEN** it uses `ResizablePanelGroup` with sensible min/max sizes
- **AND** on viewports below `md` the panes stack

#### Scenario: Scroll region inside fixed panel

- **WHEN** a panel has a bounded height and overflowing content
- **THEN** it uses `ScrollArea` so the scrollbar styling matches the rest of the app

#### Scenario: Quick switcher / command surface

- **WHEN** the page offers a keyboard-driven quick switcher
- **THEN** it uses the existing `CommandPalette` via the `useLayoutStore` hooks
- **AND** it does not install a second `cmdk` surface

### Requirement: Pages honor accessibility preferences end-to-end

The system SHALL ensure every dashboard page renders correctly under all combinations of `data-density`, `data-contrast`, `data-reduced-motion`, and `data-screen-reader` attributes on `<html>`.

#### Scenario: Compact density

- **WHEN** `data-density="compact"` is applied
- **THEN** the page's primary container adopts the `--density-*` tokens and remains readable
- **AND** no content is clipped

#### Scenario: High contrast

- **WHEN** `data-contrast="high"` is applied
- **THEN** foreground/background pairs meet WCAG AA contrast across all components on the page
- **AND** focus outlines are visible at 3px

#### Scenario: Reduced motion

- **WHEN** `data-reduced-motion="true"` is applied
- **THEN** no page animation exceeds 1ms
- **AND** scroll behavior remains `auto`

#### Scenario: Screen reader hints

- **WHEN** `data-screen-reader="true"` is applied
- **THEN** elements carrying `data-sr-hint` become visible as inline hints
- **AND** focus outlines strengthen for AT users

### Requirement: Refactored pages preserve all existing functionality

The system SHALL require that any refactor performed under this capability leaves store-hook wiring, URL parameters, i18n keys, keyboard shortcuts, and WebSocket subscriptions unchanged. A refactor that changes data behavior or routing contracts SHALL NOT be accepted under this capability.

#### Scenario: Store hooks survive the refactor

- **WHEN** a page is refactored to use the approved primitives
- **THEN** the same Zustand store hooks are called with the same arguments
- **AND** the same useEffect sequences run in the same order

#### Scenario: i18n keys survive the refactor

- **WHEN** a page is refactored
- **THEN** the same translation keys render the same copy as before
- **AND** any renamed namespace is a mechanical move, not a rewrite

#### Scenario: URL parameters survive the refactor

- **WHEN** a page uses `useSearchParams` / `useParams`
- **THEN** the same parameter names are read and written
- **AND** deep links continue to land on the same state

### Requirement: Page audit checklist is maintained

The system SHALL maintain an audit checklist in `docs/guides/frontend-components.md` (or a peer document) listing every page under `app/(dashboard)/**` and its compliance state across the requirements above. The checklist SHALL be updated in every PR that modifies a dashboard page.

#### Scenario: New page is added

- **WHEN** a new page is introduced under `app/(dashboard)/**`
- **THEN** the PR adds a row to the audit checklist
- **AND** each compliance column is filled in

#### Scenario: Page is refactored

- **WHEN** a refactor PR lands
- **THEN** the audit checklist reflects the new compliance state
- **AND** any remaining gaps are called out with a linked follow-up issue

#### Scenario: Lint rule is violated

- **WHEN** CI detects a forbidden spacing token on a dashboard page
- **THEN** the build fails and the PR cannot merge until the violation is resolved or explicitly ignored with justification
