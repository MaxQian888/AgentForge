# Contributing to AgentForge

Thank you for contributing to AgentForge. This repository is no longer a thin
starter template; it is a multi-surface workspace with a Next.js dashboard, Go
orchestrator, Bun bridge, IM bridge workspace, and Tauri desktop shell. Keep
changes scoped, repo-truthful, and verified against the surface you touched.

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

## Questions

If you are unsure about repository truth, inspect the current code and workflow
files first. For contribution-related questions, start with the repository
issues in `Arxtect/AgentForge` and link the exact file or command that seems
unclear.
