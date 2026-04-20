# Spec 3D — Qianchuan Strategy Execution Loop (canonical DAG + 3 new node types + snapshot/action storage)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 落地 Spec 3 核心策略循环：3 个新节点（metrics_fetcher / strategy_runner / action_executor）+ 2 张时序/审计表 + canonical `system:qianchuan_strategy_loop` DAG seed + per-binding schedule trigger 物化。

**Architecture:** Spec 1 schedule trigger 按 binding+strategy 触发 → metrics_fetcher 节点用 Provider.FetchMetrics 拉数据 + UPSERT 时序表 → strategy_runner 节点解析 parsed_spec 评估条件 + emit actions（落 action_logs 行 status='pending'）→ loop 调用 action_executor 节点（policy gate 由 3E 加在前面）→ im_send 摘要。

**Tech Stack:** Go (新节点 + applier + Provider 调用 + 时序表 UPSERT), Postgres jsonb + 时序索引.

**Depends on:** 3A (Provider methods), 3C (strategy.parsed_spec format), 1B (secrets.Resolve for tokens), Spec 1 schedule trigger源.

**Parallel with:** 3B (OAuth + refresh) — disjoint files

**Unblocks:** 3E（policy gate wraps action_executor + FE 概览页 reads 时序+action_logs）

---

## Coordination notes (read before starting)

- **Migration numbering**: latest is `066_workflow_run_parent_link_parent_kind.up.sql`. Spec 1A claims `067` and `068`; Spec 1B/1E claim further numbers; Spec 3A claims `qianchuan_bindings` + `qianchuan_strategies` (070+071 per spec §6 ordering). This plan claims **072** (`qianchuan_metric_snapshots`) and **073** (`qianchuan_action_logs`). If preceding plans land later/earlier numbers, bump in lockstep — only filenames carry the integer.
- **Provider seam**: 3A owns `internal/adsplatform/provider.go` (`Provider` interface) and `internal/qianchuan/provider.go` (Qianchuan impl with `FetchMetrics` + `ApplyAction`). This plan ASSUMES the spec §8 signatures unchanged. If 3A renames `ApplyAction` or splits per-action methods, sync before D2/D4.
- **Strategy parser seam**: 3C owns `internal/strategy/manifest.go` + `engine.go`. This plan ASSUMES `qianchuan_strategies.parsed_spec` is a JSONB blob conforming to `strategy.ParsedSpec` and that `strategy.engine.Evaluate(snapshot, parsedSpec) []strategy.Action` is exported. If 3C exposes only a higher-level "EvaluateAndPersist", this plan's strategy_runner must downgrade to call the lower-level.
- **Secrets seam**: 1B owns `internal/secrets/service.go` with `Resolve(ctx, projectID, fieldPath, template) (string, error)`. metrics_fetcher and action_executor appliers call `Resolve(..., "qianchuan.fetch.token", "{{secrets.qianchuan."+bindingID+".access_token}}")`. The fieldPath string is the gate key — it MUST be on 1B's allowlist before 3D ships, or every fetch will fail with `secret:not_allowed_field`. Add the two fieldPaths (`qianchuan.fetch.token`, `qianchuan.action.token`) to 1B's whitelist constant during D2 Step 2.4 — owner agreed in spec §12 review.
- **Schedule trigger source**: Spec 1 §5 added `model.TriggerSourceSchedule` and the `schedule_ticker` (see `internal/trigger/schedule_ticker.go`). This plan creates `workflow_triggers` rows with `source='schedule'`, `created_via='manual'`, and a `Config` JSON containing `cron`, `binding_id`, `strategy_id`. The ticker's existing dedupe (`lastFire` map) is per-trigger-per-minute and is sufficient — do NOT add a Redis lock here (Spec 3 §5 mentions one but Spec 3E owns it as a per-binding gate).
- **Spec 3 §10 expression-unresolved drift**: spec §11 forbids `EvaluateCondition` from returning silent `true` on unresolved templates. 3C is supposed to deliver a hardened evaluator wrapper. If 3C did NOT land this wrapper yet, strategy_runner MUST treat `engine.Evaluate` errors as `noop` (write a `record_event` row with `outcome:'noop', detail:'expression_unresolved:<path>'`), per spec §11 — record this in §14 Drifts on completion.
- **Old code deletion**: greenfield (per spec §4 row 14). No legacy paths to remove.
- **Canonical DAG note**: Spec 3 §10 specifies the policy gate sits BETWEEN strategy_runner and action_executor. 3D ships the executor unconditionally; 3E wraps it. The `system:qianchuan_strategy_loop` definition seeded here therefore has NO gate node yet — 3E owns the surgical edit that inserts the gate edge. Document this clearly in the seed file's docstring so 3E's diff is mechanical.

---

## Task 1 — Migration 072: `qianchuan_metric_snapshots` (minute-bucketed time series)

