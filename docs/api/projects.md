# Project API / 项目 API

This document covers the project CRUD surface plus the project member seam used
today.

See also: [`docs/api/project-templates.md`](./project-templates.md) — the
`templateSource` / `templateId` parameters that extend `POST /projects`, plus
the `project_templates` CRUD surface and `save-as-template` route.

## Overview

- Base path: `/api/v1/projects`
- Auth: every endpoint requires an access token
- Project creation also bootstraps a wiki space and built-in wiki templates
- Project creation optionally clones a project template when
  `templateSource`+`templateId` are present in the body; see
  [Project Templates](./project-templates.md) for the contract

## Bootstrap And Handoff Notes

The repository's current project-management flow treats project creation as the
start of a bootstrap lifecycle rather than the end of setup.

- Frontend project-entry surfaces SHOULD preserve explicit project context after
  creation or selection instead of relying only on ambient dashboard selection.
- The canonical bootstrap entry used by the current dashboard surfaces is
  `/?project=<project-id>`.
- Project-scoped workspaces MAY also accept explicit handoff query params such
  as `project`, `section`, `tab`, `focus`, or `action` to open the relevant
  management surface directly for the same project context.
- Project creation returning `ProjectDTO` is therefore important not only for
  CRUD confirmation, but also for the immediate handoff into settings, team,
  template, planning, and delivery workspaces.

## Core DTOs

### `ProjectDTO`

Key fields:

- `id`, `name`, `slug`, `description`, `repoUrl`, `defaultBranch`
- `settings`
  - `codingAgent`
  - `reviewPolicy`
  - `budgetGovernance`
  - `webhook`
- `codingAgentCatalog`
- `createdAt`

### `CreateProjectRequest`

```json
{
  "name": "AgentForge",
  "slug": "agentforge",
  "description": "Mixed human + AI engineering team",
  "repoUrl": "https://github.com/Arxtect/AgentForge.git"
}
```

### `UpdateProjectRequest`

Patchable fields:

- `name`
- `description`
- `repoUrl`
- `defaultBranch`
- `settings`

`settings` can patch:

- `codingAgent`: `runtime`, `provider`, `model`
- `reviewPolicy`: required layers, manual approval, risk threshold, auto-trigger, plugin dimensions
- `budgetGovernance`: task/daily caps and alert threshold
- `webhook`: URL, secret, events, active flag

## Endpoint Summary

| Method | Path | Purpose |
| --- | --- | --- |
| `POST` | `/api/v1/projects` | Create a project |
| `GET` | `/api/v1/projects` | List projects (default filter hides archived — see Archival below) |
| `GET` | `/api/v1/projects/:id` | Get one project |
| `PUT` | `/api/v1/projects/:id` | Update a project |
| `DELETE` | `/api/v1/projects/:id` | Delete a project (**must be archived first** — see Archival) |
| `POST` | `/api/v1/projects/:pid/archive` | Archive a project (owner only) |
| `POST` | `/api/v1/projects/:pid/unarchive` | Restore an archived project (owner only) |
| `POST` | `/api/v1/projects/:pid/members` | Add a project member |
| `GET` | `/api/v1/projects/:pid/members` | List project members |
| `PUT` | `/api/v1/members/:id` | Update a member |
| `DELETE` | `/api/v1/members/:id` | Remove a member |

## `POST /api/v1/projects`

Behavior:

- validates `name` and `slug`
- stores the project with default branch `main`
- initializes a wiki space
- seeds built-in wiki templates
- returns `201 Created` with `ProjectDTO`

Typical failure modes:

- `400 Bad Request`: malformed JSON
- `422 Unprocessable Entity`: validation failure
- `500 Internal Server Error`: project or wiki bootstrap failure

## `PUT /api/v1/projects/:id`

Notes:

- the backend returns a runtime-aware `codingAgentCatalog`
- webhook secrets are not echoed back in the response DTO
- persisted settings are stored as JSONB in `projects.settings`

## Project Members / 成员管理

Current create/update payloads:

### `POST /api/v1/projects/:pid/members`

```json
{
  "name": "Code Reviewer",
  "type": "agent",
  "role": "code-reviewer",
  "status": "active",
  "email": "",
  "imPlatform": "feishu",
  "imUserId": "ou_xxx",
  "agentConfig": "{}",
  "skills": ["security", "review"]
}
```

### `PUT /api/v1/members/:id`

Patchable fields:

- `name`
- `role`
- `status`
- `email`
- `imPlatform`
- `imUserId`
- `agentConfig`
- `skills`
- `isActive`

## Archival / 项目归档

Projects have a lifecycle status: `active`, `paused`, or `archived`. Archival
is the "freeze writes, keep data" state — the project stays readable for
audit, but any write attempt returns `409 project_archived`.

### Status transitions

```
active  ── archive  (owner) ──►  archived  ── delete (owner) ──► (gone)
   ▲                                 │
   └──────  unarchive (owner) ◄──────┘
```

All three transitions are explicit API calls. `paused` is a separate
semantics (pause but stay writable) and is not changed by this contract.

### `POST /api/v1/projects/:pid/archive`

- Requires project role `owner`.
- Flips `status=archived`, sets `archivedAt`, `archivedByUserId`.
- Best-effort cascades: cancels in-flight team runs + workflow executions;
  revokes pending invitations (when the invitation service is wired); skips
  scheduler/automation triggers. Cascade failures do NOT roll back the
  archive — the status flip is the primary commitment.
- Returns `200 OK` with the refreshed `ProjectDTO`.

Failure modes:

- `409 Conflict` with message "project is archived and read-only" — already
  archived.
- `403 Forbidden` — caller is not `owner`.

### `POST /api/v1/projects/:pid/unarchive`

- Requires project role `owner`.
- Flips `status=active`, clears `archivedAt` and `archivedByUserId`.
- Does NOT auto-resume cancelled runs. Users must re-dispatch manually.
- Returns `200 OK` with the refreshed `ProjectDTO`.

### `DELETE /api/v1/projects/:id` (tightened semantics)

- The project **must be archived** before delete. Deleting a non-archived
  project returns `409 project_must_be_archived_before_delete`.
- Optional query param `keepAudit` (default `true`) controls whether audit
  events survive the delete. The server currently relies on FK cascade for
  audit rows; `keepAudit=true` is the advertised default but a dedicated
  retain path is not yet implemented.

### `GET /api/v1/projects` filtering

- Default: returns only `active` + `paused` projects. Archived projects are
  hidden.
- `?includeArchived=true`: returns active + paused + archived.
- `?status=archived`: returns only archived projects. Accepts a comma-separated
  list of statuses (e.g. `?status=archived,paused`).

### Archived-project error body

Writes blocked by the archived-guard middleware return `409 Conflict` with a
structured body that UIs can use to render an "archived" banner:

```json
{
  "message": "project is archived and read-only",
  "code": 409,
  "errorCode": "project_archived",
  "archivedAt": "2026-04-17T10:15:00Z",
  "archivedByUserId": "…"
}
```

Reads (`GET`, `HEAD`, `OPTIONS`) pass through the guard unchanged. The
`POST /projects/:pid/unarchive` path is explicitly whitelisted so owners can
restore the project.

## Invite Note

Human member onboarding uses the project-scoped invitation flow. See
[`invitations.md`](./invitations.md) for the full contract: create, list,
revoke, resend, accept, and decline.

Direct `POST /api/v1/projects/:pid/members` remains available for agent
members and IM-identity bindings, but human member creation now returns
`410 HumanMemberCreationMovedToInvitationFlow` unless the caller uses the
invitation flow.
