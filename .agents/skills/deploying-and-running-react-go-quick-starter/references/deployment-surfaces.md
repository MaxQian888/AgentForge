# Deployment Surfaces

Use this file when the task is about packaging, releasing, or deploying the repository rather than just running it locally.

## Static Web Deployment

- The frontend is configured with `output: "export"` in `next.config.ts`.
- `pnpm build` generates the deployable static site in `out/`.
- Deploy the `out/` directory to a static host.
- Do not model production around `pnpm start`; that command is incompatible with export mode in this repo.

## GitHub Actions Entry Points

- `.github/workflows/ci.yml`
  - Runs on pushes and pull requests to `master` and `develop`
  - Orchestrates `quality.yml`, `test.yml`, `go-ci.yml`, and `build-tauri.yml`
- `.github/workflows/release.yml`
  - Runs on tags matching `v*`
  - Re-runs checks, builds Tauri artifacts, and creates a draft GitHub release
- `.github/workflows/deploy.yml`
  - Exists, but deployments are disabled by default
- `.github/workflows/build-tauri.yml`
  - Produces unsigned Linux, Windows, and macOS desktop artifacts unless signing secrets are explicitly enabled

## Current CI/CD Shape

- Quality checks run `pnpm lint`, `pnpm exec tsc --noEmit`, `pnpm audit`, and `pnpm outdated`.
- Frontend tests run `pnpm test:coverage`, then `pnpm build`, then upload `out/`.
- Go CI has its own workflow and runs integration tests with Postgres and Redis services.
- Tauri builds run in a matrix across Ubuntu, Windows, and macOS, and build the Go sidecar before bundling the desktop app.

## Deploy Workflow Guardrails

- `deploy.yml` ships with `DEPLOY_ENABLED: false`.
- Preview and production Vercel steps are still commented out.
- Production deployment also expects GitHub Environment protection to be configured before enabling it.

## Release Workflow Guardrails

- A release starts with a git tag such as `v1.0.0`.
- `release.yml` creates a draft GitHub release and attaches built installers.
- Windows signing and macOS signing or notarization are optional and disabled unless secrets are supplied and workflow sections are uncommented.

## Artifact Expectations

- Static web output: `out/`
- Windows desktop installers: `src-tauri/target/<target>/release/bundle/msi/` and `.../nsis/`
- macOS desktop bundles: `.../bundle/dmg/` and `.../bundle/macos/`
- Linux desktop bundles: `.../bundle/appimage/` and `.../bundle/deb/`
