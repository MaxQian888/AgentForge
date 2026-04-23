---
title: "Spec 2: API Key Management"
date: 2026-04-23
status: draft
depends_on: [1]
---

# API Key Management

## Problem

There is no way for external systems or CLI tools to authenticate against the AgentForge API. All access requires JWT tokens obtained through the login flow, which is unsuitable for automation, CI/CD integrations, and third-party tooling.

## Current State

- JWT auth with access/refresh tokens for browser sessions
- Per-project secret storage (for user-managed credentials, not API access)
- No programmatic access tokens
- No token scoping or rotation

## Design

### Data Model

```sql
CREATE TABLE api_keys (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id        UUID REFERENCES organizations(id),        -- nullable for personal keys
  user_id       UUID NOT NULL REFERENCES users(id),
  name          VARCHAR(128) NOT NULL,
  key_prefix    VARCHAR(8) NOT NULL,                       -- first 8 chars for identification
  key_hash      VARCHAR(128) NOT NULL,                     -- SHA-256 of the full key
  scopes        JSONB NOT NULL DEFAULT '[]',               -- e.g. ["projects:read", "tasks:write"]
  expires_at    TIMESTAMPTZ,                               -- nullable = no expiry
  last_used_at  TIMESTAMPTZ,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  revoked_at    TIMESTAMPTZ
);

CREATE INDEX idx_api_keys_user ON api_keys(user_id);
CREATE INDEX idx_api_keys_prefix ON api_keys(key_prefix) WHERE revoked_at IS NULL;
```

### Key Format

```
af_live_<32_random_bytes_base64url>   — production keys
af_test_<32_random_bytes_base64url>   — test-mode keys (limited scope)
```

The full key is shown **once** at creation time and never stored. Only `key_hash` (SHA-256) is persisted.

### Scope System

Scopes follow the pattern `resource:action`:

| Scope | Description |
|-------|-------------|
| `projects:read` | List and view projects |
| `projects:write` | Create and update projects |
| `tasks:read` | View tasks |
| `tasks:write` | Create, update, transition tasks |
| `agents:read` | View agent status |
| `agents:write` | Spawn, manage agents |
| `reviews:read` | View reviews |
| `reviews:write` | Approve, reject reviews |
| `workflows:read` | View workflow definitions |
| `workflows:write` | Create, execute workflows |
| `knowledge:read` | Search and view knowledge assets |
| `knowledge:write` | Create and modify knowledge assets |
| `secrets:read` | Read secret names (never values) |
| `admin` | Full access within scoped org |

Keys are scoped to either an org (can access all org projects) or personal (only the user's own projects).

### API Endpoints

```
# Key management (requires auth via JWT, not API key)
POST   /api/v1/api-keys                     — Create key
GET    /api/v1/api-keys                     — List my keys (masked)
GET    /api/v1/api-keys/:id                 — Get key metadata
PUT    /api/v1/api-keys/:id                 — Update key (name, scopes)
DELETE /api/v1/api-keys/:id                 — Revoke key
POST   /api/v1/api-keys/:id/rotate          — Rotate (revoke old, return new)
```

### Auth Middleware

New middleware: `APIKeyAuth` — runs **before** JWT auth in the middleware chain:

```
APIKeyAuth (checks Authorization: Bearer af_live_...) → JWT Auth (session tokens) → RBAC
```

If the `Authorization` header starts with `af_live_` or `af_test_`, the middleware:
1. Hashes the provided key with SHA-256
2. Looks up `api_keys` by `key_hash` where `revoked_at IS NULL`
3. Checks `expires_at` if set
4. Loads the user and their org memberships
5. Verifies the requested action matches the key's `scopes`
6. Updates `last_used_at`

### Rate Limiting

API keys get their own rate limit bucket separate from JWT auth:
- Default: 100 requests/minute per key
- Configurable per key at creation time
- Redis-based sliding window counter

### Audit

Every API key request logs:
- Key ID and prefix
- User ID
- Request path and method
- Response status code
- Timestamp

Logged to the existing `audit_events` table with a new source type `api_key`.

### Frontend

**New pages:**
- Settings → API Keys tab (list, create, revoke, rotate)
- Org Settings → API Keys tab (org-level keys)

**New components:**
- `ApiKeyCreateDialog` — name, scopes selector, expiry picker, org selector
- `ApiKeyList` — table with prefix, name, scopes, last used, created, actions
- `ApiKeyRevealBanner` — one-time display of full key with copy button

### Security Considerations

- Keys are hashed with SHA-256 (same pattern as GitHub personal access tokens)
- Key rotation creates a new key and immediately revokes the old one
- Rate limiting prevents brute-force discovery
- Scope enforcement is AND'd with RBAC — a key's scopes can only narrow, never widen, the user's permissions
- Org admin can view and revoke any key in their org
- Platform admin can view and revoke any key
