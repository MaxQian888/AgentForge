## Context

AgentForge's product and architecture documents already treat the TypeScript bridge as the runtime boundary for real Claude Agent SDK execution, but the code in `src-bridge/src/handlers/execute.ts` still emits hard-coded simulated events. At the same time, the current Go client and TypeScript server do not agree on their HTTP surface: Go calls `/api/execute`, `/api/status/:id`, `/api/cancel/:id`, and `/health` with camelCase fields, while the bridge currently serves `/bridge/*` routes with snake_case request bodies validated by Zod.

This change crosses the TypeScript bridge runtime, the Go bridge client, runtime/session tracking, and build dependencies. It also introduces an external runtime dependency on the Claude Agent SDK, which makes a design pass useful before implementation. The goal is to land a truthful runtime contract for agent execution without expanding into unrelated IM orchestration, review pipeline, or frontend work.

## Goals / Non-Goals

**Goals:**
- Replace `simulateAgentExecution(...)` with a real Claude Agent SDK-backed execution path inside the existing bridge runtime.
- Align Go and TypeScript around one canonical bridge API contract for execute, status, cancel, and health operations.
- Preserve the existing `AgentEvent` envelope while defining how SDK messages map into output, tool, cost, error, and lifecycle events.
- Keep budget, role injection, tool restriction, and cancellation enforcement inside the current bridge seams so apply work can be incremental.
- Store enough session metadata and snapshot state to support diagnostics and future resume work.

**Non-Goals:**
- Rebuilding the bridge around a new transport such as gRPC.
- Expanding this change to IM command routing, review agents, or frontend UI wiring.
- Solving full human-in-the-loop resume workflows beyond storing and exposing the state needed for later follow-up work.
- Generalizing the bridge for non-Claude providers in this change.

## Decisions

### 1. Keep the existing HTTP + WebSocket bridge architecture and retrofit the real runtime into it
We will keep the current Hono server, `RuntimePoolManager`, `EventStreamer`, and `SessionManager` structure instead of introducing a second execution stack. This matches the latest repository direction in `docs/PRD.md`, keeps scope contained, and lets implementation focus on replacing the simulated loop with a real SDK adapter.

Alternatives considered:
- Reintroduce gRPC as part of the runtime swap. Rejected because it multiplies scope and conflicts with the repo's current HTTP+WS direction.
- Build a parallel SDK worker service. Rejected because it duplicates lifecycle logic that already exists in `src-bridge`.

### 2. Standardize the runtime contract on the bridge-owned `/bridge/*` surface and snake_case payloads
The TypeScript bridge already validates snake_case request shapes and serves `/bridge/*` endpoints. The least disruptive path is to update the Go client to match that contract and keep the event envelope consistent with the existing bridge types.

Alternatives considered:
- Change the TypeScript bridge to `/api/*` and camelCase. Rejected because it requires broader server/schema churn with no runtime benefit.
- Support both contracts indefinitely. Rejected because dual contracts create silent drift and make status/cancel behavior harder to reason about.

### 3. Isolate Claude Agent SDK wiring behind a bridge runtime adapter
Implementation should introduce a focused adapter layer that builds SDK options from `ExecuteRequest`, invokes `query()`, classifies SDK messages, and updates `AgentRuntime`. `handleExecute` remains the orchestration entrypoint, while the adapter owns SDK-specific concerns such as message parsing, usage extraction, and abort handling.

Alternatives considered:
- Put all SDK handling directly in `handleExecute`. Rejected because it would entangle transport, runtime bookkeeping, and SDK parsing in one file.
- Push normalization into Go. Rejected because the bridge is the natural boundary that understands SDK semantics.

### 4. Preserve the existing `AgentEvent` categories, but define deterministic mapping rules from SDK messages
The bridge should normalize SDK output into the current event types already expected by Go: `status_change`, `output`, `tool_call`, `tool_result`, `cost_update`, `error`, and `snapshot`. `AgentRuntime` remains the source of current status, turn count, last tool, and spend. This lets Go continue consuming one event model while the bridge gains a truthful upstream source.

Alternatives considered:
- Expose raw SDK messages over WebSocket. Rejected because it leaks SDK internals across the process boundary and would force Go-side parsing churn.
- Collapse tool and output events into one free-form stream. Rejected because it loses structured data already modeled in the current contract.

### 5. Enforce cancellation and budget limits inside the bridge runtime, with snapshot persistence as a continuity boundary
The bridge will continue to own local abort control. Cancel requests, explicit aborts, and budget exhaustion should all terminate the active SDK run through the runtime's `AbortController`, emit terminal events, and persist the latest known session metadata into `SessionManager`. This change stores continuity state, but leaves full resume orchestration to a later follow-up.

Alternatives considered:
- Leave budget enforcement to Go only. Rejected because the bridge has the first trustworthy view of SDK usage and must be able to stop overspend promptly.
- Promise full resume behavior in this change. Rejected because there is not enough existing end-to-end resume wiring yet.

## Risks / Trade-offs

- [SDK package/runtime compatibility with Bun compile] -> Mitigation: treat SDK package selection and build validation as a first-class task, and keep the bridge adapter small so fallback refactors stay local.
- [SDK message shapes may not map one-to-one to current event types] -> Mitigation: define explicit normalization rules and cover them with bridge-focused tests.
- [Usage fields may be partial or absent on some responses] -> Mitigation: prefer API-reported usage when present and fall back to the existing pricing calculator only where needed.
- [Contract alignment can break Go callers during rollout] -> Mitigation: update the Go client in the same change and verify execute/status/cancel/health together rather than piecemeal.
- [Snapshot support may over-promise resume readiness] -> Mitigation: scope this change to persistence of continuity state and call out full resume as follow-up work.

## Migration Plan

1. Add the Claude Agent SDK dependency and any bridge-local configuration needed to bootstrap it safely.
2. Implement the bridge runtime adapter and swap `handleExecute` from simulation to real query-driven execution.
3. Align the Go client with the canonical bridge routes and payload schema.
4. Extend runtime/session tracking so cancel, failure, and terminal states persist the latest session continuity metadata.
5. Verify the bridge build and at least one bridge-level execution path end to end with the canonical HTTP contract.

Rollback strategy:
- Revert the bridge runtime adapter and Go client contract changes together so the system returns to the previous simulated path without leaving a half-aligned API surface.

## Open Questions

- Which exact Claude Agent SDK package/version should the bridge standardize on, given repository docs currently reference both `@anthropic-ai/claude-agent-sdk` and `@anthropic-ai/agent-sdk` in different places?
- What authentication/configuration bootstrap should the bridge use for local development versus packaged sidecar execution?
- Should the first implementation expose pause/resume HTTP routes now, or leave resume as stored state only until the Go orchestration path is ready?
