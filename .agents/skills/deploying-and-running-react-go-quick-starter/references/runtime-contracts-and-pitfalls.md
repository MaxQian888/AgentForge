# Runtime Contracts And Pitfalls

Use this file when a command path looks inconsistent, when a runtime fails to start, or when a deployment plan needs the repo-specific contracts instead of generic framework advice.

## Current Verified Command Behavior

The following behaviors were re-checked against the local repository snapshot while authoring this skill:

- `pnpm build` succeeds and generates static export output.
- `pnpm start` fails with: `Error: "next start" does not work with "output: export" configuration. Use "npx serve@latest out" instead.`
- `pnpm build:backend:dev` fails on this Windows machine when `/bin/bash` is unavailable.
- `cd src-go && go build ./cmd/server` succeeds as a narrow backend fallback check.
- `docker compose config` resolves successfully, but warns that the top-level `version` field is obsolete.

Re-check these if the repo changes.

## Port And URL Contract

- `hooks/use-backend-url.ts`
  - Browser mode uses `process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777"`.
  - Tauri mode calls the Rust command `get_backend_url`.
- `.env.local.example`
  - Documents `NEXT_PUBLIC_API_URL=http://localhost:7777`.
- `src-go/.env.example`
  - Defaults the backend port to `7777` and allows browser plus Tauri origins.
- `src-tauri/src/lib.rs`
  - Hardcodes the Tauri sidecar port as `7777`.

If the backend port changes, update all four surfaces together.

## Build Output Contract

- `next.config.ts`
  - Uses `output: "export"` and writes deployable assets into `out/`.
- `src-tauri/tauri.conf.json`
  - Uses `devUrl: "http://localhost:3000"`
  - Uses `frontendDist: "../out"`
  - Uses `beforeBuildCommand: "pnpm build:backend && pnpm build"`
  - Expects `externalBin: ["binaries/server"]`
- `scripts/build-backend.sh`
  - Writes sidecar binaries into `src-tauri/binaries/`
  - Names them as `server-<target-triple>` with `.exe` on Windows

If the binary name or location changes, update the script and Tauri config together.

## Common Failure Modes

### Generic Next.js production advice

Symptom:

- Someone proposes `pnpm start` or `next start` after `pnpm build`

Reality:

- This repo uses static export, so production delivery is based on `out/`, not a Next.js Node server.

### Windows sidecar build failure

Symptom:

- `pnpm build:backend` or `pnpm build:backend:dev` exits before compiling the Go binary

Reality:

- The repo script requires `bash`. On Windows without WSL or Git Bash on the expected path, use direct Go commands for diagnosis and call out the packaging dependency explicitly.

### Desktop app starts without working backend

Symptom:

- The Tauri shell launches but API calls fail or the sidecar is missing

Reality:

- The sidecar binary contract is `binaries/server` plus target-specific suffixes, and the Rust layer expects port `7777`.

### Backend starts but frontend still cannot reach it

Symptom:

- Browser requests still point to the wrong API origin

Reality:

- Check `.env.local`, `NEXT_PUBLIC_API_URL`, and `hooks/use-backend-url.ts` before changing server code.
