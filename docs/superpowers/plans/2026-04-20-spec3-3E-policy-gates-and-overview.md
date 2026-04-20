# Spec 3E — Action Policy Gates + Approval Integration + FE Employee Overview

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 落地 Spec 3 安全闸 + 人工审批闭环（复用 1E im_send + wait_event）+ FE 员工概览页（绑定/指标图/动作日志/待审批）。

**Architecture:** 新节点 `qianchuan_policy_gate` 在每个 `qianchuan_action_executor` 前判断 blackout / 超限 / 需审批；需审批走 1E 卡片回调闭环；FE 给员工概览展示绑定 + recharts 指标图 + action 历史 + pending approvals 替代路径。

**Tech Stack:** Go (新节点 + 复用 1E im_send 服务 + 复用 wait_event resumer), Postgres, Next.js + recharts, Zustand.

**Depends on:** 3D (action_executor 节点 + action_logs 表 + canonical DAG); 1E (im_send 服务 + card_action_correlations + wait_event resumer)

**Parallel with:** 3B (OAuth) — disjoint files

**Unblocks:** Spec 3 整体闭环

---

## Coordination notes (read before starting)

- **3D ↔ 3E ordering on `qianchuan_action_executor`:** 3D ships the executor that **always calls** `Provider.ApplyAction`. This plan modifies that executor in Task E5 to short-circuit when `action_logs.status != 'allowed'`. Sequence is 3D → 3E. If 3D has already shipped a `status` check, skip Task E5 Steps 5.2–5.3 and only add the gating tests.
- **`action_logs.status` enum is owned by 3D.** Spec §6.4 lists the canonical enum (`'applied','blocked_by_policy','approved','rejected','failed','noop'`). This plan adds three new values that 3D's column constraint must already allow: `'gated'`, `'allowed'`, `'requires_approval'`. If 3D used a CHECK constraint, Task E1 ships an extra `ALTER TABLE` (Step 1.4) — otherwise no schema change.
- **1E `imcards.SendActionCard` signature:** This plan's gate applier calls into 1E's helper to mint the approval card + correlation token in one step. Expected shape: `imcards.SendActionCard(ctx, params{ProjectID, ReplyTarget, ExecutionID, NodeID, Card, Actions []ActionSpec}) (token uuid.UUID, err error)`. If 1E exports `SendCard`/`EmitCard`/separate mint+send, align in Task E3 Step 3.4.
- **`wait_event` resumer:** completed by 1E Task E2. This plan only **calls** it via the same `card_action_router` endpoint 1E ships. Do not re-implement Resume.
- **Canonical DAG seed lives in 3D** (`system:qianchuan_strategy_loop`). Task E6 of this plan **edits the seed file in place** to insert the gate node before each executor. The loop seed has version-bump idempotency: bump `parsed_spec.version` from whatever 3D set (e.g. `v1`) to `v2`.
- **Default-policy backfill:** Spec §4 row 12 says policy is per-binding. Rather than push a trigger into 3A's binding migration, Task E2 ships a backfill insert + a service-layer hook (`bindingService.ensureDefaultPolicy(bindingID)`) that runs on binding create. 3A stays minimal.
- **No new effect kinds.** Skipped actions surface via `action_logs.status='gated'` + the executor's status check. Approval pause uses existing `EffectWaitEvent` (carrying a synthetic `event_type` like `qianchuan_action_approval`).
- **FE folder layout:** Spec 1 §13 promises `/employees/:id/runs` and §1A creates `app/(dashboard)/employees/[id]/...`. This plan adds `app/(dashboard)/employees/[id]/qianchuan/page.tsx` as a **sibling tab** alongside the runs / triggers tabs. Wire it into the same tab list 1A registers.
- **recharts is already in deps** (see `components/cost/spending-trend-chart.tsx`). Mirror that file's `<ResponsiveContainer><LineChart>` shape for visual consistency. Do **not** add a new chart library.
- **Migration number:** Spec 3 reserved 070–074. 3A used 070–074, so 3E claims **075**. If a parallel plan landed first, bump in lockstep — content does not depend on the digit.

