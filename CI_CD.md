# CI/CD Pipeline Documentation

This document describes the GitHub Actions workflows that are actually present
in this repository today. AgentForge uses a modular pipeline: frontend quality
and tests live at the repo root, Go and bridge validation run as separate
workflows, desktop builds are matrix-based, and review automation has its own
PR-triggered path.

## Workflow Inventory

### Core Delivery Workflows

- `.github/workflows/ci.yml`
  - Main push / pull request orchestrator for `master` and `develop`
- `.github/workflows/quality.yml`
  - Reusable/manual frontend quality checks
- `.github/workflows/test.yml`
  - Reusable/manual frontend test + build workflow
- `.github/workflows/go-ci.yml`
  - Go unit, lint, and integration validation
- `.github/workflows/desktop-tauri-logic.yml`
  - Reusable/manual desktop Rust lib tests + runtime-logic coverage gate
- `.github/workflows/build-tauri.yml`
  - Reusable/manual desktop build matrix
- `.github/workflows/release.yml`
  - Tag-triggered draft release flow with updater artifacts
- `.github/workflows/deploy.yml`
  - Callable/manual deploy workflow, disabled by default

### Review Automation Workflows

- `.github/workflows/agent-review.yml`
  - Layer 1 PR review for `agent/*` branches
- `.github/workflows/review-layer2.yml`
  - Layer 2 deep-review trigger based on Layer 1 metadata

## Main CI Orchestrator

The main repository pipeline is `.github/workflows/ci.yml`.

It runs on:

- push to `master`
- push to `develop`
- pull requests targeting `master`
- pull requests targeting `develop`

The current job graph is:

1. `quality`
2. `test`
3. `go-ci`
4. `bridge-typecheck`
5. `im-bridge-build`
6. `build-tauri`

`build-tauri` only starts after the first five jobs succeed.

## What Each Core Job Does

### 1. Quality

Source: `.github/workflows/quality.yml`

Environment:

- Ubuntu
- Node.js 22.x
- pnpm 10

Current checks:

- `pnpm install --frozen-lockfile`
- `pnpm lint`
- `pnpm exec tsc --noEmit`
- `pnpm audit --audit-level=moderate`
- `pnpm outdated`

Important current behavior:

- `pnpm lint` is `continue-on-error: true`
- `pnpm audit` is `continue-on-error: true`
- `pnpm outdated` is `continue-on-error: true`
- TypeScript typecheck is still a blocking step

### 2. Test

Source: `.github/workflows/test.yml`

Environment:

- Ubuntu
- Node.js 22.x
- pnpm 10

Current checks:

- `pnpm install --frozen-lockfile`
- `pnpm test:coverage`
- `pnpm build`

Artifacts:

- `test-results` from `coverage/junit.xml`
- `coverage-report` from `coverage/`
- `nextjs-build` from `out/`

Important current behavior:

- The workflow assumes `pnpm build` emits the deployable static site to `out/`
  because `next.config.ts` currently enables `output: "export"`.
- Bundle-size reporting also reads from `out/`.
- Optional Codecov steps are still commented out by default.

### 3. Go CI

Source: `.github/workflows/go-ci.yml`

Jobs:

- `go-unit`
- `go-integration`

`go-unit` runs:

- `go vet ./...`
- `golangci-lint`
- `go test ./... -count=1 -race -timeout 120s`

`go-integration` runs:

- Postgres 16 and Redis 7 service containers
- `go test ./... -tags integration -count=1 -v`

Important current behavior:

- When `go-ci.yml` is called from `ci.yml` or `release.yml` via `workflow_call`,
  the integration job runs.
- Integration tests use:
  - `TEST_POSTGRES_URL=postgres://dev:dev@localhost:5432/appdb_test?sslmode=disable`
  - `TEST_REDIS_URL=redis://localhost:6379`

### 4. Bridge Typecheck

Defined inline in `.github/workflows/ci.yml`.

Current steps:

- checkout
- `bun install --frozen-lockfile` in `src-bridge/`
- `bun run typecheck`

This is currently a typecheck gate only. It does not run a separate repo-level
bridge test workflow.

### 5. IM Bridge Build

Defined inline in `.github/workflows/ci.yml`.

Current steps:

- checkout
- setup Go using `src-im-bridge/go.mod`
- `go build ./...` in `src-im-bridge/`

This is a build-validity gate for the IM bridge workspace.

### 6. Build Tauri

Source: `.github/workflows/build-tauri.yml`

Build matrix:

- `ubuntu-latest` / `x86_64-unknown-linux-gnu`
- `windows-latest` / `x86_64-pc-windows-msvc`
- `macos-latest` / `x86_64-apple-darwin`
- `macos-latest` / `aarch64-apple-darwin`

Current steps per platform:

- checkout
- setup pnpm 10 and Node 22
- install Rust stable
- install dependencies
- setup Go
- `node scripts/build-backend.js --current-only`
- `pnpm tauri build --target <target>`

Produced artifacts:

