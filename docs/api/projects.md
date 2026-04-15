# Project API / 项目 API

This document covers the project CRUD surface plus the project member seam used
today.

## Overview

- Base path: `/api/v1/projects`
- Auth: every endpoint requires an access token
- Project creation also bootstraps a wiki space and built-in wiki templates

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
| `GET` | `/api/v1/projects` | List projects |
| `GET` | `/api/v1/projects/:id` | Get one project |
| `PUT` | `/api/v1/projects/:id` | Update a project |
| `DELETE` | `/api/v1/projects/:id` | Delete a project |
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

## Invite Note

The OpenSpec task calls out "member management". Current repository truth does
not expose a dedicated `/api/v1/users/invite` endpoint. The project-scoped
member creation endpoint above is the invite/onboarding seam used by the app.
