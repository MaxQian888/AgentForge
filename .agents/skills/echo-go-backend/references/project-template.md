# Project Template

Use this file when the task targets this repository's existing Go backend instead of a blank Echo app.

## Current layout

- `src-go/cmd/server/main.go`
  Loads config, applies `--port` overrides, configures `slog`, opens Postgres and Redis, wires repositories and services, creates Echo, registers routes, and handles graceful shutdown.
- `src-go/internal/server/server.go`
  Builds the Echo instance, registers the validator and centralized error handler, and applies global middleware such as `Recover`, `RequestID`, `RequestLoggerWithConfig`, `CORS`, `Secure`, `Gzip`, and `Timeout`.
- `src-go/internal/server/routes.go`
  Registers `/health`, `/api/v1/health`, auth endpoints, protected user endpoints, and `/ws`. Use this as the first stop for new groups, route-local middleware, or versioned APIs.
- `src-go/internal/handler`
  Contains thin HTTP handlers such as health, auth, websocket, and shared error shaping.
- `src-go/internal/middleware/jwt.go`
  Implements the current Bearer-token validation and blacklist lookup path. Reuse it when adding protected routes.
- `src-go/internal/service`
  Holds business logic such as auth flows, token issuance, refresh, and logout orchestration.
- `src-go/internal/repository`
  Holds Postgres and Redis-facing data access seams.
- `src-go/pkg/database`
  Holds shared database connection and migration helpers.
- `src-go/migrations`
  Holds SQL migrations and embedded migration assets.

## Existing route shape

- Public health: `GET /health`
- Versioned health: `GET /api/v1/health`
- Public auth: `POST /api/v1/auth/register`, `POST /api/v1/auth/login`, `POST /api/v1/auth/refresh`
- Protected auth: `POST /api/v1/auth/logout`
- Protected user info: `GET /api/v1/users/me`
- WebSocket: `GET /ws`

When adding new authenticated API endpoints, prefer extending the existing `/api/v1` grouping and reusing the current JWT middleware instead of inventing a separate auth surface.

## Existing backend conventions

- Keep handler methods small and return HTTP responses directly.
- Keep domain or persistence mapping out of `c.Bind(...)` targets.
- Reuse `model.ErrorResponse` or the repo's existing response shape for errors.
- Let the centralized error handler remain the catch-all, but still return precise handler-level errors for expected cases such as validation, unauthorized access, or conflicts.
- Keep structured request logging aligned with `slog`.

## Existing verification commands

- `cd src-go && go test ./... -count=1`
- `cd src-go && go test ./... -v -count=1`
- `cd src-go && go run ./cmd/server`
- `cd src-go && go mod tidy`
- `cd src-go && make test`
- `cd src-go && make lint`
- `pnpm build:backend`
- `pnpm build:backend:dev`
- `pnpm tauri:dev`

On this repository, the root build scripts call `bash scripts/build-backend.sh`. If the shell environment cannot run `bash`, fall back to direct `go` commands inside `src-go` for narrow verification before touching the Tauri path.

## Config and environment touch points

- Backend env defaults live in `src-go/internal/config/config.go`.
- Example local env values live in `src-go/.env.example`.
- Development dependencies for Postgres and Redis are documented in `docker-compose.yml`.
- Tauri reads the backend through `src-tauri/src/lib.rs`, which currently assumes port `7777`.

If a change affects ports, origins, health endpoints, or build outputs, inspect both `src-go` and `src-tauri` before declaring the integration complete.