- [ ] Step 1.1 — write the up migration
  - File: `src-go/migrations/072_create_qianchuan_metric_snapshots.up.sql`
    ```sql
    -- Per-binding minute-bucketed metric snapshots.
    -- The (binding_id, minute_bucket) UNIQUE supports UPSERT on the polling
    -- path so duplicate ticks within the same minute are idempotent.
    CREATE TABLE IF NOT EXISTS qianchuan_metric_snapshots (
        id            BIGSERIAL PRIMARY KEY,
        binding_id    UUID NOT NULL REFERENCES qianchuan_bindings(id) ON DELETE CASCADE,
        minute_bucket TIMESTAMPTZ NOT NULL,
        payload       JSONB NOT NULL,
        created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
        UNIQUE (binding_id, minute_bucket)
    );
    CREATE INDEX IF NOT EXISTS idx_qms_binding_time
        ON qianchuan_metric_snapshots (binding_id, minute_bucket DESC);
    ```
- [ ] Step 1.2 — write the down migration
  - File: `src-go/migrations/072_create_qianchuan_metric_snapshots.down.sql`
    ```sql
    DROP INDEX IF EXISTS idx_qms_binding_time;
    DROP TABLE IF EXISTS qianchuan_metric_snapshots;
    ```
- [ ] Step 1.3 — extend `migrations/embed_test.go` if it asserts a fixed count (search `TestEmbed` for length assertions and bump). Run `rtk cargo test -p server -run TestMigrations` to confirm sequence intact.
  - Acceptance: green; both files picked up by the embed FS.

---

## Task 2 — Migration 073: `qianchuan_action_logs` (per-action audit trail)

- [ ] Step 2.1 — write the up migration
  - File: `src-go/migrations/073_create_qianchuan_action_logs.up.sql`
    ```sql
    -- Per-action audit row. status enum:
    --   'pending'  — emitted by strategy_runner, not yet acted on
    --   'gated'    — policy_gate (3E) blocked or routed for human approval
    --   'applied'  — action_executor applied successfully
    --   'rejected' — operator rejected via approval card
    --   'failed'   — provider call returned an error
    CREATE TABLE IF NOT EXISTS qianchuan_action_logs (
        id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
        binding_id       UUID NOT NULL REFERENCES qianchuan_bindings(id) ON DELETE CASCADE,
        strategy_id      UUID REFERENCES qianchuan_strategies(id) ON DELETE SET NULL,
        strategy_run_id  UUID NOT NULL,
        rule_name        VARCHAR(128),
        action_type      VARCHAR(32) NOT NULL,
        target_ad_id     VARCHAR(64),
        params           JSONB NOT NULL,
        status           VARCHAR(16) NOT NULL DEFAULT 'pending',
        gate_reason      VARCHAR(128),
        applied_at       TIMESTAMPTZ,
        error_message    TEXT,
        created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
    );
    CREATE INDEX IF NOT EXISTS idx_qal_binding_time
        ON qianchuan_action_logs (binding_id, created_at DESC);
    CREATE INDEX IF NOT EXISTS idx_qal_run
        ON qianchuan_action_logs (strategy_run_id);
    ```
- [ ] Step 2.2 — write the down migration
  - File: `src-go/migrations/073_create_qianchuan_action_logs.down.sql`
    ```sql
    DROP INDEX IF EXISTS idx_qal_run;
    DROP INDEX IF EXISTS idx_qal_binding_time;
    DROP TABLE IF EXISTS qianchuan_action_logs;
    ```
- [ ] Step 2.3 — `rtk cargo test -p server -run TestMigrations` (or repo equivalent). Acceptance: green.

---

## Task 3 — Repos for snapshot + action_log (wire to existing `pkg/db` style)

- [ ] Step 3.1 — write failing repo tests for snapshot UPSERT and action_log create
  - File: `src-go/internal/repository/qianchuan_snapshot_repo_test.go`
    ```go
    package repository

    import (
        "context"
        "encoding/json"
        "testing"
        "time"

        "github.com/google/uuid"
    )

    func TestQianchuanSnapshotRepo_Upsert_Idempotent(t *testing.T) {
        repo := newTestSnapshotRepo(t)  // helper to be added next to existing newTestRepo
        ctx := context.Background()
        binding := uuid.New()
        bucket := time.Now().UTC().Truncate(time.Minute)
        payload1 := json.RawMessage(`{"ads":[{"ad_id":"AD7","roi":1.2}]}`)
        payload2 := json.RawMessage(`{"ads":[{"ad_id":"AD7","roi":1.4}]}`)
        if err := repo.Upsert(ctx, binding, bucket, payload1); err != nil { t.Fatal(err) }
        if err := repo.Upsert(ctx, binding, bucket, payload2); err != nil { t.Fatal(err) }
        rows, err := repo.ListByBinding(ctx, binding, 10)
        if err != nil { t.Fatal(err) }
        if len(rows) != 1 { t.Fatalf("expected 1 row after idempotent UPSERT, got %d", len(rows)) }
        if string(rows[0].Payload) != string(payload2) {
            t.Fatalf("payload not overwritten on conflict: got %s", rows[0].Payload)
        }
    }
    ```
  - Run: `rtk cargo test -p server -run TestQianchuanSnapshotRepo` → red.
- [ ] Step 3.2 — implement `internal/repository/qianchuan_snapshot_repo.go`
  - Struct fields: `BindingID uuid.UUID`, `MinuteBucket time.Time`, `Payload json.RawMessage`, `CreatedAt time.Time`.
  - Methods: `Upsert(ctx, bindingID, minuteBucket, payload) error` using `INSERT ... ON CONFLICT (binding_id, minute_bucket) DO UPDATE SET payload = EXCLUDED.payload`; `ListByBinding(ctx, bindingID, limit) ([]Snapshot, error)`; `Latest(ctx, bindingID) (*Snapshot, error)`.
