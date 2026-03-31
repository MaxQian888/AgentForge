# Contributing to AgentForge

Thank you for contributing to AgentForge. This repository is no longer a thin
starter template; it is a multi-surface workspace with a Next.js dashboard, Go
orchestrator, Bun bridge, IM bridge workspace, and Tauri desktop shell. Keep
changes scoped, repo-truthful, and verified against the surface you touched.

## Quick Start In 5 Minutes

If you want the fastest repo-truthful local path:

1. Install the required toolchains:
   - Node.js 20+
   - pnpm 10 recommended
   - Go 1.25+
   - Bun
   - Rust 1.77.2+
   - Docker Desktop or another Docker environment
2. Install dependencies from the repo root:

```bash
pnpm install
```

3. Start the local web stack:

```bash
pnpm dev:all
```

4. Confirm the stack is healthy:

```bash
pnpm dev:all:status
```

5. Open the app:
   - frontend: `http://localhost:3000`
   - Go health: `http://localhost:7777/health`
   - bridge health: `http://localhost:7778/bridge/health`

## Getting Started

1. Fork the repository on GitHub.
2. Clone your fork locally:

```bash
git clone https://github.com/YOUR_USERNAME/AgentForge.git
cd AgentForge
```

3. Add the main repository as `upstream`:

```bash
git remote add upstream https://github.com/Arxtect/AgentForge.git
```

## Prerequisites

- Node.js 20+
- pnpm 10 recommended
- Go 1.25+ for `src-go/`
- Bun for `src-bridge/`
- Rust 1.77.2+ for Tauri work
- Docker Desktop or another Docker environment when you need local Postgres and Redis

## Development Setup

Install dependencies:

```bash
pnpm install
```

Preferred local workflows:

```bash
# Preferred full local web stack
pnpm dev:all

# Frontend-only development
pnpm dev

# Desktop shell development
pnpm tauri:dev
```

Useful status commands:

```bash
pnpm dev:all:status
pnpm dev:all:logs
pnpm dev:all:stop
```

## Environment Variable Cheat Sheet

The full reference lives in
[`docs/deployment/environment-variables.md`](./docs/deployment/environment-variables.md).
The most important local values are:

| Variable | Surface | Typical local value |
| --- | --- | --- |
| `NEXT_PUBLIC_API_URL` | frontend | `http://localhost:7777` |
| `POSTGRES_URL` | Go backend | `postgres://dev:dev@localhost:5432/appdb?sslmode=disable` |
| `REDIS_URL` | Go backend | `redis://localhost:6379` |
| `JWT_SECRET` | Go backend | `change-me-in-production-at-least-32-chars` |
| `BRIDGE_URL` | Go backend | `http://localhost:7778` |
| `GO_WS_URL` | TS bridge | `ws://localhost:7777/ws/bridge` |
| `AGENTFORGE_API_BASE` | IM bridge | `http://localhost:7777` |
| `IM_PLATFORM` | IM bridge | `feishu` or the platform you are testing |

Notes:

- the current checkout does not require `.env.local.example` or `src-go/.env.example` to exist
- prefer local env files over hard-coding values into scripts
- never expose secrets through `NEXT_PUBLIC_*`

## Verify Your Setup

Run the checks that match the surface you will modify:

```bash
# Root frontend checks
pnpm lint
pnpm test
pnpm build

# Rust desktop checks
pnpm test:tauri
```

For deeper guidance, use:

- [`TESTING.md`](./TESTING.md) for test surfaces and commands
- [`CI_CD.md`](./CI_CD.md) for GitHub Actions truth
- [`README.md`](./README.md) and [`README_zh.md`](./README_zh.md) for current runtime and workspace overview

## Branching

Create feature branches from `master`:

```bash
git fetch upstream
git checkout master
git merge upstream/master
git checkout -b <type>/<description>
```

Common branch prefixes:

- `feat/`
- `fix/`
- `docs/`
- `refactor/`
- `test/`
- `chore/`
- `ci/`

Examples:

- `feat/project-dashboard-widget-refresh`
- `fix/bridge-runtime-readiness`
- `docs/update-testing-guide`