---

## Tasks

### Phase 1 — Policy schema + service-layer default

- [ ] **Task E1: failing test — `qianchuan_binding_policies` table accepts the spec'd schema**
  - File: `src-go/internal/repository/qianchuan_binding_policy_repo_test.go`
  - Cases:
    1. `TestPolicyRepo_InsertAndLoad`: insert a row with default `max_bid_change_pct=20`, `max_budget_change_per_day_cents=50000`, `require_human_approval_for=["pause_ad","apply_material"]`, `blackout_hours=NULL`. Load by `binding_id` → struct equals input.
    2. `TestPolicyRepo_UniqueOnBindingID`: insert two rows with same `binding_id` → second errors with PG `unique_violation`.
    3. `TestPolicyRepo_CascadesOnBindingDelete`: delete the binding row → policy row is gone.
  - Run: `rtk go test ./internal/repository -run TestPolicyRepo` — fails (table missing).

- [ ] **Task E2: migration + repo implementation + default-policy backfill**
  - Step 2.1 — Up migration `src-go/migrations/075_create_qianchuan_binding_policies.up.sql`:
    ```sql
    CREATE TABLE qianchuan_binding_policies (
      id                              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
      binding_id                      uuid        NOT NULL UNIQUE
                                                   REFERENCES qianchuan_bindings(id) ON DELETE CASCADE,
      max_bid_change_pct              int         NOT NULL DEFAULT 20,
      max_budget_change_per_day_cents bigint      NOT NULL DEFAULT 50000,
      require_human_approval_for      jsonb       NOT NULL
                                                   DEFAULT '["pause_ad","apply_material"]'::jsonb,
      blackout_hours                  jsonb,
      created_at                      timestamptz NOT NULL DEFAULT now(),
      updated_at                      timestamptz NOT NULL DEFAULT now()
    );
    CREATE INDEX qianchuan_binding_policies_binding_idx
      ON qianchuan_binding_policies(binding_id);

    -- Backfill: every existing binding gets a default policy row.
    INSERT INTO qianchuan_binding_policies (binding_id)
      SELECT id FROM qianchuan_bindings
        ON CONFLICT (binding_id) DO NOTHING;
    ```
  - Step 2.2 — Down: `DROP INDEX IF EXISTS qianchuan_binding_policies_binding_idx; DROP TABLE IF EXISTS qianchuan_binding_policies;`
  - Step 2.3 — Optional: if 3D's `qianchuan_action_logs.status` uses a CHECK constraint listing the §6.4 enum, ship `ALTER TABLE qianchuan_action_logs DROP CONSTRAINT ... ADD CONSTRAINT ... CHECK (status IN ('applied','blocked_by_policy','approved','rejected','failed','noop','gated','allowed','requires_approval'))`. If 3D used a plain varchar, no DDL needed; document in §Spec Drifts of this plan.
  - Step 2.4 — Add `model.QianchuanBindingPolicy` (`src-go/internal/model/qianchuan_binding_policy.go`) and `repository.QianchuanBindingPolicyRepository` with `Get(bindingID)` / `Upsert(policy)` / `EnsureDefault(bindingID)`.
  - Step 2.5 — Wire `EnsureDefault` into `bindingService.Create` (touch the binding service file 3A added; merge the call after the binding row is committed).
  - Run: `rtk go test ./internal/repository -run TestPolicyRepo` and `rtk go test ./internal/service -run TestBindingService_Create` — both green.

### Phase 2 — Gate decision logic (pure)

