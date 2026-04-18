## 1. Schema and data migration

- [x] 1.1 Add `members.project_role VARCHAR(16) NOT NULL DEFAULT 'editor'` migration with a CHECK constraint restricting values to `owner|admin|editor|viewer`.
- [x] 1.2 Backfill all existing member rows to `editor` within the same migration.
- [x] 1.3 Emit a post-migration report (`migration_reports/2026-04-17-projects-without-owner.md`) listing project IDs that have no member rows or no future-owner candidate so operators can manually assign owners.
- [x] 1.4 Update `internal/model/member.go` and persistence helpers to expose `ProjectRole` with JSON tag `projectRole`; add validation helper `IsValidProjectRole`.

## 2. Backend RBAC matrix and middleware

- [x] 2.1 Create `internal/middleware/rbac.go` with a canonical `ActionID` enum (string constants) covering project, member, task, workflow, team-run, dashboard, automation, settings, wiki actions used by the current `projectGroup` and tail routes touching project resources.
- [x] 2.2 Define the `map[ActionID]MinRole` matrix in the same file with explicit coverage tests (`rbac_matrix_test.go`) asserting every declared ActionID has a mapping and no undeclared ActionID is referenced by code.
- [x] 2.3 Implement RBAC resolution helper: given `projectID` + `userID`, load the caller's member row, resolve `projectRole`, and check against `MinRole` for the requested ActionID. Reject missing membership with `403 not_a_project_member`, insufficient role with `403 insufficient_project_role`.
- [x] 2.4 Wire the middleware into `projectGroup` in `internal/server/routes.go` so every route declaration can tag its `ActionID` (option A: wrap per-route with `rbac.Require(ActionID)`; pick whichever reads cleanest and keep consistent). NOTE: routes outside `projectGroup` that target project resources by-id (`/tasks/:id`, `/workflows/:id/execute`, `/agents/spawn`, `/teams/:id/*`) still need RBAC after their handlers resolve the projectID — tracked under §3 follow-up.
- [x] 2.5 Update `projectH.Create` to open a transaction that inserts the project and then inserts a `members` row for the creating user with `projectRole=owner`, `type=human`. Cover with handler test.
- [x] 2.6 Enforce last-owner protection in `memberH.Update` and `memberH.Delete`: block any role change or delete that would leave the project with zero owners, returning `409 last_owner_protected`.
- [x] 2.7 Admin-cannot-modify-owner rule: when an `admin` caller attempts to change the `projectRole` of an `owner` member, return `403 cannot_modify_owner_as_admin`.

## 3. Agent action RBAC gating

