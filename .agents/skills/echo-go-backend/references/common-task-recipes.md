# Common Task Recipes

Use this file when the task is concrete and you need to know where Echo work should land in this repository.

## Add a new public REST endpoint

1. Add or extend the route in `src-go/internal/server/routes.go`.
2. Add a thin handler method under `src-go/internal/handler`.
3. Add request or response DTOs in `src-go/internal/model` if the shape is not already defined.
4. Add or extend the service method in `src-go/internal/service`.
5. Add narrow handler or route tests:
   - `cd src-go && go test ./internal/handler ./internal/server -count=1`

Use official docs:
- Routing
- Binding
- Error Handling

## Add a protected endpoint

1. Check whether the route belongs under the existing `users := v1.Group("/users", jwtMw)` pattern or a sibling protected group.
2. Reuse `appMiddleware.JWTMiddleware(cfg.JWTSecret, cache)` from `src-go/internal/middleware/jwt.go`.
3. Extract claims with `middleware.GetClaims(c)` only inside handlers that truly require identity.
4. Preserve blacklist behavior and current Bearer-token semantics.
5. Run:
   - `cd src-go && go test ./internal/middleware ./internal/server ./internal/handler -count=1`

Use official docs:
- Routing groups
- JWT middleware page only for comparison, not as a default replacement

## Change request binding or validation

1. Inspect the target request DTO in `src-go/internal/model`.
2. Bind with `c.Bind(...)`.
3. Validate with `c.Validate(...)`.
4. Map into service input explicitly instead of reusing a persistence struct.
5. Add handler tests that exercise bad JSON, validation errors, and success paths.
6. Run:
   - `cd src-go && go test ./internal/handler -count=1`

Use official docs:
- Binding
- Error Handling

## Add or change global middleware

1. Edit `src-go/internal/server/server.go`.
2. Preserve middleware ordering unless you have a concrete reason to change it.
3. For structured request logs, stay on `RequestLoggerWithConfig` and `slog`.
4. For CORS or timeout changes, also inspect frontend and Tauri call paths.
5. Add or update route-level behavior tests in `src-go/internal/server/server_test.go` or a sibling test.
6. Run:
   - `cd src-go && go test ./internal/server -count=1`

Use official docs:
- Request Logger
- Recover
- Rate Limiter

## Change centralized error handling

1. Edit `src-go/internal/handler/errors.go`.
2. Preserve committed-response checks.
3. Preserve `model.ErrorResponse` unless the whole API contract is intentionally changing.
4. Add or update `src-go/internal/handler/errors_test.go`.
5. Run:
   - `cd src-go && go test ./internal/handler -count=1`

Use official docs:
- Error Handling

## Change websocket behavior

1. Inspect `src-go/internal/handler/ws.go`.
2. Preserve the current token handshake unless the client contract is changing.
3. If the transport library changes, verify the whole frontend and Tauri interaction path, not just the handler compile surface.
4. Add route or handler tests for unauthenticated and authenticated paths where practical.
5. Run:
   - `cd src-go && go test ./internal/server ./internal/handler -count=1`

Use official docs:
- WebSocket cookbook

## Change an API contract consumed by the frontend or desktop app

1. Inspect `lib/api-client.ts`, `hooks/use-backend-url.ts`, and `references/frontend-desktop-integration.md`.
2. Check whether the change touches:
   - path prefix
   - health endpoints
   - error JSON shape
   - Bearer-token header expectations
   - websocket handshake
   - backend URL or port discovery
3. If desktop mode is involved, inspect `src-tauri/src/lib.rs`, `src-tauri/tauri.conf.json`, and `src-tauri/capabilities/default.json`.
4. Verify both the backend seam and the consumer seam.
5. Run:
   - `cd src-go && go test ./internal/server ./internal/handler -count=1`
   - `pnpm test -- lib/api-client.test.ts`
   - `pnpm build:backend:dev`

## Change startup, shutdown, config, or port behavior

1. Inspect `src-go/cmd/server/main.go` first.
2. Inspect `src-go/internal/config/config.go` for defaults and env parsing.
3. Inspect `scripts/build-backend.sh` and `src-tauri/src/lib.rs` if the output binary, port, or lifecycle contract changes.
4. Re-run the narrowest backend test set first, then the build path if needed.
5. Run:
   - `cd src-go && go test ./... -count=1`
   - `pnpm build:backend:dev`

Use official docs:
- Quick Start
- Graceful Shutdown cookbook

## Change database or cache-backed auth flows

1. Trace handler -> service -> repository before editing.
2. Keep repository-specific failures in repository or service layers.
3. Preserve user-facing HTTP mapping in handlers.
4. Prefer service tests and repository tests over forcing end-to-end HTTP coverage for every branch.
5. Load `references/auth-data-lifecycle.md` before changing degraded startup behavior, refresh-token storage, blacklist semantics, or migration policy.
6. Run:
   - `cd src-go && go test ./internal/service ./internal/repository -count=1`
   - `cd src-go && go test ./pkg/database -count=1`

Use official docs:
- Binding for request ingress only
- Error Handling for uncaught framework-level failures
