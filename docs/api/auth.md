# Auth API / 认证 API

This document describes the current authentication surface implemented in
`src-go/internal/server/routes.go`, `src-go/internal/handler/auth.go`, and
`src-go/internal/model/user.go`.

## Overview

- Base path: `/api/v1/auth`
- Public endpoints are rate-limited in-memory by Echo
- Session model: JWT access token + Redis-backed refresh token
- Redis is part of the auth truth for refresh and revocation paths

## Standard Auth Response

Successful `register`, `login`, and `refresh` calls return:

```json
{
  "accessToken": "<jwt-access-token>",
  "refreshToken": "<refresh-token>",
  "user": {
    "id": "uuid",
    "email": "user@example.com",
    "name": "Operator",
    "createdAt": "2026-03-31T12:34:56Z"
  }
}
```

## Endpoint Summary

| Method | Path | Auth | Purpose |
| --- | --- | --- | --- |
| `POST` | `/api/v1/auth/register` | Public | Create a user and issue tokens |
| `POST` | `/api/v1/auth/login` | Public | Authenticate with email/password |
| `POST` | `/api/v1/auth/refresh` | Public | Exchange a refresh token for a new session |
| `POST` | `/api/v1/auth/logout` | Access token | Revoke the current session |

## `POST /api/v1/auth/register`

Request body:

```json
{
  "email": "user@example.com",
  "password": "at-least-8-chars",
  "name": "Operator"
}
```

Validation:

- `email`: required, valid email
- `password`: required, minimum 8 chars
- `name`: required

Typical responses:

- `201 Created`: returns the standard auth response
- `400 Bad Request`: malformed JSON
- `422 Unprocessable Entity`: validation failure
- `409 Conflict`: email already exists
- `503 Service Unavailable`: database or cache dependency unavailable

## `POST /api/v1/auth/login`

Request body:

```json
{
  "email": "user@example.com",
  "password": "plain-text-password"
}
```

Typical responses:

- `200 OK`: returns the standard auth response
- `401 Unauthorized`: invalid credentials
- `422 Unprocessable Entity`: validation failure
- `503 Service Unavailable`: auth dependency unavailable

## `POST /api/v1/auth/refresh`

Request body:

```json
{
  "refreshToken": "<refresh-token>"
}
```

Typical responses:

- `200 OK`: returns a new token pair
- `401 Unauthorized`: invalid/expired refresh token
- `422 Unprocessable Entity`: missing `refreshToken`
- `503 Service Unavailable`: Redis or DB unavailable

## `POST /api/v1/auth/logout`

Headers:

```http
Authorization: Bearer <access-token>
```

Response:

```json
{
  "message": "logged out successfully"
}
```

Behavior:

- resolves the current JWT claims
- blacklists the current token JTI in Redis for the remaining TTL
- deletes the stored refresh token for the user

Typical responses:

- `200 OK`: logout completed
- `401 Unauthorized`: invalid or missing access token
- `503 Service Unavailable`: revocation storage unavailable

## Related Profile Endpoints

User profile endpoints are implemented under `/api/v1/users`:

- `GET /api/v1/users/me`
- `PUT /api/v1/users/me`
- `PUT /api/v1/users/me/password`

See [users.md](./users.md) for the profile contract.