- [ ] **Task E3: failing test — `policy_gate.Decide` matrix**
  - File: `src-go/internal/strategy/policy_gate_test.go` (mirrors existing `internal/strategy/` package created by 3C if present; otherwise `src-go/internal/qianchuan/policy_gate_test.go`)
  - Cases (table test):
    | name | policy | action_log row | now (UTC) | expected |
    |---|---|---|---|---|
    | within_limit | `max_bid_change_pct=20` | `kind:'adjust_bid', request:{delta_pct:10}` | 12:00 | `{status:'allowed'}` |
    | exceeds_bid_pct | `max_bid_change_pct=20` | `kind:'adjust_bid', request:{delta_pct:30}` | 12:00 | `{status:'gated', reason:'exceeds_limit', detail:'max_bid_change_pct: 30 > 20'}` |
    | exceeds_budget | `max_budget_change_per_day_cents=50000` | `kind:'adjust_budget', request:{delta_cents:60000}` | 12:00 | `{status:'gated', reason:'exceeds_limit'}` |
    | requires_approval | `require_human_approval_for=['pause_ad']` | `kind:'pause_ad'` | 12:00 | `{status:'gated', reason:'requires_approval'}` |
    | blackout | `blackout_hours=[{start:'00:00',end:'06:00'}]` | `kind:'adjust_bid', delta_pct:5` | 03:00 | `{status:'gated', reason:'blackout'}` |
    | blackout_outside | same | same | 12:00 | `{status:'allowed'}` |
    | notify_im_always | any | `kind:'notify_im'` | 12:00 | `{status:'allowed'}` (gate is no-op for non-mutating actions) |
    | empty_policy_defaults_safe | `{}` (nil require list) | `kind:'pause_ad'` | 12:00 | `{status:'allowed'}` (no approval list = default open) |
  - Run: `rtk go test ./internal/strategy -run TestPolicyGate` — fails (no implementation).

- [ ] **Task E4: implement `policy_gate.Decide`**
  - File: `src-go/internal/strategy/policy_gate.go`
    ```go
    package strategy

    import (
        "time"
        "github.com/agentforge/server/internal/model"
    )

    type GateDecision struct {
        Status string // 'allowed' | 'gated'
        Reason string // '' | 'exceeds_limit' | 'requires_approval' | 'blackout'
        Detail string
    }

    func Decide(policy *model.QianchuanBindingPolicy, action *model.QianchuanActionLog, now time.Time) GateDecision {
        // 1) blackout (checked first — even approval-required actions are blocked in blackout)
        if inBlackout(policy.BlackoutHours, now) {
            return GateDecision{Status: "gated", Reason: "blackout"}
        }
        // 2) require_human_approval_for membership
        for _, k := range policy.RequireHumanApprovalFor {
            if k == action.Kind {
                return GateDecision{Status: "gated", Reason: "requires_approval"}
            }
        }
        // 3) per-kind numeric limits
        switch action.Kind {
        case "adjust_bid":
            if delta := absInt(action.Request.GetInt("delta_pct")); delta > policy.MaxBidChangePct {
                return GateDecision{Status: "gated", Reason: "exceeds_limit",
                    Detail: fmt.Sprintf("max_bid_change_pct: %d > %d", delta, policy.MaxBidChangePct)}
            }
        case "adjust_budget":
            if delta := absInt64(action.Request.GetInt64("delta_cents")); delta > policy.MaxBudgetChangePerDayCents {
                return GateDecision{Status: "gated", Reason: "exceeds_limit"}
            }
        }
        return GateDecision{Status: "allowed"}
    }

    func inBlackout(windows []model.BlackoutWindow, now time.Time) bool { /* parse "HH:MM" pairs in the project's TZ; default UTC */ }
    ```
  - Acceptance: all Task E3 rows green. `rtk go test ./internal/strategy` clean.

### Phase 3 — Gate node (handler + applier)