Notes:

- `master` is the current default integration branch in this repository.
- Branches prefixed with `agent/` are reserved for the automated review flow in
  `.github/workflows/agent-review.yml`. Do not use that prefix unless you
  intentionally want that workflow behavior.

## Making Changes

- Keep changes focused on one concern.
- Do not mix unrelated refactors into feature or bugfix branches.
- Update docs when behavior, commands, or runtime expectations change.
- Preserve repo conventions already present in the touched area instead of
  introducing parallel patterns.

## Local Quality Gates

The current pre-commit gate is defined by `.husky/pre-commit` and runs:

```bash
pnpm exec lint-staged --concurrent false --max-arg-length 8000
pnpm exec tsc --noEmit
```

If your change affects staged JavaScript or TypeScript files, make sure it is
compatible with that hook before opening a PR.

## Testing Expectations

AgentForge does not have one universal test command for every workspace.

- Root Next.js/dashboard work should usually run `pnpm test` and `pnpm build`.
- Go changes should run `go test ./...` from `src-go/`.
- Bridge changes should run focused `bun test ...` plus `bun run typecheck` from `src-bridge/`.
- IM bridge changes should run `go test ./...` from `src-im-bridge/`.
- Desktop/Tauri changes should run `pnpm test:tauri` and, when relevant,
  `pnpm tauri:build`.

Document the exact verification you ran in your PR description.

## Pull Request Process

1. Update your branch from `upstream/master`.
2. Run the local checks that match the affected surface.
3. Push your branch to your fork.
4. Open a pull request against `master`.
5. Summarize scope, key behavior changes, and validation.
6. Include screenshots or recordings for UI changes when they materially help review.
7. Address feedback with follow-up commits unless maintainers request squashing.

Recommended PR checklist:

- [ ] Scope is focused and coherent
- [ ] Tests or build checks were run for the touched surface
- [ ] Documentation was updated where behavior or commands changed
- [ ] No unrelated generated files or experiments were included
- [ ] CI checks pass

## Commit Guidelines

Use Conventional Commits:

```text
<type>(<scope>): <description>
```

Examples:

- `feat(project-dashboard): add widget refresh feedback`
- `fix(auth): fail closed when refresh token validation fails`
- `docs(testing): clarify bridge and tauri verification paths`
- `ci(release): validate updater artifacts before draft release`

## Documentation

Update docs when you change:

- runtime commands
- verification commands
- CI behavior
- frontend workspaces or operator flows
- plugin/runtime contracts
- environment or deployment expectations

High-signal repository docs include:

- [`README.md`](./README.md)
- [`README_zh.md`](./README_zh.md)
- [`TESTING.md`](./TESTING.md)
- [`CI_CD.md`](./CI_CD.md)
- [`docs/PRD.md`](./docs/PRD.md)

## FAQ / Troubleshooting

### Port already in use (`EADDRINUSE`)

- run `pnpm dev:all:status` to see which managed services are already running
- stop the managed stack with `pnpm dev:all:stop`
- if the port is occupied by a non-AgentForge process, free that process or change your local override

### Node / pnpm / Bun / Go version mismatch

- confirm Node.js 20+ and pnpm 10+
- use Bun for `src-bridge/`
- use Go 1.25+ for `src-go/` and `src-im-bridge/`
- keep Rust installed for Tauri work even if you are mostly editing the web UI

### Tauri build or dev mode fails

- verify `pnpm desktop:dev:prepare` or `pnpm desktop:build:prepare` completes
- confirm `src-tauri/binaries/` contains the expected sidecars
- review [`docs/deployment/desktop-build.md`](./docs/deployment/desktop-build.md)

### Database or Redis connection failures

- start infra with `docker compose up -d`
- confirm Postgres on `5432` and Redis on `6379`
- verify `POSTGRES_URL` and `REDIS_URL`
- remember that auth refresh/revocation paths fail closed when Redis is unavailable

## Questions

If you are unsure about repository truth, inspect the current code and workflow
files first. For contribution-related questions, start with the repository
issues in `Arxtect/AgentForge` and link the exact file or command that seems
unclear.
