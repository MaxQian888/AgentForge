## Context

AgentForge's review pipeline has a functioning Layer 2 bridge and ReviewPlugin aggregation, but the end-to-end loop is broken in four places:

1. **Policy persistence gap** — `ProjectSettings` in Go only stores `codingAgent`; the frontend's `reviewPolicy` (requiredLayers, requireManualApproval, minRiskLevelForBlock) is never saved or read back from the backend.
2. **State-machine corruption** — `ReviewService.Complete` writes findings, executionMetadata, summary, and costUSD unconditionally; Approve/Reject/RequestChanges all re-invoke `Complete` with a sparse payload, destroying original evidence.
3. **Frontend blindspots** — The TS `ReviewRecord` type is missing `executionMetadata`; `pending_human` has no action surface; WebSocket `review.*` events are never consumed.
4. **CI / Layer 1 disconnect** — The backend exposes `/reviews/ci-result` but no workflow calls it; `agent-review.yml` hard-codes `needs_deep_review: true` instead of emitting structured JSON.

## Goals / Non-Goals

**Goals:**
- ReviewPolicy fields are persisted in the backend and read back faithfully
- `ReviewService.Complete` evaluates project policy before resolving — routes to `pending_human` when required
- Human transitions (approve / request-changes / false-positive) append a decision record without touching review evidence
- Layer 1 CI workflow emits structured JSON and calls `/reviews/ci-result`
- Frontend type matches backend DTO; `pending_human` is actionable; WebSocket events update the review backlog live
- IM `/review` gains `deep`, `approve`, `request-changes` subcommands with action buttons

**Non-Goals:**
- Rewriting Layer 2 bridge from heuristics to a multi-Agent runtime (deferred)
- Cross-platform GitHub App integration or new OAuth scopes
- Changing existing review plugin manifest format or MCP transport

## Decisions

### D1: ReviewPolicy lives inside `ProjectSettings.GovernanceSettings`

**Decision:** Add a `ReviewPolicy` struct to `GovernanceSettings` (not as a top-level field on `Project`). The existing `JSONB`-backed settings column stores the full governance document; no new DB column is needed.

**Rationale:** `ProjectSettings` is already a JSONB settings bag. Adding `reviewPolicy` alongside existing budget/alert fields is zero-migration and stays consistent with the existing merge-and-persist pattern. Alternatives considered: (a) a dedicated `project_review_policies` table — rejected for over-engineering a small document; (b) top-level `Project` columns — rejected because they'd require a schema migration for every new policy field.

### D2: Review state machine via dedicated transition methods

**Decision:** Introduce `ApproveReview`, `RequestChangesReview`, and `MarkFalsePositive` on `ReviewService`. Each method loads the existing review, writes a `ReviewDecision` struct to a new `decisions []ReviewDecision` field in `ExecutionMetadata`, updates `Status` and `Recommendation`, and saves — without touching `Findings`, `Summary`, `CostUSD`, or plugin provenance.

**Rationale:** Re-using `Complete` for human decisions conflates two things: writing automated evidence (Done once, by the bridge) and recording human judgement (Done zero or more times after). The separation makes evidence immutable post-automation, and it enables an audit trail of decisions. Alternative considered: event-sourced append-only log — correct in principle but adds infrastructure; `decisions` array inside existing JSONB metadata is the minimum viable form.

### D3: pending_human routing is policy-driven inside Complete

**Decision:** After a bridge result is persisted, `ReviewService.Complete` loads the project settings and checks: (a) `policy.requireManualApproval` — if true, transition to `pending_human`; (b) `policy.minRiskLevelForBlock` — if the result's max finding severity meets or exceeds the threshold, also route to `pending_human`. Only when neither condition fires does `Complete` auto-resolve.

**Rationale:** The existing `RequestHumanApproval` helper was never wired in. Integrating the check directly inside `Complete` is the minimal change: one place, one decision, correct for all callers.

### D4: Standalone deep review by PR URL

**Decision:** Extend `CreateReview` so that when `taskId` is absent but `prUrl` is present, the service creates a detached review record (with a synthetic or nil task reference) and proceeds. Return the new `reviewId` immediately; a background goroutine triggers the bridge run.

**Rationale:** The spec requires `/review deep <pr-url>` from IM to work without a pre-existing task. Returning the review ID synchronously lets the caller track progress via status or WebSocket events. Alternative: require callers to create a task first — rejected because it adds friction for ad-hoc manual reviews.

### D5: Layer 1 workflow emits structured JSON to ci-result endpoint

**Decision:** Rewrite `agent-review.yml` to: (1) run a lightweight analysis step (file count, diff size, label presence) that outputs `needs_deep_review: true/false` with reason; (2) POST the result to `POST /api/v1/reviews/ci-result` using a repository secret for authentication. Keep `review-layer2.yml` as the listener for `Agent PR Review` events.

**Rationale:** The backend endpoint already exists; wiring it in closes the Layer 1 → backend feedback loop without new Go code. The lightweight analysis step can be a shell script; it does not need a full LLM call at Layer 1.

### D6: Frontend type parity and WS event registration

**Decision:** Extend the TS `ReviewRecord` interface to include `executionMetadata?: ExecutionMetadata`; add `requestChanges` and `markFalsePositive` actions to the review store; register `review.completed`, `review.pending_human`, and `review.updated` handlers in `ws-store` that call `reviewStore.updateReview`.

**Rationale:** These are pure alignment fixes — the backend already emits the data and events; the frontend just needs to declare and consume them.

## Risks / Trade-offs

- **Risk: Existing Approve/Reject call sites break** — The handler layer today delegates to `Complete`. Switching to the new transition methods requires updating all callers (handler, IM action executor). If any call site is missed, status updates will silently fail.
  → **Mitigation:** Grep all callers before removing the old path; keep `Complete` for bridge results only, mark old delegation with a compile-time guard.

- **Risk: Settings merge logic drops reviewPolicy** — If the existing project settings merge helper does a shallow merge, new `reviewPolicy` sub-fields could be dropped on save.
  → **Mitigation:** Unit-test the merge function specifically for nested `reviewPolicy` before wiring the handler.

- **Risk: Layer 1 CI secret exposure** — Posting to the backend from CI requires a service token stored as a GitHub secret.
  → **Mitigation:** Use a scoped CI-only token with write-review-result permission only; rotate on bridge re-key.

- **Risk: detached review (no taskId) breaks task-state follow-up** — If a standalone deep review resolves, there is no task to update.
  → **Mitigation:** Skip task-state update when `taskId` is nil; log and emit a review event only.

## Migration Plan

1. Deploy backend changes first (ProjectSettings schema extension, new transition methods, Complete routing) — fully backward-compatible; existing reviews continue to work.
2. Deploy frontend type and WS changes — no functional impact until new reviews with `executionMetadata` arrive.
3. Merge CI workflow changes — activates Layer 1 → backend feedback; requires the CI service token secret to be set in the repository.
4. No database migrations needed for the policy change (JSONB column already exists).

## Open Questions

- Should `decisions []ReviewDecision` be persisted in the existing `execution_metadata` JSONB column, or in a separate `review_decisions` column? (Current decision: reuse `execution_metadata` for minimum schema change.)
- Should the Layer 1 script be a composite GitHub Action or an inline shell step? (Current decision: inline shell for simplicity; can be extracted later.)
