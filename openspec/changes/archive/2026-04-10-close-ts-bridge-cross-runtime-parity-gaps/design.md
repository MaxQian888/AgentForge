## Context

The TS Bridge already exposes the canonical `/bridge/*` control plane, a runtime registry, and per-runtime adapters for Claude Code, Codex, OpenCode, and the newer CLI-backed runtimes. The remaining contract gaps are no longer about missing route scaffolding; they are about truthfulness and parity at the runtime seam.

The current implementation shows three pressure points:

- `src-bridge/src/handlers/opencode-runtime.ts` creates an OpenCode session and sends `prompt_async`, but the execute path currently only guarantees `provider`, `model`, and `prompt`. Parity-sensitive inputs such as `attachments`, `env`, and `web_search` are not yet carried through an official OpenCode transport path.
- `src-bridge/src/runtime/registry.ts` closes canonical rollback only for Claude Code. OpenCode still reports `rollback` as unsupported even though the bridge already owns continuity, revert, and unrevert paths; Codex also remains stuck at blanket unsupported despite thread continuity already existing for fork and resume.
- The runtime catalog is generated centrally through `buildInteractionCapabilities()`, but some capability states are still static or runtime-wide when the true answer depends on provider auth, per-input prerequisites, or continuity-backed control availability.

This change is intentionally narrower than the earlier “runtime completeness” wave. It focuses on the contract gaps that can cause upstream Go, frontend, or IM surfaces to make wrong assumptions today: silent input drops, rollback dead ends, and capability metadata drift.

## Goals / Non-Goals

**Goals:**
- Make parity-sensitive `ExecuteRequest` inputs truthfully handled across runtimes, prioritizing OpenCode and removing silent drops.
- Close canonical `/bridge/rollback` through runtime-specific continuity or upstream control paths for Claude Code, Codex, and OpenCode.
- Ensure `/bridge/runtimes` publishes interaction capability state from the same truth source used by execute preflight and control routes.
- Add focused verification around execute input parity, rollback behavior, and capability metadata.

**Non-Goals:**
- Adding new runtimes, providers, or route families.
- Reworking Go proxy routes, frontend runtime pickers, or IM control-plane consumers.
- Expanding into unrelated OpenCode features such as PTY, share, or summarize.
- General prompt-shaping or role-system redesign beyond what is required to preserve execute input truthfulness.

## Decisions

### D1: Preflight parity validation happens at the registry boundary

`AgentRuntimeRegistry.resolveExecute()` becomes the contract gate for parity-sensitive execute inputs. The bridge will validate requested `attachments`, `env`, `web_search`, and other runtime-gated inputs before acquiring a pool runtime, using the same runtime/provider-specific truth that later drives catalog metadata.

This avoids the current failure mode where request fields survive schema parsing but disappear in a runtime handler. The alternative—letting each handler silently ignore unsupported fields—keeps execution “working” while making upstream capability decisions untrustworthy.

### D2: OpenCode execute setup is split into session bootstrap/config and prompt payload composition

OpenCode cannot stay as a plain `createSession()` + `prompt_async()` text-only flow if it wants to claim parity with the shared execute contract. The bridge will treat OpenCode execute-time setup in two phases:

1. **Session/config phase**: apply provider/model selection and any server-backed config surfaces (for example env or search-related configuration) through official OpenCode transport endpoints when available.
2. **Prompt payload phase**: build the prompt parts that will be sent to `prompt_async`, including supported attachment parts and the final task text.

If the selected OpenCode server/provider cannot truthfully represent a requested input, the bridge will reject the request before prompt submission. The rejected alternative is encoding unsupported inputs into free-form prompt text, because that only simulates parity instead of preserving it.

### D3: Rollback is defined as a continuity-backed control, not a static per-runtime constant

Canonical rollback will be driven by runtime continuity and upstream-native controls:

- **Claude Code** continues to use `query.rewindFiles()` when checkpointed continuity exists.
- **OpenCode** resolves rollback targets from bound session continuity or recovered message history and delegates to official revert/unrevert endpoints.
- **Codex** uses saved thread continuity plus a dedicated bridge-owned rollback runner, instead of hardcoding `unsupported_operation`.

The key design choice is that rollback support is not published or enforced as a compile-time constant. It is a runtime truth derived from the current task’s continuity plus the selected runtime’s native control surface.

### D4: Capability metadata and route behavior share reason codes and support-state rules

`buildInteractionCapabilities()` and the execute/control handlers must stop drifting. This change will centralize support-state reasoning so that:

- execute preflight errors,
- `/bridge/rollback` and related control-route errors, and
- `/bridge/runtimes` `interaction_capabilities`

all use the same support-state vocabulary (`supported`, `degraded`, `unsupported`) and the same reason codes for auth, config, missing continuity, or permanently unsupported behavior.

The alternative—continuing to hand-author catalog states separately from route logic—causes the exact mismatch this change is meant to remove.

### D5: Focused contract tests guard truthfulness instead of broad smoke-only coverage

The verification strategy will stay focused on the contract seams most likely to regress:

- OpenCode execute inputs are either mapped or rejected before `prompt_async`.
- Rollback uses runtime-specific control paths for Claude, Codex, and OpenCode.
- Catalog capability state matches route/preflight behavior, including degraded reasons for auth/config gaps.

This is preferable to broad end-to-end smoke-only coverage because the drift risk is at the normalization boundary, not at route registration.

## Risks / Trade-offs

- **[Risk] OpenCode server surfaces vary by deployment** → Model the bridge around capability-aware preflight and degraded states so unsupported server combinations fail explicitly instead of silently dropping fields.
- **[Risk] Codex rollback semantics are less stable than fork/resume** → Isolate Codex rollback behind a dedicated runner and continuity parser so future CLI changes are localized.
- **[Risk] Capability-state logic becomes harder to reason about** → Keep support-state derivation centralized and reuse the same reason codes across catalog serialization, execute validation, and route errors.
- **[Trade-off] More requests fail early instead of “best effort” running** → This is intentional; truthful failure is preferable to upstream systems assuming inputs were honored when they were not.
- **[Trade-off] Runtime catalog becomes more dynamic and provider-aware** → Upstream consumers need to tolerate degraded states changing with auth/config readiness, but that is better than relying on stale static feature flags.

## Migration Plan

1. Extend the OpenSpec delta specs for cross-runtime inputs, registry metadata, OpenCode controls, and Codex rollback.
2. Update the TS Bridge registry and runtime handlers so execute preflight, rollback controls, and catalog metadata share the same capability truth.
3. Add targeted Bun tests for OpenCode execute mapping/rejection, rollback routing, and runtime catalog diagnostics.
4. Roll out without changing canonical route families or request envelopes; this is an additive contract-hardening change.
5. If a runtime-specific control proves unavailable in a live environment, keep the route and catalog in a degraded state with explicit reason codes rather than reverting to silent ignore behavior.

## Open Questions

- Which OpenCode execute-time config concerns can be applied per session versus only globally through `PATCH /config`? The implementation should prefer session-scoped truth when both exist.
- What rollback target shape is most stable for Codex: explicit checkpoint identifiers, relative turns, or a bridge-owned translation layer over thread history? The runner abstraction should allow the bridge to evolve without changing the external `/bridge/rollback` contract.
