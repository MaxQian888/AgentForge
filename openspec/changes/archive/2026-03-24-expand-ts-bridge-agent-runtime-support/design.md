## Context

`src-bridge` today has one hard-wired execute path: `server.ts` validates an execute request, `handleExecute(...)` acquires a runtime from the pool, and `claude-runtime.ts` invokes the Claude-specific query runner while normalizing its output into AgentForge events. That architecture delivered a truthful Claude-backed baseline, but it also means every new coding agent backend would currently require another round of branching inside `handleExecute(...)`, ad hoc environment handling, and backend-specific event shaping mixed into the same path.

At the same time, the repository is already moving toward richer agent orchestration. Go persists `provider` and `model`, role YAML files define reusable agent behavior, and the bridge is the canonical execution boundary. Supporting Claude Code, Codex, and OpenCode is therefore not just a dependency addition; it changes how execute requests identify their runtime, how configuration is validated, and how multiple backends share one lifecycle contract without fragmenting the event stream or cancellation model.

## Goals / Non-Goals

**Goals:**
- Introduce one extensible runtime registry for coding-agent backends in `src-bridge`.
- Support Claude Code, Codex, and OpenCode as first-class runtime keys behind the same `/bridge/execute` surface.
- Separate runtime selection from provider/model hints so the bridge can choose an execution backend explicitly instead of inferring it from Claude-specific assumptions.
- Preserve the existing AgentForge lifecycle guarantees for status transitions, event streaming, budget checks, cancellation, and session snapshots across all supported runtimes.
- Keep follow-up runtime additions cheap by defining a stable adapter contract, configuration model, and verification surface.

**Non-Goals:**
- Rework lightweight AI flows such as decomposition or review in this change.
- Promise feature parity across all runtimes for every tool, model, or streaming primitive on day one.
- Redesign frontend runtime selection UI or broader task-management workflows in this proposal.
- Replace the current Claude-backed implementation before alternative runtimes are verified; Claude remains the baseline adapter during migration.

## Decisions

### 1. Add an explicit `runtime` selector to the bridge execution contract
The canonical execute request will gain an optional `runtime` field whose stable values are runtime keys such as `claude_code`, `codex`, and `opencode`. The bridge will treat this as the primary backend selector. Existing `provider` and `model` fields remain available as runtime-specific hints or backward-compatibility inputs, but they no longer define which adapter is chosen.

This avoids overloading `provider` with two meanings: model-provider selection for lightweight generation and agent-runtime selection for coding backends. It also lets the bridge reject unsupported runtime requests clearly instead of silently treating everything as Claude.

Alternatives considered:
- Keep using `provider` as the runtime key. Rejected because it cements the current ambiguity and collides with the separate provider-support work already active in the repo.
- Infer runtime from role names or model strings. Rejected because those are unstable inputs and would make future runtime expansion brittle.

### 2. Resolve execute requests through a runtime registry before pool acquisition
`handleExecute(...)` will no longer know how to launch specific backends directly. Instead, the server/handler layer will resolve the request through a runtime registry that returns adapter metadata, normalized configuration, and any launch prerequisites before a runtime is acquired from the pool.

This keeps validation and runtime selection ahead of stateful work. It also means unsupported runtime keys, missing credentials, or unavailable binaries fail fast before the bridge creates an active runtime entry.

Alternatives considered:
- Leave selection inside `handleExecute(...)` and add more conditionals. Rejected because it repeats the current coupling and makes every new backend touch core lifecycle code.
- Resolve the runtime in Go instead of the bridge. Rejected because the bridge is already the execution boundary and should remain the single source of truth for backend launch semantics.

### 3. Define one adapter interface per coding runtime
Each runtime backend will implement the same bridge-local adapter interface: build launch options from the canonical request, start execution, stream native events through a normalizer, expose cancellation hooks, and report runtime metadata such as default models and required configuration. The current Claude-backed runner becomes one adapter in that set, while Codex and OpenCode land as peer adapters.

This lets the bridge share runtime lifecycle code while isolating backend-specific launch mechanics and output parsing. It also creates the seam future runtimes can plug into without modifying the event contract itself.

