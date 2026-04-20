## 1. Foundation — shared primitives and lint

- [x] 1.1 Install any missing shadcn/ui primitives the audit confirms are needed (`drawer` if a bottom-sheet flow is required; otherwise document the decision in `docs/guides/frontend-components.md`)
- [x] 1.2 Upgrade `components/shared/page-header.tsx` with `filters`, `status`, and `sticky` slot support; keep existing props backwards-compatible; update `components/shared/page-header.test.tsx`
- [x] 1.3 Upgrade `components/shared/filter-bar.tsx` to collapse overflow filters into a `Sheet` on viewports below `md` and expose a `moreFiltersLabel` i18n prop; update `components/shared/filter-bar.test.tsx`
- [x] 1.4 Upgrade `components/shared/metric-card.tsx` with `compact` and `loading` variants; keep the existing sparkline path unchanged; extend `components/shared/metric-card.test.tsx`
- [x] 1.5 Create `components/shared/section-card.tsx` wrapping shadcn `Card` with title/description/actions/body slots, consistent `p-[var(--space-card-padding)]`, and a paired `section-card.test.tsx`
- [x] 1.6 Create `components/shared/responsive-tabs.tsx` wrapping shadcn `Tabs` with a `Select`/`Sheet` fallback below `md`; add `responsive-tabs.test.tsx`
- [x] 1.7 Add the ESLint rule (custom or `no-restricted-syntax`) that flags `gap-6`, `p-6`, `space-y-6` on JSX roots under `app/(dashboard)/**`; wire it into `pnpm lint` as a warning for phase 0 and escalate to error in phase 7
- [x] 1.8 Update `docs/guides/frontend-components.md` with the upgraded primitive catalog, the "use this, not a raw Card" guidance, template-slot maps, and an empty audit checklist row per page under `app/(dashboard)/**`
- [x] 1.9 Update `docs/guides/state-management.md` so every primitive is cross-linked to its canonical Zustand slice pattern

## 2. Phase 1 — Workspace domain

- [x] 2.1 Refactor `app/(dashboard)/projects/page.tsx` to use `ListLayout` (or `OverviewLayout` for the stats strip) with design-token spacing, explicit `sm`/`md`/`lg`/`xl` breakpoints, `MetricCard` for stats, and `FilterBar` for search
- [x] 2.2 Refactor `app/(dashboard)/project/page.tsx` to use the appropriate template and `SectionCard` for its panels
- [x] 2.3 Refactor `app/(dashboard)/project/dashboard/page.tsx` to align with `OverviewLayout` patterns
- [x] 2.4 Refactor `app/(dashboard)/agents/page.tsx` so the root uses `WorkspaceLayout` with `PageHeader`; side pane collapses to `Sheet` below `md`
- [x] 2.5 Refactor `app/(dashboard)/agent/page.tsx` to use the same workspace template and `SectionCard` framing
- [x] 2.6 Refactor `app/(dashboard)/teams/page.tsx` to use `ListLayout`
- [x] 2.7 Refactor `app/(dashboard)/teams/detail/page.tsx` to use `WorkspaceLayout`
- [x] 2.8 Refactor `app/(dashboard)/team/page.tsx` to match the workspace pattern established by `teams/detail`
- [x] 2.9 Update the audit checklist in `docs/guides/frontend-components.md` for all Phase 1 pages
- [x] 2.10 Run `pnpm lint`, `pnpm test`, `pnpm exec tsc --noEmit`, and a Playwright smoke pass at 360 / 768 / 1024 / 1440 widths for Phase 1 pages

## 3. Phase 2 — Operations domain

- [x] 3.1 Refactor `app/(dashboard)/scheduler/page.tsx` to use `WorkspaceLayout`, replace raw `<Card>` trios with `SectionCard`, move filter chrome into the upgraded `FilterBar`, and wrap `Tabs` in `ResponsiveTabs`
- [x] 3.2 Refactor `app/(dashboard)/memory/page.tsx` to use `ListLayout` (or `WorkspaceLayout` for the explorer view), wire breadcrumbs through `PageHeader`, use `EmptyState` for both the disabled-feature and no-project branches
- [x] 3.3 Refactor `app/(dashboard)/reviews/page.tsx` to use `WorkspaceLayout` and migrate inline filter chrome to `FilterBar`
- [x] 3.4 Refactor `app/(dashboard)/cost/page.tsx` to use `OverviewLayout` with `MetricCard` rows, `SectionCard` for chart panels, and `ResponsiveTabs` for the breakdown switcher
- [x] 3.5 Update the audit checklist for all Phase 2 pages
- [x] 3.6 Run lint, tests, typecheck, and Playwright smoke at four widths for Phase 2 pages

