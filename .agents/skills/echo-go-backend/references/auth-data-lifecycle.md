# Auth Data Lifecycle

Use this file when the task touches authentication, Postgres, Redis, migrations, config defaults, or backend startup behavior.

## Current runtime contract

This repository does not require every dependency to be present before the process starts.

- `JWT_SECRET`
  - In production, missing `JWT_SECRET` is fatal.
  - In development, `src-go/cmd/server/main.go` falls back to an insecure dev secret so the server can still start.
- PostgreSQL
  - `database.NewPostgres` returns an error when `POSTGRES_URL` is missing, malformed, or unreachable.
  - `main.go` logs a warning and continues with `db = nil`.
  - That means the server can boot in degraded mode, but auth flows backed by Postgres will not work.
- Redis
  - `database.NewRedis` returns an error for parse or ping failures.
  - `main.go` logs a warning and continues with `rdb = nil`.
  - Token cache features can therefore be unavailable without stopping the whole server.

If you change these semantics, treat it as an explicit product decision, not a small refactor.

## Data and auth boundaries

### Postgres boundary

- Connection helper: `src-go/pkg/database/postgres.go`
- User persistence: `src-go/internal/repository/user_repo.go`
- Sentinel repository error for missing DB: `repository.ErrDatabaseUnavailable`

Repository methods return wrapped errors. Service code is expected to decide whether an error becomes:

- user-facing conflict or invalid credentials
- invalid token
- generic internal failure

Do not leak raw driver errors directly to HTTP responses.

### Redis boundary

- Connection helper: `src-go/pkg/database/redis.go`
- Token cache implementation: `src-go/internal/repository/cache.go`
- Sentinel repository error for missing cache: `repository.ErrCacheUnavailable`

Current Redis responsibilities:

- store refresh token by `refresh:<userID>`
- delete refresh token on logout or token rotation
- blacklist access-token JTIs by `blacklist:<jti>`

Important current nuance:

- `CacheRepository.IsBlacklisted` fail-opens when the Redis client is nil and returns `false, nil`.
- That is different from write paths such as `SetRefreshToken` or `BlacklistToken`, which return `ErrCacheUnavailable` when Redis is absent.

Preserve this asymmetry unless the task explicitly changes the degraded-mode security model.

## JWT lifecycle

JWT logic lives in `src-go/internal/service/auth_service.go` and `src-go/internal/middleware/jwt.go`.

### Access token

- signed with `HS256`
- contains custom claims: `sub`, `email`, `jti`
- TTL uses `cfg.JWTAccessTTL`
- JTI is blacklisted on logout

### Refresh token

- also signed as JWT
- TTL uses `cfg.JWTRefreshTTL`
- stored in Redis keyed by user ID
- refresh flow requires both:
  - valid JWT signature and claims
  - exact stored refresh token match in cache

### Rotation behavior

- `Refresh(...)` parses and validates the refresh token
- checks stored refresh token equality
- reloads the user from Postgres
- deletes the old refresh token before issuing a fresh pair

### Logout behavior

- blacklists the current access token JTI for the remaining access-token TTL
- deletes the stored refresh token

If you change token claims, TTL semantics, key naming, or refresh rotation order, audit handler mapping, middleware extraction, and client expectations together.

## Migration boundary

- Embedded SQL migrations live under `src-go/migrations`
- `src-go/migrations/embed.go` exports `migrations.FS`
- `src-go/pkg/database/migrate.go` uses `golang-migrate` with `iofs`
- `main.go` runs migrations only when Postgres is available

Current behavior:

- `migrate.ErrNoChange` is treated as success
- other migration failures are logged as warnings, not fatal startup errors

Do not silently convert migration warnings into fatal boot errors unless the task explicitly changes deployment policy.

## Config defaults that affect auth and data flows

`src-go/internal/config/config.go` currently defaults:

- `PORT=7777`
- `ENV=development`
- `JWT_ACCESS_TTL=15m`
- `JWT_REFRESH_TTL=168h`
- `ALLOW_ORIGINS=http://localhost:3000,tauri://localhost,http://localhost:1420`
- `REDIS_URL=redis://localhost:6379`

When a change touches auth or backend connectivity, verify whether it should modify:

- env defaults
- `.env.example`
- `docker-compose.yml`
- Tauri port or frontend base URL assumptions

## Error mapping strategy

Error handling is layered, not uniform:

- repository layer returns wrapped DB or cache errors
- service layer converts some cases to sentinel domain errors:
  - `ErrEmailAlreadyExists`
  - `ErrInvalidCredentials`
  - `ErrInvalidToken`
- handler layer maps those sentinel errors to HTTP status codes
- centralized Echo `HTTPErrorHandler` remains a safety net for framework-level or uncaught failures

When changing auth or data logic:

- prefer updating service sentinels and handler mapping deliberately
- avoid letting new repository errors leak through as accidental raw messages
- preserve `model.ErrorResponse` shape so frontend `ApiError` extraction still works

## Test strategy anchors

Use existing tests as style and coverage anchors:

- `src-go/internal/service/auth_service_test.go`
  - mock repositories
  - success and failure branches for register, login, refresh, logout
- `src-go/internal/handler/auth_test.go`
  - handler-level status mapping and validator setup
- `src-go/internal/middleware/jwt_test.go`
  - Bearer header parsing, blacklist behavior, invalid token handling
- `src-go/pkg/database/postgres_test.go`
  - unit tests for bad URLs and unreachable hosts
- `src-go/pkg/database/postgres_integration_test.go`
  - integration test gated by `TEST_POSTGRES_URL`
- `src-go/pkg/database/migrate_test.go`
  - migration helper failure cases
- `src-go/internal/repository/user_repo_integration_test.go`
  - integration path plus migration bootstrap

Prefer the narrowest relevant test set first. For example:

- auth service only:
  - `cd src-go && go test ./internal/service -count=1`
- auth handler plus middleware:
  - `cd src-go && go test ./internal/handler ./internal/middleware ./internal/server -count=1`
- connection or migration helper:
  - `cd src-go && go test ./pkg/database -count=1`

## Optional package docs

- JWT library
  - https://pkg.go.dev/github.com/golang-jwt/jwt/v5
- PostgreSQL pool
  - https://pkg.go.dev/github.com/jackc/pgx/v5/pgxpool
- Redis client
  - https://pkg.go.dev/github.com/redis/go-redis/v9
- Migration library
  - https://pkg.go.dev/github.com/golang-migrate/migrate/v4
