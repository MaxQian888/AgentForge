## 1. Execution Contract And Runtime Registry

- [x] 1.1 Extend the bridge execute request/response types and Zod schema to support an explicit `runtime` selector plus documented backward-compatible fallback behavior for callers that still omit it.
- [x] 1.2 Add a bridge-local runtime registry module that defines the supported runtime keys (`claude_code`, `codex`, `opencode`), default runtime resolution, runtime metadata, and fail-fast validation before pool acquisition.
- [x] 1.3 Align `src-go/internal/bridge/client.go` and any affected service-layer request builders with the canonical runtime-selection contract so Go and TypeScript share one truthful execute surface.

## 2. Shared Adapter Infrastructure

- [x] 2.1 Extract the current Claude-backed execute wiring behind a common runtime adapter interface that covers launch options, event normalization, cancellation hooks, and runtime-specific metadata.
- [x] 2.2 Refactor `handleExecute(...)` and the server execute path to resolve an adapter from the registry before creating active runtime state, while preserving existing lifecycle, status, and snapshot behavior.
- [x] 2.3 Centralize runtime configuration discovery and operator-facing error handling for unknown runtimes, disabled runtimes, missing binaries, and missing credentials.

## 3. Runtime Adapter Implementations

- [x] 3.1 Preserve the existing Claude-backed runtime as the `claude_code` adapter inside the new registry and verify it still streams canonical AgentForge events.
- [x] 3.2 Implement a Codex adapter that launches through the chosen backend mechanism, normalizes native execution output into canonical bridge events, and honors cancellation/budget boundaries truthfully.
- [x] 3.3 Implement an OpenCode adapter with the same registry, configuration, and event-normalization expectations as the Claude and Codex adapters.

## 4. Verification And Operational Readiness

- [x] 4.1 Add focused bridge tests for runtime resolution, unknown-runtime rejection, configuration failures, and adapter-specific event normalization.
- [x] 4.2 Update bridge and Go integration tests to cover the canonical runtime contract, including backward-compatible requests that omit `runtime`.
- [x] 4.3 Document runtime configuration, executable/credential requirements, and local verification steps for Claude Code, Codex, and OpenCode in the relevant bridge docs or env examples.