- [ ] Step 3.3 — write failing repo tests for action_log
  - File: `src-go/internal/repository/qianchuan_action_log_repo_test.go`
    ```go
    func TestQianchuanActionLogRepo_CreatePending_AndUpdateApplied(t *testing.T) {
        repo := newTestActionLogRepo(t)
        ctx := context.Background()
        runID := uuid.New()
        log := &model.QianchuanActionLog{
            BindingID:     uuid.New(),
            StrategyRunID: runID,
            ActionType:    "adjust_bid",
            TargetAdID:    sqlString("AD7"),
            Params:        json.RawMessage(`{"delta_pct":-10}`),
            Status:        "pending",
        }
        if err := repo.Create(ctx, log); err != nil { t.Fatal(err) }
        if log.ID == uuid.Nil { t.Fatal("ID not assigned") }
        if err := repo.MarkApplied(ctx, log.ID); err != nil { t.Fatal(err) }
        got, _ := repo.GetByID(ctx, log.ID)
        if got.Status != "applied" || got.AppliedAt == nil {
            t.Fatalf("expected applied+timestamp, got %s / %v", got.Status, got.AppliedAt)
        }
    }
    ```
- [ ] Step 3.4 — implement `internal/repository/qianchuan_action_log_repo.go` and the matching `internal/model/qianchuan_action_log.go` model file (mirror the column shape; jsonb fields use `json.RawMessage`). Methods: `Create`, `GetByID`, `MarkApplied(id)`, `MarkFailed(id, errMsg)`, `MarkGated(id, reason)`, `ListByRun(runID)`, `ListByBinding(bindingID, limit)`.
- [ ] Step 3.5 — `rtk cargo test -p server -run TestQianchuan` → green.

---

## Task 4 — New node type `qianchuan_metrics_fetcher` (handler + effect kind)

- [ ] Step 4.1 — extend `nodetypes/effects.go` with the new effect kind + payload
  - File: `src-go/internal/workflow/nodetypes/effects.go`
  - Append after the existing `EffectResetNodes` line:
    ```go
    // Qianchuan effect kinds (Spec 3D). They are NOT park-effects — appliers
    // run synchronously inside the dispatcher's worker, write to the DataStore
    // under the node's id, and return.
    const (
        EffectFetchQianchuanMetrics  EffectKind = "fetch_qianchuan_metrics"
        EffectRunQianchuanStrategy   EffectKind = "run_qianchuan_strategy"
        EffectExecuteQianchuanAction EffectKind = "execute_qianchuan_action"
    )

    type FetchQianchuanMetricsPayload struct {
        BindingID  string   `json:"bindingId"`
        Dimensions []string `json:"dimensions,omitempty"`
        NodeID     string   `json:"nodeId"`
    }

    type RunQianchuanStrategyPayload struct {
        StrategyID  string          `json:"strategyId"`
        SnapshotRef json.RawMessage `json:"snapshotRef"`
        BindingID   string          `json:"bindingId"`
        NodeID      string          `json:"nodeId"`
    }

    type ExecuteQianchuanActionPayload struct {
        ActionLogID string `json:"actionLogId"`
        BindingID   string `json:"bindingId"`
        NodeID      string `json:"nodeId"`
    }
    ```
  - The three kinds must NOT be added to `IsPark()` — leave that switch as-is so they default to non-park.

- [ ] Step 4.2 — write failing handler test
  - File: `src-go/internal/workflow/nodetypes/qianchuan_metrics_fetcher_test.go`
    ```go
    func TestQianchuanMetricsFetcher_EmitsEffect_WithResolvedBindingID(t *testing.T) {
        h := QianchuanMetricsFetcherHandler{}
        req := &NodeExecRequest{
            Node:   &model.WorkflowNode{ID: "fetch_metrics"},
            Config: map[string]any{
                "binding_id_template": "{{$context.binding_id}}",
                "dimensions":          []any{"ads", "live"},
            },
            DataStore: map[string]any{
                "$context": map[string]any{"binding_id": "11111111-2222-3333-4444-555555555555"},
            },
        }
        out, err := h.Execute(context.Background(), req)
        if err != nil { t.Fatal(err) }
        if len(out.Effects) != 1 || out.Effects[0].Kind != EffectFetchQianchuanMetrics {
            t.Fatalf("expected one fetch_qianchuan_metrics effect, got %+v", out.Effects)
        }
        var p FetchQianchuanMetricsPayload
        _ = json.Unmarshal(out.Effects[0].Payload, &p)
        if p.BindingID != "11111111-2222-3333-4444-555555555555" {
            t.Fatalf("binding id template not resolved: %q", p.BindingID)
        }
        if len(p.Dimensions) != 2 || p.Dimensions[0] != "ads" {
            t.Fatalf("dimensions not propagated: %+v", p.Dimensions)
        }
    }
    ```
  - Run: `rtk cargo test -p server -run TestQianchuanMetricsFetcher` → red.
