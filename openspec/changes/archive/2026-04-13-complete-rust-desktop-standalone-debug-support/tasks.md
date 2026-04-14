## 1. Shared Rust runtime host and standalone CLI

- [x] 1.1 Extract reusable desktop runtime preflight and supervision seams from `src-tauri/src/lib.rs` so the GUI entrypoint and a standalone CLI can share one startup contract.
- [x] 1.2 Add a standalone Rust CLI binary under `src-tauri` with `check` and `run` commands for frontend preflight, sidecar readiness checks, and foreground desktop runtime launch.
- [x] 1.3 Add or update focused Rust tests covering CLI preflight failures, shared runtime-host reuse, and foreground exit or diagnostic behavior.

## 2. Repository entrypoints and debug tooling

- [x] 2.1 Expose maintained root wrapper commands in `package.json` for the standalone Rust debug workflow and keep them aligned with the current-host desktop prepare contract.
- [x] 2.2 Update `.vscode/tasks.json` and `.vscode/launch.json` so maintained desktop debug configurations clearly separate full `tauri:dev` mode from standalone Rust debug mode.
- [x] 2.3 Verify `src-tauri/tauri.conf.json` and related desktop launch wiring still reserve frontend startup ownership for full desktop mode only, while standalone CLI mode treats frontend availability as an external prerequisite.

## 3. Docs and verification

- [x] 3.1 Update `README.md`, `README_zh.md`, and any maintained desktop debug guidance to document when to use full desktop mode versus standalone Rust debug mode.
- [x] 3.2 Run focused verification for the new desktop standalone surfaces, including the Rust CLI target plus any touched JS or IDE-facing contract checks, and record the exact command evidence.
