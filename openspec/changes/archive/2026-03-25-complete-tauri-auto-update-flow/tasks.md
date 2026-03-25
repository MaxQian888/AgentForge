## 1. Updater Runtime Contract

- [x] 1.1 Extend `lib/platform-runtime.ts` and `hooks/use-platform-capability.ts` with normalized update metadata, install progress, failure, and relaunch contracts instead of the current check-only result.
- [x] 1.2 Add the Tauri updater and process guest-plugin integration needed by the shared facade, including any required root `package.json` dependencies and desktop capability permissions.
- [x] 1.3 Update the existing desktop-facing UI surface in `app/(dashboard)/plugins/page.tsx` to show available version metadata, install progress, install outcome, and restart action through the shared facade.

## 2. Tauri Configuration And Distribution

- [x] 2.1 Update `src-tauri/tauri.conf.json`, `src-tauri/Cargo.toml`, and capability files so desktop builds support updater artifacts, verifier public-key configuration, release endpoints, and relaunch permissions.
- [x] 2.2 Replace or retire the current check-only updater plumbing in `src-tauri/src/lib.rs` as needed so the desktop update flow has one clear source of truth.
- [x] 2.3 Extend the desktop build and release workflows under `.github/workflows/` plus any helper scripts to generate signed updater artifacts and publish a Tauri-compatible static update manifest from released assets.
- [x] 2.4 Make the release path fail explicitly when required updater signing inputs or manifest prerequisites are missing, without breaking normal local development flows.

## 3. Verification And Operator Readiness

- [x] 3.1 Add or update automated tests for platform-runtime updater behavior, desktop/web fallback semantics, and the desktop panel update UX.
- [x] 3.2 Add focused validation for release-manifest assembly and updater artifact completeness so malformed or partial platform entries are caught before publication.
- [x] 3.3 Run the repo-truthful verification path for the touched surfaces and document the updater prerequisites or release inputs needed by operators to ship future desktop updates safely.