- [ ] Step 4.3 — implement the handler
  - File: `src-go/internal/workflow/nodetypes/qianchuan_metrics_fetcher.go`
    ```go
    package nodetypes

    import (
        "context"
        "encoding/json"
    )

    // QianchuanMetricsFetcherHandler implements the "qianchuan_metrics_fetcher"
    // node type. It emits a single EffectFetchQianchuanMetrics effect; the
    // applier resolves the access_token via Spec 1B's secrets store and calls
    // adsplatform.Provider.FetchMetrics. The applier writes the snapshot back
    // to dataStore[nodeID] = {snapshot} so downstream nodes can reference
    // {{$dataStore.<nodeID>.snapshot}} via ResolveTemplateVars.
    type QianchuanMetricsFetcherHandler struct{}

    func (QianchuanMetricsFetcherHandler) Execute(_ context.Context, req *NodeExecRequest) (*NodeExecResult, error) {
        var (
            bindingTpl string
            dims       []string
        )
        if req != nil {
            bindingTpl, _ = req.Config["binding_id_template"].(string)
            if raw, ok := req.Config["dimensions"].([]any); ok {
                for _, d := range raw {
                    if s, ok := d.(string); ok {
                        dims = append(dims, s)
                    }
                }
            }
        }
        binding := ResolveTemplateVars(bindingTpl, req.DataStore)
        nodeID := ""
        if req != nil && req.Node != nil { nodeID = req.Node.ID }
        payload, _ := json.Marshal(FetchQianchuanMetricsPayload{
            BindingID: binding, Dimensions: dims, NodeID: nodeID,
        })
        return &NodeExecResult{
            Effects: []Effect{{Kind: EffectFetchQianchuanMetrics, Payload: payload}},
        }, nil
    }

    func (QianchuanMetricsFetcherHandler) ConfigSchema() json.RawMessage {
        return json.RawMessage(`{
          "type": "object",
          "required": ["binding_id_template"],
          "properties": {
            "binding_id_template": {"type": "string"},
            "dimensions": {"type": "array", "items": {"type": "string"}}
          }
        }`)
    }

    func (QianchuanMetricsFetcherHandler) Capabilities() []EffectKind {
        return []EffectKind{EffectFetchQianchuanMetrics}
    }
    ```
- [ ] Step 4.4 — re-run handler test → green. Run `rtk cargo build -p server` to confirm no `IsPark` regressions.

---

## Task 5 — Applier branch for `EffectFetchQianchuanMetrics`

- [ ] Step 5.1 — wire deps into `EffectApplier` (fields only, no behavior yet)
  - File: `src-go/internal/workflow/nodetypes/applier.go`
  - Add to `EffectApplier` (after `SubWorkflowGuard`):
    ```go
    // Qianchuan deps (Spec 3D). All three may be nil in tests/builds that
    // don't compile in the qianchuan provider; in that case the applier
    // surfaces a structured "qianchuan: not configured" error rather than
    // silently no-op'ing.
    QianchuanProvider QianchuanProvider
    SecretsResolver   SecretsResolver
    SnapshotRepo      QianchuanSnapshotRepo
    ActionLogRepo     QianchuanActionLogRepo
    StrategyLoader    QianchuanStrategyLoader
    StrategyEvaluator QianchuanStrategyEvaluator
    BindingLookup     QianchuanBindingLookup
    ```
  - Define the local interfaces in `applier.go` (Go convention: keep import tree clean):
    ```go
    // QianchuanProvider is the subset of adsplatform.Provider used by the
    // metrics_fetcher and action_executor appliers. The qianchuan package's
    // *Provider satisfies this structurally.
    type QianchuanProvider interface {
        FetchMetrics(ctx context.Context, token string, ref any, dims []string) (any, time.Time, error)
        ApplyAction(ctx context.Context, token string, ref any, action any) (any, error)
    }

    // SecretsResolver is the subset of secrets.Service the appliers call.
    type SecretsResolver interface {
        Resolve(ctx context.Context, projectID uuid.UUID, fieldPath, template string) (string, error)
    }

    type QianchuanSnapshotRepo interface {
        Upsert(ctx context.Context, bindingID uuid.UUID, bucket time.Time, payload json.RawMessage) error
    }

    type QianchuanActionLogRepo interface {
        Create(ctx context.Context, log *model.QianchuanActionLog) error
        GetByID(ctx context.Context, id uuid.UUID) (*model.QianchuanActionLog, error)
        MarkApplied(ctx context.Context, id uuid.UUID) error
        MarkFailed(ctx context.Context, id uuid.UUID, msg string) error
    }

    type QianchuanStrategyLoader interface {
        Load(ctx context.Context, id uuid.UUID) (parsedSpec json.RawMessage, err error)
    }

    type QianchuanStrategyEvaluator interface {
        Evaluate(ctx context.Context, parsedSpec, snapshot json.RawMessage) (rules []QianchuanRuleMatch, err error)
    }

    type QianchuanRuleMatch struct {
        RuleName string          `json:"rule_name"`
        Actions  []QianchuanEmittedAction `json:"actions"`
    }
    type QianchuanEmittedAction struct {
        Type    string          `json:"type"`
        Target  string          `json:"target"`
        Params  json.RawMessage `json:"params"`
    }

    type QianchuanBindingLookup interface {
        GetByID(ctx context.Context, id uuid.UUID) (*model.QianchuanBinding, error)
    }
    ```
  - These local interfaces let 3D's tests inject fakes without importing `internal/qianchuan` (which 3A owns). The same shim shape mirrors how `EmployeeSpawner` is wired today.

