## Context

`src-bridge` already treats `codex` as a first-class runtime key in the shared runtime registry, runtime catalog, status payloads, and validation rules. However, the live execution path is still a generic command adapter shared with `opencode`: the bridge spawns the configured command, writes one JSON request to `stdin`, and expects newline-delimited JSON events back on `stdout`.

That contract is bridge-local, not a native Codex contract. The repository currently documents the expectation in `README.md` / `README_zh.md`, but it does not ship a repository-owned Codex connector that actually implements it. As a result, `runtime=codex` is visible across the product surface, yet the runtime is not truthfully launchable unless operators build their own out-of-band shim. The broader `complete-ts-bridge-api-surface` change explicitly excludes new Codex adapter work, so this change stays focused on the missing Codex bridge seam only.

## Goals / Non-Goals

**Goals:**
- Replace the placeholder Codex command assumption with a repository-owned Codex connector inside `src-bridge`.
- Preserve the existing `/bridge/*` HTTP contract and canonical `AgentEvent` stream seen by Go.
- Make Codex readiness diagnostics actionable before launch by checking the real connector prerequisites rather than only command discovery.
- Persist enough Codex-native continuity metadata to make pause, resume, and restart recovery truthful.
- Keep the scope limited to Codex so OpenCode and the already-broad bridge API-surface change do not get mixed into the same implementation thread.

**Non-Goals:**
- Do not redesign the Go-to-bridge REST contract or the frontend runtime catalog UI.
- Do not reopen OpenCode connectivity work in this change.
- Do not add new coding-agent runtimes beyond the existing `claude_code`, `codex`, and `opencode`.
- Do not change the canonical AgentForge event vocabulary consumed by Go.

## Decisions

### D1: Codex gets a dedicated bridge-owned runtime adapter instead of reusing the generic command adapter

The bridge will introduce a dedicated Codex adapter module, separate from the generic `command-runtime` path currently shared by `codex` and `opencode`. The Codex adapter will own launch configuration, native output translation, authentication checks, and continuity metadata for Codex specifically.

This keeps the existing generic command adapter available for OpenCode while acknowledging that Codex now needs materially different behavior: official automation-surface integration, richer readiness checks, and truthful resume semantics. Extending the generic adapter with Codex-only branches would keep the placeholder abstraction in place and make OpenCode and Codex harder to evolve independently.

Alternative considered:
- Keep `codex` on the generic command adapter and ask operators to supply a custom wrapper. Rejected because the repository would still not own the live contract it claims to support.

### D2: The dedicated Codex adapter targets an official programmatic Codex surface, not a raw `codex` stdin/stdout protocol

The bridge-owned Codex adapter will connect to an official Codex automation surface and translate between that surface and the canonical AgentForge runtime contract. The design assumes a programmatic TypeScript integration path is preferred because it provides structured output and resumable session continuity without pretending that the bare `codex` binary natively speaks the bridge's JSONL protocol.

The bridge may preserve `CODEX_RUNTIME_COMMAND` only as a transitional override for exceptional environments, but the default `runtime=codex` path must no longer depend on operators discovering or maintaining an external shim on their own.

Alternatives considered:
- Continue documenting `CODEX_RUNTIME_COMMAND=codex` as if the raw CLI were sufficient. Rejected because that does not match the actual bridge-local protocol.
- Build only a repo-local subprocess wrapper and keep the real Codex integration hidden behind another private JSONL boundary. Rejected as the primary design because it adds another process boundary without solving the bridge-owned lifecycle problem unless the wrapper is also maintained as first-party runtime code.

### D3: Session snapshots store Codex continuity state explicitly

`SessionSnapshot` will be extended with runtime-specific continuity metadata so Codex pause/resume flows can preserve the native session or thread identity needed to continue work truthfully. For Codex runs, the bridge will persist the resolved runtime identity plus the connector-owned continuation payload whenever a run pauses, completes, or reaches a recoverable terminal state.

`/bridge/resume` will use this continuity payload when resuming Codex. It must not fall back to replaying only the original prompt unless the design explicitly marks that path as a fresh restart. This prevents duplicate work, broken operator expectations, and misleading "resume" semantics.

Alternative considered:
- Keep storing only the original `ExecuteRequest` and treat resume as a rerun. Rejected because it is not a truthful resume contract for Codex.

### D4: Codex readiness diagnostics become connector-aware

The runtime registry will treat Codex availability as the combination of connector availability, supported authentication state, and any required local executable or toolchain prerequisites. Diagnostics should remain compatible with the existing catalog shape, but their messages and blocking decisions must reflect real Codex launch prerequisites instead of a single `where codex` check.

This allows upstream catalog consumers to warn before launch and gives operators a stable place to debug Codex readiness without starting execution.

Alternative considered:
- Leave diagnostics at "executable missing" and let launch fail later. Rejected because the current problem is precisely that the visible product surface overstates Codex readiness.

## Risks / Trade-offs

- [Risk] Official Codex automation surfaces may evolve faster than the current bridge internals. -> Mitigation: isolate all Codex-specific integration behind one dedicated adapter and keep the Go-facing bridge contract unchanged.
- [Risk] Codex continuity support may depend on connector state that is unavailable for older local setups. -> Mitigation: make missing continuity metadata an explicit resume error and document the supported Codex setup clearly.
- [Risk] Supporting both transitional command overrides and the new dedicated adapter can create confusing precedence. -> Mitigation: define one canonical default path, keep compatibility overrides explicit, and surface the active mode in diagnostics/logging.
- [Risk] Runtime-specific snapshot metadata expands the persistence contract. -> Mitigation: keep the snapshot extension additive and version-tolerant so older snapshot files can still be ignored or restored safely.

## Migration Plan

1. Introduce the dedicated Codex adapter and runtime-specific snapshot metadata behind the existing `runtime=codex` selection path.
2. Update the registry so Codex diagnostics and execution resolution use the new adapter by default, while OpenCode remains on the generic command adapter.
3. Update bridge README documentation to describe the supported Codex setup, auth prerequisites, and truthful pause/resume behavior.
4. Verify that `/bridge/execute`, `/bridge/status/:id`, `/bridge/pause`, and `/bridge/resume` continue to expose the same canonical payload shapes to Go.
5. If rollback is required, restore the previous registry wiring for `codex`, remove the new snapshot metadata fields, and revert the dedicated adapter dependency/module set.

## Open Questions

- Which official Codex integration surface provides the best balance of structured events and resumable continuity in this environment, and does it require an additional bridge dependency or only configuration plus a thin wrapper?
- Should transitional `CODEX_RUNTIME_COMMAND` support remain available for one release cycle, or should this change immediately hard-fail unsupported wrapper-based setups to reduce ambiguity?
