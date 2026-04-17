## Context

The recently completed sprint workspace made `/sprints` usable, but the sprint control plane is still split across inconsistent contracts:

- `src-im-bridge/client/agentforge.go` asks `GET /api/v1/projects/:pid/sprints?status=active` for the current sprint, while `src-go/internal/handler/sprint_handler.go` ignores the `status` query entirely and `src-go/internal/repository/sprint_repo.go` returns every sprint ordered by `start_date DESC`. Current sprint resolution therefore depends on list order instead of a canonical rule.
- `src-go/internal/handler/budget_query_handler.go` and `SprintHandler.Burndown` serve sprint detail by `sid` on non-project-scoped routes, so budget and burndown reads bypass the same explicit project contract used everywhere else in sprint CRUD and metrics.
- `app/(dashboard)/sprints/page.tsx` only seeds from `project` and `action=create-sprint`; it has no explicit `sprint` query contract, so dashboard, roadmap, or future operator surfaces cannot deep-link a selected sprint detail.
- The repository already contains useful seams to reuse, including `SprintRepository.GetActive`, project-group sprint routes, and query-seeded state patterns on `/project`, but they are not composed into one truthful sprint operator contract.

This change is cross-cutting because it touches Go handlers and repositories, Next.js stores and routing, and IM bridge client behavior at the same time.

## Goals / Non-Goals

**Goals:**
- Define one canonical project-scoped sprint read surface for list filtering, current sprint lookup, and sprint detail reads.
- Make current sprint semantics deterministic and operator-visible instead of silently depending on list ordering.
- Enforce a single-active-sprint invariant at the mutation boundary so current sprint lookup stays trustworthy.
- Let `/sprints` open an explicit selected sprint from route input while preserving existing project and action handoff conventions.
- Move first-party IM and web consumers onto the same sprint contract.

**Non-Goals:**
- Adding new sprint analytics, retrospective workflows, or capacity-planning features.
- Designing a new task execution workspace inside `/sprints`.
- Introducing new slash-command families or broadening `/sprint` into full CRUD from IM in this wave.
- Reworking milestone management or project bootstrap beyond the sprint selection handoff they already provide.

## Decisions

### Decision 1: Canonical sprint read interfaces live under project-scoped routes

The canonical operator-facing sprint interfaces will live under project-scoped routes so list, current sprint, metrics, burndown, and budget reads all share the same explicit project contract. The shape will center on:

- `GET /api/v1/projects/:pid/sprints?status=<status>` for filtered list reads
- `GET /api/v1/projects/:pid/sprints/current` for current sprint resolution
- `GET /api/v1/projects/:pid/sprints/:sid/metrics`
- `GET /api/v1/projects/:pid/sprints/:sid/burndown`
- `GET /api/v1/projects/:pid/sprints/:sid/budget`

First-party consumers will migrate to these routes in the same change. Sid-only detail routes are treated as legacy and should not remain the source of truth after migration.

**Why this over keeping mixed route shapes?** Mixed project-scoped and sid-only sprint reads are what created the current drift. Re-centering around one project-scoped surface makes access checks, testing, and consumer wiring all deterministic.

### Decision 2: Current sprint resolution and activation share one active-sprint invariant

Current sprint semantics will be defined as “the single sprint in `active` status for a project.” Activation and current lookup must therefore share the same invariant: a project may not have two active sprints at once.

Implementation-wise, activation checks will inspect active sprints in the target project before accepting a transition to `active`. If another sprint is already active, the mutation returns a conflict response that identifies the existing sprint instead of auto-closing it or silently picking one by timestamp. Current sprint lookup will use the same active-sprint query path and return a clear no-current or conflict outcome rather than falling back to arbitrary ordering.

**Alternatives considered:**
- **Infer current sprint from the first listed row**: rejected because this is the current bug.
- **Auto-close the previous active sprint**: rejected because it mutates another sprint implicitly and hides operator intent.

### Decision 3: `/sprints` uses query-seeded selected sprint state, not URL-locked state

The sprint workspace will accept `?project=<id>&sprint=<sid>` as an initial selection seed. After the project’s sprint list loads, the page will apply the requested sprint once if it belongs to that project; otherwise it will fall back to the current active sprint or the first available sprint. After that seed is applied, manual selection stays local so operators can click between sprints without the URL trapping the workspace in one state.

This mirrors the repo’s existing “seed once, then allow manual overrides” pattern already used on `/project` for sprint filters.

### Decision 4: IM keeps the same command names but switches to the canonical sprint contract

`/sprint status` and `/sprint burndown` will keep their existing user-facing command names, but the bridge client will stop inferring current sprint from an untrusted filtered list response. Instead it will call the canonical current sprint endpoint and then fetch burndown or other detail through the same project-scoped sprint contract used by the web app.

**Why no new IM commands in this change?** The pressing issue is truthfulness, not command breadth. Keeping the surface area stable reduces rollout risk while still fixing the broken contract underneath.

## Risks / Trade-offs

- **[Existing data may already contain multiple active sprints]** → Detect and surface conflict explicitly in current-sprint resolution and activation flows; add targeted tests for zero, one, and many active sprint cases so the system fails loudly instead of guessing.
- **[Route migration can break first-party consumers]** → Update web and IM consumers in the same change, keep route helper and client tests alongside handler tests, and verify there are no remaining first-party calls to sid-only sprint detail routes.
- **[Query-seeded selection can reference stale sprint ids]** → Apply the seed only after project-scoped sprint data loads, then fall back to active or first sprint when the requested sprint is absent.
- **[Conflict responses add another error case to the sprint edit flow]** → Reuse the sprint workspace’s existing inline form-error surface so activation conflicts show up where operators already expect lifecycle validation failures.

## Migration Plan

1. Extend sprint repository and handler contracts to support filtered list queries, current sprint resolution, and project-scoped detail endpoints, plus conflict-aware activation checks.
2. Update Next.js sprint store, route helpers, and `/sprints` page selection logic to consume the canonical sprint contract and accept `sprint` query seeds.
3. Update IM bridge client and `/sprint` commands to use the canonical current/detail routes.
4. Remove or stop using sid-only sprint detail calls from first-party code, then verify the new contract with targeted Go, Jest, and IM bridge tests.

## Open Questions

- None for proposal readiness. If implementation reveals pre-existing multi-active sprint data in seeded environments, the change can add a one-time repair or operator guidance step without reopening the contract.
