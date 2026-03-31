# Testing Guide

AgentForge no longer has a single "one command covers everything" test surface.
The repository is split across a Next.js frontend, a Go orchestrator, a Bun-based
bridge, an IM bridge workspace, and a Tauri desktop shell. Use the verification
entrypoint that matches the surface you changed.

## Current Test Surfaces

### 1. Root Web/UI Surface

- Runner: Jest 30 via `next/jest`
- Environment: `jsdom`
- Primary scope: `app/`, `components/`, `hooks/`, and `lib/`
- Coverage output: `coverage/`

This is the main test surface for the Next.js dashboard and shared frontend
logic. It is also the surface used by `.github/workflows/test.yml`.

### 2. Go Orchestrator

- Workspace: `src-go/`
- Primary entrypoint: `go test ./...`
- CI integration path: `go test ./... -tags integration -count=1 -v`

Use this for backend handlers, services, repositories, scheduler logic, auth,
and Go-side plugin/runtime behavior.

### 3. TypeScript Bridge

- Workspace: `src-bridge/`
- Primary entrypoints:
  - `bun test <paths>`
  - `bun run typecheck`

The bridge currently does not expose a dedicated `test` script in
`src-bridge/package.json`, so focused `bun test` commands are the expected path.

### 4. IM Bridge

- Workspace: `src-im-bridge/`
- Primary entrypoints:
  - `go test ./...`
  - `go build ./...`

Use this for messaging platform adapters, rendering, rate limiting, native
payload handling, and platform metadata logic.

### 5. Desktop / Tauri Shell

- Workspace: `src-tauri/`
- Primary verification path today:
  - `pnpm test:tauri`
  - `pnpm test:tauri:coverage`
  - `pnpm tauri:dev`
  - `pnpm tauri:build`
  - `.github/workflows/desktop-tauri-logic.yml`
  - `.github/workflows/build-tauri.yml`

Current repository truth for desktop verification is split into two seams:

- `pnpm test:tauri` proves the Rust desktop library tests still pass.
- `pnpm test:tauri:coverage` enforces the 80%+ unit-test gate on
  `src-tauri/src/runtime_logic.rs`, the extracted deterministic logic seam
  behind the desktop shell.
- `pnpm tauri:dev` / `pnpm tauri:build` still cover the runtime/build-oriented
  desktop path.

## Root Jest Commands

From the repository root:

```bash
pnpm test
pnpm test:watch
pnpm test:coverage
pnpm test:tauri
pnpm test:tauri:coverage
```

Recommended targeted commands, especially on Windows:

```bash
pnpm exec jest app/(dashboard)/page.test.tsx --runInBand
pnpm exec jest components/dashboard/widgets.test.tsx --runInBand
pnpm exec jest --testNamePattern="workflow" --runInBand
```

Using `pnpm exec jest ... --runInBand` is the most reliable path for focused
debugging when a broader `pnpm test -- ...` invocation becomes noisy.

## Root Jest Configuration Truth

The source of truth is [`jest.config.ts`](./jest.config.ts).

Important current behavior:

- `jest.setup.ts` configures the shared test environment and framework mocks.
- `collectCoverageFrom` covers:
  - `app/**/*.{js,jsx,ts,tsx}`
  - `components/**/*.{js,jsx,ts,tsx}`
  - `hooks/**/*.{js,jsx,ts,tsx}`
  - `lib/**/*.{js,jsx,ts,tsx}`
- Coverage explicitly excludes `components/ui/**`.
- Test path ignores currently include:
  - `src-bridge/`
  - `src-tauri/`
  - `plugins/reviews/`
  - `.next/`
  - `out/`
  - `app/page.test.tsx`
  - `scripts/build-go-wasm-plugin.test.mjs`

That means root Jest is intentionally **not** the authoritative test surface for
the bridge, Rust desktop code, or review-plugin workspaces.

## Coverage and Reports

Running `pnpm test:coverage` generates reports under `coverage/`:

- `coverage/index.html`
- `coverage/lcov.info`
- `coverage/cobertura-coverage.xml`
- `coverage/clover.xml`
- `coverage/junit.xml`

Current global thresholds in `jest.config.ts`:

- Branches: 60%
- Functions: 60%
- Lines: 70%
- Statements: 70%

## Backend and Bridge Commands

### Go Orchestrator

```bash
cd src-go
go test ./...
```

Integration-style verification requires Postgres and Redis plus the expected env
vars:

