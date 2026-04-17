## 1. Schema and model

- [ ] 1.1 Migration: add `projects.archived_at TIMESTAMPTZ NULL`, `projects.archived_by_user_id UUID NULL`; backfill `status='active'` where null; add index `(status, archived_at)`.
- [ ] 1.2 Update `internal/model/project.go` to include `ArchivedAt`, `ArchivedByUserID` with JSON tags `archivedAt`, `archivedByUserId`; update `ProjectDTO` accordingly.
- [ ] 1.3 Validation: `status` enum restricted to `active|paused|archived` at model + migration CHECK constraint.

## 2. Lifecycle service and middleware

- [ ] 2.1 `internal/service/project_lifecycle_service.go`: implement `Archive(ctx, projectID, ownerUserID)`, `Unarchive(ctx, projectID, ownerUserID)`, `DeleteArchived(ctx, projectID, ownerUserID, opts)`. Archive wraps: status flip + archived_at/by + cascade (invitation revoke, in-flight run cancel, automation skip signal). Unarchive flips back and clears archived_at/by.
- [ ] 2.2 Cascade helpers (best-effort, not in main transaction): `invitation_service.RevokeAllPending(projectID, reason='project_archived')`; `team_service.CancelAllActive(projectID)`; `workflow_service.CancelAllExecutions(projectID)`; `automation_engine.DisableForArchivedProject(projectID)`. Each returns a per-resource failure list reported as audit events but not failing Archive.
- [ ] 2.3 `internal/middleware/archived_guard.go`: loads project status (via project middleware cache) and rejects with `409 project_archived` unless the ActionID is in the read-only whitelist (any `.view` action plus `project.unarchive`, `project.delete`, `audit.read`).
- [ ] 2.4 Wire guard in `internal/server/routes.go` between RBAC middleware and handler; ensure ordering RBAC → archived_guard is covered by a wiring test.

## 3. Handlers and routes

- [ ] 3.1 `projectH.Archive` (`POST /projects/:pid/archive`, owner only) and `projectH.Unarchive` (`POST /projects/:pid/unarchive`, owner only) — declare new ActionIDs `project.archive` / `project.unarchive` in the RBAC matrix.
- [ ] 3.2 `projectH.Delete` semantics change: reject with `409 project_must_be_archived_before_delete` unless `status='archived'`. Support optional `keepAudit` bool (default true) controlling whether audit events survive.
- [ ] 3.3 `projectH.List`: honor `includeArchived=true` or `status=archived`; default filter `status IN ('active','paused')`. Pagination and existing filters preserved.

## 4. Downstream enforcement

- [ ] 4.1 Scheduler: before executing a job tied to a project, fetch project status; skip + audit-log if archived.
- [ ] 4.2 Automation engine: evaluate project status in rule evaluation; archived project → no-op + audit `automation_skipped_project_archived`.
- [ ] 4.3 Dispatch/team/workflow services: in each initiator-bound action (`task.dispatch`, `team.run.start`, `workflow.execute`, etc.) re-check project status at service entry; reject with structured error if archived. This is in addition to the middleware guard (belt and suspenders for internal callers that bypass HTTP).

## 5. Frontend

- [ ] 5.1 `lib/stores/project-store.ts`: extend list query with `includeArchived` flag; expose two views (active, archived) to the projects page.
- [ ] 5.2 `app/(dashboard)/projects/page.tsx`: add a tab or select switching between active and archived lists.
- [ ] 5.3 `components/project/project-card.tsx`: render archived badge + "archived on {date} by {user}" tooltip when `status='archived'`; disable write CTAs on archived cards.
- [ ] 5.4 `components/project/edit-project-dialog.tsx` or new `archive-project-dialog.tsx`: owner-only archive button with double-confirm (type project name to confirm); unarchive button on archived project detail.
- [ ] 5.5 Workspace-level read-only mode: when current selected project is archived, surface a banner at the top of every workspace tab explaining read-only status and pointing to unarchive for owners.
- [ ] 5.6 Localization keys in `messages/en/projects.json` + `messages/zh-CN/projects.json` for archive/unarchive strings and banner copy.

## 6. Tests

- [ ] 6.1 Backend: archive/unarchive happy path; delete succeeds only after archive; delete on active returns 409; cascade revokes pending invitations and cancels in-flight runs.
- [ ] 6.2 Backend: archived_guard rejects each write ActionID; read-only whitelist endpoints still work.
- [ ] 6.3 Backend: scheduler + automation + dispatch each skip archived projects; initiator services reject at entry.
- [ ] 6.4 Backend wiring test: middleware chain order `project → rbac → archived_guard → handler` asserted at registration time.
- [ ] 6.5 Frontend: project list default hides archived; archived view shows archived projects; archived workspace banner renders; write CTAs hidden/disabled.
- [ ] 6.6 `pnpm exec tsc --noEmit`, `pnpm test`, `cd src-go && go test ./...`.

## 7. Docs

- [ ] 7.1 API reference: archive/unarchive endpoints, delete semantics change, error codes.
- [ ] 7.2 Ops runbook: how to unarchive, what cascades on archive, what happens to existing runs.
