# User API / 用户 API

This document describes the current user-profile surface and the repo-truthful
invite equivalent used today.

## Overview

- Base path: `/api/v1/users`
- Auth: all profile endpoints require an access token
- Current repo truth:
  - profile management exists under `/users/me`
  - password change exists under `/users/me/password`
  - member invitations are project-scoped; see [`invitations.md`](./invitations.md)

## Endpoint Summary

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/users/me` | Return the current authenticated user |
| `PUT` | `/api/v1/users/me` | Update the current user's display name |
| `PUT` | `/api/v1/users/me/password` | Change the current user's password |
| `POST` | `/api/v1/projects/:pid/members` | Direct member creation (agents only; humans use invitation flow) |

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

Human onboarding uses the project-scoped invitation flow:

- `POST /api/v1/projects/:pid/invitations` — create an invitation
- `GET /api/v1/projects/:pid/invitations` — list pending invitations
- `POST /api/v1/invitations/accept` — accept via token

See [`invitations.md`](./invitations.md) for the full state machine, token
semantics, and error codes.

Direct `POST /api/v1/projects/:pid/members` is still available for adding AI
agent members and members with IM identity metadata, but human member creation
has moved to the invitation flow (`409 HumanMemberCreationMovedToInvitationFlow`
is returned for direct human creation attempts).
