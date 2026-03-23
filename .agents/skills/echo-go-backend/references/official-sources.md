# Official Sources

Use these sources when you need up-to-date Echo guidance. The links below were checked on 2026-03-23.

## Version drift first

- Current official docs now show `github.com/labstack/echo/v5` in the primary Quick Start and Testing examples.
- This repository currently pins `github.com/labstack/echo/v4 v4.15.1` in `src-go/go.mod`.
- Before copying any example, verify:
  - import path and module major version
  - middleware package path and config names
  - handler signatures and helper availability
  - whether the example assumes a package that this repo does not use yet

If the task is not an explicit Echo upgrade, translate the docs into the repo's current compile surface instead of forcing v5-only APIs into a v4 codebase.

## Core docs

- Quick Start
  - https://echo.labstack.com/docs/quick-start
  - Use for current install path, minimal bootstrapping, basic middleware registration, and the clearest sign that the docs are currently written around v5.
- Routing
  - https://echo.labstack.com/docs/routing
  - Use for groups, route matching order, path and query parameters, wildcard routes, and route-level middleware patterns.
- Binding
  - https://echo.labstack.com/docs/binding
  - Use for source tags, bind order, direct bind helpers, and the security warning about mapping bound DTOs into separate business structs.
- Error Handling
  - https://echo.labstack.com/docs/error-handling
  - Use for centralized `HTTPErrorHandler`, committed-response checks, and custom error rendering strategy.
- Testing
  - https://echo.labstack.com/docs/testing
  - Use for `httptest`-based handler tests and the newer `echotest` helpers shown in current docs.

## Middleware docs

- Request Logger
  - https://echo.labstack.com/docs/middleware/logger
  - Use for `RequestLoggerWithConfig`, structured logging integration, and `HandleError` behavior.
- Recover
  - https://echo.labstack.com/docs/middleware/recover
  - Use for panic recovery behavior and recover configuration knobs.
- Rate Limiter
  - https://echo.labstack.com/docs/middleware/rate-limiter
  - Use for the default in-memory rate limiter, custom identifier extraction, and the official caution about high concurrency or large identifier sets.

## Optional package docs

- Echo JWT middleware package for v4
  - https://pkg.go.dev/github.com/labstack/echo-jwt/v4
  - Read only if the task explicitly evaluates moving from the repo's custom JWT middleware to the framework-provided package.
- JWT library used by this repo
  - https://pkg.go.dev/github.com/golang-jwt/jwt/v5
  - Read when token claims, signing, parsing, or expiry behavior is central to the task.
- PGX pool
  - https://pkg.go.dev/github.com/jackc/pgx/v5/pgxpool
  - Read when Postgres connection setup, pooling, or ping behavior is relevant.
- Redis client
  - https://pkg.go.dev/github.com/redis/go-redis/v9
  - Read when URL parsing, connection behavior, or cache command semantics matter.
- golang-migrate
  - https://pkg.go.dev/github.com/golang-migrate/migrate/v4
  - Read when startup migration policy or embedded migration behavior is involved.

## Practical reading order

1. Open Quick Start only to confirm current official version expectations.
2. Open Routing or Binding depending on whether the task changes endpoints or request parsing.
3. Open Error Handling and middleware pages when changing logging, panic recovery, auth, or shared response behavior.
4. Open Testing last to match the changed seam instead of introducing new helpers by default.
