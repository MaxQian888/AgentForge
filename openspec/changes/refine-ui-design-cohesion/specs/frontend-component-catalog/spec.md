## ADDED Requirements

### Requirement: Shared workspace primitives are documented as the primary path

The system SHALL document `PageHeader`, `FilterBar`, `MetricCard`, `EmptyState`, `ErrorBanner`, the shared `skeleton-layouts`, `SectionCard`, and `ResponsiveTabs` in `docs/guides/frontend-components.md` as the primary primitives for dashboard pages. The doc SHALL explicitly call out that raw shadcn `<Card>` + `<CardHeader>` + `<CardTitle>` trios are NOT a valid substitute for `SectionCard`, and raw `<Tabs>` are NOT a valid substitute for `ResponsiveTabs` when the content needs a mobile fallback.

#### Scenario: Contributor looks up the primary section primitive

- **WHEN** a contributor needs to frame a section with a title, description, optional actions, and body
- **THEN** the doc directs them to `SectionCard` with a full props table, usage example, and "when not to use" guidance
- **AND** the guidance explicitly names the raw-Card anti-pattern to avoid

#### Scenario: Contributor looks up the responsive tabs primitive

- **WHEN** a contributor needs a tab bar that degrades gracefully on narrow viewports
- **THEN** the doc directs them to `ResponsiveTabs` with a props table, usage example, and notes on the mobile fallback (`Select` or `Sheet`)
- **AND** the doc explains when the raw shadcn `Tabs` are still acceptable (desktop-only panels with ≤3 tabs)

#### Scenario: Contributor looks up metric display

- **WHEN** a contributor needs to render a KPI strip
- **THEN** the doc directs them to `MetricCard` with coverage of the compact, trend-only, sparkline, and loading-skeleton variants
- **AND** the doc links to the matching skeleton under `components/shared/skeleton-layouts/`

### Requirement: Layout template catalog lists slots, breakpoints, and usage

The system SHALL document the four approved layout templates (`OverviewLayout`, `ListLayout`, `SettingsLayout`, `WorkspaceLayout`) in `docs/guides/frontend-components.md` with a full slot map, breakpoint behavior, and a one-paragraph "when to choose this" description per template.

#### Scenario: Contributor chooses between list and workspace

- **WHEN** a contributor has to decide which template fits a page
- **THEN** the doc offers a decision tree (overview → list → workspace → settings) and example screenshots or wireframe links
- **AND** each template entry lists its required and optional props

#### Scenario: Contributor needs tablet behavior

- **WHEN** a contributor is unsure how a template behaves at `md` viewports (768–1023px)
- **THEN** the doc spells out column counts, side-pane treatment, and filter overflow at `sm`, `md`, `lg`, and `xl`

### Requirement: Page audit checklist is part of the component catalog

The system SHALL include a page-level audit checklist in `docs/guides/frontend-components.md` listing every page under `app/(dashboard)/**` with columns for each compliance concern: template usage, `PageHeader`, `MetricCard`, `FilterBar`, `EmptyState`/`ErrorBanner`, spacing tokens, breakpoint coverage, density/contrast/reduced-motion parity, and shadcn primitive coverage.

#### Scenario: Reviewer checks page compliance

- **WHEN** a reviewer opens the catalog to validate a PR
- **THEN** the checklist row for the touched page shows the current compliance state at a glance
- **AND** any red cells are either fixed in the PR or tracked in a linked follow-up

#### Scenario: New page is introduced

- **WHEN** a contributor adds a new file under `app/(dashboard)/**/page.tsx`
- **THEN** the PR appends a row to the checklist with every column filled in
- **AND** the PR cannot merge with missing checklist rows

### Requirement: Shared state documentation cross-references primitives

The system SHALL update `docs/guides/state-management.md` so that every primitive listed in the upgraded catalog is cross-linked to the stores it most commonly pairs with (e.g., `PageHeader` ↔ `useBreadcrumbs`; `FilterBar` ↔ the relevant domain filter slice).

#### Scenario: Contributor wires a filter bar to a store

- **WHEN** a contributor needs to drive `FilterBar` from a Zustand slice
- **THEN** the state-management doc shows the canonical hook names and reset pattern
- **AND** links back to the `FilterBar` section of the component catalog
