## Why

AgentForge now has a usable sprint planning workspace, but the underlying sprint control-plane interfaces still drift across web, backend, and IM consumers. Current sprint resolution depends on list order instead of a canonical contract, `status=active` is requested by clients but ignored by the backend list handler, and sprint detail reads such as burndown and budget still bypass project-scoped route semantics.

## What Changes

- Add a canonical sprint control-plane interface for project-scoped sprint reads, including filtered list queries, current sprint resolution, and scoped sprint detail contracts for metrics, burndown, and budget data.
- Guard sprint activation so each project has one truthful current sprint at a time instead of letting web or IM surfaces silently infer the current sprint from whichever record happens to be first.
- Extend the `/sprints` workspace so operators can open a specific sprint explicitly and keep selected sprint detail aligned with the canonical current/detail interfaces.
- Update existing IM and client consumers to use the truthful sprint interface contract instead of relying on list-order assumptions or sid-only sprint detail routes.
- Deprecate legacy sid-only sprint detail reads in favor of project-scoped access-checked routes, with compatibility handled during migration rather than leaving ambiguous contracts in place.

## Capabilities

### New Capabilities
- `sprint-control-plane-interfaces`: Defines truthful project-scoped sprint query, current-sprint resolution, activation invariants, and canonical sprint detail interfaces consumed by web and IM operator surfaces.

### Modified Capabilities
- `sprint-management-workspace`: The sprint workspace requirements expand to support explicit sprint selection and inline handling of current-sprint activation conflicts while consuming the canonical sprint detail contract.
- `budget-query-api`: Sprint budget detail requirements expand to require project-scoped access checks and a canonical sprint detail route shape that matches the broader sprint control-plane contract.

## Impact

- Frontend sprint planning surfaces and route helpers such as `app/(dashboard)/sprints/page.tsx`, `lib/stores/sprint-store.ts`, `lib/route-hrefs.ts`, and project/dashboard handoff callers that need explicit sprint selection.
- Backend sprint query and lifecycle code in `src-go/internal/handler/sprint_handler.go`, `src-go/internal/handler/budget_query_handler.go`, `src-go/internal/repository/sprint_repo.go`, `src-go/internal/model/sprint.go`, and `src-go/internal/server/routes.go`.
- IM bridge sprint consumers in `src-im-bridge/client/agentforge.go`, `src-im-bridge/commands/sprint.go`, and related command/client tests.
- Sprint-focused tests across web, Go handler/service/repository layers, and IM bridge integration paths that currently assume ambiguous current-sprint behavior.
