## Why

The dashboard surface has grown to 24 pages across workspace, operations, marketplace, and settings domains. While strong design primitives already exist (`PageHeader`, `MetricCard`, `FilterBar`, `EmptyState`, `OverviewLayout`, `ListLayout`, `SettingsLayout`, `WorkspaceLayout`, density/contrast tokens in `globals.css`), adoption is uneven: some pages still hand-roll `flex flex-col gap-6` layouts, skip breadcrumbs, render raw `<Card>` grids instead of `MetricCard`, and ship custom filter/header chrome instead of the shared `PageHeader`/`FilterBar` components. Several pages have no mobile-first breakpoints for their grids, and a handful use hardcoded `gap-6` or `p-6` instead of the design-token spacing variables. The result is a visually fragmented product that undersells shadcn/ui — pages look like they were built by different teams, and the experience degrades on narrow viewports. We are in the internal-testing window (breaking changes are free) and this is the right moment to enforce a cohesive design contract before the UI freezes.

## What Changes

- Introduce a `ui-design-consistency` capability that codifies the mandatory design contract every dashboard page must satisfy: layout-template usage, shared-primitive adoption, shadcn/ui component coverage, design-token spacing/typography, and responsive behavior.
- Audit every page under `app/(dashboard)/**` against the new contract and refactor non-compliant pages to use `OverviewLayout` / `ListLayout` / `SettingsLayout` / `WorkspaceLayout` (or the upgraded primitives below) instead of ad-hoc wrappers. High-traffic gaps identified: `marketplace`, `memory`, `settings`, `scheduler`, `cost`, `docs`, `documents`, `im`, `reviews`, `sprints`, `plugins`, `teams`, `agent`.
- Upgrade the shared primitives so every page has a natural migration target:
  - `PageHeader` gains sticky-on-scroll behavior by default on mobile, slot-based `filters` area, and optional `status` ribbon.
  - `FilterBar` gains responsive collapse (chips + overflow sheet on mobile), saved-view integration, and matches `PageHeader` spacing tokens.
  - `MetricCard` gains skeleton, compact, and trend-only variants so raw `<Card>` grids can retire.
  - New `SectionCard` primitive (shadcn `Card` wrapper) with consistent title/description/actions slots, replacing the copy-pasted `CardHeader`+`CardTitle`+`CardDescription` blocks on `cost`, `scheduler`, `memory`, `settings`.
  - New `ResponsiveTabs` primitive wrapping shadcn `Tabs` that degrades to a `Select` or `Sheet` on small viewports.
- Establish shadcn/ui coverage expectations: pages SHOULD prefer `Sheet` for mobile panels, `Drawer` (new install) for slide-up flows, `Command` for quick-switchers, `Accordion` for collapsible sections, `Resizable` for split views, `Tooltip` on all icon-only buttons.
- Normalize spacing: **BREAKING** — no raw `gap-6`, `p-6`, `space-y-6` on page roots; use `gap-[var(--space-section-gap)]` / `p-[var(--space-page-inline)]` / `gap-[var(--space-grid-gap)]` tokens. Lint rule or audit checklist enforces.
- Ensure every page ships dark-mode parity, high-contrast parity, density-aware padding, reduced-motion respect, and breakpoint coverage at `sm` (≥640), `md` (≥768), `lg` (≥1024), `xl` (≥1280).
- Preserve all existing functionality, store wiring, routes, analytics, and i18n keys — this change is purely presentational/structural. Any data-flow or behavior change is out of scope and must be a separate proposal.

## Capabilities

### New Capabilities

- `ui-design-consistency`: Defines the mandatory design contract every dashboard page must satisfy — layout template usage, shared-primitive coverage, shadcn/ui adoption, spacing/typography tokens, responsive breakpoints, accessibility parity, and audit/enforcement guarantees. Becomes the single source of truth for "what a well-built AgentForge page looks like."

### Modified Capabilities

- `responsive-layout-system`: Extend requirements so that every dashboard page under `app/(dashboard)/**` MUST wrap its content in one of the four approved layout templates (or a newly approved template) and MUST declare breakpoint behavior at `sm`/`md`/`lg`/`xl`. Today the spec describes shell-level responsiveness but does not bind individual pages to the templates.
- `frontend-component-catalog`: Extend the documentation contract to enumerate the upgraded primitives (`SectionCard`, `ResponsiveTabs`, upgraded `FilterBar`/`PageHeader`/`MetricCard`) and record the "use this, not a raw shadcn Card" guidance so contributors are directed to the shared primitives first.

## Impact

- **Affected code**: all 24 files under `app/(dashboard)/**/page.tsx`, their owned section components under `components/<domain>/`, the shared primitives in `components/shared/` and `components/layout/templates/`, plus any Storybook / docs entries in `docs/guides/frontend-components.md`.
- **APIs**: no backend API changes. No store signature changes expected. Props for `PageHeader`, `FilterBar`, `MetricCard` gain additive optional fields — existing callers keep compiling.
- **Dependencies**: may add `@/components/ui/drawer` and `@/components/ui/carousel` from shadcn if needed for the upgraded mobile/sheet flows; no new npm packages beyond those shadcn installers generate. `tw-animate-css` is already present.
- **Tests**: Jest tests for `PageHeader`, `MetricCard`, `FilterBar`, and the layout templates need updated snapshots plus new cases for the new variants; page-level smoke tests should be reviewed so breadcrumb/title assertions still match.
- **i18n**: reuse of existing keys wherever possible. Any new copy (e.g., "more filters" overflow label) adds entries to the relevant namespaces in `lib/i18n/messages/`.
- **Accessibility**: audit confirms every refactored page honors `data-density`, `data-contrast`, `data-reduced-motion`, and `data-screen-reader` preferences that `globals.css` already defines.
- **Desktop (Tauri)**: no changes to `src-tauri/`; drag-region and `data-desktop-no-drag` semantics are preserved by the shared header chrome.
- **Risk**: moderate — the change touches many files but each refactor is structural, behind the same stores and routes, so regression surface is visual-only. Phased per-domain rollout keeps PRs reviewable.