Alternatives considered:
- Separate execute endpoints per backend. Rejected because it pushes routing complexity to Go and fragments cancellation/status semantics.
- One giant universal runtime class. Rejected because the backends will differ in launch method, event shape, and config, so forcing one implementation would reduce clarity.

### 4. Centralize runtime configuration and availability checks
The registry will own default runtime selection, per-runtime executable/API-key validation, runtime-specific model defaults, and any bridge-local capability metadata. Configuration errors must be reported distinctly, for example: unknown runtime, runtime disabled, required executable missing, or required credential missing.

This matters because Claude Code, Codex, and OpenCode are likely to differ in whether they launch through SDKs, CLIs, or local wrappers. Without a centralized configuration layer, each adapter would rediscover environment conventions independently and drift over time.

Alternatives considered:
- Let each adapter read environment variables ad hoc. Rejected because it hides operator-facing requirements and makes diagnostics inconsistent.
- Require every caller to provide all runtime details explicitly. Rejected because the bridge should remain operable with sensible defaults and environment-backed configuration.

### 5. Preserve the canonical AgentForge event model across runtimes
The bridge will keep the existing `AgentEvent` categories and runtime pool/session semantics. Each adapter is responsible for translating its backend-native stream into the canonical events the Go orchestrator already understands: `output`, `tool_call`, `tool_result`, `status_change`, `cost_update`, `error`, and `snapshot`.

This protects downstream consumers from runtime-specific branching and lets new backends reuse the same status, cancel, and observability paths. Runtime-specific raw fields may still exist internally, but they do not become part of the public bridge contract.

Alternatives considered:
- Expose each runtime's native event shape upstream. Rejected because it would force Go and the frontend to understand three incompatible protocols.
- Normalize only status/output and leave the rest runtime-specific. Rejected because cost, tool activity, and cancellation are part of the current contract and would become the next source of drift.

## Risks / Trade-offs

- [Runtime selection adds a second contract axis beyond provider/model] -> Mitigation: make `runtime` explicit, keep backward-compatible fallbacks limited, and document the precedence rules in schema/specs.
- [Codex and OpenCode may require different launch mechanisms than the current Claude-backed adapter] -> Mitigation: put launch/config validation inside adapter metadata and stage each runtime behind the same interface instead of assuming identical bootstrap paths.
- [Event normalization may be lossy for runtimes with richer or poorer native streams] -> Mitigation: preserve the canonical required fields and allow adapter-local metadata only behind internal seams until a broader contract change is justified.
- [Introducing multiple runtimes can make local setup and CI verification noisier] -> Mitigation: define a focused verification matrix and require explicit unavailable-runtime errors instead of best-effort silent fallbacks.
- [Bridge and Go persistence may drift if runtime selection is not represented consistently] -> Mitigation: include Go bridge-client alignment and backward-compatibility behavior in early implementation tasks.

## Migration Plan

1. Extend the bridge execute schema/types and the Go bridge client contract to accept an explicit `runtime` selector, while preserving a safe fallback path for current callers.
2. Add a bridge-local runtime registry plus shared adapter interfaces and move the current Claude-backed runtime behind that abstraction.
3. Implement Codex and OpenCode adapters with centralized configuration discovery and normalized event translation.
4. Update execute-path validation, error handling, and runtime selection so unsupported or misconfigured runtimes fail before pool acquisition.
5. Verify bridge-focused tests for runtime resolution, adapter normalization, cancellation, and Go/bridge contract alignment.

Rollback strategy:
- Revert the registry/adapters together and fall back to the current single Claude-backed execute path if one of the alternative runtimes introduces instability that cannot be isolated quickly.

## Open Questions

- Should the first implementation use direct CLI process adapters for Codex/OpenCode, bridge-local wrapper scripts, or SDK integrations where available?
- How much backward compatibility should the bridge preserve for callers that only send `provider` without `runtime`?
- Does the Go persistence layer need a first-class `runtime` field immediately, or can it bridge through existing provider/model storage for the first iteration?
