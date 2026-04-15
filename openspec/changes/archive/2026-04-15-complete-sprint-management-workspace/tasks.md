## 1. Align sprint contracts and shared state

- [x] 1.1 Extend the sprint backend contract and tests so create and update flows can round-trip optional milestone assignment, canonical date values, and existing status transition validation without silent drift.
- [x] 1.2 Update the sprint client/store layer to normalize sprint form dates before submission and to track sprint budget detail, loading state, and error state alongside existing sprint metrics.
- [x] 1.3 Add shared refetch helpers or invalidation flow so selected sprint metrics and budget detail can be refreshed after sprint saves and realtime sprint events.

## 2. Complete the `/sprints` planning workspace

- [x] 2.1 Update `app/(dashboard)/sprints/page.tsx` create and edit flows to use the truthful sprint contract, preserve inline form errors, and keep explicit `project` plus `action=create-sprint` handoff behavior.
- [x] 2.2 Add a selected sprint detail surface on `/sprints` that shows sprint metrics, budget threshold state, per-task budget breakdown, and clear empty or unconfigured states.
- [x] 2.3 Add operator actions from the selected sprint detail surface for opening sprint-scoped execution work in the existing project task workspace.

## 3. Wire explicit sprint-to-execution handoff

- [x] 3.1 Add route-building support for navigating from `/sprints` into `/project` with explicit project and sprint scope.
- [x] 3.2 Update `app/(dashboard)/project/page.tsx` to read the sprint handoff input, validate it against the active project's sprint list, and seed the shared sprint filter and metrics resolution path.
- [x] 3.3 Ensure invalid or stale sprint handoff input falls back cleanly to the normal project task workspace state instead of leaving the workspace in a broken or mismatched filter state.

## 4. Verify workspace truthfulness

- [x] 4.1 Add or update targeted frontend tests for the sprint workspace and project workspace handoff covering explicit project scope, creation handoff, selected sprint detail, and valid or invalid sprint handoff behavior.
- [x] 4.2 Add or update targeted Go tests for sprint handler and related budget or milestone seams covering milestone persistence, date contract alignment, and invalid sprint update handling.
- [x] 4.3 Run focused frontend and backend verification for sprint workspace flows and record any remaining gaps before moving to `/opsx:apply`.
