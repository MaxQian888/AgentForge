## Context

AgentForge already has most of the raw seams needed for sprint management, but they are split across multiple surfaces and do not yet behave like one trustworthy planning workspace:

- `app/(dashboard)/sprints/page.tsx` already provides a dedicated sprint page with project-scoped loading, sprint cards, create/edit dialogs, milestone selection UI, and a burndown panel.
- `app/(dashboard)/project/page.tsx` and the shared task workspace already understand sprint filters and sprint metrics, which makes the project task workspace the natural execution plane for sprint work.
- `src-go/internal/handler/sprint_handler.go` and `lib/stores/sprint-store.ts` already cover sprint CRUD and metrics, while `src-go/internal/handler/budget_query_handler.go` already exposes sprint budget detail.
- Existing realtime wiring in `lib/stores/ws-store.ts` already upserts sprint records on `sprint.updated` and `sprint.transitioned`.

The current gaps are not about missing a brand-new subsystem. They are about broken continuity and contract drift:

- `/sprints` still behaves like a lightweight card page instead of a control plane that helps operators inspect sprint health and continue into execution.
- Sprint create and update flows drift across frontend and Go contracts for date values and milestone persistence.
- Budget detail exists on the backend but is not surfaced in the sprint workspace.
- Sprint-to-task handoff still depends on ambient state instead of an explicit route-scoped sprint context.

This design keeps the scope focused on the existing planning seam. It does not reopen milestone, budget, or task workspace architecture beyond what is required to make sprint management truthful and connected.

## Goals / Non-Goals

**Goals:**

- Make `/sprints` the canonical planning control plane for project-scoped sprint management.
- Keep `/project` as the execution workspace, and add an explicit handoff from a selected sprint into that existing task workspace.
- Make sprint create and edit flows round-trip truthful date, milestone, budget, and status data through the existing persisted sprint model.
- Surface selected sprint metrics and budget detail in one operator-facing detail surface without creating duplicate sprint APIs.
- Preserve compatibility with existing project-scoped handoff conventions such as `?project=` and `?action=create-sprint`.

**Non-Goals:**

- Do not rebuild the project task workspace inside `/sprints`.
- Do not introduce a new sprint-specific task list or backlog API when existing task workspace and budget seams are sufficient.
- Do not define a new capacity-planning system, story-point model, or multi-sprint forecasting feature.
- Do not change core sprint lifecycle rules beyond the existing persisted status model and transition validation.
- Do not absorb the broader lifecycle continuity work already being handled by `complete-project-bootstrap-and-handoff`.

## Decisions

### Decision 1: `/sprints` is the planning control plane, `/project` remains the execution plane

The sprint workspace will own sprint selection, sprint CRUD, selected sprint health, budget visibility, and navigation actions. The project task workspace will continue to own Board/List/Timeline/Calendar execution work, bulk task operations, and task-level planning changes.

This keeps one clear boundary:

- `/sprints`: inspect and manage the sprint itself
- `/project`: execute and manage tasks within that sprint

**Rationale:** The repository already has a mature shared task workspace and sprint-aware filtering there. Duplicating that execution surface inside `/sprints` would create a second task workspace with diverging behavior.

**Alternative considered:** Expand `/sprints` into a second task board with sprint-specific backlog and execution panels.  
**Rejected because:** it would duplicate `task-multi-view-board`, split task interactions across two control planes, and enlarge the change far beyond the approved sprint seam.

### Decision 2: The selected sprint detail surface will compose existing metrics and budget endpoints instead of introducing a new aggregate backend contract

When an operator selects a sprint in `/sprints`, the page will load:

- sprint metrics from the existing sprint metrics endpoint
- sprint budget detail from the existing sprint budget endpoint

The detail surface will present these as one operator-facing panel: burndown and headline metrics, budget threshold status, and per-task budget rows. This gives the workspace meaningful depth without inventing a second sprint summary service.

**Rationale:** The backend already has authoritative seams for both metrics and budget detail. The gap is consumption and presentation, not backend resource discovery.

**Alternative considered:** Add a new `GET /api/v1/projects/:pid/sprints/:sid/summary` endpoint that merges metrics and budget detail.  
**Rejected because:** it duplicates existing backend logic, expands the API surface, and increases contract maintenance for little operator benefit.

### Decision 3: Sprint forms keep calendar-date UX but normalize to one canonical API contract at the UI boundary

The sprint workspace will continue to use date inputs for human editing, but before submission it will normalize the entered calendar dates into the canonical sprint API contract expected by the Go backend. Create and update flows will use the same normalization path, and the page will rehydrate forms from persisted sprint values so reopening a sprint shows the same entered dates.

