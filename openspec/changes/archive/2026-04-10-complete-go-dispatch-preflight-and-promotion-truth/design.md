## Context

The Go dispatch stack already covers assignment-triggered dispatch, manual spawn, queue admission, and queue promotion, but the remaining truth surface is split across three different seams. `DispatchPreflightHandler` performs a thinner advisory check than the real dispatch path, `promoteQueuedAdmission(...)` mutates queue state without persisting a matching dispatch history verdict, and dispatch observability exposes only a subset of the queue lifecycle that operators need to diagnose recoverable versus terminal outcomes.

This change stays narrow on purpose. It does not reopen the earlier control-plane foundation work, add new runtimes, or redesign operator UI. The goal is to make the existing Go dispatch seams agree on the same preflight truth, promotion verdict handling, and observability contract.

## Goals / Non-Goals

**Goals:**
- Reuse one canonical Go preflight evaluation path for advisory reads, manual spawn, and queued promotion revalidation.
- Make promotion rechecks produce the same machine-readable verdict shape as immediate dispatch decisions.
- Persist promotion verdicts into dispatch history so task timelines reflect queued, recoverable, terminal, and promoted outcomes truthfully.
- Expand dispatch stats and realtime queue payloads so operators can diagnose promotion success, cancellation, and terminal failure without parsing free-form reason strings.
- Keep the API evolution additive so current consumers can adopt the richer contract without a breaking migration.

**Non-Goals:**
- No new runtime or provider features.
- No new dashboard product surface or broad frontend redesign.
- No rewrite of the entire dispatch architecture into a new standalone service layer.
- No scope expansion into unrelated IM command UX beyond what naturally follows from shared Go DTO contracts.

## Decisions

### Decision 1: Introduce a shared canonical preflight evaluation helper instead of keeping handler- and promotion-specific checks

The current mismatch comes from multiple seams making similar but not identical admission decisions. This change will introduce one shared preflight evaluation helper inside the existing Go dispatch service layer. That helper will resolve task and member context, task/sprint/project budget pressure, active-run conflicts, pool readiness, and transient system guardrails, then return one canonical advisory verdict object.

`DispatchPreflightHandler` will call that helper in read-only mode and map the result to the preflight API response. `TaskDispatchService` and queued promotion revalidation will call the same helper before they commit to runtime startup or queue mutation. The shared helper owns non-start classification; the surrounding service still owns side effects such as queue writes, notifications, and runtime start.

**Alternative considered:** leave preflight as a handler-local approximation and patch promotion separately.  
**Rejected because:** it preserves the exact drift this change is trying to remove.

### Decision 2: Keep queue state and dispatch history as separate but coordinated truth surfaces

Queue entries are the authoritative current-state record for admission lifecycle, while dispatch attempts are the authoritative chronological history of dispatch decisions. This change will keep that split, but promotion revalidation will now write a dispatch attempt whenever it produces a new queued, blocked, failed, or promoted verdict.

That means operators can read current queue state from the queue roster and reconstruct the task-level timeline from dispatch history without inferring promotion events indirectly from queue mutations. The attempt record will stay summary-oriented: queue linkage, trigger lineage, guardrail classification, and resolved runtime tuple are persisted, but full pool snapshots or raw bridge diagnostics are not.

**Alternative considered:** derive promotion history lazily from queue entries when reading history.  
**Rejected because:** it loses chronology, trigger semantics, and earlier verdicts once the queue entry changes state.

### Decision 3: Upgrade contracts additively across preflight, stats, and realtime queue events

The external contract changes stay additive rather than replacing the existing fields. Preflight will gain explicit non-start guardrail metadata so blocked member or system verdicts stop masquerading as budget-only blocks. Dispatch stats will gain promotion lifecycle metrics and optional time-window filtering. Queue promotion realtime payloads will be emitted only after the queue entry has its final promoted linkage so consumers receive a truthful `promoted` queue record with the linked run.

This approach keeps current clients compatible while giving new consumers a stable machine-readable path. Existing human-readable `reason` strings remain for display, but they stop being the only source of truth.

**Alternative considered:** replace the current DTOs outright with a new contract.  
**Rejected because:** it would turn a focused seam fix into a broad multi-surface migration.

### Decision 4: Treat recoverable and terminal promotion failures as distinct lifecycle outcomes

Promotion revalidation must distinguish between conditions that can clear on a later retry and conditions that permanently invalidate the queued request. Recoverable outcomes stay in queue with refreshed verdict metadata and generate a new dispatch attempt. Terminal outcomes mark the queue entry failed, generate a terminal attempt, and preserve the original admission context for diagnosis. Successful promotions persist a promoted verdict, complete the queue entry with run linkage, and emit the final promoted payload.

**Alternative considered:** keep all promotion failures as a generic failed or generic blocked branch.  
**Rejected because:** it collapses the operator distinction between “still waiting” and “never going to start.”

## Risks / Trade-offs

- **[Risk] Shared preflight logic could still drift if commit paths add new guardrails outside the helper** → **Mitigation:** keep non-start classification inside the helper and make new guardrails extend that one seam first.
- **[Risk] Promotion rechecks will create more dispatch-attempt rows** → **Mitigation:** record only decision points that materially change queue or runtime truth, not every background poll.
- **[Risk] Time-window stats can increase query complexity on larger projects** → **Mitigation:** make filters optional, keep bounded defaults, and reuse existing indexed timestamps before considering wider schema changes.
- **[Risk] Realtime payload ordering bugs can survive if queue completion and event emission remain loosely coupled** → **Mitigation:** complete queue state first, then emit the promoted payload from the finalized queue DTO.

## Migration Plan

1. Introduce the shared preflight evaluation helper and wire `DispatchPreflightHandler`, `TaskDispatchService`, and queued promotion to it in additive fashion.
2. Extend dispatch attempt recording so promotion revalidation writes canonical verdict-bearing history entries.
3. Expand dispatch stats and queue lifecycle payload shaping, including the finalized promoted queue payload and optional stats filters.
4. Update OpenAPI and AsyncAPI contracts to match the richer additive response fields.
5. Verify with focused Go tests around preflight, promotion revalidation, history, stats, and realtime queue payload ordering.

## Open Questions

- Should successful promotion be recorded in dispatch history as a dedicated `promoted` attempt, or as a `started` attempt with mandatory queue linkage? Current leaning: keep `started` as the outcome and require queue linkage so history stays aligned with existing dispatch status branches.
- Should preflight expose bridge or worktree diagnostics directly, or only return guardrail classification plus human-readable reason? Current leaning: keep full diagnostics internal and expose only the canonical machine-readable summary.
