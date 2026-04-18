# Project Archival — Ops Runbook

Archival is the one-way "freeze writes, keep data" lifecycle state for a
project. This runbook covers the operational questions that come up when a
project is archived, unarchived, or deleted.

## Status machine

```
active  ── archive  (owner) ──►  archived  ── delete (owner) ──► (gone)
   ▲                                 │
   └──────  unarchive (owner) ◄──────┘
```

- All three transitions are explicit API calls. No auto-transitions.
- `paused` is separate semantics (pause but stay writable); archival does
  not change it.
- `delete` on a non-archived project is rejected with `409
  project_must_be_archived_before_delete`.

## What archive does on the backend

When `POST /api/v1/projects/:pid/archive` succeeds (owner only), in order:

1. `projects.status` flips to `archived`; `archived_at` and
   `archived_by_user_id` are stamped. This is the primary commitment — after
   this succeeds, the project is archived even if later steps fail.
2. Cascade (best-effort, logged but never rolled back):
   - All active/pending/paused **workflow executions** for the project get
     cancelled via `DAGWorkflowService.CancelAllActiveForProject`.
   - All non-terminal **team runs** for the project get cancelled via
     `TeamService.CancelAllActiveForProject`.
   - Pending **invitations** for the project are revoked with reason
     `project_archived` (once the invitation service is wired; Wave 2).
3. The archived-guard middleware starts rejecting every write on any
   `/api/v1/projects/:pid/...` route (except `/unarchive`) with `409
   project_archived`.
4. The dispatch service, automation engine, and DAG workflow engine all
   re-check project status at their entry points; they reject writes on
   archived projects even for internal callers that bypass HTTP.

## What unarchive does NOT do

- **It does NOT auto-resume cancelled runs.** A team run that was cancelled
  because of archival stays cancelled — users must re-dispatch it. Rationale
  in [change design](../../openspec/changes/2026-04-17-add-project-archival/design.md)
  §6.

## Common incidents

### "I archived a project by accident"

- UI requires typing the project name to confirm (see
  `ProjectCard.confirmArchive`), so accidental clicks should be rare.
- Recovery: call `POST /api/v1/projects/:pid/unarchive`. The project is
  active again. In-flight runs that got cancelled during archive are NOT
  auto-restarted — the user needs to re-dispatch them.

### "Writes still fail even after unarchive"

- Confirm via `GET /api/v1/projects/:pid` that `status` is `active` and
  `archivedAt` is null.
- If the DB row shows `archived` but the API returns 409 with the
  `project_archived` error code, the repository layer has not reloaded. Bounce
  the API server or check for a stale in-memory cache.

### "Archive cascade left an agent run still running"

- Expected: cascade is best-effort. Check the logs for
  `team service: cascade cancel for archived project failed` or the
  equivalent DAG workflow log.
- Manual remediation: look up the offending `team_id` / `execution_id` and
  call the explicit cancel endpoints — RBAC allows those on an archived
  project via the whitelist (they are `.cancel` actions, which are
  suffix-allowed by the archived-guard).

### "Automation keeps firing on an archived project"

- `AutomationEngineService.EvaluateRulesWithSummary` short-circuits on
  archived projects. If automation keeps firing, confirm that the project
  repo was wired into the engine at startup via
  `automationEngine.SetProjectStatusLookup(projectRepo)`.

### "Scheduler cost-reconciler picks up an archived project"

- Cost reconciliation calls `projects.List(ctx)` which, by default, returns
  only non-archived projects. Archived projects are invisible to the default
  list. To force inclusion for a one-off ops run, call `ListWithFilter` with
  `Statuses: ["archived"]`.

## DB migration notes

Migration `060_add_project_archival.up.sql` adds:

- `projects.status` (NOT NULL, default `active`, `CHECK (status IN ('active','paused','archived'))`).
- `projects.archived_at TIMESTAMPTZ`.
- `projects.archived_by_user_id UUID REFERENCES users(id)`.
- Index `idx_projects_status_archived_at` on `(status, archived_at)` to
  support the default-list and archived-view queries.

Backfill sets every existing row to `status='active'`. No automatic detection
of prior "[archived] foo" naming conventions — the migration is intentionally
conservative and operators should flip those rows manually.

## Security notes

- Archive / unarchive / delete all require project role `owner` (enforced by
  the RBAC matrix).
- The archived-guard middleware runs AFTER RBAC. A viewer who attempts a
  write on an archived project gets `403 Forbidden` from RBAC *before* ever
  hitting the guard — the 403 vs 409 distinction tells the UI whether it's a
  role problem or a state problem.
- Reads, including audit-log reads, remain available on archived projects.
  This is intentional — archived projects should stay auditable.
