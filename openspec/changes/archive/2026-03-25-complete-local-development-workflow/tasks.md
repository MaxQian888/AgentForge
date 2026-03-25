## 1. Shared Dev Workflow Foundation

- [x] 1.1 Audit the current root scripts, `.vscode/tasks.json`, Docker compose files, and runtime health endpoints, then codify the supported service matrix for the full-stack local workflow.
- [x] 1.2 Add shared service-definition and runtime-state helpers for repo-local log paths, pid metadata, health checks, and managed-versus-reused service tracking.
- [x] 1.3 Define the repo-local state and diagnostics contract under `.codex/` so startup, status, stop, and logs all use the same metadata.

## 2. Full-Stack Startup Command

- [x] 2.1 Implement the root `dev:all` startup flow so it prepares PostgreSQL/Redis, then starts or reuses the Go Orchestrator, TS Bridge, and frontend in dependency order.
- [x] 2.2 Add prerequisite detection and health-aware readiness checks for Docker/Compose, Go, Bun, Node.js, pnpm, and the repo-truthful service endpoints.
- [x] 2.3 Handle duplicate-start and listener-conflict cases by reusing healthy AgentForge services, rejecting unknown listeners, and reporting the result per service.
- [x] 2.4 Persist startup results to the shared runtime-state file and write per-service stdout/stderr logs to discoverable paths.

## 3. Status, Stop, And Diagnostics Commands

- [x] 3.1 Add root command surfaces for `dev:all:status`, `dev:all:stop`, and `dev:all:logs` that read the shared runtime-state contract.
- [x] 3.2 Implement status reconciliation so stale pid records or failed health probes are corrected before reporting service state.
- [x] 3.3 Implement stop behavior that only terminates services marked as managed by the workflow and explicitly preserves reused or external services.
- [x] 3.4 Make diagnostics output point developers to the relevant service endpoint, failure category, and latest log location for follow-up troubleshooting.

## 4. Docs And Command Surface Integration

- [x] 4.1 Update root `package.json` to expose the supported full-stack local workflow commands without regressing existing `dev`, `plugin:dev`, or `tauri:dev` scripts.
- [x] 4.2 Update `README.md` and `README_zh.md` so the documented startup path, ports, prerequisites, and log/state locations match the new command surface.
- [x] 4.3 Align any related developer guidance, including VS Code task notes or setup documentation, with the supported `dev:all` workflow and current env-file reality.

## 5. Verification

- [x] 5.1 Add focused tests for shared helpers, startup idempotence, listener-conflict handling, runtime-state persistence, and managed-versus-reused stop semantics.
- [x] 5.2 Run the targeted verification for the new workflow commands and ensure the final docs reflect the same supported behavior and diagnostics paths.
