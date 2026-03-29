## Why

The TypeScript Bridge already advertises `codex` as a supported coding-agent runtime, but the live implementation still depends on a placeholder command contract: it writes one AgentForge JSON request to `stdin` and expects newline-delimited JSON events back from the configured `codex` command. The repository documents that assumption but does not provide a repo-owned Codex connector that actually implements it, so pointing `CODEX_RUNTIME_COMMAND=codex` does not produce a truthful, resumable Codex runtime in practice.

This gap now needs a focused change because the broader `complete-ts-bridge-api-surface` change explicitly avoids new Codex adapter work, while the requested outcome is to make the existing TS Bridge to Codex path complete rather than leaving it as a catalog-only runtime entry.

## What Changes

- Replace the placeholder Codex command assumption with a repository-owned Codex connector inside `src-bridge` that talks to a real Codex automation surface and adapts it to the canonical AgentForge runtime events.
- Tighten Codex readiness diagnostics so the bridge can distinguish missing executable, missing authentication, and unsupported connector setup before execution starts.
- Persist Codex-specific continuity metadata in bridge snapshots so pause, resume, and restart recovery use the real Codex session state instead of only replaying the original execute request.
- Define truthful failure, cancellation, and event-normalization rules for Codex runs so Go can observe Codex execution through the same `/bridge/*` contract without runtime-specific guesswork.
- Add focused bridge documentation and verification coverage for Codex launch, diagnostics, cancellation, and resume behavior.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `bridge-agent-runtime-registry`: Codex runtime readiness and catalog diagnostics must validate a concrete connector contract instead of only checking whether a bare executable name resolves on `PATH`.
- `agent-sdk-bridge-runtime`: Codex runs must execute through a repository-owned connector, emit canonical AgentForge runtime events, and preserve resumable continuity metadata across pause and resume flows.

## Impact

- Affected code: `src-bridge/src/runtime/registry.ts`, `src-bridge/src/handlers/command-runtime.ts` or a replacement Codex-specific adapter, `src-bridge/src/handlers/execute.ts`, `src-bridge/src/server.ts`, `src-bridge/src/session/manager.ts`, runtime types/tests, and bridge runtime documentation in the root README files.
- Affected systems: TS Bridge runtime startup, runtime catalog diagnostics, bridge snapshot persistence, pause/resume semantics, and Go-side observability for Codex runs.
- Affected dependencies: possible addition of an official Codex automation dependency or a repo-owned wrapper/connector module, depending on the final connector shape chosen in design.
