## 1. Schema and data migration

- [ ] 1.1 Add `members.project_role VARCHAR(16) NOT NULL DEFAULT 'editor'` migration with a CHECK constraint restricting values to `owner|admin|editor|viewer`.
- [ ] 1.2 Backfill all existing member rows to `editor` within the same migration.
- [ ] 1.3 Emit a post-migration report (`migration_reports/2026-04-17-projects-without-owner.md`) listing project IDs that have no member rows or no future-owner candidate so operators can manually assign owners.
- [ ] 1.4 Update `internal/model/member.go` and persistence helpers to expose `ProjectRole` with JSON tag `projectRole`; add validation helper `IsValidProjectRole`.

## 2. Backend RBAC matrix and middleware

- [ ] 2.1 Create `internal/middleware/rbac.go` with a canonical `ActionID` enum (string constants) covering project, member, task, workflow, team-run, dashboard, automation, settings, wiki actions used by the current `projectGroup` and tail routes touching project resources.
- [ ] 2.2 Define the `map[ActionID]MinRole` matrix in the same file with explicit coverage tests (`rbac_matrix_test.go`) asserting every declared ActionID has a mapping and no undeclared ActionID is referenced by code.
- [ ] 2.3 Implement RBAC resolution helper: given `projectID` + `userID`, load the caller's member row, resolve `projectRole`, and check against `MinRole` for the requested ActionID. Reject missing membership with `403 not_a_project_member`, insufficient role with `403 insufficient_project_role`.
- [ ] 2.4 Wire the middleware into `projectGroup` in `internal/server/routes.go` so every route declaration can tag its `ActionID` (option A: wrap per-route with `rbac.Require(ActionID)`; pick whichever reads cleanest and keep consistent).
- [ ] 2.5 Update `projectH.Create` to open a transaction that inserts the project and then inserts a `members` row for the creating user with `projectRole=owner`, `type=human`. Cover with handler test.
- [ ] 2.6 Enforce last-owner protection in `memberH.Update` and `memberH.Delete`: block any role change or delete that would leave the project with zero owners, returning `409 last_owner_protected`.
- [ ] 2.7 Admin-cannot-modify-owner rule: when an `admin` caller attempts to change the `projectRole` of an `owner` member, return `403 cannot_modify_owner_as_admin`.

## 3. Agent action RBAC gating

- [ ] 3.1 Refactor service signatures to add required `initiatorUserID string` (or a typed `Caller` struct with `UserID string; SystemInitiated bool`) across: task dispatch/assign/transition, team run start/retry/cancel, workflow execute, automation manual trigger, agent spawn. Non-optional at the type level.
- [ ] 3.2 Inside those services, resolve the initiator's `projectRole` (for non-system calls) and check the matching `ActionID` against the same RBAC matrix; return a domain error on deny.
- [ ] 3.3 Wire handler→service call sites to pass `initiatorUserID` from `c.Get("user_id")` (or equivalent middleware key); for scheduler/IM webhook/automation auto paths set `SystemInitiated=true` and also record the configured-by user for audit.
- [ ] 3.4 For automation auto-triggered runs: resolve the `configured-by userID` and re-check that user's current `projectRole` is still `≥ admin`; if not, reject the run and emit a `rbac_snapshot_invalid` warning event (consumer TBD; specified here).
- [ ] 3.5 Add focused service-level tests: `viewer` initiator is rejected for each gated action; `editor` initiator is allowed for editor-level actions and rejected for admin-level; missing `initiatorUserID` panics at compile (or fails a strict static test).

## 4. Member API contract updates

- [ ] 4.1 `POST /projects/:pid/members`: require `projectRole` in the request payload; default to `editor` only if the request explicitly sends `null` and caller is `admin+`; reject if caller is `editor`/`viewer`.
- [ ] 4.2 `PATCH /projects/:pid/members/:id`: accept `projectRole` as an updatable field; enforce (a) caller ≥ admin, (b) admin cannot modify an owner, (c) last-owner protection on role downgrade.
- [ ] 4.3 `GET /projects/:pid/members`: include `projectRole` in each row; add optional `role` filter.
- [ ] 4.4 Add permissions-lookup endpoint `GET /auth/me/projects/:pid/permissions` returning `{ projectRole, allowedActions: [ActionID…] }` derived from the server-side matrix — this is the canonical source the frontend consumes.

## 5. Frontend store, hook, and UI gating

- [ ] 5.1 Extend `MemberDTO` types and `lib/stores/member-store.ts` to surface `projectRole` alongside existing fields.
- [ ] 5.2 Add `lib/stores/auth-store.ts` (or equivalent) methods to fetch and cache `/auth/me/projects/:pid/permissions`; invalidate on project switch and on member mutations.
- [ ] 5.3 Introduce `hooks/use-project-role.ts` returning `{ projectRole, can(actionID): boolean }` backed by the cached permissions response.
- [ ] 5.4 Gate write-capable UI entry points using `can(actionID)`: team management (`components/team/*`), task dispatch (`components/tasks/dispatch-preflight-dialog.tsx`, `spawn-agent-dialog.tsx`), project settings (`components/project/edit-project-dialog.tsx`), workflow editor toolbar, automation config, dashboard edit, wiki edit. Hide or disable + tooltip explaining required role.
- [ ] 5.5 On any write attempt still reaching the backend, handle `403 insufficient_project_role` and `403 not_a_project_member` with a consistent toast + optional "request elevated access" CTA (CTA wiring is spec-only in this change).

## 6. Tests and verification

- [ ] 6.1 Backend: `rbac_matrix_test.go` table-driven coverage; at least one integration test per ActionID class asserting correct allow/deny for each role.
- [ ] 6.2 Backend: team-run, dispatch, workflow execute, automation manual trigger end-to-end tests covering viewer/editor/admin initiators.
- [ ] 6.3 Backend: project creation test asserting creator becomes owner; last-owner protection tests for both update and delete.
- [ ] 6.4 Frontend: tests for `use-project-role` cache invalidation on project switch and permission update; `disabled`/hidden rendering snapshots for viewer role across the gated components.
- [ ] 6.5 Run `pnpm exec tsc --noEmit`, `pnpm test`, `cd src-go && go test ./...`, and document any unrelated baseline debt separately.

## 7. Docs

- [ ] 7.1 Update API reference docs with `projectRole` field, new permissions endpoint, and error codes.
- [ ] 7.2 Add a short ops runbook entry describing last-owner protection behavior and how to manually elevate an owner for migration-reported projects.