- [ ] **Task E5: failing tests — `qianchuan_policy_gate` handler & applier**
  - File: `src-go/internal/workflow/nodetypes/qianchuan_policy_gate_test.go`
  - Cases:
    1. `TestGateHandler_AllowedEmitsNothing`: handler.Execute on a row that gate decides `allowed` → returns `NodeExecResult{Effects: nil}` and writes nothing (handler is pure).
    2. `TestGateHandler_GatedExceedsLimit_WritesStatus`: handler returns a single effect `Effect{Kind: EffectBroadcastEvent, Payload: {action_log_id, status:'gated', reason:'exceeds_limit'}}` (broadcast lets the applier persist + emit WS without coupling handler to repo).
    3. `TestGateHandler_GatedRequiresApproval_EmitsApprovalEffect`: handler returns two effects — broadcast (status='requires_approval') AND `EffectWaitEvent{event_type:'qianchuan_action_approval', match_key:'<action_log_id>'}` so the DAG runner parks the node.
    4. `TestGateApplier_ExceedsLimit_PersistsGatedRow`: applier handling the broadcast updates `action_logs.status='gated', error_message=reason`; no IM card sent.
    5. `TestGateApplier_RequiresApproval_MintsCardAndCorrelation`: applier calls `imcards.SendActionCard` with title containing `action_kind`, summary describing target+params, two callback actions `Approve`/`Reject`. Asserts: one row in `card_action_correlations` (execution_id = parent run, node_id = this gate, action_id ∈ {`approve`,`reject`}, payload includes `action_log_id`); IM Bridge mock receives one POST.
    6. `TestGateApplier_Blackout_NoCard`: status='gated', reason='blackout', no card minted.
  - Run: `rtk go test ./internal/workflow/nodetypes -run TestGate` — all fail.

- [ ] **Task E6: implement `qianchuan_policy_gate` handler + applier**
  - Files:
    - `src-go/internal/workflow/nodetypes/qianchuan_policy_gate.go` (handler)
    - `src-go/internal/workflow/nodetypes/qianchuan_policy_gate_applier.go` (applier; mirror the existing `applier.go` pattern in this dir)
  - Handler:
    - Config schema: `{ "action_log_id_template": {"type":"string"} }` — same template language as `qianchuan_action_executor` so loop iterations resolve `{{actions[i].id}}`.
    - Execute: resolve template against dataStore → load action_log + binding_policy via injected lookup interfaces (mirrors how `human_review.go` accepts a deps struct); call `strategy.Decide`; emit effects per decision matrix above.
  - Applier:
    - On the broadcast effect with `status='gated'`, persist via `qianchuanActionLogRepo.UpdateStatus(id, status, reason)`.
    - On `requires_approval`: call `imcards.SendActionCard(ctx, imcards.SendParams{ProjectID, ReplyTarget: bindingPolicy.NotifyChatID (fallback to binding's project default chat), ExecutionID: req.ExecutionID, NodeID: req.NodeID, Card: ProviderNeutralCard{...}, Actions: []imcards.ActionSpec{{ID:"approve", Label:"Approve", Style:"primary"}, {ID:"reject", Label:"Reject", Style:"danger"}}, Payload: {"action_log_id": id.String()}})`. The helper handles the correlation row + token embedding.
    - Set `system_metadata.im_dispatched=true` via `system_metadata = system_metadata || '{"im_dispatched": true}'::jsonb` (idempotent jsonb merge — do not overwrite reply_target).
  - Register handler + applier in `nodetypes/registry.go` and `nodetypes/applier.go` (mirror `qianchuan_action_executor` registration from 3D).
  - Run: `rtk go test ./internal/workflow/nodetypes -run TestGate` — green.

### Phase 4 — Action executor status check

