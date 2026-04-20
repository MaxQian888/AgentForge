## Context

AgentForge's frontend has accrued 24 dashboard pages spanning project, operations, marketplace, IM, docs, and settings domains. The codebase already ships a credible design system:

- Design tokens in `app/globals.css`: OKLCH palette, density/contrast/reduced-motion/screen-reader data-attributes, fluid typography (`text-fluid-title`, `text-fluid-body`, `text-fluid-metric`, `text-fluid-caption`), spacing tokens (`--space-page-inline`, `--space-section-gap`, `--space-grid-gap`, `--space-card-padding`), status colors.
- Layout templates in `components/layout/templates/`: `OverviewLayout`, `ListLayout`, `SettingsLayout`, `WorkspaceLayout`.
- Shared primitives in `components/shared/`: `PageHeader`, `MetricCard`, `FilterBar`, `EmptyState`, `ErrorBanner`, `ResponsiveGrid`, `ResponsiveTable`, `StatusDot`, `CommandPalette`, skeleton layouts.
- shadcn/ui in `components/ui/`: 30+ components installed and available, mostly Radix-backed.

Despite this, adoption is uneven. Spot checks:

- `app/(dashboard)/memory/page.tsx` uses `flex flex-col gap-6` instead of `gap-[var(--space-section-gap)]` and omits breadcrumbs on the disabled-feature branch.
- `app/(dashboard)/marketplace/page.tsx` rolls its own header/search chrome without `PageHeader` or `FilterBar`, and ships no consistent breadcrumb.
- `app/(dashboard)/cost/page.tsx` and `app/(dashboard)/scheduler/page.tsx` open raw `<Card><CardHeader><CardTitle>…` trios everywhere instead of a single `SectionCard` primitive.
- `app/(dashboard)/projects/page.tsx` uses `PageHeader` + `MetricCard` correctly but still hardcodes `gap-6` on the root and `sm:grid-cols-2 lg:grid-cols-3` without a `md` breakpoint, which makes tablet layouts jump.
- `app/(dashboard)/agents/page.tsx` wraps everything in a bare `space-y-6` with no `PageHeader` and no breadcrumbs at the root (they are set inside `AgentsPageInner` but the visual header lives in `AgentWorkspace`).

Constraints:

- We are in the internal-testing stage per `project_api_stability_stage.md` — breaking visual contracts is permitted if they are presentational only. Store/API signatures must stay stable.
- No behavior regressions: all i18n keys, store calls, URL parameters, keyboard shortcuts, and WebSocket wiring must survive the refactor untouched.
- Tauri drag-region (`[data-tauri-drag-region]`) and `[data-desktop-no-drag="true"]` markers must be preserved where they currently live.
- Change scope is wide (24 pages) so we need a path that keeps PRs reviewable and does not block other feature work.

Stakeholders: frontend contributors, product design, QA (web + desktop), i18n reviewers.

## Goals / Non-Goals

**Goals:**

- Every page under `app/(dashboard)/**` wraps its content in one of the approved layout templates (Overview / List / Settings / Workspace) — and every page declares explicit breakpoint behavior at `sm`, `md`, `lg`, `xl`.
- Every page renders its header chrome through the upgraded `PageHeader` primitive (title, breadcrumbs, description, actions, optional status ribbon, optional sticky) — no hand-rolled `<h1>` + flex row replacements.
- Every metric strip uses `MetricCard` (not raw `<Card>` + `<CardContent>` layouts); every filter/search uses `FilterBar`; every empty/error state uses `EmptyState` / `ErrorBanner`; every loading state uses the shared `skeleton-layouts`.
- Every page respects the design-token spacing scale: `gap-[var(--space-section-gap)]`, `gap-[var(--space-grid-gap)]`, `p-[var(--space-card-padding)]`, `p-[var(--space-page-inline)]`. No raw `gap-6` / `p-6` on page roots.
- Every page is dark-mode parity, high-contrast parity, density-responsive, reduced-motion respectful, and screen-reader navigable.
- Every page is usable at 360px width: header collapses, filters move to overflow, tables degrade to cards, side panels move into `Sheet`/`Drawer`.
- shadcn/ui primitives cover their full intended roles: `Tooltip` on icon-only buttons, `Sheet`/`Drawer` for mobile side flows, `Command` for command surfaces, `Accordion` for collapsible sections, `Resizable` for split views, `ScrollArea` for scroll regions.
- New `SectionCard` and `ResponsiveTabs` primitives exist and are the only recommended path for section framing / tab navigation.

**Non-Goals:**

