## 1. Desktop Command And Build Surface

- [x] 1.1 Add a maintained IM Bridge build helper under `scripts/` that can produce current-host and packaging-ready binaries in `src-tauri/binaries/`.
- [x] 1.2 Add root script aliases for `build:im-bridge`, `build:im-bridge:dev`, `desktop:dev:prepare`, and `desktop:build:prepare`, then rewire `tauri:dev` and `build:desktop` to use them.
- [x] 1.3 Update `src-tauri/tauri.conf.json` so desktop build inputs and pre-build hooks include the IM Bridge sidecar instead of only the backend and TS bridge.

## 2. Full-Stack Local Development Workflow

- [x] 2.1 Extend `scripts/dev-all.js` service definitions so the supported local stack starts, reuses, and health-checks IM Bridge with backend-first defaults and repo-local diagnostics.
- [x] 2.2 Update local workflow state, status, stop, and log reporting so IM Bridge is tracked with the same managed versus reused semantics as the existing services.
- [x] 2.3 Add or update focused workflow tests covering IM Bridge startup, conflict handling, and failure diagnostics for `dev:all`.

## 3. Desktop Runtime Supervision And Event Contracts

- [x] 3.1 Update the Rust desktop runtime manager to supervise backend, TS bridge, and IM Bridge as separate required sidecars with correct startup ordering and bounded restart behavior.
- [x] 3.2 Extend desktop runtime snapshot and event payloads so frontend consumers receive IM Bridge state alongside backend, bridge, and overall status.
- [x] 3.3 Add or update Rust/runtime contract tests that cover three-sidecar ready, degraded, and recovery scenarios.

## 4. Debug Entrypoints, Docs, And Verification

- [x] 4.1 Update `.vscode/tasks.json` and `.vscode/launch.json` so maintained desktop debug entry points reuse the shared desktop prepare commands instead of an IDE-only sidecar list.
- [x] 4.2 Update `README.md` and `README_zh.md` to document the supported startup, desktop debug, and packaging commands with IM Bridge included in the command matrix and service topology.
- [x] 4.3 Run targeted verification for the updated script tests, desktop runtime tests, and representative command/status checks so the new startup and debug contract is backed by fresh evidence.
