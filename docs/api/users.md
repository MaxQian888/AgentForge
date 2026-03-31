# User API / 用户 API

This document describes the current user-profile surface and the repo-truthful
invite equivalent used today.

## Overview

- Base path: `/api/v1/users`
- Auth: all profile endpoints require an access token
- Current repo truth:
  - profile management exists under `/users/me`
  - password change exists under `/users/me/password`
  - there is no dedicated `/users/invite` endpoint yet
  - project membership creation is the current invite-equivalent seam

## Endpoint Summary

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/users/me` | Return the current authenticated user |
| `PUT` | `/api/v1/users/me` | Update the current user's display name |
| `PUT` | `/api/v1/users/me/password` | Change the current user's password |
| `POST` | `/api/v1/projects/:pid/members` | Project-scoped invite/member creation path |

## `GET /api/v1/users/me`

Response:

```json
{
  "id": "uuid",
  "email": "user@example.com",
  "name": "Operator",
  "createdAt": "2026-03-31T12:34:56Z"
}
```

Typical responses:

- `200 OK`
- `401 Unauthorized`
- `503 Service Unavailable`

## `PUT /api/v1/users/me`

Request body:

```json
{
  "name": "Updated Name"
}
```

Validation:

- `name` is required

## `PUT /api/v1/users/me/password`

Request body:

```json
{
  "currentPassword": "old-password",
  "newPassword": "new-password-with-8-chars"
}
```

Validation:

- `currentPassword`: required
- `newPassword`: required, minimum 8 chars

## Invite / 邀请说明

The change task mentions `me/invite`. In current code, the equivalent invite
workflow is project membership creation rather than a standalone user endpoint.

Use:

- `POST /api/v1/projects/:pid/members`

to add:

- a human contributor
- an AI agent member
- a member with IM identity metadata