- Linux AppImage and `.deb`
- Windows MSI and NSIS `.exe`
- macOS `.app`, `.dmg`, and updater tarballs

Important current behavior:

- Updater-specific signing is optional and controlled by workflow inputs.
- The active workflow currently wires Tauri updater signing inputs, not a full
  per-platform certificate/notarization flow.

### 7. Desktop Tauri Logic

Source: `.github/workflows/desktop-tauri-logic.yml`

Environment:

- `windows-latest`
- Rust stable
- `cargo-llvm-cov`

Current checks:

- `cargo test --manifest-path src-tauri/Cargo.toml --lib`
- `cargo llvm-cov --manifest-path src-tauri/Cargo.toml --lib --ignore-filename-regex 'lib[.]rs$' --fail-under-lines 80 --fail-under-functions 80`

Current role in the pipeline:

- This workflow is callable and manually runnable.
- It is not currently wired into `.github/workflows/ci.yml`.
- The repo-level scripts `pnpm test:tauri` and `pnpm test:tauri:coverage`
  mirror this validation seam for local use.

## Release Flow

Source: `.github/workflows/release.yml`

Trigger:

- push tags matching `v*`

Current release stages:

1. `quality`
2. `test`
3. `go-ci`
4. `build-tauri`
5. `create-release`

Release-specific behavior:

- `build-tauri` is called with:
  - `updater-enabled: true`
  - `updater-pubkey: ${{ vars.TAURI_UPDATER_PUBKEY }}`
- The workflow inherits secrets for updater signing.
- `create-release` downloads artifacts, builds `artifacts/latest.json`, validates
  the updater payload set, and creates a **draft** GitHub release.

Release assets currently attached:

- `*.AppImage*`
- `*.deb`
- `*.msi*`
- `*.exe*`
- `*.app.tar.gz*`
- `*.dmg`
- `latest.json`

## Deploy Flow

Source: `.github/workflows/deploy.yml`

Triggers:

- `workflow_call`
- `workflow_dispatch`

Inputs:

- `environment=preview`
- `environment=production`

Current truth:

- `DEPLOY_ENABLED` is hard-coded to `false`
- Vercel deployment steps are still commented out
- The workflow downloads `nextjs-build` from `out/` only if deployments are
  explicitly enabled
- This workflow is **not** called from `.github/workflows/ci.yml` by default

Treat deploy as a prepared but inactive path until the workflow file is updated
and the required secrets are configured.

## Review Automation

### Layer 1 Review

Source: `.github/workflows/agent-review.yml`

Trigger:

- pull requests with types `opened`, `synchronize`, `ready_for_review`

Important current behavior:

- The review job only runs when `github.head_ref` starts with `agent/`
- It uses `anthropics/claude-code-action@v1`
- It persists review-decision metadata as a workflow artifact
- If `AGENTFORGE_API_URL` and `AGENTFORGE_CI_TOKEN` are available, it also
  reports a normalized CI result back to AgentForge

### Layer 2 Deep Review

Source: `.github/workflows/review-layer2.yml`

Trigger:

- `workflow_run` after `Agent PR Review`

Important current behavior:

- It downloads the Layer 1 artifact from the previous run
- It only triggers deep review when Layer 1 marked
  `needs_deep_review=true`
- It POSTs the diff and review dimensions to
  `POST /api/v1/reviews/trigger` on the AgentForge API

## Artifacts

Common artifact names in the current workflows:

- `test-results`
- `coverage-report`
- `nextjs-build`
- `tauri-linux-*-appimage`
- `tauri-linux-*-deb`
- `tauri-windows-*-msi`
- `tauri-windows-*-nsis`
- `tauri-macos-*-updater`
- `tauri-macos-*-dmg`
- `tauri-macos-*-app`
- `layer1-review-decision`

## Secrets and Variables

### Baseline CI

The core `ci.yml` path works without repository secrets.

### Optional / Conditional Inputs

#### Desktop updater release

- `vars.TAURI_UPDATER_PUBKEY`
- `secrets.TAURI_SIGNING_PRIVATE_KEY`
- `secrets.TAURI_SIGNING_PRIVATE_KEY_PASSWORD` (optional)

#### Deploy workflow

- `VERCEL_TOKEN`
- `VERCEL_ORG_ID`
- `VERCEL_PROJECT_ID`
- `DEPLOY_ENABLED=true` in the workflow file

#### Review automation

- `ANTHROPIC_API_KEY`
- `AGENTFORGE_API_URL`
- `AGENTFORGE_CI_TOKEN`
- `AGENTFORGE_TOKEN`

## Operational Notes

- The frontend build artifact is the static-export `out/` directory, not a
  server-oriented `.next` deployment package.
- Root frontend CI and Go CI are separate truths; passing one does not imply the
  other also passed.
- The release flow currently validates updater artifacts before draft release
  creation, so `latest.json` is part of the release contract.
- Deploy remains intentionally disabled; do not assume preview or production
  hosting is active just because `deploy.yml` exists.
