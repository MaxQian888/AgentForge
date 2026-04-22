# AgentForge Desktop Updater Release Inputs

AgentForge's desktop auto-update flow now assumes GitHub Releases as the first
distribution source for `latest.json` and signed updater bundles.

## Current Implementation Snapshot

As of `2026-04-22`, the updater flow is implemented as follows:

- `src-tauri/src/lib.rs` wires the Tauri updater plugin into the desktop shell.
- `lib/platform-runtime.ts` and `hooks/use-platform-capability.ts` expose the
  normalized desktop update flow to the frontend.
- `app/(dashboard)/plugins/page.tsx` is the current operator-facing surface that
  consumes update metadata, install progress, and relaunch state.
- `src-tauri/tauri.conf.json` carries the production updater endpoint, while
  `src-tauri/tauri.updater.conf.json` enables `bundle.createUpdaterArtifacts`
  for releasable builds.
- `scripts/release/build-updater-manifest.js` builds a Tauri v2-compatible static
  manifest from authoritative artifacts, and
  `scripts/release/validate-updater-artifacts.js` rejects missing files or missing
  required platform entries before release publication.

## Required Release Inputs

- `TAURI_UPDATER_PUBKEY`
  - Recommended as a GitHub Actions repository variable.
  - This is the public verification key exposed to desktop clients.
  - It is injected into the Tauri updater plugin at build time.
- `TAURI_SIGNING_PRIVATE_KEY`
  - Required GitHub Actions secret for tag-based desktop releases.
  - Used by Tauri to sign updater artifacts.
- `TAURI_SIGNING_PRIVATE_KEY_PASSWORD`
  - Optional GitHub Actions secret when the signing key is password protected.

## Release Workflow Behavior

- Normal local development and non-release builds continue to use the base
  `src-tauri/tauri.conf.json` and do not require updater signing inputs.
- Local Rust/Tauri logic verification is available separately through
  `pnpm test:tauri`, `pnpm test:tauri:coverage`, and the callable
  `.github/workflows/desktop-tauri-logic.yml` workflow; these validate the
  desktop lib and runtime-logic seam before release packaging.
- Tag-triggered releases call `.github/workflows/build-tauri.yml` with
  `updater-enabled: true`, which switches Tauri to the
  `src-tauri/tauri.updater.conf.json` overlay and requires updater signing
  inputs.
- The release job generates `artifacts/latest.json` with
  `scripts/release/build-updater-manifest.js`.
- The release job then validates that every manifest entry resolves to a real
  downloaded artifact through `scripts/release/validate-updater-artifacts.js`.

## Local And Release Commands

- `pnpm tauri:dev`
  - Desktop development with current-platform sidecars.
  - Does not require updater signing secrets.
- `pnpm test:tauri`
  - Runs the `src-tauri` Rust library tests used by the desktop logic workflow.
- `pnpm test:tauri:coverage`
  - Enforces the runtime-logic coverage gate mirrored by
    `.github/workflows/desktop-tauri-logic.yml`.
- `pnpm build:desktop`
  - Runs `build:backend`, `build:bridge`, and `pnpm tauri build`.
  - This is the repo-level full desktop packaging entrypoint.
- `pnpm build:updater-manifest -- --artifacts-root <dir> --base-download-url <url> --release-version <version>`
  - Generates `latest.json` from produced updater artifacts.
- `pnpm verify:updater-artifacts -- --artifacts-root <dir> --manifest <file>`
  - Fails if the manifest points at missing artifacts or omits required
    platforms.

## Published Assets

- Linux updater bundles: `*.AppImage` with matching `*.AppImage.sig`
- macOS updater bundles: `*.app.tar.gz` with matching `*.app.tar.gz.sig`
- Windows updater bundles:
  - `*.exe` with matching `*.exe.sig`
  - `*.msi` with matching `*.msi.sig`
- Release manifest: `latest.json`

## Failure Modes

- Missing `TAURI_UPDATER_PUBKEY` or `TAURI_SIGNING_PRIVATE_KEY` blocks the
  updater-enabled release build before publication.
- Missing bundle/signature pairs or malformed manifest URLs fail release
  validation before the GitHub Release draft is created.
- Missing required platform entries in `latest.json` also fails validation;
  current validation expects at least `linux-x86_64`, `windows-x86_64`,
  `darwin-x86_64`, and `darwin-aarch64`.