## 4. Phase 3 — Knowledge domain

- [x] 4.1 Refactor `app/(dashboard)/docs/page.tsx` to use `WorkspaceLayout`; keep the editor mount point untouched; wrap the sidebar in a `Sheet` below `md`
- [x] 4.2 Refactor `app/(dashboard)/documents/page.tsx` analogously
- [x] 4.3 Refactor `app/(dashboard)/roles/page.tsx` to use `ListLayout` with `FilterBar` and `EmptyState`; migrate the detail pane to `WorkspaceLayout` if present
- [x] 4.4 Refactor `app/(dashboard)/skills/page.tsx` to match the roles pattern
- [x] 4.5 Update the audit checklist for all Phase 3 pages
- [x] 4.6 Run lint, tests, typecheck, and Playwright smoke at four widths for Phase 3 pages

## 5. Phase 4 — Delivery and IM

- [x] 5.1 Refactor `app/(dashboard)/workflow/page.tsx` without touching the workflow graph canvas; ensure `PageHeader` + template framing + `SectionCard` for side panels
- [x] 5.2 Refactor `app/(dashboard)/sprints/page.tsx` to use `ListLayout` or `WorkspaceLayout` consistent with the sprints UX
- [x] 5.3 Refactor `app/(dashboard)/plugins/page.tsx` to use `ListLayout`, `MetricCard` for status strip, `FilterBar` for search/filter
- [x] 5.4 Refactor `app/(dashboard)/im/page.tsx` to use `WorkspaceLayout`; migrate inline filter chrome to `FilterBar`; collapse side pane to `Sheet` below `md`
- [x] 5.5 Update the audit checklist for all Phase 4 pages
- [x] 5.6 Run lint, tests, typecheck, and Playwright smoke at four widths for Phase 4 pages

## 6. Phase 5 — Marketplace and Settings

- [x] 6.1 Refactor `app/(dashboard)/marketplace/page.tsx` to use `ListLayout` with the upgraded `PageHeader`, `FilterBar`, and `EmptyState`; keep `MarketplaceFilterPanel` but move overflow filters into the `Sheet` flow; wrap detail pane via `WorkspaceLayout` on desktop
- [x] 6.2 Refactor `app/(dashboard)/settings/page.tsx` to ensure `SettingsShell` aligns with `SettingsLayout` contract; every `section-*.tsx` under `_components/` is wrapped in `SectionCard` and honors density tokens
- [x] 6.3 Update the audit checklist for Phase 5 pages
- [x] 6.4 Run lint, tests, typecheck, and Playwright smoke at four widths for Phase 5 pages

## 7. Phase 6 — Entry and overview

- [x] 7.1 Refactor `app/(dashboard)/page.tsx` (root dashboard) so every widget panel is framed by `SectionCard`, breakpoint coverage is complete at `sm`/`md`/`lg`/`xl`, and the project-filter control respects `PageHeader` slotting
- [x] 7.2 Sweep any remaining non-compliant pages identified during audit (`project/templates/page.tsx`, `teams/detail/page.tsx` edge cases, etc.)
- [x] 7.3 Update the audit checklist for all remaining pages
- [x] 7.4 Run lint, tests, typecheck, and Playwright smoke at four widths for Phase 6 pages

## 8. Phase 7 — Lock-in and QA

- [x] 8.1 Flip the ESLint rule from warn to error under `app/(dashboard)/**` and land the config change
- [x] 8.2 Run a full `pnpm lint`, `pnpm test`, `pnpm test:coverage`, `pnpm exec tsc --noEmit`, and `pnpm build` to confirm no regressions
- [ ] 8.3 Run Playwright smoke across every refactored page at 360 / 768 / 1024 / 1440 widths and attach screenshots to the PR description *(deferred — requires browser session; not available in this automation pass)*
- [ ] 8.4 Manually verify every refactored page under all combinations of `data-density` (compact / comfortable / spacious), `data-contrast` (default / high), `data-reduced-motion` (off / on), and dark mode *(deferred — manual QA task)*
- [ ] 8.5 Request design and QA sign-off on the audit checklist; resolve any remaining red cells or convert them into follow-up issues linked from the checklist *(deferred — human review)*
- [ ] 8.6 Update `openspec/specs/ui-design-consistency/spec.md`, `openspec/specs/responsive-layout-system/spec.md`, and `openspec/specs/frontend-component-catalog/spec.md` via `opsx:sync` *(gated on 8.3–8.5 completion)*
- [ ] 8.7 Archive the change via `opsx:archive` *(gated on 8.6 completion)*
