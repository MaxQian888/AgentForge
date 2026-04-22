# Redis Schema / Redis Key Pattern 与 TTL 策略

This document consolidates the Redis usage that is currently implemented in code
and the adjacent runtime patterns already documented in the repository.

## Source Of Truth

- `src-go/internal/repository/cache.go`
- `src-go/pkg/database/redis.go`
- `docs/architecture/data-realtime-design.md`

## Current Implemented Keys

The cache repository used by auth and dashboard widgets currently persists:

| Key pattern | Type | TTL | Purpose |
| --- | --- | --- | --- |
| `refresh:{userID}` | String | `JWT_REFRESH_TTL` | store the active refresh token for a user |
| `blacklist:{jti}` | String (`"1"`) | remaining access-token TTL | revoke a JWT access token |
| `widget:{key}` | String | caller-defined | cache computed widget payloads |

## Authentication Semantics

### `refresh:{userID}`

- set by `SetRefreshToken`
- read by `GetRefreshToken`
- deleted by `DeleteRefreshToken`
- must match the presented refresh token during `/api/v1/auth/refresh`

### `blacklist:{jti}`

- set by `BlacklistToken`
- checked by `IsBlacklisted`
- TTL is aligned with the remaining lifetime of the access token being revoked
- used to fail closed on logout/revocation-sensitive paths

## Documented Runtime Patterns

`docs/part/DATA_AND_REALTIME_DESIGN.md` also defines the naming patterns used by
the broader runtime architecture:

| Key pattern | Type | TTL / retention | Purpose |
| --- | --- | --- | --- |
| `af:task:queue` | Stream | queue retention policy | task dispatch queue |
| `af:agent:pool:{project_id}` | Hash / JSON blob | runtime-managed | per-project agent-pool state |
| `af:session:{session_id}` | Hash | `24h` | agent session cache and resume metadata |
| `af:ws:channel:{channel_name}` | Pub/Sub channel | ephemeral | websocket fan-out |
| `af:rate:{endpoint}:{client_id}` | Counter | short-lived | rate limiting |
| `af:lock:{resource}` | Lock key | short-lived | distributed coordination |
| `af:cache:{entity}:{id}` | String / JSON | caller-defined | generic cached entity data |

These patterns are important because some runtime surfaces are already designed
around them even if a given key family is not yet wrapped by the current
`CacheRepository`.

## TTL Strategy

- refresh tokens live for `JWT_REFRESH_TTL`
- blacklist entries live for the remaining access-token lifetime
- widget cache TTL is chosen by the caller
- session cache in the design docs uses `24h`

## Operational Guidance

- do not silently bypass Redis for revocation-sensitive auth flows
- prefer explicit prefixes for every new key family
- keep TTL tied to the lifecycle of the resource being modeled
- document any new key family in this file and in the relevant runtime doc
