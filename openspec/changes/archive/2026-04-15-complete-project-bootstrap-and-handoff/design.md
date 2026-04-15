## Context

AgentForge already exposes most of the product surfaces needed for project management:

- `/projects` can create and list projects.
- `/settings` manages project identity, runtime defaults, budget governance, and review policy.
- `/team` and `/teams` manage members and team runs.
- `/docs` and the wiki editor expose project-scoped docs and document templates.
- `/workflow` exposes workflow config, definitions, executions, reviews, and workflow templates.
- `/project`, `/sprints`, `/reviews`, and `/project/dashboard` cover planning, delivery, and visibility.

The missing piece is not another isolated workspace. The missing piece is lifecycle continuity.

- Project creation only collects a small amount of metadata and does not hand the operator into a guided next step.
- Several pages still depend on `selectedProjectId` and fall back to empty or no-project states even though the user may have arrived from a specific project flow.
- The root dashboard can show operational insights, but it does not yet behave like a project bootstrap surface for incomplete projects.
- Existing active changes already cover template management and task-triggered workflow orchestration. This change should connect those surfaces, not absorb their implementation scope.

## Goals / Non-Goals

**Goals:**

- Define a truthful project bootstrap summary that shows which parts of the project-management lifecycle still need operator action.
- Make project creation and project-entry flows preserve explicit project context and hand operators into the next useful management surface.
- Provide consistent project-scoped handoff actions into settings, team, docs and templates, workflows, planning, and visibility surfaces.
- Let dashboard insights act as a lifecycle-aware entry point for incomplete projects without turning the dashboard into a second settings or task page.

**Non-Goals:**

- Do not create a brand-new generic PM engine or replace the existing task, team, docs, workflow, or dashboard workspaces.
- Do not auto-create fake tasks, teams, workflows, or dashboards just to mark setup as complete.
- Do not reopen template-management or task-triggered workflow-orchestration details already captured by the active sibling changes.
- Do not redesign every route shape in one pass; route-context migration should stay backward-compatible where the current repo already has established links.

## Decisions

### Decision 1: Bootstrap state is a derived project summary, not a persisted checklist

This change will model project bootstrap as a project-scoped derived summary rather than a new persisted wizard state. The summary should evaluate the lifecycle using current repo truth from the existing surfaces, not a separate table of manually checked boxes.

The summary should cover at least:

- governance readiness
- team readiness
- reusable playbook readiness
- planning readiness
- delivery handoff readiness

Each phase should expose:

- a status
- a short explanation or blocking reason
- the next recommended action

This keeps the bootstrap model truthful even when a project is created outside the "happy path" or when an operator partially configures the project over time.

**Alternative considered:** Add a new persisted setup wizard record with manually completed checkboxes.  
**Rejected because:** it would drift from the actual project state and create a second source of truth for setup.

### Decision 2: Project-scoped handoffs use explicit route context instead of ambient selection only

The current workspace family mixes explicit and ambient scope:

- `/project` already consumes a query-scoped project identifier
- `/team` accepts `?project=...`
- `/settings`, `/workflow`, `/docs`, and `/project/dashboard` still rely heavily on `selectedProjectId`

The bootstrap and handoff contract should standardize around explicit project-scoped handoffs. Destinations may continue to accept current legacy query keys during migration, but the new lifecycle contract should treat explicit project context as canonical whenever it is present.

In practice, handoff actions should be able to carry:

- the project identity
- the destination workspace
- an optional focus intent such as a target section, tab, filter, or create action

**Alternative considered:** Keep bootstrap as a purely client-side helper that only mutates `selectedProjectId` and hopes destination pages pick it up.  
**Rejected because:** it is exactly the current failure mode that produces scope loss and no-project placeholders.

### Decision 3: Bootstrap entry stays inside existing project-entry surfaces

This change should not introduce a new orphan top-level route just to show setup phases. Instead, the bootstrap summary should live in the surfaces where operators already enter project management:

- immediately after creating or reopening a project from `/projects`
- from lifecycle-aware cards or empty states on the root dashboard

The bootstrap summary then hands off into the existing workspaces that actually perform the work.

This preserves the current information architecture and keeps the change focused on continuity rather than route sprawl.

**Alternative considered:** Create a dedicated `/project/bootstrap` workspace as the only supported setup entry.  
**Rejected because:** it would add another management surface instead of making the current ones work together.

### Decision 4: Handoff destinations must consume focus intent directly

Scope-preserving links are not enough if the destination still lands on a generic default tab. The lifecycle contract should let handoff actions target a specific intent, for example:

- open settings on governance or runtime diagnostics
- open team management in an attention or setup-required view
- open workflow on templates or config
- open docs in template mode
- open planning surfaces in create-sprint or create-task mode

The destination surface should consume that intent directly instead of requiring the operator to rediscover the missing step manually.

**Alternative considered:** Only preserve project scope and leave destination focus implicit.  
**Rejected because:** it still forces operators to re-navigate and breaks the "guided lifecycle" goal.

### Decision 5: Bootstrap completion must stay non-destructive and truthful

The lifecycle summary can recommend actions and detect missing setup, but it should never manufacture fake completeness. If a project has no team members, no plan, or no runnable delivery surface, that should remain visible. Built-in seeded assets such as wiki templates or global workflow starters can count as available baselines only when they actually exist, but they must not imply that the project's custom setup work is already complete.

This matters because the user explicitly wants full-flow management without omission or simplification. The bootstrap summary should reveal missing steps, not hide them behind optimistic defaults.

**Alternative considered:** Auto-create default dashboards, sample tasks, or placeholder teams to make new projects look complete immediately.  
**Rejected because:** it would produce noisy data and false lifecycle signals.

## Risks / Trade-offs

- [Risk] Derived readiness rules become too heuristic or surprising. -> Mitigation: keep phases small, explain each status with visible reasons, and avoid hidden scoring systems.
- [Risk] Explicit handoff context could conflict with legacy route parameters and stored selection. -> Mitigation: treat explicit project context as authoritative and keep legacy aliases during migration.
- [Risk] Touching multiple destination surfaces can expand the change. -> Mitigation: limit destination work to scope consumption and intent handling, not full workspace redesign.
- [Risk] Dashboard entry could become overloaded with setup concerns. -> Mitigation: keep dashboard guidance lightweight and route operators quickly into the specialized workspace for each action.
- [Risk] Built-in templates or starters may make playbook readiness look complete too early. -> Mitigation: distinguish between baseline availability and operator-ready project setup when explaining the phase state.

## Migration Plan

1. Define the bootstrap and handoff contract in OpenSpec and align it with the existing dashboard-insights capability.
2. Introduce a derived bootstrap summary shape and route-context helpers on top of current project, settings, member, docs, workflow, and planning data.
3. Wire `/projects` and root dashboard entry surfaces to the bootstrap summary and project-scoped handoff actions.
4. Update destination workspaces to consume explicit project and focus context before relying on ambient `selectedProjectId`.
5. If rollout causes navigation regressions, keep the bootstrap summary visible but temporarily narrow handoff actions to the already-stable destinations while preserving the derived lifecycle status model.

## Open Questions

- Should a brand-new project land back on `/projects` with an inline bootstrap panel, or should creation deep-link straight into the highest-priority next surface?
- Should reusable playbook readiness treat built-in workflow starters and seeded document templates as "ready", or only as baseline availability that still requires operator confirmation?
- Which destination intents need first-class query support in the initial iteration, and which can remain lightweight deep-link conventions?