- No new backend APIs, store contracts, WebSocket messages, or data migrations.
- No new dashboard pages, routes, or domain features.
- No route restructuring (no renames of `app/(dashboard)/*/page.tsx`).
- No changes to login/registration pages beyond sharing primitives if trivially applicable.
- No upgrade of shadcn/ui / Tailwind / React major versions.
- No rework of editor surfaces (`@blocknote/*`) beyond wrapping them in the shared primitives.
- No change to the i18n store contract or locale negotiation.
- No new analytics/telemetry instrumentation.

## Decisions

### D1: Introduce a dedicated `ui-design-consistency` capability rather than overload existing ones

**Decision:** Add `openspec/specs/ui-design-consistency/spec.md` as the single source of truth for "what every AgentForge dashboard page must satisfy." Keep it disjoint from `responsive-layout-system` (which remains about shell/template responsive *behavior*) and `frontend-component-catalog` (which remains about component *documentation*).

**Alternatives considered:**

- *Expand `responsive-layout-system` alone.* Rejected — it is narrowly about shell-and-template responsive behavior; mandating shared-primitive adoption and spacing-token usage stretches its purpose.
- *Expand `frontend-component-catalog` alone.* Rejected — that spec is doc-contract-focused; adding runtime requirements muddies the intent.
- *No new capability, only delta edits.* Rejected — future contributors need one place to look up "is my page compliant?"

**Rationale:** A tight, auditable contract document is easier to enforce than edits scattered across three specs. The modified capabilities still get light deltas so the cross-references stay correct.

### D2: Upgrade shared primitives instead of replacing them

**Decision:** Extend `PageHeader`, `FilterBar`, `MetricCard` with additive optional props; add `SectionCard` and `ResponsiveTabs` as brand-new primitives. Do NOT rename, move, or remove the existing components.

**Alternatives considered:**

- *Rewrite from scratch under a v2 folder.* Rejected — doubles maintenance cost during migration.
- *Inline the additions per page.* Rejected — defeats the consistency goal.

**Rationale:** Additive props let every existing caller keep compiling. New primitives (SectionCard, ResponsiveTabs) address the two gaps we see repeatedly in the audit (raw `<Card>` with header trios; raw `<Tabs>` with no mobile fallback). Zero breakage for untouched call sites.

### D3: Enforce spacing and layout contracts via audit checklist + lint, not runtime validation

**Decision:** Add a reviewable "page audit" table to `docs/guides/frontend-components.md` (or similar) listing each page and its compliance state. Add an ESLint rule (`no-restricted-syntax` / custom) flagging `gap-6` / `p-6` / `space-y-6` on JSX roots. Run the rule over `app/(dashboard)/**` only.

**Alternatives considered:**

- *Runtime design-system validator that yells in dev.* Rejected — runtime cost + false positives; tokens get hashed by Tailwind JIT.
- *Codemod that rewrites all pages in one PR.* Rejected — unreviewable diff, breaks ongoing feature branches.

**Rationale:** Lint is cheap, visible in CI, and catches regressions. Audit checklist gives reviewers a checklist to tick per-PR. Runtime validation is out of proportion.

### D4: Mandatory breakpoint matrix: `sm` (640), `md` (768), `lg` (1024), `xl` (1280)

**Decision:** Every grid/flex layout used on a dashboard page MUST declare breakpoint behavior at all four. If a breakpoint intentionally does not change, the reviewer confirms that explicitly (comment or audit checklist tick).

**Alternatives considered:**

- *Mobile-first without `md`.* Rejected — tablet layouts (768–1023) are where our layouts most often collapse today.
- *Keep existing ad-hoc breakpoints.* Rejected — this is exactly what causes the fragmented feel.

**Rationale:** Four explicit anchors match Tailwind defaults and cover the devices our users actually hold. Forcing `md` declarations prevents the tablet dead zone.

### D5: Phased rollout by domain, not by primitive

**Decision:** Land the upgraded primitives first (phase 1). Then migrate pages by domain in small batches — Workspace (projects, agents, teams, team, agent), Operations (scheduler, memory, reviews, cost), Knowledge (docs, documents, roles, skills), Delivery (workflow, sprints, plugins, im), Marketplace + Settings, Overview + Entry. Each domain ships as one PR.

**Alternatives considered:**

- *Big-bang PR.* Rejected — unreviewable, merge conflicts with feature branches.
- *One page per PR.* Rejected — ~24 PRs dilute review attention and slow velocity.

**Rationale:** Domain batches keep PRs reviewable (3–5 pages each) while preserving coherent visual diffs for the designer to eyeball.

### D6: shadcn/ui coverage targets

**Decision:** Install any missing shadcn primitives up front (`drawer`, `carousel` if needed for mobile flows) in phase 1, then enforce coverage:

- Icon-only buttons → always wrapped in `Tooltip`.
- Mobile panels / confirmations → `Sheet` (side) or `Drawer` (bottom).
- Collapsible config sections → `Accordion` (not hand-rolled toggles).
- Split views > 640px → `ResizablePanelGroup`; stack below.
- Scroll regions inside fixed-height panels → `ScrollArea`.
- Quick-switchers / command surfaces → `Command` via the existing `CommandPalette`.