- [ ] Step 5.2 — write failing applier test for the fetch effect
  - File: `src-go/internal/workflow/nodetypes/qianchuan_metrics_fetcher_applier_test.go`
    - Use a fake `QianchuanProvider` that returns a fixed snapshot `{"ads":[{"ad_id":"AD7","roi":1.2}]}` and bucket `2026-04-20T10:00:00Z`.
    - Use a fake `SecretsResolver` that returns `"tok-A"` for the binding's access_token.
    - Use an in-memory `QianchuanSnapshotRepo` capturing the UPSERT.
    - Assert: snapshot row written; `dataStore[nodeID]["snapshot"]` populated; `Resolve` was called with `fieldPath == "qianchuan.fetch.token"` and the binding-templated string.

- [ ] Step 5.3 — extend the applier dispatch (the switch over `Effect.Kind`)
  - Add a new branch `case EffectFetchQianchuanMetrics:` in the applier's effect-dispatch switch (the one used by `Apply`/`ApplyAll` — locate by `rtk grep "case Effect" src-go/internal/workflow/nodetypes/applier`*).
  - Implementation:
    1. Decode `FetchQianchuanMetricsPayload`.
    2. `binding, err := a.BindingLookup.GetByID(ctx, parsed.BindingID)`. On error: return wrapped error `"qianchuan: binding lookup: %w"`.
    3. `tok, err := a.SecretsResolver.Resolve(ctx, binding.ProjectID, "qianchuan.fetch.token", "{{secrets.qianchuan."+binding.ID.String()+".access_token}}")`.
    4. `snapshot, bucket, err := a.QianchuanProvider.FetchMetrics(ctx, tok, /*BindingRef*/{binding.AdvertiserID, binding.AwemeID}, payload.Dimensions)`.
    5. `snapJSON, _ := json.Marshal(snapshot)`. `bucket = bucket.UTC().Truncate(time.Minute)`.
    6. `a.SnapshotRepo.Upsert(ctx, binding.ID, bucket, snapJSON)`.
    7. Write `dataStore[payload.NodeID] = map[string]any{"snapshot": rawSnapshot, "bucket": bucket.Format(time.RFC3339)}` via the existing dataStore-write helper used by other appliers.
- [ ] Step 5.4 — add `qianchuan.fetch.token` to 1B's secret-resolver allowlist
  - File: `src-go/internal/secrets/resolver.go` (or wherever 1B parks the const). Append `"qianchuan.fetch.token"` and `"qianchuan.action.token"` to the `allowedFieldPaths` slice/set.
  - Add a regression unit test in 1B's resolver test file asserting both new paths resolve successfully (template `{{secrets.foo}}` works) and a sibling path like `qianchuan.unrelated` rejects with `secret:not_allowed_field`.
- [ ] Step 5.5 — re-run applier test → green. Confirm no other applier branch broke: `rtk cargo test -p server ./internal/workflow/nodetypes/...`.

---

## Task 6 — New node type `qianchuan_strategy_runner` (handler + applier)

- [ ] Step 6.1 — write failing handler test asserting effect emission and dataStore-template resolution for `snapshot_ref`
  - File: `src-go/internal/workflow/nodetypes/qianchuan_strategy_runner_test.go`
    - Assert: handler resolves `strategy_id_template` and `snapshot_ref` against dataStore; emits exactly one `EffectRunQianchuanStrategy` whose payload carries the parsed strategy id and the **resolved snapshot JSON** (not the template string).
- [ ] Step 6.2 — implement the handler
  - File: `src-go/internal/workflow/nodetypes/qianchuan_strategy_runner.go`
    - `Config`: `{strategy_id_template, snapshot_ref, binding_id_template}` (binding id propagates through to applier so action_log rows can be inserted without a separate lookup).
    - Use `ResolveTemplateVars` for all three template fields. For `snapshot_ref`, the resolved string is a JSON value (object) — pass it through `json.RawMessage` into the payload.
    - Capability: `[]EffectKind{EffectRunQianchuanStrategy}`.
- [ ] Step 6.3 — write failing applier test
  - File: `src-go/internal/workflow/nodetypes/qianchuan_strategy_runner_applier_test.go`
    - Fake `StrategyLoader` returns a fixed `parsedSpec` blob.
    - Fake `StrategyEvaluator` returns one rule match `roi-degradation` with two actions `adjust_bid` (target `AD7`) + `notify_im`.
    - Fake `ActionLogRepo` captures all `Create` calls.
    - Assert: 2 action_log rows persisted with `status='pending'`, `strategy_run_id` is the same UUID across both rows and is also written into `dataStore[nodeID].strategy_run_id`; `dataStore[nodeID].actions` is a slice of `{action_log_id, action_type, target}` records suitable for the loop node downstream.