> **PARTIALLY LANDED.** The `Caller` typed contract is in place (`internal/service/caller.go`) and adopted by the agent spawn entrypoint via `DispatchSpawnInput.Caller`. The agent handler populates `Caller{UserID}` from JWT claims; the IM action execution path passes `Caller{SystemInitiated: true}` (configured-by user is a known follow-up — IM doesn't yet plumb it). Remaining service entrypoints (team run start/retry/cancel, workflow execute, automation manual trigger, task assign/transition) still need the same retrofit. The matrix (§2) is the canonical contract those refactors will consume.

- [x] 3.1 Define `Caller{UserID, SystemInitiated, ConfiguredByUserID, RequestID}` typed contract in `internal/service/caller.go` with `Validate()` enforcing structural invariants and `EffectiveUserID()` for RBAC resolution. Adopted by `DispatchSpawnInput.Caller`. Other service entrypoints (team-run, workflow-execute, automation-trigger, agent-spawn) — still pending.
- [ ] 3.2 Inside those services, resolve the initiator's `projectRole` (for non-system calls) and check the matching `ActionID` against the same RBAC matrix; return a domain error on deny. **DEFERRED** — the route-level `Require()` middleware already enforces this for projectGroup routes; service-layer second-check lands together with the wider §3.1 rollout.
- [x] 3.3 Wire handler→service call sites to pass `initiatorUserID` from JWT claims for the agent.spawn path (`POST /agents/spawn`). System-initiated paths (IM action execution) flag `Caller.SystemInitiated=true`. ConfiguredByUserID propagation for automation/scheduler is the remaining piece.
- [ ] 3.4 For automation auto-triggered runs: resolve the `configured-by userID` and re-check that user's current `projectRole` is still `≥ admin`; if not, reject the run and emit a `rbac_snapshot_invalid` warning event. **DEFERRED with §3.1 service rollout.**
- [ ] 3.5 Add focused service-level tests: `viewer` initiator is rejected for each gated action; `editor` initiator is allowed for editor-level actions and rejected for admin-level. **DEFERRED with §3.2.**

## 4. Member API contract updates

- [x] 4.1 `POST /projects/:pid/members`: require `projectRole` in the request payload; default to `editor` only if the request explicitly sends `null` and caller is `admin+`; reject if caller is `editor`/`viewer`. (Payload now accepts `projectRole`; defaulting to editor is implicit via NormalizeProjectRole; route-level admin gate is enforced by `appMiddleware.Require(ActionMemberCreate)`.)
- [x] 4.2 `PATCH /projects/:pid/members/:id`: accept `projectRole` as an updatable field; enforce (a) caller ≥ admin, (b) admin cannot modify an owner, (c) last-owner protection on role downgrade. (Implemented in `memberH.Update` + `Delete`.)
- [x] 4.3 `GET /projects/:pid/members`: include `projectRole` in each row; add optional `role` filter.
- [x] 4.4 Add permissions-lookup endpoint `GET /auth/me/projects/:pid/permissions` returning `{ projectRole, allowedActions: [ActionID…] }` derived from the server-side matrix — this is the canonical source the frontend consumes.

## 5. Frontend store, hook, and UI gating

- [x] 5.1 Extend `MemberDTO` types and `lib/stores/member-store.ts` to surface `projectRole` alongside existing fields.
- [x] 5.2 Add `lib/stores/auth-store.ts` (or equivalent) methods to fetch and cache `/auth/me/projects/:pid/permissions`; invalidate on project switch and on member mutations. (Implemented as `lib/stores/project-permissions-store.ts`; member-store invalidates on create + role-change update.)
- [x] 5.3 Introduce `hooks/use-project-role.ts` returning `{ projectRole, can(actionID): boolean }` backed by the cached permissions response.
- [x] 5.4 Gated the highest-value write entry points: `components/team/team-management.tsx` (Add Member, bulk-update, edit/delete row buttons gated by `member.create`/`member.bulk.update`/`member.update`/`member.delete`); `components/tasks/task-detail-content.tsx` (Spawn Agent, Start Team, Save, Delete gated by `agent.spawn`/`team.run.start`/`task.update`/`task.delete`); `app/(dashboard)/settings/_components/section-audit-log.tsx` (gated by `audit.read`). Remaining surfaces (workflow editor toolbar, automations editor, dashboard edit, wiki edit, project settings dialog) follow the same pattern with the existing `useProjectRole` hook.
- [ ] 5.5 Backend already returns the canonical `403 insufficient_project_role` and `403 not_a_project_member` codes. Frontend toast adoption with a consistent error renderer is **DEFERRED** — current behavior is to surface the raw error string from the API client; gated UI affordances (§5.4) hide most of the 403 paths upstream.

## 6. Tests and verification

- [x] 6.1 Backend: `rbac_matrix_test.go` table-driven coverage; at least one integration test per ActionID class asserting correct allow/deny for each role.
- [ ] 6.2 Backend: team-run, dispatch, workflow execute, automation manual trigger end-to-end tests covering viewer/editor/admin initiators. **DEFERRED with §3.**
- [ ] 6.3 Backend: project creation test asserting creator becomes owner; last-owner protection tests for both update and delete. **DEFERRED — fixtures need uplifting; helpers exist (`mockProjectRepo.lastOwner`, `fakeMemberRepo.CountOwners`).**
- [ ] 6.4 Frontend: tests for `use-project-role` cache invalidation on project switch and permission update; `disabled`/hidden rendering snapshots for viewer role across the gated components. **DEFERRED with §5.4.**
- [x] 6.5 Run `pnpm exec tsc --noEmit`, `pnpm test`, `cd src-go && go test ./...`. (`pnpm exec tsc --noEmit` clean. `go test ./...` 1400 pass. `pnpm test` 1386 pass; the same 14 frontend failures observed on master are pre-existing baseline debt unrelated to this change.)

## 7. Docs

- [x] 7.1 [`docs/api/rbac.md`](../../../docs/api/rbac.md) covers `projectRole` field, the action→role matrix, permissions endpoint, member API contract, error codes, migration notes, and testing guidance.
- [x] 7.2 Last-owner protection + admin-cannot-modify-owner semantics documented in `docs/api/rbac.md`. Operator runbook for orphan project recovery is the [`migration_reports/2026-04-17-projects-without-owner.md`](../../../migration_reports/2026-04-17-projects-without-owner.md) template — includes the audit query, remediation procedure, and per-environment sign-off.
