# Local Run Matrix

Use this file when the task is to start, build, or smoke-test the project locally.

## Prerequisites

- Node.js 20+ is documented for local use. GitHub Actions currently uses Node 22.x.
- `pnpm` is the package manager used by the repo scripts.
- Go must satisfy `src-go/go.mod`, which currently declares `go 1.25.0`.
- Rust must satisfy `src-tauri/Cargo.toml`, which currently declares `rust-version = "1.77.2"`.
- Docker Compose is the expected way to bring up Postgres and Redis for backend-backed local work.

## Web-Only Local Development

1. Run `pnpm install`.
2. Run `pnpm dev`.
3. Open `http://localhost:3000`.

Notes:

- The web app reads `NEXT_PUBLIC_API_URL` from `.env.local`, then falls back to `http://localhost:7777`.
- Use `.env.local.example` as the starting point when the frontend must call a non-default backend URL.

## Backend-Only Local Development

1. Run `docker compose up -d postgres redis`.
2. Copy `src-go/.env.example` to `src-go/.env` and adjust values if needed.
3. Run `cd src-go && go run ./cmd/server`.

Notes:

- The default backend port is `7777`.
- The backend health surfaces are registered under `/health` and `/api/v1/health`.
- `src-go/.env.example` currently expects `ALLOW_ORIGINS=http://localhost:3000,tauri://localhost`.

## Desktop Development

Preferred path:

1. Ensure frontend dependencies are installed.
2. Ensure the Go toolchain and Rust toolchain are available.
3. Run `pnpm tauri:dev`.

What this does:

- Runs `pnpm build:backend:dev`
- Then runs `pnpm tauri dev`
- `src-tauri/tauri.conf.json` starts the frontend dev server via `beforeDevCommand: "pnpm dev"`
- `src-tauri/src/lib.rs` spawns the Go sidecar named `server` on port `7777`

Windows warning:

- `pnpm build:backend:dev` shells out to `bash scripts/build-backend.sh --current-only`.
- If `/bin/bash` is missing, the helper command fails before Tauri starts.
- In that case, use direct Go commands for narrow verification and explain that a real Tauri run still needs a sidecar binary that matches the script's output naming under `src-tauri/binaries/`.

## Production-Oriented Local Builds

- Static web assets: `pnpm build`
- Desktop bundle: `pnpm tauri:build`
- Go binary fallback check: `cd src-go && go build ./cmd/server`

Notes:

- `next.config.ts` sets `output: "export"`, so `pnpm build` writes static assets into `out/`.
- `src-tauri/tauri.conf.json` points `frontendDist` at `../out`.
- `src-tauri/tauri.conf.json` uses `beforeBuildCommand: "pnpm build:backend && pnpm build"`.