- [ ] Step 6.4 — implement the applier branch `case EffectRunQianchuanStrategy:`
  1. Decode payload.
  2. `parsedSpec, err := a.StrategyLoader.Load(ctx, parsed.StrategyID)`. Wrap errors `"qianchuan: load strategy: %w"`.
  3. `matches, err := a.StrategyEvaluator.Evaluate(ctx, parsedSpec, parsed.SnapshotRef)`. **Drift handling** (per spec §11): if err is non-nil, write a single `record_event` action_log row with `status='failed', error_message=err.Error()` and emit empty actions list — do NOT short-circuit the whole DAG.
  4. `runID := uuid.New()`. For each match rule, for each action: build a `model.QianchuanActionLog{BindingID, StrategyID, StrategyRunID: runID, RuleName: match.RuleName, ActionType: action.Type, TargetAdID: action.Target, Params: action.Params, Status: "pending"}`, call `a.ActionLogRepo.Create`, append `{action_log_id: log.ID, action_type, target}` to a slice.
  5. Write `dataStore[payload.NodeID] = {"strategy_run_id": runID, "actions": [...]}`.
- [ ] Step 6.5 — re-run tests → green. Run `rtk cargo build -p server` to confirm interface methods line up.

---

## Task 7 — New node type `qianchuan_action_executor` (handler + applier)

- [ ] Step 7.1 — write failing handler test
  - File: `src-go/internal/workflow/nodetypes/qianchuan_action_executor_test.go`
    - Assert: handler resolves `action_log_id_template` against the loop's per-iteration dataStore (typically `{{$dataStore.run_strategy.actions[$iter].action_log_id}}`) and emits a single `EffectExecuteQianchuanAction` with the resolved id.
- [ ] Step 7.2 — implement handler
  - File: `src-go/internal/workflow/nodetypes/qianchuan_action_executor.go`
  - `Config`: `{action_log_id_template, binding_id_template}`. Capability: `[]EffectKind{EffectExecuteQianchuanAction}`.
- [ ] Step 7.3 — write failing applier test for success + failure paths
  - File: `src-go/internal/workflow/nodetypes/qianchuan_action_executor_applier_test.go`
    - Pre-seed `ActionLogRepo` with a `pending` row of `action_type='adjust_bid'`, `params={"delta_pct":-10}`, `target_ad_id='AD7'`.
    - Fake provider asserts the action method dispatches by action_type (success path returns `{"ok":true}`).
    - Assert: log row's `status` flips to `applied` and `applied_at` is non-nil; `dataStore[nodeID] = {success: true}`.
    - Add a second test where the provider returns an error: log row goes `status='failed', error_message=...`; `dataStore[nodeID] = {success: false, error: "..."}`; the applier returns nil error (so the DAG keeps going for sibling actions).
- [ ] Step 7.4 — implement applier branch `case EffectExecuteQianchuanAction:`
  1. Load `log` by id; load `binding` by `log.BindingID`.
  2. Resolve token via `a.SecretsResolver.Resolve(ctx, binding.ProjectID, "qianchuan.action.token", "{{secrets.qianchuan."+binding.ID+".access_token}}")`.
  3. Build a generic `Action` value (map: `{kind: log.ActionType, target_ref: log.TargetAdID, params: log.Params}`) and call `a.QianchuanProvider.ApplyAction(ctx, tok, BindingRef{...}, action)`. (Per coord note: 3A's `Provider.ApplyAction` is the single entry; per-kind dispatch lives inside the qianchuan package's `mapping.go`.)
  4. On success: `a.ActionLogRepo.MarkApplied(ctx, log.ID)`; write `dataStore[nodeID] = {success: true}`.
  5. On error: `a.ActionLogRepo.MarkFailed(ctx, log.ID, err.Error())`; write `dataStore[nodeID] = {success: false, error: err.Error()}`; return nil.
- [ ] Step 7.5 — re-run tests → green.

---

## Task 8 — Register the three new node types in `bootstrap.go`

- [ ] Step 8.1 — extend `RegisterBuiltins`
  - File: `src-go/internal/workflow/nodetypes/bootstrap.go`
  - Append three entries to the slice:
    ```go
    {"qianchuan_metrics_fetcher", QianchuanMetricsFetcherHandler{}},
    {"qianchuan_strategy_runner", QianchuanStrategyRunnerHandler{}},
    {"qianchuan_action_executor", QianchuanActionExecutorHandler{}},
    ```
- [ ] Step 8.2 — write failing registry-bootstrap test asserting all three names resolve from the global scope
  - File: `src-go/internal/workflow/nodetypes/bootstrap_test.go` — extend `TestRegisterBuiltins_*` (or add `TestRegisterBuiltins_RegistersQianchuanNodes`) asserting `r.Resolve(uuid.Nil, "qianchuan_metrics_fetcher")` etc. all return non-zero entries with the expected `Capabilities()` set.
- [ ] Step 8.3 — `rtk cargo test -p server -run TestRegisterBuiltins` → green.

---

## Task 9 — Canonical DAG seed `system:qianchuan_strategy_loop`

