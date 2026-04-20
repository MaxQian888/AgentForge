## ADDED Requirements

### Requirement: Dashboard pages must wrap content in an approved layout template

The system SHALL require every page file under `app/(dashboard)/**/page.tsx` to render its content inside one of the approved layout templates exported from `components/layout/templates/`: `OverviewLayout`, `ListLayout`, `SettingsLayout`, or `WorkspaceLayout`. A page SHALL NOT ship with a hand-rolled flex or grid shell at its root.

#### Scenario: Contributor selects a template

- **WHEN** a contributor creates or refactors a dashboard page
- **THEN** they pick the template whose shape best matches the page (overview / list / settings / workspace)
- **AND** any content that does not fit the template's slots is wrapped in a `SectionCard` inside the content slot

#### Scenario: Template consumes shared primitives

- **WHEN** the selected template renders its header area
- **THEN** it uses the upgraded `PageHeader` primitive under the hood so breadcrumbs, title, actions, and sticky behavior are uniform
- **AND** the template honors the design-token spacing scale (`--space-section-gap`, `--space-grid-gap`, `--space-page-inline`)

#### Scenario: Template escape hatch

- **WHEN** a page genuinely does not fit any of the four templates
- **THEN** the contributor proposes a new template under `components/layout/templates/` rather than hand-rolling a one-off shell
- **AND** the new template undergoes the same design review before it is accepted

### Requirement: Every grid and flex container declares behavior at sm, md, lg, xl

The system SHALL require every responsive container on a dashboard page to declare breakpoint behavior at `sm` (≥640px), `md` (≥768px), `lg` (≥1024px), and `xl` (≥1280px). When a breakpoint intentionally keeps the previous layout, the absence of a class is acceptable only if documented in the page's audit checklist row.

#### Scenario: Content grid covers all four breakpoints

- **WHEN** a page renders a content grid
- **THEN** its class list declares column counts at `sm`, `md`, `lg`, and `xl` (or documents in the audit why a breakpoint is intentionally skipped)
- **AND** a designer reviewing the PR can see the tablet (768–1023px) behavior at a glance

#### Scenario: Filter bar adapts on narrow viewports

- **WHEN** a filter bar overflows its container at `md` or below
- **THEN** overflow filters move into a `Sheet` triggered by a "More filters" button
- **AND** the sheet receives focus for keyboard users

#### Scenario: Workspace side pane collapses on mobile

- **WHEN** a page using `WorkspaceLayout` renders below `md`
- **THEN** the side pane moves into a `Sheet` triggered by a visible button in the header
- **AND** the main pane takes the full viewport width

### Requirement: Refactored pages honor the density, contrast, and motion preferences

The system SHALL ensure any refactor performed against the approved layout templates continues to respond to the `data-density`, `data-contrast`, `data-reduced-motion`, and `data-screen-reader` attributes declared in `app/globals.css`.

#### Scenario: Compact density shrinks section gaps

- **WHEN** `data-density="compact"` is active
- **THEN** the layout templates reduce `--space-section-gap` per the existing token override
- **AND** the page content remains readable without clipping

#### Scenario: High contrast mode strengthens borders

- **WHEN** `data-contrast="high"` is active
- **THEN** template-level borders, dividers, and focus outlines use the high-contrast token set
- **AND** text/background pairs remain at WCAG AA or better

#### Scenario: Reduced motion disables template animations

- **WHEN** `data-reduced-motion="true"` is active
- **THEN** template-owned animations (fade-in header, sheet transitions) fall back to 1ms durations
- **AND** the page remains fully navigable
