## 1. Canonical dispatch control-plane core

- [ ] 1.1 Add shared Go dispatch decision / guardrail models and DTO mappings for `started`, `queued`, `blocked`, and `skipped`, including queue and machine-readable reason metadata.
- [ ] 1.2 Refactor `TaskDispatchService` assignment/manual-spawn flows and `AgentService.RequestSpawn(...)` to use the shared dispatch control-plane preflight instead of duplicating guardrail checks.
- [ ] 1.3 Update queued promotion to reuse the canonical dispatch preflight and classify promotion rechecks into promoted, still-queued, or terminal failed outcomes.

## 2. Layered budget governance and persistence

- [ ] 2.1 Add the persistence, model, and repository changes required for project-level dispatch budget state and any queue metadata needed to preserve budget guardrail context.
- [ ] 2.2 Implement task / sprint / project budget snapshot resolution and admission-time preflight checks for assignment-triggered dispatch, manual spawn, and queued promotion.
- [ ] 2.3 Align scheduler-side cost reconciliation and related repositories so multi-scope spend snapshots stay truthful even when runtime updates or retries drift.

## 3. Runtime cost enforcement and queue visibility

- [ ] 3.1 Unify runtime cost handling behind the new budget governance component so warning and exceeded actions come from one authoritative path.
- [ ] 3.2 Apply warning / exceeded actions to the triggering run and task lifecycle, and freeze or unblock further admissions for the affected dispatch scope as required by the new rules.
- [ ] 3.3 Expand queue roster and pool lifecycle data so operator-facing APIs and realtime events expose still-queued, budget-blocked, and failed-promotion states with their latest guardrail reason.

## 4. API, WebSocket, IM, and frontend contract alignment

- [ ] 4.1 Update task assignment, agent spawn, and related realtime payloads to emit the canonical dispatch / queue / budget contract without losing existing event compatibility.
- [ ] 4.2 Mirror the canonical dispatch DTO in `src-im-bridge/client/**` and refresh `/task assign` plus `/agent spawn` reply formatting for `started`, `queued`, `blocked`, and `skipped` branches.
- [ ] 4.3 Update the relevant frontend stores and consumers to decode and render canonical dispatch reason, queue, and budget-scope metadata truthfully.

## 5. Verification and rollout hardening

- [ ] 5.1 Add focused Go tests for assignment dispatch, manual spawn, queued promotion, recoverable vs terminal guardrail failures, and multi-scope budget governance.
- [ ] 5.2 Add IM and frontend coverage for queued/skipped/budget-blocked contract decoding and user-facing reply/render branches.
- [ ] 5.3 Run targeted verification for the Go backend, IM bridge, and related TS consumers, then document migration/backfill expectations introduced by the new control plane.