- [ ] Step 9.1 — create the seed package with the workflow definition constant
  - File: `src-go/internal/workflow/system/qianchuan_strategy_loop.go`
  - Export a `Definition` (type `*model.WorkflowDefinition`) named `QianchuanStrategyLoopDefinition` with:
    - `Name: "system:qianchuan_strategy_loop"`, `IsSystem: true` (or whatever the existing `WorkflowDefinition` flag is — confirm by `rtk grep "IsSystem\|System " src-go/internal/model/workflow_definition.go`).
    - Nodes (positions arbitrary but deterministic for diff-friendliness):
      - `id: "trigger"`, `type: "trigger"`, `config: {source: "schedule"}`
      - `id: "fetch_metrics"`, `type: "qianchuan_metrics_fetcher"`, `config: {binding_id_template: "{{$context.binding_id}}", dimensions: ["ads","live","materials"]}`
      - `id: "run_strategy"`, `type: "qianchuan_strategy_runner"`, `config: {strategy_id_template: "{{$context.strategy_id}}", snapshot_ref: "{{$dataStore.fetch_metrics.snapshot}}", binding_id_template: "{{$context.binding_id}}"}`
      - `id: "has_actions"`, `type: "condition"`, `config: {expression: "len($dataStore.run_strategy.actions) > 0"}`
      - `id: "actions_loop"`, `type: "loop"`, `config: {target_node: "execute_action", max_iterations: 64, exit_condition: "$dataStore.actions_loop._iter >= len($dataStore.run_strategy.actions)"}`
      - `id: "execute_action"`, `type: "qianchuan_action_executor"`, `config: {action_log_id_template: "{{$dataStore.run_strategy.actions[$dataStore.actions_loop._iter].action_log_id}}", binding_id_template: "{{$context.binding_id}}"}`
      - `id: "summary_card"`, `type: "im_send"`, `config: {...templated card body referencing strategy_run_id + applied/failed counts...}` — keep `card_template` minimal in 3D (a plain `ProviderNeutralCard` with title/summary); 3E enriches.
    - Edges: `trigger → fetch_metrics → run_strategy → has_actions`; `has_actions --true--> actions_loop → execute_action → actions_loop` (loop self-edge is the standard pattern from existing loop tests); `has_actions --false--> end-of-DAG`; `actions_loop --exit--> summary_card → end-of-DAG`.
    - **DOCSTRING REQUIREMENT**: the file's package doc comment MUST contain the literal sentence: "Spec 3E inserts a `qianchuan_policy_gate` node on the `run_strategy → actions_loop` edge; do not add it here." This is the load-bearing handoff signal for 3E — sub-agents grep for it.
- [ ] Step 9.2 — write failing test for the seed
  - File: `src-go/internal/workflow/system/qianchuan_strategy_loop_test.go`
    - Validate: definition has exactly 7 nodes by id; every node type appears in `nodetypes.RegisterBuiltins`'s registered set (test imports the registry, registers builtins with `BuiltinDeps{DefRepo: nil}` is OK for this test since it only resolves names); edges form a DAG (no cycles except the explicit loop self-edge); `binding_id_template` and `strategy_id_template` reference `$context` (so the trigger router must populate them).
- [ ] Step 9.3 — implement; test → green.
- [ ] Step 9.4 — wire seed into server bootstrap
  - File: `src-go/internal/server/bootstrap.go` (or wherever `RegisterBuiltins` is currently invoked at startup — confirm by `rtk grep "RegisterBuiltins" src-go`).
  - After `LockGlobal()`, add `if err := system.SeedQianchuanStrategyLoop(ctx, deps.WorkflowDefRepo); err != nil { log.Fatalf(...) }`.
  - `SeedQianchuanStrategyLoop` is an idempotent UPSERT-by-name function in the same `system/` package: looks up by `name='system:qianchuan_strategy_loop'`, creates if missing, replaces nodes/edges if present (the spec keeps the seed authoritative across upgrades).

---

## Task 10 — Per-binding schedule trigger materialization endpoint

- [ ] Step 10.1 — write failing handler test
  - File: `src-go/internal/handler/qianchuan_strategy_assign_test.go`
    - `POST /api/v1/qianchuan/bindings/:id/strategy` body `{strategy_id, schedule_override?}`. Assert: a `workflow_triggers` row is created with `source='schedule'`, `created_via='manual'`, `target_kind='dag'`, `workflow_id=<resolved id of system:qianchuan_strategy_loop>`, `Config.cron == strategy.Schedule || schedule_override`, `Config.binding_id`, `Config.strategy_id`. Binding row's `strategy_id` and `trigger_id` are updated.
    - Re-POST with a different `strategy_id`: same trigger row is reused (UPDATE), no second row.
    - `DELETE /api/v1/qianchuan/bindings/:id/strategy`: trigger row deleted, binding's `strategy_id` and `trigger_id` cleared.
- [ ] Step 10.2 — implement the handler in a new file
  - File: `src-go/internal/handler/qianchuan_strategy_handler.go`
  - Use the existing `RequireProjectRole("editor")` middleware (per spec §12) to gate both endpoints.
  - For schedule cron: read from the strategy's `parsed_spec.schedule.cron` (Spec 3 §9 schema; 3C ships this). If `schedule_override` is non-empty it wins. Validate via `cron.NewParser` (same parser as the ticker in `internal/trigger/schedule_ticker.go`). Reject with `qianchuan:invalid_cron` on parse failure.
  - The trigger Config payload shape:
    ```json
    {"cron": "*/1 * * * *", "binding_id": "...", "strategy_id": "...", "timezone": "UTC"}
    ```
    — `binding_id` and `strategy_id` are the keys the trigger router will copy into the execution's `$context` so `system:qianchuan_strategy_loop` node templates can resolve them.
