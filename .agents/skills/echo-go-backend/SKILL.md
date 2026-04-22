---
name: echo-go-backend
description: Use whenever working on a Go backend built with LabStack Echo, especially in `src-go` workspaces or repositories that use a layered `cmd`/`internal`/`pkg` layout. Covers route registration, handlers, middleware, binding, validation, error handling, auth, testing, migrations, and Tauri sidecar-aware backend changes. Make sure to apply this skill before adding or changing Echo routes, middleware, DTOs, services, repositories, or backend build/test flows, even if the user only mentions a small backend tweak.
---

# Go Echo Backend

Treat Echo work as version-sensitive, repo-specific backend work. Start from the repository's actual module major version and existing layering, then translate current official docs into that local shape instead of pasting generic snippets.

## First Pass

1. Inspect `src-go/go.mod` to confirm the installed Echo major version before using any online example.
2. Read `src-go/cmd/server/main.go`, `src-go/internal/server/server.go`, and `src-go/internal/server/routes.go` to locate bootstrap, global middleware, and route composition.
3. Classify the task before editing: bootstrap/config, route wiring, handler or DTO, service logic, repository or data, middleware, migration, or tests.
4. Reuse the existing seam before introducing a new package, a second framework style, or a parallel folder structure.

## Task Map

- New REST endpoint or route group: inspect `src-go/internal/server/routes.go`, the target handler, and `references/common-task-recipes.md`.
- Request binding, validation, DTO shape, or status-code mapping: inspect `src-go/internal/handler`, `src-go/internal/model`, and `references/common-task-recipes.md`.
- Cross-cutting middleware, request logging, timeout, CORS, or secure headers: inspect `src-go/internal/server/server.go` and `references/common-task-recipes.md`.
- JWT or protected-route work: inspect `src-go/internal/middleware/jwt.go`, `src-go/internal/service/auth_service.go`, and `references/version-translation.md`.
- JWT, refresh-token, Redis cache, PostgreSQL auth flow, migration, or degraded-mode startup work: inspect `src-go/internal/service/auth_service.go`, `src-go/internal/repository`, `src-go/pkg/database`, `src-go/internal/config/config.go`, and `references/auth-data-lifecycle.md`.
- WebSocket or long-lived connection work: inspect `src-go/internal/handler/ws.go`, the frontend client path, and `references/common-task-recipes.md`.
- Startup, shutdown, config, port, migration, or sidecar packaging work: inspect `src-go/cmd/server/main.go`, `src-go/internal/config/config.go`, `scripts/build-backend.sh`, and `src-tauri/src/lib.rs`.
- Frontend API contract, backend URL discovery, or desktop handoff work: inspect `hooks/use-backend-url.ts`, `lib/api-client.ts`, `next.config.ts`, `src-tauri/src/lib.rs`, and `references/frontend-desktop-integration.md`.

## Workflow

### 1. Preserve the repository template

- Keep `src-go/cmd/server/main.go` focused on config loading, dependency wiring, startup, and graceful shutdown.
- Keep `src-go/internal/server/server.go` responsible for the Echo instance, validator setup, global middleware, and centralized `HTTPErrorHandler`.
- Keep `src-go/internal/server/routes.go` responsible for route groups, route-level or group-level middleware, and handler assembly.
- Keep `src-go/internal/handler` thin: bind, validate, map request and response DTOs, and return HTTP status codes.
- Keep `src-go/internal/service` responsible for business rules and cross-repository orchestration.
- Keep `src-go/internal/repository` responsible for Postgres or Redis access details.
- Keep `src-go/pkg/database` and `src-go/migrations` as infrastructure seams instead of calling them directly from handlers.

### 2. Translate official docs through the installed version

- Official Echo docs checked on 2026-03-23 already show `github.com/labstack/echo/v5` in core examples.
- This repository currently depends on `github.com/labstack/echo/v4 v4.15.1`.
- Verify imports, helper APIs, middleware signatures, and test helpers against `src-go/go.mod` and local compile errors before adopting any doc example.
- When docs and repo differ, prefer repo-compatible changes unless the task is explicitly a version migration.

### 3. Add endpoints the repo-native way

- Add global middleware in `src-go/internal/server/server.go`.
- Add public, protected, or versioned groups in `src-go/internal/server/routes.go`.
- Add non-trivial request or response DTOs in `src-go/internal/model`.
- Instantiate handlers with explicit dependencies from bootstrap or route wiring; avoid global state or hidden singletons.
- Keep websocket, auth, and health seams grouped with the current server layout rather than scattering them across new packages.
- When adding a new resource surface, prefer extending the existing `/api/v1` grouping before inventing a parallel prefix.
- When a new feature needs both route wiring and business logic, change route -> handler -> service -> repository in that order so each layer stays explicit.

### 4. Bind and validate safely

