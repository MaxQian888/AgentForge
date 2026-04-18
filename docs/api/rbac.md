# Project RBAC

AgentForge enforces a four-level project role taxonomy and a central
action→role matrix. The backend is authoritative; the frontend gates UI
visibility from the same matrix surfaced via a per-project permissions
endpoint.

See also:
- [`docs/api/audit.md`](./audit.md) — the audit log that records every gated
  attempt against this matrix.
- `openspec/specs/project-access-control/spec.md` — normative spec.

## Roles

Hard-coded; custom roles are out of scope.

| Role     | Includes                                   | Examples of capabilities                                                                 |
|----------|--------------------------------------------|------------------------------------------------------------------------------------------|
| `owner`  | Everything                                 | Delete project, transfer ownership, add/remove other owners                              |
| `admin`  | Everything except project delete + owner mgmt | Member management, settings, automation/dashboard config; cannot modify owners        |
| `editor` | Tasks, dispatch, team runs, workflow execute, wiki write | Cannot change membership or settings                                          |
| `viewer` | Read-only                                  | Cannot trigger any write or cost-incurring action                                        |

Ranking: `owner > admin > editor > viewer`. `ProjectRoleAtLeast(have, need)`
in [`internal/model/member.go`](../../src-go/internal/model/member.go) is
the canonical comparator.

## Action → Role matrix

The full enum lives in
[`internal/middleware/rbac.go`](../../src-go/internal/middleware/rbac.go)
(`var matrix`). Selected entries:

| ActionID                     | Min role |
|------------------------------|----------|
| `project.read`               | viewer   |
| `project.update`             | admin    |
| `project.delete`             | owner    |
| `member.create` / `update`   | admin    |
| `member.role.change`         | admin    |
| `task.read`                  | viewer   |
| `task.create` / `update` / `delete` | editor |
| `task.dispatch`              | editor   |
| `team.run.start` / `retry` / `cancel` | editor |
| `team.update` / `delete`     | admin    |
| `workflow.read`              | viewer   |
| `workflow.write` / `execute` | editor   |
| `automation.read`            | viewer   |
| `automation.write`           | admin    |
| `automation.trigger`         | editor   |
| `settings.update`            | admin    |
| `dashboard.write`            | admin    |
| `wiki.read`                  | viewer   |
| `wiki.write` / `delete`      | editor   |
| `audit.read`                 | admin    |
| `project.save_as_template`   | admin    |
| `project.created_from_template` | viewer (audit-only; never enforced via `Require`) |

Adding a new gated action:
1. Add an `ActionID` constant in `internal/middleware/rbac.go`.
2. Add the matrix entry alongside existing rows.
3. The matrix test (`rbac_matrix_test.go`) automatically asserts every
   declared ActionID has a mapping.
4. Tag the route via `appMiddleware.Require(ActionID)` and (for service-
   layer agent actions) call `appMiddleware.Authorize` with the `Caller`.

## Per-project permissions endpoint

```
GET /api/v1/auth/me/projects/{projectId}/permissions
Authorization: Bearer <accessToken>

200 OK
{
  "projectId":     "uuid",
  "projectRole":   "owner|admin|editor|viewer",
  "allowedActions": ["project.read", "task.dispatch", ...]
}

403 not_a_project_member  — caller is not a member; does NOT leak project existence
```

Frontend hook: `hooks/use-project-role.ts`
```ts
const { projectRole, can, loading } = useProjectRole(projectId);
if (can("task.dispatch")) { ... }
```

The hook fetches and caches per-project permissions; member mutations
invalidate the cache automatically.

## Member API contract

Members carry a `projectRole` field on every read and accept it on write.

```
POST /api/v1/projects/{pid}/members          Require(member.create)  → admin+
PATCH /api/v1/projects/{pid}/members/{mid}   Require(member.update)  → admin+
GET   /api/v1/projects/{pid}/members?role=editor   Require(member.read) → viewer+
DELETE /api/v1/members/{mid}                  Require(member.delete) → admin+
```

Request payload examples:
```json
// POST/PATCH body
{ "name": "...", "type": "human", "projectRole": "editor" }
```

Backend invariants enforced regardless of caller role:

- **Last-owner protection** — `409 last_owner_protected` if a role change
  or delete would leave the project with zero owners.
- **Admin cannot modify owners** — `403 cannot_modify_owner_as_admin` if
  an `admin` caller tries to change or delete an `owner` member.
- **Project creator auto-owner** — `POST /projects` inserts the creator's
  `members` row with `projectRole=owner` in the same transaction as the
  project itself. A project never exists without ≥1 owner at the moment
  creation returns.

## Error codes

| HTTP | i18n key                       | When                                                              |
|------|--------------------------------|-------------------------------------------------------------------|
| 401  | `Unauthorized`                 | Missing/invalid JWT before the RBAC check runs                    |
| 403  | `not_a_project_member`         | Caller is authenticated but not a member of the target project    |
| 403  | `insufficient_project_role`    | Caller's role is below the action's `MinRole`                     |
| 403  | `cannot_modify_owner_as_admin` | Admin tried to change/delete an owner member                      |
| 409  | `last_owner_protected`         | Role change or delete would leave the project with zero owners    |
| 500  | `unknown project action`       | Route registered with an ActionID that has no matrix entry        |

The frontend maps these to a consistent toast through the standard error
handler; UI gating from `useProjectRole` should hide write-capable
affordances upstream so the 403 path is rare in practice.

## Migration & ops

Migration `057_add_member_project_role.up.sql` adds `members.project_role`
with a CHECK constraint and backfills existing rows to `editor`. Owners
are NOT auto-promoted.

Run the query in
[`migration_reports/2026-04-17-projects-without-owner.md`](../../migration_reports/2026-04-17-projects-without-owner.md)
on each environment after migration to identify projects that need an
owner assigned manually. Until they do, every gated write against those
projects will fail with `insufficient_project_role`.

## Testing

- `internal/middleware/rbac_matrix_test.go` — table-driven tests covering
  every (role, representative ActionID) pair plus subset invariants
  (`viewer ⊆ editor ⊆ admin ⊆ owner`).
- Add new tests next to the matrix entry when introducing actions.