This change also makes milestone association a first-class part of the same sprint form contract rather than a half-connected UI field. Create and update flows will both support optional milestone assignment using the existing sprint persistence model.

**Rationale:** The repository already treats the persisted sprint model as authoritative. The drift comes from frontend form handling, not from the core data model.

**Alternative considered:** Teach the backend to accept both RFC3339 values and ad hoc `YYYY-MM-DD` input.  
**Rejected because:** dual parsing hides drift instead of removing it, increases ambiguity around validation, and makes clients less predictable.

### Decision 4: Sprint-to-execution handoff uses explicit route-scoped sprint context

The sprint workspace will provide an execution action that opens the existing `/project` route with explicit project and sprint context. The route contract will stay consistent with current repo conventions:

- `/sprints` continues consuming `?project=` and `?action=...`
- `/project` continues consuming `?id=...` for project scope
- sprint handoff adds an explicit `?sprint=<sid>` input for initial task workspace scope

`app/(dashboard)/project/page.tsx` will read that sprint input once when resolving initial workspace state, validate that the sprint belongs to the active project, and seed the shared sprint filter accordingly.

**Rationale:** This matches the project-scoped handoff direction already emerging elsewhere in the repo and removes dependence on ambient store state for sprint continuation.

**Alternative considered:** Continue relying on `useTaskWorkspaceStore` state mutation before navigation.  
**Rejected because:** that approach is fragile across refresh, direct links, and future bootstrap or dashboard entry flows.

### Decision 5: Realtime sprint updates refresh both the list state and the selected detail truth

Existing websocket upserts already keep sprint card data reasonably fresh, so this design will reuse them for list-level state. When the currently selected sprint receives an update or transition event, the workspace will also refresh selected metrics and budget detail so the operator does not end up with stale burndown or budget breakdown while the summary card has already changed.

**Rationale:** Without this extra refresh step, selected sprint detail would lag behind list state because only the sprint summary record is pushed through websocket events today.

**Alternative considered:** Let realtime update only the sprint list and require manual page refresh for selected detail.  
**Rejected because:** it would preserve inconsistent state inside the very control plane this change is trying to make truthful.

## Risks / Trade-offs

- [Risk] This change overlaps adjacent lifecycle continuity work around project-scoped handoffs.  
  Mitigation: keep the sprint change limited to `/sprints` and `/project` seam compatibility, and preserve existing `?project=` plus `?action=create-sprint` conventions rather than inventing a broader routing redesign.

- [Risk] Selected sprint detail now depends on multiple asynchronous reads, which can produce partial loading or stale subpanels.  
  Mitigation: treat metrics and budget detail as separate loading states inside one detail surface, and refetch both when the selected sprint changes or receives realtime updates.

- [Risk] Extending sprint create or update persistence for milestone association can expose latent backend validation gaps.  
  Mitigation: keep milestone assignment optional, validate ownership within the same project scope, and fall back to inline error handling instead of optimistic silent failure.

- [Risk] Adding explicit `?sprint=` handoff to `/project` could conflict with saved workspace state or stale filter state.  
  Mitigation: use explicit sprint handoff only as initial scope seeding, validate the sprint against the active project's sprint list, and ignore invalid values rather than corrupting workspace state.

- [Risk] Operators may expect `/sprints` to become a second full task workspace.  
  Mitigation: make the detail surface rich enough for planning and inspection, but use clear “open sprint tasks” handoff actions instead of embedding duplicate task interaction models.

## Migration Plan

1. Define the sprint management workspace capability and the task workspace handoff delta in OpenSpec.
2. Extend the sprint client/store layer so selected sprint detail can load metrics and budget detail, and forms can normalize date values consistently before create or update.
3. Align Go sprint create and update handling with the sprint workspace form contract, including optional milestone persistence and existing status transition validation.
4. Add explicit sprint-scoped handoff into `/project` and seed the shared sprint filter from route input while preserving current project-scoped route conventions.
5. Reuse existing websocket sprint events to refresh list state, then refetch selected detail state for the active sprint when updates arrive.
6. If rollout reveals instability, keep the contract fixes and selected sprint summary, and temporarily narrow the execution handoff or budget detail presentation without reverting the truthful persistence changes.

## Open Questions

- Should the sprint workspace preserve the last manually selected sprint across refreshes, or continue using the current active-sprint-first fallback whenever no explicit sprint is requested?
- Should the execution handoff open the existing task workspace with its current saved view mode, or should sprint-scoped handoff force a specific default such as Board or List for consistency?