- [ ] Step 10.3 — extend the trigger router's schedule-source `$context` population
  - File: `src-go/internal/trigger/router.go` (or wherever the `Route` method assembles execution seed) — look for the schedule-source case.
  - When `tr.Source == TriggerSourceSchedule`, copy `Config.binding_id` and `Config.strategy_id` into the spawned execution's `$context` map. Add a unit test (`router_schedule_context_test.go`) asserting the propagation.
- [ ] Step 10.4 — register the two new routes in the router setup
  - File: `src-go/internal/server/router.go` (or equivalent — `rtk grep "RequireProjectRole.*editor" src-go/internal/server` to confirm location).
- [ ] Step 10.5 — `rtk cargo test -p server -run TestQianchuanStrategyHandler` → green.

---

## Task 11 — Integration test: Trace A (silent tick — no actions)

- [ ] Step 11.1 — write the integration test
  - File: `src-go/internal/integration/qianchuan_loop_trace_a_test.go`
  - Setup (mirrors `trigger_flow_integration_test.go` style): testcontainers PG + an in-process server with stubbed `QianchuanProvider` returning `{ads:[{ad_id:"AD7", roi:2.5}]}` and a strategy whose only rule is `when: "ads[0].roi < 1.5"`.
  - Steps:
    1. Apply migrations 070–073.
    2. Seed `qianchuan_bindings` row, `qianchuan_strategies` row (parsed_spec contains the rule + `schedule.cron='*/1 * * * *'`).
    3. POST `/api/v1/qianchuan/bindings/:id/strategy` to materialize the trigger.
    4. Force a `ScheduleTicker.Tick` at the next minute boundary using the test's frozen `Clock`.
    5. Assertions:
       - exactly one row in `qianchuan_metric_snapshots` for the binding+bucket
       - zero rows in `qianchuan_action_logs`
       - exactly one workflow_executions row for the seeded definition with `status='success'`
       - no IM dispatch (mock IM Bridge captures zero calls)
- [ ] Step 11.2 — run: `rtk cargo test -p server -run TestQianchuan_TraceA -tags=integration` → green.

---

## Task 12 — Integration test: Trace B (auto-applied action)

- [ ] Step 12.1 — write the integration test
  - File: `src-go/internal/integration/qianchuan_loop_trace_b_test.go`
  - Same scaffolding as Trace A but the stub provider returns `{ads:[{ad_id:"AD7", roi:1.2, bid:5.0}]}`. Strategy rule emits `adjust_bid {target: ads[0].ad_id, params: {delta_pct: -10}}`. The stub `ApplyAction` records the call and returns `{ok:true}`.
  - Assertions:
    - one snapshot row
    - one action_log row, `status='applied'`, `applied_at IS NOT NULL`, `target_ad_id='AD7'`, `params.delta_pct == -10`
    - the stub provider received exactly one `ApplyAction` call with the expected `Action.kind` and `params`
    - mock IM Bridge captured one `summary_card` send (im_send node fired)
- [ ] Step 12.2 — run: `rtk cargo test -p server -run TestQianchuan_TraceB -tags=integration` → green.

---

## Task 13 — Wiring + smoke

- [ ] Step 13.1 — confirm `cmd/server` constructs the `EffectApplier` with the new fields populated when the qianchuan provider is compiled in. Check `internal/server/dependency_injection.go` (or whichever file currently builds the applier struct literal — `rtk grep "EffectApplier{" src-go/internal/server`).
  - Acceptance: `rtk cargo build -p server` clean; `rtk cargo test -p server` end-to-end clean; no `nil` deref on the new applier branches at startup.
- [ ] Step 13.2 — run `rtk lint` over touched files (`pnpm exec tsc --noEmit` is irrelevant here; this slice is Go-only). Acceptance: zero new lint regressions.
- [ ] Step 13.3 — verification before completion
  - `rtk cargo test -p server -run "TestQianchuan|TestRegisterBuiltins|TestMigrations"` → green
  - `rtk cargo test -p server -tags=integration -run TestQianchuan_Trace` → green
  - Skim the seeded definition via a one-off `rtk cargo run -- print-system-workflow system:qianchuan_strategy_loop` (or DB query) and confirm 7 nodes / expected edges.

---

## §14 Drifts (fill at end of execution)

- [ ] If 3C did NOT land the strategy_evaluator drift fix (spec §11 unresolved-template hardening), record here that 3D's strategy_runner applier currently treats unresolved templates as `noop` rule failures. 3E should add a regression test once 3C's fix lands.
- [ ] If 3A's `Provider.ApplyAction` signature drifted from the spec §8 shape, record the actual signature and how the local `QianchuanProvider` shim in `applier.go` adapts.
- [ ] If the schedule ticker proves unable to fire faster than 60s and a binding's strategy specified `schedule.cron='* * * * * *'` (sub-minute), record here per spec §14 row 3.
