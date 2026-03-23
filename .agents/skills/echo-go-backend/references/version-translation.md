# Version Translation

Use this file when the latest Echo docs and the current repository do not describe the same compile surface.

## Current mismatch

- Official docs checked on 2026-03-23 use `github.com/labstack/echo/v5` in Quick Start and Testing examples.
- This repository currently uses `github.com/labstack/echo/v4 v4.15.1`.

That means the docs are still valuable, but they are not paste-ready.

## Translate by feature, not by copy-paste

### Core import path

- Official docs may show `github.com/labstack/echo/v5`.
- This repository must stay on `github.com/labstack/echo/v4` until `src-go/go.mod` is intentionally upgraded.

### JWT guidance

- Official docs point to `github.com/labstack/echo-jwt/v5`.
- This repository currently uses a custom middleware in `src-go/internal/middleware/jwt.go`.
- Default framework middleware behavior is not a drop-in replacement here because this app also checks a token blacklist and stores custom claims in context.

### Request logging

- Official docs often present small logger examples for demonstration.
- This repository already uses `RequestLoggerWithConfig` plus `slog` in `src-go/internal/server/server.go`.
- Prefer adapting new logging behavior into that structured logger path instead of downgrading to a simple logger middleware example.

### Testing

- Official docs now show `echotest` helpers in addition to `httptest`.
- This repository already has a stable `httptest` pattern across `server_test.go`, `jwt_test.go`, and handler tests.
- Use `echotest` only when it materially simplifies a new test; do not introduce it just because the docs mention it.

### Error handling

- Official docs emphasize centralized `HTTPErrorHandler` and committed-response checks.
- This repository already has that seam in `src-go/internal/handler/errors.go`.
- Adapt new framework-level error behavior through that seam instead of scattering fallback JSON responses across unrelated middleware.

### WebSocket

- Official cookbook shows websocket examples but does not define this repository's auth handshake.
- This repository currently accepts a JWT from query string or Bearer header and uses `golang.org/x/net/websocket`.
- Preserve the client-facing handshake unless a broader migration is explicitly requested.

### Graceful shutdown

- Official cookbook demonstrates `http.Server#Shutdown()`.
- This repository already performs graceful shutdown from `src-go/cmd/server/main.go` with signal handling, timeout, and cleanup of DB and Redis connections.
- Extend that code path instead of adding a second lifecycle mechanism.

### Rate limiting

- Official docs note that the default memory store is aimed at correctness, not large-scale throughput.
- This repository currently uses the in-memory limiter for auth endpoints only.
- Keep that scope narrow unless the task also designs a persistent or distributed rate-limit store.

## Safe adaptation checklist

1. Confirm the local module major version in `src-go/go.mod`.
2. Compare the official example's package path and helper names with the local code.
3. Reuse the existing repository seam first.
4. Compile or test the changed package before trusting the translation.