- Bind into request DTOs, not persistence structs or service structs.
- Call `c.Bind(...)` and then `c.Validate(...)`, then map explicitly into service inputs.
- Use explicit status codes in handlers and let centralized error handling remain the safety net for uncaught failures.
- Remember that Echo binding can overwrite values when multiple sources are present; do not rely on combined binding for security-sensitive fields.
- If headers must be bound, use the direct header binder instead of assuming `Context.Bind` includes headers.
- Prefer handler-local validation responses for expected request-shape failures and reserve the centralized error handler for uncaught or framework-level failures.

### 5. Be deliberate about auth and middleware

- Reuse the existing JWT blacklist flow in `src-go/internal/middleware/jwt.go` when protecting routes in this repository.
- Do not replace the custom auth path with `echo-jwt` unless the task is specifically to migrate to framework middleware.
- Prefer group middleware for bounded protected surfaces and route-level middleware for narrow exceptions.
- Keep request logging aligned with the current structured logging setup rather than falling back to string-template logs.
- For rate limiting, remember the default in-memory store is fine for development or small traffic but the official docs caution against assuming it scales to high concurrency or very high identifier cardinality.
- When changing CORS, timeout, request logging, recover, or secure headers, verify the effect on existing route tests and on the desktop sidecar call path.
- For JWT changes, verify token extraction source, blacklist semantics, and the exact unauthorized status behavior before assuming framework defaults match the current app.
- For auth and data changes, remember this repository is designed to start in degraded mode when Postgres or Redis are unavailable; do not accidentally turn optional development dependencies into hard startup failures unless the task explicitly changes that contract.

### 6. Test and verify the changed seam

- Handler or middleware work: use `httptest`, `echo.New()`, `e.NewContext(...)`, and targeted package tests.
- Service or repository work: keep tests at their layer; do not force Echo into pure business logic tests.
- Build or sidecar changes: also verify the backend build path used by `pnpm build:backend` or `pnpm build:backend:dev`.
- Run the narrowest useful command first, then a broader check if the first one passes.
- Treat docs examples that mention `echotest` as optional guidance; do not introduce new helpers if the repo's current tests already cover the seam cleanly.
- Reuse existing tests such as `src-go/internal/server/server_test.go`, `src-go/internal/handler/errors_test.go`, and `src-go/internal/middleware/jwt_test.go` as style anchors before inventing a new test style.

### 7. Keep desktop integration in mind

- This repository's Tauri app expects the Go backend as a sidecar built from `scripts/build-backend.sh`.
- Port defaults and `--port` override live in `src-go/cmd/server/main.go` and must stay compatible with the frontend or Tauri caller.
- If changing CORS, ports, health endpoints, or binary outputs, verify both `src-go` and `src-tauri` touch points.
- The current websocket handler accepts the token from either `?token=` or `Authorization: Bearer ...`; preserve that contract unless the task explicitly changes the client handshake.
- In browser mode, frontend code falls back to `NEXT_PUBLIC_API_URL` and then `http://localhost:7777`; backend changes that affect origin, path prefixes, or response shape must still work without Tauri.
- In desktop mode, the frontend resolves the backend URL through the Tauri `get_backend_url` command, so Rust-managed port or base-URL changes must stay synchronized with `hooks/use-backend-url.ts` and `lib/api-client.ts`.

## Guardrails

- Do not import examples from Gin, Chi, Fiber, or `net/http` mux patterns into Echo code unless the task is an explicit migration.
- Do not move business rules into handlers just because an Echo example is small.
- Do not add a second validation stack when the repo already uses `go-playground/validator`.
- Do not widen middleware scope without checking which groups and routes already inherit parent middleware.
- Do not claim a docs snippet is safe for this repo until the local major version, imports, and tests agree.
- Do not migrate websocket, JWT, or test helpers to a new package family just because the latest docs showcase a different default.
- Do not change API path prefixes, websocket handshake format, health endpoint shape, or backend port assumptions without tracing the frontend and Tauri consumers first.
- Do not silently change degraded-mode behavior for missing Postgres, missing Redis, missing `JWT_SECRET`, refresh-token persistence, or token blacklist semantics.

## References

- Read `references/project-template.md` when you need the repository's actual backend structure, commands, and Tauri coupling.
- Read `references/official-sources.md` when you need current Echo docs links, version-drift notes, or official guidance on routing, binding, error handling, testing, and middleware.
- Read `references/common-task-recipes.md` when you need a concrete mapping from task type to files, docs, and verification commands in this repository.
- Read `references/version-translation.md` when the latest Echo docs and the local installed version do not line up cleanly.
- Read `references/frontend-desktop-integration.md` when a backend change could affect Next.js fetch calls, websocket URLs, Tauri commands, sidecar startup, static export, or environment-based URL discovery.
- Read `references/auth-data-lifecycle.md` when the task touches Postgres, Redis, migrations, JWT issuance, refresh-token rotation, blacklist behavior, repo-level degraded mode, or auth-related test strategy.
