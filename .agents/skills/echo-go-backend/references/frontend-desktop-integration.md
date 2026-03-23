# Frontend Desktop Integration

Use this file when a Go Echo backend change can affect the Next.js frontend, the Tauri desktop shell, or both.

## Backend URL contract

There are two active modes in this repository:

- Web or separated mode
  - `hooks/use-backend-url.ts` returns `process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777"`.
  - That means backend route, CORS, and response-shape changes must still work when the frontend is not running inside Tauri.
- Tauri desktop mode
  - `hooks/use-backend-url.ts` dynamically imports `@tauri-apps/api/core` and invokes `get_backend_url`.
  - `src-tauri/src/lib.rs` currently manages that URL as `http://localhost:7777`.

If you change backend port, base URL, or how the desktop app discovers the backend, update the Rust command surface and the frontend hook together.

## API client contract

`lib/api-client.ts` is the frontend contract layer for HTTP and websocket access.

- HTTP requests always join `baseUrl` and `path` after trimming a trailing slash.
- Requests default to `Content-Type: application/json`.
- Authenticated requests send `Authorization: Bearer <token>`.
- Errors are normalized into `ApiError` and try to read `body.message`.
- `wsUrl(path, token?)` converts `http -> ws` and `https -> wss`, then appends `?token=` when a token exists.

Backend implications:

- Preserve `message` in error JSON if you want frontend error handling to stay informative.
- Preserve Bearer-token semantics unless you also update the frontend caller layer.
- Preserve websocket token-via-query support unless the client handshake is intentionally redesigned.

## Health and route shape

Current backend route expectations include:

- `GET /health`
- `GET /api/v1/health`
- auth endpoints under `/api/v1/auth/...`
- user identity endpoint under `/api/v1/users/me`
- websocket endpoint at `/ws`

If you rename, move, or version these routes differently, search both frontend and desktop consumers before assuming the change is isolated.

## Desktop sidecar contract

`src-tauri/src/lib.rs` currently does all of the following:

- exposes `get_backend_url` to the frontend
- stores backend URL in managed state
- spawns the Go sidecar named `server`
- passes `--port 7777`
- logs sidecar stdout and stderr

`src-tauri/capabilities/default.json` currently grants shell sidecar execution for `server`.

`src-tauri/tauri.conf.json` currently expects:

- `build.devUrl = "http://localhost:3000"`
- `build.frontendDist = "../out"`
- `build.beforeBuildCommand = "pnpm build:backend && pnpm build"`
- `bundle.externalBin = ["binaries/server"]`

If you rename the binary, change the output location, or alter startup arguments, update the Rust code, Tauri config, capability, and build script together.

## Build and export coupling

`next.config.ts` uses:

- `output: "export"`
- `images.unoptimized = true`
- `assetPrefix` based on `TAURI_DEV_HOST`

That means the desktop production build depends on Next.js static export into `out/`, and the dev experience depends on a stable dev server URL.

Backend-adjacent implications:

- Do not assume the desktop app talks to a Next.js API route; it talks to the Go backend directly.
- Do not move backend-only runtime config into a build-time-only frontend variable unless you intend to freeze it at build time.

## Verification checklist

When backend changes may affect consumers, verify at least the relevant subset:

- Go route or handler tests:
  - `cd src-go && go test ./internal/server ./internal/handler -count=1`
- Frontend API client tests:
  - `pnpm test -- lib/api-client.test.ts`
- Backend sidecar build:
  - `pnpm build:backend:dev`
- Desktop startup path when relevant:
  - `pnpm tauri:dev`

If the change is only an internal service or repository refactor, you usually do not need the frontend or Tauri checks. If the change affects ports, URLs, path prefixes, error JSON, auth headers, websocket handshake, health endpoints, or startup arguments, you usually do.
