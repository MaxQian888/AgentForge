# AgentForge Desktop Updater Release Inputs

AgentForge's desktop auto-update flow now assumes GitHub Releases as the first
distribution source for `latest.json` and signed updater bundles.

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
- Tag-triggered releases call `.github/workflows/build-tauri.yml` with
  `updater-enabled: true`, which switches Tauri to the
  `src-tauri/tauri.updater.conf.json` overlay and requires updater signing
  inputs.
- The release job generates `artifacts/latest.json` with
  `scripts/build-updater-manifest.js`.
- The release job then validates that every manifest entry resolves to a real
  downloaded artifact through `scripts/validate-updater-artifacts.js`.

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