**Rationale:** Shadcn primitives already exist in the repo; the problem is underuse, not absence. Concrete coverage rules make reviews simple.

### D7: Preserve all existing functionality verbatim

**Decision:** Every refactored page keeps identical store-hook wiring, effect sequences, route-parameter handling, keyboard shortcuts, and WebSocket subscriptions. We only move JSX around and swap primitive implementations. i18n keys stay bound to the same rendered copy unless a primitive renames slots — in which case message keys are moved, not rewritten.

**Rationale:** The mandate is presentational only. Any behavior drift is a separate proposal.

## Risks / Trade-offs

- **[Risk]** A 24-page refactor during active feature work risks merge conflicts with in-flight branches. **Mitigation:** Phased domain rollout (D5); coordinate merge windows with the branch that owns the domain; cherry-pick into feature branches rather than rebasing.
- **[Risk]** Visual regressions slip through because page-level snapshot tests under-cover the UI. **Mitigation:** Playwright smoke run per phase hitting each refactored page; manual design QA checklist attached to each phase PR; breakpoint screenshots at 360/768/1024/1440.
- **[Risk]** `SectionCard` / `ResponsiveTabs` over-abstract and become a pain to use, re-introducing the pattern they replace. **Mitigation:** Ship with two real migrations in the same PR as the primitive itself to validate the API before rolling out broadly.
- **[Risk]** Density / high-contrast / reduced-motion modes are currently under-tested; refactoring exposes gaps. **Mitigation:** Part of the audit explicitly verifies all four data-attribute modes per page, recorded on the audit checklist.
- **[Risk]** i18n copy gets subtly rewritten when we reroute slots. **Mitigation:** Move existing keys rather than define new ones; any copy change flagged to an i18n reviewer.
- **[Trade-off]** Enforcing design-token spacing means reviewers must learn the token names. **Mitigation:** Documented in `docs/guides/frontend-components.md`; lint rule links to the doc.
- **[Trade-off]** Adding breakpoint declarations at `md` increases class-list length. **Mitigation:** Accept the verbosity; tablet coverage matters more than class brevity.
- **[Trade-off]** Additive-only primitive upgrades may leave old call sites on minimal props forever. **Mitigation:** Audit checklist calls out primitive adoption-level per page, giving reviewers a nudge.

## Migration Plan

**Phase 0 — Foundation (no page edits):**

1. Install any missing shadcn primitives (`drawer`, `carousel` if the audit confirms need).
2. Upgrade `PageHeader`, `FilterBar`, `MetricCard` with additive optional props; add `SectionCard`, `ResponsiveTabs`. Ship with unit tests and Storybook-style examples inside `components/shared/**.test.tsx`.
3. Add the ESLint rule banning `gap-6` / `p-6` / `space-y-6` on JSX roots under `app/(dashboard)/**`; seed the audit checklist in `docs/guides/frontend-components.md`.

**Phase 1 — Workspace domain:** refactor `projects`, `project/`, `agents`, `agent`, `teams`, `team/`. One PR.

**Phase 2 — Operations domain:** refactor `scheduler`, `memory`, `reviews`, `cost`. One PR.

**Phase 3 — Knowledge domain:** refactor `docs`, `documents`, `roles`, `skills`. One PR.

**Phase 4 — Delivery & IM:** refactor `workflow`, `sprints`, `plugins`, `im`. One PR.

**Phase 5 — Marketplace & Settings:** refactor `marketplace`, `settings` (using the existing `SettingsShell` as the template anchor). One PR.

**Phase 6 — Entry / overview:** refactor the root `/` dashboard page and any residue. One PR.

**Phase 7 — Lock-in:** enable the lint rule in CI (was warn-only through migration); mark all audit rows as green; close the change by archiving.

**Rollback:** each phase PR is reverted independently — primitives remain backwards-compatible, so reverting page refactors does not break shared components. If the primitives themselves regress, revert phase 0.

## Open Questions

- Does the designer sign off on the four-breakpoint matrix (`sm/md/lg/xl`) or want a fifth (e.g., `2xl`) for ultra-wide desktops? Default: stick with four.
- Do we need a `Drawer` primitive now, or does `Sheet` cover every mobile flow? Default: install only if a concrete mobile flow requires bottom-sheet semantics (marketplace filter panel is a candidate).
- Should `CommandPalette` gain project-aware scopes during this change? Default: **no** — out of scope; deferred to a separate proposal.
- Does the audit checklist live in `docs/guides/frontend-components.md` or a new `docs/guides/page-audit.md`? Default: embed in the existing file to keep discoverability.
