## 1. Dashboard Workspace Shell And State

- [x] 1.1 Extend `lib/stores/dashboard-store.ts` with project-dashboard workspace state for dashboard loading/errors, active dashboard selection, widget request state, and dashboard/widget mutation feedback while preserving existing dashboard summary behavior.
- [x] 1.2 Rework `app/(dashboard)/project/dashboard/page.tsx` so the project dashboard route supports empty/loading/error states, query-backed active dashboard selection, and create/rename/delete flows instead of always rendering `dashboards[0]`.
- [x] 1.3 Add or update supporting workspace management UI for dashboard selection and metadata actions, including the fallback behavior after creating or deleting the active dashboard.

## 2. Widget Layout And Lifecycle Operations

- [x] 2.1 Refactor `components/dashboard/dashboard-grid.tsx` to render a real dashboard workspace that consumes persisted widget position/layout data, keeps a local layout draft, and surfaces save-in-progress or save-failed feedback.
- [x] 2.2 Expand `components/dashboard/widget-wrapper.tsx` into a unified widget chrome with refresh, configure, remove, empty-state, and retryable error handling for each widget card.
- [x] 2.3 Replace the raw widget picker in `components/dashboard/add-widget-dialog.tsx` with a catalog that shows widget metadata and default configuration, and add a matching configuration surface for existing widgets using the current save widget contract.
- [x] 2.4 Wire widget add, update, refresh, and delete flows so successful mutations update the active dashboard immediately and failed mutations do not leave stale widget controls or phantom layout slots.

## 3. Verification And Regression Coverage

- [x] 3.1 Add or update focused Jest coverage for `app/(dashboard)/project/dashboard/page.tsx`, `components/dashboard/dashboard-grid.tsx`, `components/dashboard/add-widget-dialog.tsx`, `components/dashboard/widget-wrapper.tsx`, and `lib/stores/dashboard-store.ts` to cover workspace selection, layout persistence feedback, widget lifecycle actions, and local failure states.
- [x] 3.2 Run the targeted dashboard workspace verification commands and fix regressions in touched files before closing the change.
