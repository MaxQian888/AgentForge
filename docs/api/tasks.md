# Task API / 任务 API

This document covers the task surface implemented by
`src-go/internal/server/routes.go`, `src-go/internal/handler/task_handler.go`,
`src-go/internal/model/task.go`, and `src-go/internal/model/task_comment.go`.

## Overview

Task APIs are split into:

- project-scoped collection routes under `/api/v1/projects/:pid/tasks`
- global single-task routes under `/api/v1/tasks/:id`
- project-scoped task comments under `/api/v1/projects/:pid/tasks/:tid/comments`

All routes require a valid access token.

## Core DTOs

### `TaskDTO`

Representative fields:

- `id`, `projectId`, `parentId`, `sprintId`, `milestoneId`
- `title`, `description`, `status`, `priority`
- `assigneeId`, `assigneeType`, `reporterId`
- `labels`, `blockedBy`
- `budgetUsd`, `spentUsd`
- `agentBranch`, `agentWorktree`, `agentSessionId`, `prUrl`, `prNumber`
- `plannedStartAt`, `plannedEndAt`
- `progress`
- `createdAt`, `updatedAt`, `completedAt`

### Allowed status values

- `inbox`
- `triaged`
- `assigned`
- `in_progress`
- `in_review`
- `changes_requested`
- `done`
- `cancelled`
- `blocked`
- `budget_exceeded`

### Allowed priority values

- `critical`
- `high`
- `medium`
- `low`

## Endpoint Summary

| Method | Path | Purpose |
| --- | --- | --- |
| `POST` | `/api/v1/projects/:pid/tasks` | Create a task |
| `GET` | `/api/v1/projects/:pid/tasks` | List tasks with filters |
| `GET` | `/api/v1/tasks/:id` | Fetch one task |
| `PUT` | `/api/v1/tasks/:id` | Update task fields |
| `DELETE` | `/api/v1/tasks/:id` | Delete a task |
| `POST` | `/api/v1/tasks/:id/transition` | Change task status |
| `POST` | `/api/v1/tasks/:id/assign` | Assign to a human or agent |
| `GET` | `/api/v1/tasks/:id/recommend-assignee` | Return assignment candidates |
| `POST` | `/api/v1/tasks/:id/decompose` | Decompose a task into subtasks |
| `GET` | `/api/v1/tasks/:tid/dispatch/history` | Read dispatch history when enabled |
| `GET` | `/api/v1/projects/:pid/tasks/:tid/comments` | List task comments |
| `POST` | `/api/v1/projects/:pid/tasks/:tid/comments` | Create a comment |
| `PATCH` | `/api/v1/projects/:pid/tasks/:tid/comments/:cid` | Update comment resolution state |
| `DELETE` | `/api/v1/projects/:pid/tasks/:tid/comments/:cid` | Delete a comment |

## `POST /api/v1/projects/:pid/tasks`

Request body:

```json
{
  "title": "Implement review detail panel",
  "description": "Wire decision actions and loading states",
  "priority": "high",
  "parentId": "optional-parent-task-id",
  "sprintId": "optional-sprint-id",
  "labels": ["frontend", "execution:agent"],
  "budgetUsd": 5,
  "plannedStartAt": "2026-03-31T09:00:00Z",
  "plannedEndAt": "2026-04-02T18:00:00Z"
}
```

Notes:

- `title` is required, min 1, max 200
- `priority` must be `critical | high | medium | low`
- new tasks start in `inbox`
- planned end cannot be earlier than planned start

## `GET /api/v1/projects/:pid/tasks`

Supported query params:

- `status`
- `assignee_id`
- `sprint_id`
- `priority`
- `search`
- `page`
- `limit`
- `sort`
- `customFieldFilters` as JSON
- `customFieldSort` as JSON

Response shape:

```json
{
  "items": ["TaskDTO", "TaskDTO"],
  "total": 42,
  "page": 1,
  "limit": 20
}
```

## `PUT /api/v1/tasks/:id`

Patchable fields:

- `title`
- `description`
- `priority`
- `sprintId`
- `milestoneId`
- `labels`
- `blockedBy`
- `budgetUsd`
- `plannedStartAt`
- `plannedEndAt`

Handler behavior:

- invalid or duplicate `blockedBy` IDs are sanitized/rejected
- a task cannot depend on itself
- dependency changes can trigger blocked/ready reconciliation
- task-progress snapshots and automation events are updated when available

## `POST /api/v1/tasks/:id/transition`

Request body:

```json
{
  "status": "in_review",
  "reason": "implementation complete"
}
```

Behavior:

- validates the transition
- records task-progress activity
- may auto-unblock dependent tasks
- emits workflow and automation events

Project workflow trigger configs currently normalize to these canonical action names:

- `dispatch_agent`
- `start_workflow`
- `notify`
- `auto_transition`

Legacy aliases may still be accepted by the backend for compatibility, but new configs should use the canonical names above.

## `POST /api/v1/tasks/:id/assign`

Request body:

```json
{
  "assigneeId": "member-uuid",
  "assigneeType": "human"
}
```

If the dispatch service is enabled, the handler can return:

```json
{
  "task": "TaskDTO",
  "dispatch": {
    "status": "started | queued | blocked | skipped",
    "reason": "optional reason",
    "guardrailType": "budget | pool | target | task | system",
    "guardrailScope": "task | sprint | project"
  }
}
```

## `GET /api/v1/tasks/:id/recommend-assignee`

Returns assignment candidates for the task based on the current member and
agent roster.

## `POST /api/v1/tasks/:id/decompose`

Bridge-backed decomposition endpoint. Typical failure modes:

- `404 Not Found`: task missing
- `409 Conflict`: task already has children
- `502 Bad Gateway`: bridge returned invalid decomposition content

## Task Comments

Comment DTOs include:

- `id`, `taskId`, `parentCommentId`
- `body`, `mentions`
- `resolvedAt`
- `createdBy`, `createdAt`, `updatedAt`, `deletedAt`

Create request:

```json
{
  "body": "Please attach the PR link",
  "parentCommentId": "optional-parent-comment-id"
}
```

Update request:

```json
{
  "resolved": true
}
```
