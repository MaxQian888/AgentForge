## 1. Shared Script Contract

- [x] 1.1 Audit the current plugin-related build/debug/run commands and codify the supported root-level CLI surface for maintained plugin workflows.
- [x] 1.2 Add shared manifest and target-resolution helpers so plugin scripts can derive entrypoints, runtime metadata, output paths, and actionable validation errors from one place.

## 2. Build And Debug Workflows

- [x] 2.1 Extend the Go WASM build workflow and root package scripts so maintained plugins can be built by manifest or target path without editing `scripts/build-go-wasm-plugin.js`.
- [x] 2.2 Add a local Go-hosted plugin debug runner that replays the `AGENTFORGE_*` envelope contract, captures stdout/stderr, and reports structured failures.

## 3. Local Run Workflow

- [x] 3.1 Add a minimal plugin development stack command that starts or reuses the Go Orchestrator and TS Bridge, waits for readiness, and prints health endpoints plus next-step guidance.
- [x] 3.2 Add prerequisite detection and failure reporting so the run workflow distinguishes missing dependencies from unhealthy subprocesses.

## 4. Verification And Documentation

- [x] 4.1 Add focused tests or smoke checks for the new script helpers and the maintained Go sample plugin workflow.
- [x] 4.2 Update `README.md`, `README_zh.md`, `docs/GO_WASM_PLUGIN_RUNTIME.md`, and related plugin guidance to document the supported build/debug/run/verify loop and current scope boundaries.
