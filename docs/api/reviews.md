# Review API / 审查 API

This document describes the review control-plane surface implemented in
`src-go/internal/handler/review_handler.go` and `src-go/internal/model/review.go`.

## Overview

Review records model both automated and human-in-the-loop review passes.

- layers:
  - `1`: CI / quick review
  - `2`: deep review
  - `3`: human review
- statuses:
  - `pending`
  - `in_progress`
  - `completed`
  - `failed`
  - `pending_human`
- recommendations:
  - `approve`
  - `request_changes`
  - `reject`

## `ReviewDTO`

Key fields:

- `id`, `taskId`, `prUrl`, `prNumber`
- `layer`, `status`, `riskLevel`
- `findings[]`
- `executionMetadata`
- `summary`, `recommendation`, `costUsd`
- `createdAt`, `updatedAt`

Each finding may include:

- `id`
- `category`, `subcategory`
- `severity`
- `file`, `line`
- `message`, `suggestion`
- `cwe`, `sources[]`
- `dismissed`

## Endpoint Summary

| Method | Path | Auth | Purpose |
| --- | --- | --- | --- |
| `POST` | `/api/v1/reviews/trigger` | Review-trigger middleware | Create a review job |
| `POST` | `/api/v1/reviews/ci-result` | Review-trigger middleware | Ingest CI findings |
| `GET` | `/api/v1/reviews` | Access token | List reviews |
| `GET` | `/api/v1/reviews/:id` | Access token | Get one review |
| `GET` | `/api/v1/tasks/:taskId/reviews` | Access token | List reviews for a task |
| `POST` | `/api/v1/reviews/:id/complete` | Access token | Complete a review with findings |
| `POST` | `/api/v1/reviews/:id/approve` | Access token | Approve a review |
| `POST` | `/api/v1/reviews/:id/reject` | Access token | Reject a review |
| `POST` | `/api/v1/reviews/:id/request-changes` | Access token | Request changes and route a fix request |
| `POST` | `/api/v1/reviews/:id/false-positive` | Access token | Mark findings as false positives |

## `POST /api/v1/reviews/trigger`

Request body:

```json
{
  "taskId": "task-uuid",
  "projectId": "project-uuid",
  "prUrl": "https://github.com/org/repo/pull/123",
  "prNumber": 123,
  "trigger": "manual",
  "event": "pull_request.updated",
  "dimensions": ["security", "architecture"],
  "changedFiles": ["src-go/internal/server/routes.go"],
  "diff": "optional raw diff"
}
```

Auth notes:

- protected by `ReviewTriggerAuthMiddleware`
- can be called by authenticated users or trusted automation using the backend token

## `POST /api/v1/reviews/ci-result`

Request body:

```json
{
  "taskId": "optional-task-id",
  "prUrl": "https://github.com/org/repo/pull/123",
  "ciSystem": "github-actions",
  "status": "failed",
  "findings": [],
  "needs_deep_review": true,
  "reason": "security findings detected",
  "confidence": "high"
}
```

## `GET /api/v1/reviews`

Supported query params:

- `status`
- `riskLevel`
- `limit` (default `50`)

## `POST /api/v1/reviews/:id/complete`

Request body:

```json
{
  "riskLevel": "high",
  "findings": [],
  "executionMetadata": {
    "triggerEvent": "pull_request.updated",
    "projectId": "project-uuid",
    "changedFiles": ["src-go/internal/server/routes.go"],
    "dimensions": ["security"],
    "results": []
  },
  "summary": "Deep review found one blocking issue.",
  "recommendation": "request_changes",
  "costUsd": 0.42
}
```

## Decision Endpoints

### `POST /api/v1/reviews/:id/approve`

```json
{
  "comment": "Looks good"
}
```

### `POST /api/v1/reviews/:id/reject`

```json
{
  "comment": "Cannot merge",
  "reason": "security regression"
}
```

### `POST /api/v1/reviews/:id/request-changes`

```json
{
  "comment": "Fix the blocking findings"
}
```

Findings flagged by the request-changes flow surface in the review's findings list; auto-fix proposals are emitted by the automation rule on EventReviewCompleted (see Spec 2D).

### `POST /api/v1/reviews/:id/false-positive`

```json
{
  "findingIds": ["finding-1", "finding-2"],
  "reason": "covered by an existing guardrail"
}
```

## Typical Failure Modes

- `400 Bad Request`: invalid ID or malformed JSON
- `404 Not Found`: review or task not found
- `409 Conflict`: invalid review transition
- `422 Unprocessable Entity`: validation failure
- `500 Internal Server Error`: service-layer failure