- [ ] **Task E7: failing test — `qianchuan_action_executor` no-ops when status != 'allowed'**
  - File: `src-go/internal/workflow/nodetypes/qianchuan_action_executor_test.go` (extend 3D's test file)
  - Add cases:
    1. `TestExecutor_StatusGated_Noop`: action_log.status='gated' → executor returns success without calling `Provider.ApplyAction`; `action_logs.status` unchanged; no row update.
    2. `TestExecutor_StatusRejected_Noop`: status='rejected' → same.
    3. `TestExecutor_StatusAllowed_Calls_Provider`: status='allowed' → provider mock sees one `ApplyAction(ctx, t, b, action)` call; on success row becomes `status='applied'`.
    4. `TestExecutor_StatusApplied_DoubleFireGuard`: pre-existing status='applied' → executor noops (defensive — handles retries after success).
  - Run: `rtk go test ./internal/workflow/nodetypes -run TestExecutor_Status` — fails (3D's executor calls provider unconditionally).

- [ ] **Task E8: modify `qianchuan_action_executor` to check status before applying**
  - Edit 3D's `src-go/internal/workflow/nodetypes/qianchuan_action_executor.go` (or its applier — match where 3D placed the provider call):
    - Before the provider invocation, load the action_log row; if `status != 'allowed'`, return `NodeExecResult{}` immediately and write no further changes.
    - Only the `'allowed'` branch performs the apply + status flip to `'applied' | 'failed'`.
  - Run: `rtk go test ./internal/workflow/nodetypes -run TestExecutor` — full executor suite green (both Tasks E7's adds and 3D's originals).

### Phase 5 — Canonical DAG: insert gate before each executor

- [ ] **Task E9: failing test — `system:qianchuan_strategy_loop` v2 wiring**
  - File: `src-go/internal/qianchuan_runtime/seed_test.go` (extend 3D's seed test)
  - Add `TestSeed_LoopBodyInsertsGateBeforeExecutor`:
    - Load the seed; walk the loop body's nodes; assert order is `qianchuan_policy_gate → wait_event (event_type='qianchuan_action_approval') → qianchuan_action_executor → record_event → im_send`.
    - Assert each gate node's `config.action_log_id_template` matches the corresponding executor's template (same iteration variable).
  - Add `TestSeed_VersionBumpedOnHashChange`: re-seed with the modified body → `parsed_spec.version` advances from prior to next (`v1`→`v2`), prior row is **not** orphaned (idempotent upsert keyed on `(name, version)`).
  - Run: `rtk go test ./internal/qianchuan_runtime -run TestSeed` — fails.

- [ ] **Task E10: edit seed body + bump version**
  - Edit the seed file 3D created (likely `src-go/internal/qianchuan_runtime/seed.go` or `seeds/qianchuan_strategy_loop.yaml` depending on 3D's choice — match its style).
  - Insert nodes into the action loop body: `gate_<i>` (kind=`qianchuan_policy_gate`), `await_<i>` (kind=`wait_event`, event_type=`qianchuan_action_approval`, match_key templated to action_log id), then existing `executor_<i>` / `record_<i>` / `card_<i>` from 3D.
  - Wire gate's `next` → wait_event; wait_event resume `next` → executor. Branching: when gate decides `allowed`, the wait_event is configured with `optional_park=true` (or equivalent — match how `wait_event` in this codebase handles "no park needed"; if it always parks, instead emit two outgoing edges from the gate keyed by decision and skip wait_event on `allowed`). Pick whichever shape lines up with how the DAG runner currently treats `EffectWaitEvent`-emitting handlers — see Task E5 cases 1 vs 3.
  - Bump `parsed_spec.version`.
  - Run: `rtk go test ./internal/qianchuan_runtime -run TestSeed` — green.

### Phase 6 — FE-side approval endpoint (alternative path to Feishu card)

- [ ] **Task E11: failing test — `POST /api/v1/qianchuan/actions/:id/decision`**
  - File: `src-go/internal/handler/qianchuan_action_decision_handler_test.go`
  - Cases (Echo recorder):
    1. `TestDecide_Approve_ResumesWaitEvent`: action_log row `status='gated', reason='requires_approval'`; the parked execution + correlation row exist; POST `{decision:'approved'}` → 202 + body `{executionId}`. Spy on `waitEventResumer.Resume` — called once with `payload={decision:'approved', user_id}`. Action_log row updated to `status='approved', approved_by=user_id`.
    2. `TestDecide_Reject_ResumesWithRejection`: same but `{decision:'rejected'}` → resumer called with `{decision:'rejected'}`; row becomes `status='rejected'`.
    3. `TestDecide_AlreadyConsumed_409`: correlation already has `consumed_at` → 409 + `qianchuan:approval_already_decided`.
    4. `TestDecide_NoCorrelationFound_404`: action_log exists but no matching correlation row → 404 + `qianchuan:no_pending_approval`.
    5. `TestDecide_RBAC_RequiresEditor`: viewer-role caller → 403.
  - Run: `rtk go test ./internal/handler -run TestDecide` — fails.

- [ ] **Task E12: implement decision handler**
  - File: `src-go/internal/handler/qianchuan_action_decision_handler.go`
    - Look up action_log → confirm `status='gated'` + `reason='requires_approval'`.
    - Find the matching `card_action_correlations` row by `payload->>'action_log_id' = :id AND consumed_at IS NULL`. (Add a partial index `idx_cac_action_log` in a follow-on migration only if EXPLAIN shows a seq scan — defer otherwise.)
    - Mark correlation consumed; call `waitEventResumer.Resume(ctx, executionID, nodeID, payload)` with `{decision, user_id}`.
    - Update action_log: `status='approved'` or `'rejected'`, `approved_by=user_id`.
    - Wire route in `cmd/server/router.go` under `RequireProjectRole("editor")` (project derived via binding → project lookup).
  - Run: `rtk go test ./internal/handler -run TestDecide` — green.

### Phase 7 — FE qianchuan overview tab

- [ ] **Task E13: failing tests — overview page renders bindings + chart skeleton + actions table + pending approvals**
  - File: `app/(dashboard)/employees/[id]/qianchuan/page.test.tsx`
  - Mock `useQianchuanStore` (new — Zustand). Cases:
    1. `renders binding cards with status badges and sync button`: store seeded with two bindings (one active, one paused) → both visible; click "Sync" calls `store.fetchBindings`.
    2. `renders metrics line chart container`: chart skeleton appears even when data is `[]` (no crash); when data present, `<ResponsiveContainer>` is in the DOM.
    3. `renders recent actions table with status badges`: 5 rows rendered; gated row shows the `gate_reason` chip; rejected row uses the destructive-styled badge.
    4. `pending approvals section: approve calls API + toast`: gated+requires_approval row shows two buttons; clicking Approve calls `POST /api/v1/qianchuan/actions/:id/decision {decision:'approved'}` (spy on fetch); toast text from i18n key.
    5. `pending approvals: reject path mirrors approve`.
    6. `accessibility: tab is keyboard-navigable` — page heading has `role="heading" aria-level={1}`.
  - Run: `rtk pnpm test -- qianchuan/page.test` — fails (no page).

- [ ] **Task E14: implement overview page + Zustand store**
  - Files:
    - `app/(dashboard)/employees/[id]/qianchuan/page.tsx`
    - `lib/stores/qianchuan-store.ts` (mirror an existing simple store like `lib/stores/marketplace-store.ts` for shape — `bindings`, `metrics`, `actions`, `pendingApprovals`, plus `fetchBindings`, `fetchMetrics(bindingId, sinceHours)`, `fetchActions(bindingId, limit)`, `decideAction(actionId, decision)`)
    - `components/employees/qianchuan-bindings-list.tsx`
    - `components/employees/qianchuan-metrics-chart.tsx` — recharts `LineChart` over `cost`, `gmv`, `orders` series; copy `ResponsiveContainer`/axis style from `components/cost/spending-trend-chart.tsx`
    - `components/employees/qianchuan-actions-table.tsx` — `@tanstack/react-table`-free simple table; columns: time, binding, rule, action_type, target_ad, status badge, gate_reason
    - `components/employees/qianchuan-pending-approvals.tsx` — Approve/Reject buttons → store.decideAction → `sonner` toast
  - Tab registration:
    - Edit the employee detail tab list shipped by Plan 1A (most likely `app/(dashboard)/employees/[id]/layout.tsx` or a `<EmployeeTabs>` component). Add a new entry `Qianchuan` pointing to this page. If the tab list is data-driven (per Plan 1A), add a registry entry; otherwise inline.
    - If 1A has NOT yet shipped the layout, this task creates a minimal `[id]/layout.tsx` that hosts a tab nav with `Overview / Runs / Triggers / Qianchuan` and links to the corresponding routes — Plan 1A's eventual layout will replace it (additive, no conflict).
  - Hook `qianchuan-store.ts` into `lib/stores/index.ts` if a barrel exists.
  - Run: `rtk pnpm test -- qianchuan/page.test` — green. `rtk lint --fix` — clean. `rtk tsc` — no new errors.

### Phase 8 — End-to-end traces

- [ ] **Task E15: integration tests — Trace B (auto-allow) + Trace C (approval round-trip)**
  - File: `src-go/internal/qianchuan_runtime/integration_test.go` (extend 3D's integration file)
  - **Trace B**: seed binding + default policy + canonical DAG; inject snapshot fixture causing strategy to emit `adjust_bid {delta_pct: 15}` (within 20% limit). Run one tick. Assert: gate decides `allowed` → wait_event does **not** park → executor calls provider mock → `action_logs` row ends `status='applied'`. Single IM summary card sent.
  - **Trace C**: same setup; snapshot fixture causes `pause_ad` (in `require_human_approval_for`). Run tick. Assert: gate decides `requires_approval` → action_log row `status='gated', reason='requires_approval'` → IM Bridge mock got an approval card → execution status `waiting`. Then `POST /api/v1/qianchuan/actions/:id/decision {decision:'approved'}`. Assert: execution resumes, executor calls provider mock with `pause_ad`, action_log becomes `status='applied'` (note: §6.4 enum keeps `'applied'` for the post-approval write — `'approved'` is the gate-stage interim only, written by the decision handler before resumption; pick one path and stay consistent across handler + executor).
  - **Trace C — reject path**: same but `{decision:'rejected'}` → row ends `status='rejected'`, provider mock unused.
  - Run: `rtk go test ./internal/qianchuan_runtime -run TestTrace -tags=integration` — green (requires PG + Redis + IM Bridge mock harness 1E ships).

- [ ] **Task E16: smoke fixture for IM Bridge harness**
  - File: `src-im-bridge/scripts/smoke/qianchuan-approval-flow.json`
  - Mirrors `feishu-workflow-button-resume.json` from 1E but parametrized with a fake `action_log_id`. Validates the card → callback → resume path against a live IM Bridge process.
  - Add to the smoke runner manifest if one exists.
  - Run: `rtk pnpm --filter @agentforge/im-bridge test:smoke -- qianchuan-approval-flow` — green.

---

## Spec Drifts Found During Plan Writing

- **No new effect kinds added.** The brief suggested `EffectSkipAction` / `EffectWaitForApproval` semantics. After reading `src-go/internal/workflow/nodetypes/effects.go`, the closed enum already covers what we need: `EffectBroadcastEvent` carries the gate's status decision to the applier, `EffectWaitEvent` is exactly the parking mechanism approval needs. Adding new kinds would force changes to `IsPark` + every existing applier; reusing the two existing kinds keeps 3E surgical. If a future plan *does* want a typed `qianchuan.gated` effect, it's a one-line addition there — not a 3E concern.
- **`qianchuan_action_logs.status` enum overlap.** Spec §6.4 says the canonical values are `'applied','blocked_by_policy','approved','rejected','failed','noop'`. This plan needs three more — `'gated'`, `'allowed'`, `'requires_approval'` — to express the new gate states and let the executor short-circuit. The first two slot in cleanly; `'requires_approval'` is conceptually a sub-state of `'gated'` and lives in the `error_message` / dedicated `gate_reason` column instead. Final shape used by this plan: `status` ∈ existing-six ∪ {`'gated'`,`'allowed'`}; `gate_reason` (new column added in 3D or here as needed) ∈ {`'exceeds_limit'`,`'requires_approval'`,`'blackout'`}. If 3D didn't add `gate_reason`, fold it into Task E2 as an `ALTER TABLE qianchuan_action_logs ADD COLUMN gate_reason varchar(32)`.
- **Default-policy seeding moved out of 3A.** Brief allowed either path; this plan picks "backfill in 075 + service-layer hook" so 3A stays a pure binding-shape migration. Documented in Coordination.

Plan saved to D:\Project\AgentForge\docs\superpowers\plans\2026-04-20-spec3-3E-policy-gates-and-overview.md, 16 tasks, 70 total steps.