```bash
cd src-go
$env:TEST_POSTGRES_URL="postgres://dev:dev@localhost:5432/appdb_test?sslmode=disable"
$env:TEST_REDIS_URL="redis://localhost:6379"
go test ./... -tags integration -count=1 -v
```

### TypeScript Bridge

```bash
cd src-bridge
bun test src/runtime/registry.test.ts src/server.test.ts
bun run typecheck
```

Use focused file lists that match the bridge slice you changed.

### IM Bridge

```bash
cd src-im-bridge
go test ./...
go build ./...
```

## IM Bridge End-To-End Flow

Use this when you need to verify the IM bridge against the real backend control
plane instead of only unit tests.

1. Start the shared local stack:

```bash
pnpm dev:all
```

2. Configure `src-im-bridge/.env` from the values in
   `src-im-bridge/.env.example`.
3. Start the bridge from `src-im-bridge/`:

```bash
go run ./cmd/bridge
```

4. Verify control-plane registration and status:
   - `POST /api/v1/im/bridge/register`
   - `POST /api/v1/im/bridge/heartbeat`
   - `GET /api/v1/im/bridge/status`
   - `GET /api/v1/im/event-types`
5. Exercise at least one inbound and one outbound path for the platform you are
   testing.

## Bridge Test Matrix

The TypeScript bridge does not have a single one-size-fits-all command. Use the
smallest focused matrix that matches your change:

| Change area | Recommended commands |
| --- | --- |
| Runtime registry / execution routing | `bun test src/runtime/registry.test.ts src/server.test.ts` |
| Claude/Codex/OpenCode runtime adapters | focused `bun test` on the touched runtime tests plus `bun run typecheck` |
| Session / resume continuity | `bun test src/session/manager.test.ts ...` |
| Transport / OpenCode HTTP adapter | `bun test src/opencode/transport.test.ts ...` |
| Plugin SDK / MCP glue | focused `bun test` in the relevant plugin or SDK area plus `bun run typecheck` |

On this repo, a truthful bridge verification normally ends with:

```bash
cd src-bridge
bun run typecheck
```

## Build Verification

For the web app, remember that `next.config.ts` currently enables
`output: "export"`.

```bash
pnpm build
```

This produces the deployable static site in `out/`. The CI test workflow uploads
`out/` as the frontend build artifact after tests pass.

## CI Usage

The main workflows connected to testing are:

- `.github/workflows/test.yml`
  - runs `pnpm test:coverage`
  - uploads `coverage/`
  - runs `pnpm build`
  - uploads `out/`
- `.github/workflows/desktop-tauri-logic.yml`
  - runs `cargo test --manifest-path src-tauri/Cargo.toml --lib`
  - runs the `runtime_logic.rs` coverage gate via `cargo llvm-cov`
- `.github/workflows/go-ci.yml`
  - runs Go unit, lint, and integration coverage paths
- `.github/workflows/ci.yml`
  - orchestrates root quality, root tests, Go CI, bridge typecheck, IM build,
    and then desktop builds

## Practical Guidance

- If you only changed dashboard UI, start with root Jest plus `pnpm build`.
- If you changed `src-go/`, do not stop at root Jest; run Go tests.
- If you changed `src-bridge/`, run focused `bun test` plus `bun run typecheck`.
- If you changed `src-im-bridge/`, run `go test ./...` there.
- If you changed `src-tauri/`, run `pnpm test:tauri`; if you touched
  `src-tauri/src/runtime_logic.rs` or related pure desktop helpers, also run
  `pnpm test:tauri:coverage`, then finish with `pnpm tauri:dev` or
  `pnpm tauri:build` when the runtime/build path matters.
- Prefer reporting scoped verification truthfully over claiming repo-wide
  coverage you did not actually run.

## Coverage Improvement Guide

When you need to raise coverage without guessing:

1. Start from the owning surface instead of the repo root:
   - root UI: `pnpm test:coverage`
   - Go: `go test ./...`
   - bridge: focused `bun test`
   - Tauri: `pnpm test:tauri:coverage`
2. Prioritize files with behavior, branching, or DTO normalization logic over
   static wrappers.
3. Match the test file to the source seam already used in the repo:
   - `components/foo.tsx` -> `components/foo.test.tsx`
   - `lib/stores/bar.ts` -> `lib/stores/bar.test.ts`
   - `src-go/internal/handler/...` -> handler/service/repository tests in Go
4. Prefer focused reruns while iterating, then finish with the owning broad
   command for the touched surface.
