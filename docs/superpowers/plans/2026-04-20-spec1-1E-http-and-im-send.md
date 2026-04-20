# Spec 1E — HTTP Node + IM-Send Node + Interactive Card Action Routing

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 落地 spec1 §5 三个新节点 (http_call / im_send / card_action_router) + §6.1 correlations 表 + §9 Trace B 完整闭环。让作者纯 UI 拼出"调外部 API → 富卡片回 Feishu → 用户点按钮恢复 wait_event"。

**Architecture:** http_call 节点 handler/applier，applier 调用 1B 的 secret_resolver 注入凭证；im_send 节点 applier 为每个 callback button 生成 correlation_token 写进卡片，并设 system_metadata.im_dispatched 抑制 1D 的默认 dispatcher；新 card_action_correlations 表持久化 token→(execution_id, node_id, action_id, payload) 映射；后端新 /api/v1/im/card-actions 端点查 token，命中走 wait_event resumer，未命中回退到 trigger_handler 当作新 IM 事件；IM Bridge Feishu 入站 card_action webhook 解析 token 转发到后端。

**Tech Stack:** Go (workflow node + applier + http.Client + jsonb merge), Postgres uuid + jsonb, TS/Bun (IM Bridge inbound), Next.js (config panels), Zustand.

**Depends on:** 1B (secret_resolver) + 1D (ProviderNeutralCard schema + IM Bridge `/im/send` 接口 + outbound_dispatcher 的 im_dispatched 协议)

**Parallel with:** none (final wave)

**Unblocks:** Spec 2 (Code Reviewer Employee 的 "Apply this fix" 按钮 + GitHub PAT 调 PR API), Spec 3 (千川 OAuth refresh + GMV 卡片告警)

---

## Coordination notes (read before starting)

- **`ProviderNeutralCard` canonical source**: 1D owns `src-im-bridge/core/card_schema.ts` (interface `ProviderNeutralCard` + `CardAction` union). Every reference in this plan to "the ProviderNeutralCard schema" means **that exact file**. Do NOT redefine the interface here. The Go-side mirror struct (`internal/imcards/card_template.go`) created in this plan must keep field names byte-identical (camelCase JSON tags) so the wire payload to IM Bridge matches what 1D's renderer expects.
- **`secret_resolver.Resolve` signature**: 1B owns `src-go/internal/secrets/resolver.go`. This plan ASSUMES the function shape `Resolve(ctx context.Context, projectID uuid.UUID, fieldPath string, template string) (string, error)` and that templates inside `headers` / `url` / `url_query` / `body` of an http_call node are the only call sites permitted. If 1B's signature differs, sync before E4 Step 4.2.
- **IM Bridge `/im/send` endpoint**: 1D adds `POST /im/send` accepting `{reply_target, card: ProviderNeutralCard}`. E5 (im_send applier) is its only new caller in this plan. If 1D names the route differently, sync before E5 Step 5.4.
- **`system_metadata.im_dispatched` contract**: 1D's outbound_dispatcher reads this flag to skip default回帖. E5 Step 5.5 writes it via `system_metadata = system_metadata || '{"im_dispatched": true}'::jsonb` (idempotent jsonb merge). Do NOT replace the column wholesale — that would clobber `reply_target`.
- **`wait_event` resumer**: investigation flagged the existing handler as a "minimal stub" (handler/applier emit a broadcast only — there is no public Resume entry). E2 fills that gap and is a hard prerequisite for E3. If a parallel branch already added a `Resume` method, skip E2 Steps 2.1–2.3 and only add the missing tests.
- **Card payload size**: per spec §14, button `value` carries only the `correlation_token` UUID (36 bytes) plus a short literal action_id. Renderers MUST NOT inline arbitrary payload fields into the button — those live in `card_action_correlations.payload`.
- **Old code deletion**: per spec §12, 1D deletes `feishu/live.go::renderInteractiveCard` / `renderStructuredMessage`. E6 only adds inbound parsing; do not re-introduce render logic.

---

## Task E1 — Migration: card_action_correlations table

- [x] Step 1.1 — write the up migration
  - File: `src-go/migrations/067_card_action_correlations.up.sql`
  - Content (numbering follows the latest `066_*` migration in the repo):
    ```sql
    CREATE TABLE card_action_correlations (
      token        uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
      execution_id uuid        NOT NULL REFERENCES workflow_executions(id) ON DELETE CASCADE,
      node_id      text        NOT NULL,
      action_id    text        NOT NULL,
      payload      jsonb,
      expires_at   timestamptz NOT NULL,
      consumed_at  timestamptz,
      created_at   timestamptz NOT NULL DEFAULT now()
    );

    CREATE INDEX idx_cac_active
      ON card_action_correlations (expires_at)
      WHERE consumed_at IS NULL;

    CREATE INDEX idx_cac_execution
      ON card_action_correlations (execution_id);
    ```
- [x] Step 1.2 — write the down migration
  - File: `src-go/migrations/067_card_action_correlations.down.sql`
    ```sql
    DROP INDEX IF EXISTS idx_cac_execution;
    DROP INDEX IF EXISTS idx_cac_active;
    DROP TABLE IF EXISTS card_action_correlations;
    ```
- [x] Step 1.3 — verify the migration compiles via the embed test
  - `rtk cargo test -p server -- migrations` (or whichever embed-style test the repo uses; the existing `migrations/embed_test.go` walks files via `embed.FS` and rejects non-monotonic prefixes).
  - Acceptance: green; new file is picked up; numeric sequence intact.

---

## Task E2 — Prerequisite: complete `wait_event` resumer (skip if a Resume entry already exists)

> **Stop-and-check first.** Run `rtk grep "func.*Resume.*WaitEvent\|WaitEventHandler.*Resume\|ResumeWaitEvent" src-go/internal`. If any hit lives outside this plan's diff, the resumer is already done — scrub Steps 2.1–2.3 and only run Step 2.4 (tests).

- [x] Step 2.1 — write failing tests for the resumer entry
  - File: `src-go/internal/workflow/nodetypes/wait_event_resume_test.go`
    ```go
    package nodetypes

    import (
        "context"
        "encoding/json"
        "errors"
        "testing"

        "github.com/google/uuid"
        "github.com/react-go-quick-starter/server/internal/model"
    )

    type fakeExecLookup struct {
        exec *model.WorkflowExecution
    }

    func (f *fakeExecLookup) GetExecution(_ context.Context, id uuid.UUID) (*model.WorkflowExecution, error) {
        if f.exec == nil || f.exec.ID != id {
            return nil, errors.New("not found")
        }
        return f.exec, nil
    }

    type fakeNodeExecLookup struct {
        items []*model.WorkflowNodeExecution
    }

    func (f *fakeNodeExecLookup) ListNodeExecutions(_ context.Context, _ uuid.UUID) ([]*model.WorkflowNodeExecution, error) {
        return f.items, nil
    }

    type fakeNodeExecWriter struct {
        statusByID map[uuid.UUID]string
        resultByID map[uuid.UUID]json.RawMessage
    }

    func (f *fakeNodeExecWriter) UpdateNodeExecution(_ context.Context, id uuid.UUID, status string, result json.RawMessage, _ string) error {
        if f.statusByID == nil {
            f.statusByID = map[uuid.UUID]string{}
            f.resultByID = map[uuid.UUID]json.RawMessage{}
        }
        f.statusByID[id] = status
        f.resultByID[id] = result
        return nil
    }

    type fakeAdvancer struct{ advanced []uuid.UUID }

    func (f *fakeAdvancer) AdvanceExecution(_ context.Context, id uuid.UUID) error {
        f.advanced = append(f.advanced, id)
        return nil
    }

    func TestWaitEventResumer_HappyPath(t *testing.T) {
        execID := uuid.New()
        nodeExecID := uuid.New()
        execLookup := &fakeExecLookup{exec: &model.WorkflowExecution{
            ID:     execID,
            Status: model.WorkflowExecStatusRunning, // execution itself stays running; node is waiting
        }}
        nodeLookup := &fakeNodeExecLookup{items: []*model.WorkflowNodeExecution{
            {ID: nodeExecID, ExecutionID: execID, NodeID: "wait-1", Status: model.NodeExecWaiting},
        }}
        writer := &fakeNodeExecWriter{}
        advancer := &fakeAdvancer{}
        ds := &fakeDataStoreWriter{}

        r := &WaitEventResumer{
            ExecLookup: execLookup, NodeLookup: nodeLookup,
            NodeWriter: writer, Advancer: advancer, DataStore: ds,
        }

        err := r.Resume(context.Background(), execID, "wait-1", map[string]any{"action_id": "approve", "value": "y"})
        if err != nil {
            t.Fatalf("Resume() error: %v", err)
        }
        if writer.statusByID[nodeExecID] != model.NodeExecCompleted {
            t.Errorf("node status = %q, want completed", writer.statusByID[nodeExecID])
        }
        if len(advancer.advanced) != 1 || advancer.advanced[0] != execID {
            t.Errorf("advancer.advanced = %v, want [%s]", advancer.advanced, execID)
        }
        if got, _ := ds.merged["wait-1"].(map[string]any)["action_id"]; got != "approve" {
            t.Errorf("dataStore[wait-1].action_id = %v, want approve", got)
        }
    }

    func TestWaitEventResumer_NotWaiting(t *testing.T) {
        execID := uuid.New()
        nodeExecID := uuid.New()
        r := &WaitEventResumer{
            ExecLookup: &fakeExecLookup{exec: &model.WorkflowExecution{ID: execID, Status: model.WorkflowExecStatusCompleted}},
            NodeLookup: &fakeNodeExecLookup{items: []*model.WorkflowNodeExecution{
                {ID: nodeExecID, ExecutionID: execID, NodeID: "wait-1", Status: model.NodeExecCompleted},
            }},
            NodeWriter: &fakeNodeExecWriter{}, Advancer: &fakeAdvancer{}, DataStore: &fakeDataStoreWriter{},
        }
        err := r.Resume(context.Background(), execID, "wait-1", nil)
        if !errors.Is(err, ErrWaitEventNotWaiting) {
            t.Fatalf("err = %v, want ErrWaitEventNotWaiting", err)
        }
    }
    ```
  - Add a tiny shared `fakeDataStoreWriter` to the same file:
    ```go
    type fakeDataStoreWriter struct{ merged map[string]any }
    func (f *fakeDataStoreWriter) MergeNodeResult(_ context.Context, _ uuid.UUID, nodeID string, payload map[string]any) error {
        if f.merged == nil { f.merged = map[string]any{} }
        f.merged[nodeID] = payload
        return nil
    }
    ```
  - Run: `rtk cargo test -p workflow/nodetypes -run WaitEventResumer` — confirm both fail with "WaitEventResumer not defined" / "ErrWaitEventNotWaiting not defined".

- [x] Step 2.2 — implement the resumer
  - File: `src-go/internal/workflow/nodetypes/wait_event_resumer.go`
    ```go
    package nodetypes

    import (
        "context"
        "errors"
        "fmt"

        "github.com/google/uuid"
        "github.com/react-go-quick-starter/server/internal/model"
    )

    // ErrWaitEventNotWaiting is returned when Resume is called against an
    // execution whose target node is not in the waiting state. Caller maps
    // this to HTTP 409 with code "card_action:execution_not_waiting".
    var ErrWaitEventNotWaiting = errors.New("wait_event: target node is not waiting")

    // WaitEventExecLookup loads the parent execution so the resumer can refuse
    // resume attempts against terminated executions before mutating any rows.
    type WaitEventExecLookup interface {
        GetExecution(ctx context.Context, id uuid.UUID) (*model.WorkflowExecution, error)
    }

    // WaitEventNodeLookup lists node executions so the resumer can find the
    // single waiting row for the supplied (executionID, nodeID).
    type WaitEventNodeLookup interface {
        ListNodeExecutions(ctx context.Context, executionID uuid.UUID) ([]*model.WorkflowNodeExecution, error)
    }

    // WaitEventNodeWriter transitions the node execution to completed and
    // stores the inbound payload as the node result.
    type WaitEventNodeWriter interface {
        UpdateNodeExecution(ctx context.Context, id uuid.UUID, status string, result []byte, errorMessage string) error
    }

    // WaitEventDataStoreMerger writes the resume payload into the execution
    // dataStore under the resumed nodeID so downstream nodes can reference it
    // through {{$dataStore.<nodeID>.<field>}}.
    type WaitEventDataStoreMerger interface {
        MergeNodeResult(ctx context.Context, executionID uuid.UUID, nodeID string, payload map[string]any) error
    }

    // WaitEventAdvancer kicks the DAG runner forward after the node flips.
    // Backed at wiring time by *DAGWorkflowService.AdvanceExecution.
    type WaitEventAdvancer interface {
        AdvanceExecution(ctx context.Context, executionID uuid.UUID) error
    }

    // WaitEventResumer is the public entry point card_action_router calls when
    // a callback button correlation matches a parked wait_event node.
    type WaitEventResumer struct {
        ExecLookup WaitEventExecLookup
        NodeLookup WaitEventNodeLookup
        NodeWriter WaitEventNodeWriter
        DataStore  WaitEventDataStoreMerger
        Advancer   WaitEventAdvancer
    }

    // Resume validates that exec is still alive and the named node is waiting,
    // injects payload into dataStore, marks the node completed, and triggers
    // a DAG advance. Returns ErrWaitEventNotWaiting when the precondition
    // fails so the HTTP layer can surface a structured 409.
    func (r *WaitEventResumer) Resume(ctx context.Context, executionID uuid.UUID, nodeID string, payload map[string]any) error {
        if r == nil || r.ExecLookup == nil || r.NodeLookup == nil || r.NodeWriter == nil || r.Advancer == nil {
            return fmt.Errorf("wait_event resumer is not fully wired")
        }

        exec, err := r.ExecLookup.GetExecution(ctx, executionID)
        if err != nil {
            return fmt.Errorf("load execution: %w", err)
        }
        // The execution itself may be running (the node is waiting), but if
        // it is already terminal the resume is a no-op and surfaces a 409 to
        // the caller so the IM toast says "工作流已结束".
        if exec.Status == model.WorkflowExecStatusCompleted ||
            exec.Status == model.WorkflowExecStatusFailed ||
            exec.Status == model.WorkflowExecStatusCancelled {
            return ErrWaitEventNotWaiting
        }

        nodeExecs, err := r.NodeLookup.ListNodeExecutions(ctx, executionID)
        if err != nil {
            return fmt.Errorf("list node executions: %w", err)
        }
        var target *model.WorkflowNodeExecution
        for _, ne := range nodeExecs {
            if ne.NodeID == nodeID && ne.Status == model.NodeExecWaiting {
                target = ne
                break
            }
        }
        if target == nil {
            return ErrWaitEventNotWaiting
        }

        // Merge the payload into the execution dataStore so downstream nodes
        // can reference {{$dataStore.<nodeID>.action_id}}. Soft-fail this
        // write — the node-status transition is the source of truth and the
        // DAG runner can still advance even if the merge fails (the payload
        // will simply not be visible to downstream templates).
        if r.DataStore != nil {
            if err := r.DataStore.MergeNodeResult(ctx, executionID, nodeID, payload); err != nil {
                // Logged at the caller; bubble up so card_action_router can
                // record an audit event but still try to advance.
                _ = err
            }
        }

        // Persist the payload on the node row as well, so /runs/<id> trace
        // viewers can see what input woke the node.
        var resultBytes []byte
        if payload != nil {
            // Best effort: encode; if the payload is unencodable (it
            // shouldn't be — it came in via JSON), drop the result rather
            // than fail the resume.
            if b, err := jsonMarshal(payload); err == nil {
                resultBytes = b
            }
        }
        if err := r.NodeWriter.UpdateNodeExecution(ctx, target.ID, model.NodeExecCompleted, resultBytes, ""); err != nil {
            return fmt.Errorf("update node execution: %w", err)
        }

        return r.Advancer.AdvanceExecution(ctx, executionID)
    }

    // jsonMarshal is split out to allow tests to substitute a forced-failure
    // marshaler without dragging encoding/json into the public surface.
    var jsonMarshal = func(v any) ([]byte, error) {
        return jsonEncode(v)
    }
    ```
  - And a tiny adapter file to keep the encoding/json import contained:
    File: `src-go/internal/workflow/nodetypes/json_helper.go`
    ```go
    package nodetypes

    import "encoding/json"

    func jsonEncode(v any) ([]byte, error) { return json.Marshal(v) }
    ```
  - Run the failing tests again: `rtk cargo test -p workflow/nodetypes -run WaitEventResumer` — expect green.

- [x] Step 2.3 — wire the resumer into DAGWorkflowService
  - File: `src-go/internal/service/dag_workflow_service.go` — add an adapter near the top of the file (after `PluginRunResumer` interface):
    ```go
    // WaitEventDataStoreAdapter merges a resume payload into the parent
    // execution's dataStore as `dataStore[nodeID] = payload`. Backed by the
    // execRepo's UpdateExecutionDataStore through a load-merge-save cycle so
    // the resumer does not need to know the storage shape.
    type WaitEventDataStoreAdapter struct {
        Repo DAGWorkflowExecutionRepo
    }

    func (a *WaitEventDataStoreAdapter) MergeNodeResult(ctx context.Context, executionID uuid.UUID, nodeID string, payload map[string]any) error {
        if a == nil || a.Repo == nil {
            return fmt.Errorf("wait_event datastore adapter not configured")
        }
        exec, err := a.Repo.GetExecution(ctx, executionID)
        if err != nil {
            return err
        }
        ds := map[string]any{}
        if len(exec.DataStore) > 0 {
            _ = json.Unmarshal(exec.DataStore, &ds)
        }
        ds[nodeID] = payload
        encoded, err := json.Marshal(ds)
        if err != nil {
            return err
        }
        return a.Repo.UpdateExecutionDataStore(ctx, executionID, encoded)
    }
    ```
  - Add a `WaitEventResumer()` factory on `DAGWorkflowService`:
    ```go
    // WaitEventResumer returns a fully wired resumer that the IM card-action
    // router can call to wake parked wait_event nodes.
    func (s *DAGWorkflowService) WaitEventResumer() *nodetypes.WaitEventResumer {
        return &nodetypes.WaitEventResumer{
            ExecLookup: s.execRepo,
            NodeLookup: s.nodeRepo,
            NodeWriter: s.nodeRepo,
            DataStore:  &WaitEventDataStoreAdapter{Repo: s.execRepo},
            Advancer:   s,
        }
    }
    ```
  - Note: `*DAGWorkflowService` already exposes `AdvanceExecution(ctx, id)` matching `WaitEventAdvancer`. `nodeRepo` already satisfies both `ListNodeExecutions` and `UpdateNodeExecution`. `execRepo` satisfies `GetExecution`.

- [x] Step 2.4 — add a regression test that the existing wait_event handler and the new resumer agree on `model.NodeExecWaiting`
  - File: `src-go/internal/workflow/nodetypes/wait_event_test.go` — append:
    ```go
    func TestWaitEventResumer_StatusConstantStability(t *testing.T) {
        // The resumer matches model.NodeExecWaiting verbatim. If anyone
        // changes the constant, this test catches it before card-action
        // routing silently stops resuming.
        if model.NodeExecWaiting == "" {
            t.Fatal("model.NodeExecWaiting must be a non-empty constant")
        }
    }
    ```
  - Run: `rtk cargo test -p workflow/nodetypes` — green.

---

## Task E3 — `card_action_correlations` repository, service, and HTTP handler

- [x] Step 3.1 — write failing repo tests
  - File: `src-go/internal/imcards/correlations_repo_test.go`
    ```go
    package imcards

    import (
        "context"
        "testing"
        "time"

        "github.com/google/uuid"
        "github.com/react-go-quick-starter/server/internal/testutil"
    )

    func TestCorrelationsRepo_CreateLookupConsume(t *testing.T) {
        db := testutil.PostgresForTest(t) // existing helper used by repos
        repo := NewCorrelationsRepo(db)
        ctx := context.Background()
        execID := testutil.SeedExecution(t, db) // existing helper

        token, err := repo.Create(ctx, &CorrelationInput{
            ExecutionID: execID,
            NodeID:      "wait-1",
            ActionID:    "approve",
            Payload:     map[string]any{"foo": "bar"},
            ExpiresAt:   time.Now().Add(7 * 24 * time.Hour),
        })
        if err != nil { t.Fatalf("Create: %v", err) }
        if token == uuid.Nil { t.Fatal("token is zero") }

        got, err := repo.Lookup(ctx, token)
        if err != nil { t.Fatalf("Lookup: %v", err) }
        if got.NodeID != "wait-1" || got.ActionID != "approve" {
            t.Errorf("got %+v", got)
        }
        if got.ConsumedAt != nil { t.Error("ConsumedAt should be nil before mark") }

        if err := repo.MarkConsumed(ctx, token); err != nil {
            t.Fatalf("MarkConsumed: %v", err)
        }
        again, err := repo.Lookup(ctx, token)
        if err != nil { t.Fatalf("Lookup after consume: %v", err) }
        if again.ConsumedAt == nil { t.Error("ConsumedAt should be set after mark") }
    }

    func TestCorrelationsRepo_LookupMissing(t *testing.T) {
        db := testutil.PostgresForTest(t)
        repo := NewCorrelationsRepo(db)
        _, err := repo.Lookup(context.Background(), uuid.New())
        if err != ErrCorrelationNotFound {
            t.Fatalf("err = %v, want ErrCorrelationNotFound", err)
        }
    }
    ```
  - Run: `rtk cargo test -p imcards` — fails (package missing).

- [x] Step 3.2 — implement the repo
  - File: `src-go/internal/imcards/correlations_repo.go`
    ```go
    package imcards

    import (
        "context"
        "database/sql"
        "encoding/json"
        "errors"
        "fmt"
        "time"

        "github.com/google/uuid"
    )

    // ErrCorrelationNotFound is returned by Lookup when no row matches the
    // supplied token. Distinct from a query error so the router can map it to
    // the "fall through to trigger handler" branch.
    var ErrCorrelationNotFound = errors.New("imcards: correlation not found")

    // Correlation mirrors the card_action_correlations row shape. Payload is
    // already-parsed JSON; callers receive a typed map and need not unmarshal.
    type Correlation struct {
        Token       uuid.UUID
        ExecutionID uuid.UUID
        NodeID      string
        ActionID    string
        Payload     map[string]any
        ExpiresAt   time.Time
        ConsumedAt  *time.Time
        CreatedAt   time.Time
    }

    // CorrelationInput is the Create signature. ExpiresAt MUST be set by the
    // caller — repo refuses to default it so the lifetime policy stays in the
    // im_send applier where the operator can audit it.
    type CorrelationInput struct {
        ExecutionID uuid.UUID
        NodeID      string
        ActionID    string
        Payload     map[string]any
        ExpiresAt   time.Time
    }

    type CorrelationsRepo struct{ db *sql.DB }

    func NewCorrelationsRepo(db *sql.DB) *CorrelationsRepo { return &CorrelationsRepo{db: db} }

    // Create inserts a new correlation and returns the freshly minted token.
    func (r *CorrelationsRepo) Create(ctx context.Context, in *CorrelationInput) (uuid.UUID, error) {
        if in == nil {
            return uuid.Nil, fmt.Errorf("nil input")
        }
        if in.ExpiresAt.IsZero() {
            return uuid.Nil, fmt.Errorf("ExpiresAt is required")
        }
        var payloadBytes []byte
        if in.Payload != nil {
            b, err := json.Marshal(in.Payload)
            if err != nil {
                return uuid.Nil, fmt.Errorf("marshal payload: %w", err)
            }
            payloadBytes = b
        }
        var token uuid.UUID
        err := r.db.QueryRowContext(ctx, `
            INSERT INTO card_action_correlations
                (execution_id, node_id, action_id, payload, expires_at)
            VALUES ($1, $2, $3, $4, $5)
            RETURNING token`,
            in.ExecutionID, in.NodeID, in.ActionID, payloadBytes, in.ExpiresAt,
        ).Scan(&token)
        if err != nil {
            return uuid.Nil, fmt.Errorf("insert correlation: %w", err)
        }
        return token, nil
    }

    // Lookup returns the row matching token. It does NOT enforce expiry —
    // callers (the router) compare against `time.Now()` so the failure mode
    // is distinguishable from a missing token.
    func (r *CorrelationsRepo) Lookup(ctx context.Context, token uuid.UUID) (*Correlation, error) {
        row := r.db.QueryRowContext(ctx, `
            SELECT token, execution_id, node_id, action_id, payload,
                   expires_at, consumed_at, created_at
            FROM card_action_correlations
            WHERE token = $1`, token)
        var c Correlation
        var payloadBytes []byte
        var consumed sql.NullTime
        err := row.Scan(&c.Token, &c.ExecutionID, &c.NodeID, &c.ActionID,
            &payloadBytes, &c.ExpiresAt, &consumed, &c.CreatedAt)
        if errors.Is(err, sql.ErrNoRows) {
            return nil, ErrCorrelationNotFound
        }
        if err != nil {
            return nil, fmt.Errorf("scan correlation: %w", err)
        }
        if len(payloadBytes) > 0 {
            if err := json.Unmarshal(payloadBytes, &c.Payload); err != nil {
                return nil, fmt.Errorf("unmarshal payload: %w", err)
            }
        }
        if consumed.Valid {
            t := consumed.Time
            c.ConsumedAt = &t
        }
        return &c, nil
    }

    // MarkConsumed stamps consumed_at = now() exactly once. A second call
    // succeeds but does NOT overwrite the original timestamp — uniqueness is
    // enforced by the WHERE clause.
    func (r *CorrelationsRepo) MarkConsumed(ctx context.Context, token uuid.UUID) error {
        _, err := r.db.ExecContext(ctx, `
            UPDATE card_action_correlations
               SET consumed_at = now()
             WHERE token = $1
               AND consumed_at IS NULL`, token)
        return err
    }
    ```
  - Run repo tests — green.

- [x] Step 3.3 — write failing router tests
  - File: `src-go/internal/imcards/router_test.go`
    ```go
    package imcards

    import (
        "context"
        "errors"
        "testing"
        "time"

        "github.com/google/uuid"
    )

    type stubCorrelations struct {
        c       *Correlation
        lookErr error
        marked  []uuid.UUID
    }

    func (s *stubCorrelations) Lookup(_ context.Context, t uuid.UUID) (*Correlation, error) {
        if s.lookErr != nil { return nil, s.lookErr }
        return s.c, nil
    }
    func (s *stubCorrelations) MarkConsumed(_ context.Context, t uuid.UUID) error {
        s.marked = append(s.marked, t); return nil
    }

    type stubResumer struct {
        called  bool
        retErr  error
    }

    func (s *stubResumer) Resume(_ context.Context, _ uuid.UUID, _ string, _ map[string]any) error {
        s.called = true
        return s.retErr
    }

    type stubFallback struct{ events []map[string]any }
    func (s *stubFallback) RouteAsIMEvent(_ context.Context, ev map[string]any) error {
        s.events = append(s.events, ev); return nil
    }

    type stubAudit struct{ entries []string }
    func (s *stubAudit) Record(_ context.Context, kind string, _ map[string]any) error {
        s.entries = append(s.entries, kind); return nil
    }

    func TestRouter_HitConsumes(t *testing.T) {
        execID := uuid.New()
        token := uuid.New()
        corr := &stubCorrelations{c: &Correlation{
            Token: token, ExecutionID: execID, NodeID: "wait-1", ActionID: "approve",
            ExpiresAt: time.Now().Add(time.Hour),
        }}
        res := &stubResumer{}
        r := &Router{Correlations: corr, Resumer: res, Fallback: &stubFallback{}, Audit: &stubAudit{}}
        out, err := r.Route(context.Background(), RouteInput{Token: token, ActionID: "approve"})
        if err != nil { t.Fatalf("Route: %v", err) }
        if !res.called { t.Error("resumer not called") }
        if len(corr.marked) != 1 { t.Error("token not consumed") }
        if out.Outcome != OutcomeResumed { t.Errorf("outcome = %s", out.Outcome) }
    }

    func TestRouter_Expired(t *testing.T) {
        token := uuid.New()
        corr := &stubCorrelations{c: &Correlation{
            Token: token, ExecutionID: uuid.New(), NodeID: "wait-1", ActionID: "approve",
            ExpiresAt: time.Now().Add(-time.Minute),
        }}
        r := &Router{Correlations: corr, Resumer: &stubResumer{}, Fallback: &stubFallback{}, Audit: &stubAudit{}}
        _, err := r.Route(context.Background(), RouteInput{Token: token})
        if !errors.Is(err, ErrCardActionExpired) {
            t.Fatalf("err = %v, want ErrCardActionExpired", err)
        }
    }

    func TestRouter_Consumed(t *testing.T) {
        token := uuid.New()
        now := time.Now()
        corr := &stubCorrelations{c: &Correlation{
            Token: token, ExpiresAt: time.Now().Add(time.Hour),
            ConsumedAt: &now,
        }}
        r := &Router{Correlations: corr, Resumer: &stubResumer{}, Fallback: &stubFallback{}, Audit: &stubAudit{}}
        _, err := r.Route(context.Background(), RouteInput{Token: token})
        if !errors.Is(err, ErrCardActionConsumed) {
            t.Fatalf("err = %v, want ErrCardActionConsumed", err)
        }
    }

    func TestRouter_NotFoundFallsBack(t *testing.T) {
        corr := &stubCorrelations{lookErr: ErrCorrelationNotFound}
        fb := &stubFallback{}
        r := &Router{Correlations: corr, Resumer: &stubResumer{}, Fallback: fb, Audit: &stubAudit{}}
        out, err := r.Route(context.Background(), RouteInput{
            Token: uuid.New(), ActionID: "free-form-button",
            ReplyTarget: map[string]any{"chat_id": "C1"},
        })
        if err != nil { t.Fatalf("Route: %v", err) }
        if out.Outcome != OutcomeFallback { t.Errorf("outcome = %s", out.Outcome) }
        if len(fb.events) != 1 { t.Fatal("fallback not invoked") }
    }
    ```
  - Run: fails (no Router yet).

- [x] Step 3.4 — implement the router
  - File: `src-go/internal/imcards/router.go`
    ```go
    package imcards

    import (
        "context"
        "errors"
        "fmt"
        "time"

        "github.com/google/uuid"
    )

    // Router-level errors. Each maps to a stable HTTP code via the handler
    // (E3.5) so the IM Bridge can render the right toast to the end user.
    var (
        ErrCardActionExpired      = errors.New("card_action: expired")
        ErrCardActionConsumed     = errors.New("card_action: consumed")
        ErrExecutionNotWaiting    = errors.New("card_action: execution_not_waiting")
    )

    // RouteOutcome reports what the router did, for both audit and HTTP body.
    type RouteOutcome string

    const (
        OutcomeResumed  RouteOutcome = "resumed"
        OutcomeFallback RouteOutcome = "fallback_triggered"
    )

    // CorrelationsStore is the narrow interface the router consumes from
    // CorrelationsRepo. Tests substitute a stub.
    type CorrelationsStore interface {
        Lookup(ctx context.Context, token uuid.UUID) (*Correlation, error)
        MarkConsumed(ctx context.Context, token uuid.UUID) error
    }

    // WaitEventResumer is the narrow contract the router calls when a token
    // is matched. Implemented by *nodetypes.WaitEventResumer.
    type WaitEventResumer interface {
        Resume(ctx context.Context, executionID uuid.UUID, nodeID string, payload map[string]any) error
    }

    // FallbackTriggerRouter handles the "no token match" branch by treating
    // the click as a brand-new IM event so trigger_handler can dispatch to
    // any matching workflow trigger. Implemented at wiring time by an
    // adapter around the existing IM trigger router.
    type FallbackTriggerRouter interface {
        RouteAsIMEvent(ctx context.Context, event map[string]any) error
    }

    // AuditSink records router outcomes. Payload values from end users are
    // NOT included — only the token, action_id, user_id, outcome.
    type AuditSink interface {
        Record(ctx context.Context, kind string, payload map[string]any) error
    }

    // Router is the central card-action decision point.
    type Router struct {
        Correlations CorrelationsStore
        Resumer      WaitEventResumer
        Fallback     FallbackTriggerRouter
        Audit        AuditSink
        Now          func() time.Time // override for tests
    }

    // RouteInput is the structured input forwarded by the HTTP handler.
    type RouteInput struct {
        Token       uuid.UUID
        ActionID    string
        Value       map[string]any
        ReplyTarget map[string]any
        UserID      string
        TenantID    string
    }

    // RouteResult describes what the router did so the handler can surface it
    // to the IM Bridge for toast rendering.
    type RouteResult struct {
        Outcome     RouteOutcome
        ExecutionID uuid.UUID // zero when fallback
        NodeID      string    // empty when fallback
    }

    // Route is the single entry point. Branches:
    //   1. Token missing in store           → fallback to trigger router
    //   2. Token consumed                   → ErrCardActionConsumed (409)
    //   3. Token past expires_at            → ErrCardActionExpired (410)
    //   4. Resumer reports not-waiting      → ErrExecutionNotWaiting (409)
    //   5. Otherwise: Resume + MarkConsumed → OutcomeResumed (200)
    func (r *Router) Route(ctx context.Context, in RouteInput) (*RouteResult, error) {
        if r == nil || r.Correlations == nil {
            return nil, fmt.Errorf("router not configured")
        }
        now := time.Now()
        if r.Now != nil { now = r.Now() }

        corr, err := r.Correlations.Lookup(ctx, in.Token)
        if errors.Is(err, ErrCorrelationNotFound) {
            // Fallback path: synthesize an IM event so the existing trigger
            // router can match the action_id as a free-form command.
            if r.Fallback != nil {
                ev := map[string]any{
                    "actionId":    in.ActionID,
                    "value":       in.Value,
                    "userId":      in.UserID,
                    "tenantId":    in.TenantID,
                    "replyTarget": in.ReplyTarget,
                }
                if err := r.Fallback.RouteAsIMEvent(ctx, ev); err != nil {
                    return nil, fmt.Errorf("fallback route: %w", err)
                }
            }
            r.audit(ctx, "card_action_fallback", map[string]any{
                "token":    in.Token.String(),
                "actionId": in.ActionID,
                "userId":   in.UserID,
            })
            return &RouteResult{Outcome: OutcomeFallback}, nil
        }
        if err != nil {
            return nil, fmt.Errorf("lookup: %w", err)
        }

        if corr.ConsumedAt != nil {
            return nil, ErrCardActionConsumed
        }
        if corr.ExpiresAt.Before(now) {
            return nil, ErrCardActionExpired
        }

        // Build the payload visible to the wait_event node. Action_id is
        // injected verbatim so a single wait_event can branch on which
        // button was clicked via {{$dataStore.<nodeID>.action_id}}.
        payload := map[string]any{
            "action_id": corr.ActionID,
            "value":     in.Value,
            "user_id":   in.UserID,
            "tenant_id": in.TenantID,
        }

        if err := r.Resumer.Resume(ctx, corr.ExecutionID, corr.NodeID, payload); err != nil {
            // Translate the well-known "not waiting" sentinel; everything
            // else bubbles up as 500.
            // (Resumer-package import would create a cycle, so the router
            // matches by error string-prefix used by ErrWaitEventNotWaiting.)
            if err.Error() == "wait_event: target node is not waiting" {
                return nil, ErrExecutionNotWaiting
            }
            return nil, fmt.Errorf("resume: %w", err)
        }

        if err := r.Correlations.MarkConsumed(ctx, in.Token); err != nil {
            // Resume already happened; mark-consumed failure is non-fatal
            // but logged so operators can detect duplicate-resume risk.
            r.audit(ctx, "card_action_mark_consumed_failed", map[string]any{
                "token": in.Token.String(), "error": err.Error(),
            })
        }

        r.audit(ctx, "card_action_routed", map[string]any{
            "token":       in.Token.String(),
            "actionId":    in.ActionID,
            "userId":      in.UserID,
            "executionId": corr.ExecutionID.String(),
        })
        return &RouteResult{
            Outcome:     OutcomeResumed,
            ExecutionID: corr.ExecutionID,
            NodeID:      corr.NodeID,
        }, nil
    }

    func (r *Router) audit(ctx context.Context, kind string, payload map[string]any) {
        if r.Audit == nil { return }
        _ = r.Audit.Record(ctx, kind, payload)
    }
    ```
  - **Coupling note** in the file's package doc: the string-comparison fallback for `wait_event: target node is not waiting` is intentional to keep the `imcards` package free of a backward import on `nodetypes`. If the constant ever changes, this router test must also change — covered by `TestRouter_ResumerNotWaiting` below.
  - Append a covering test:
    ```go
    func TestRouter_ResumerNotWaiting(t *testing.T) {
        token := uuid.New()
        corr := &stubCorrelations{c: &Correlation{
            Token: token, ExecutionID: uuid.New(), NodeID: "wait-1", ActionID: "x",
            ExpiresAt: time.Now().Add(time.Hour),
        }}
        // Match the sentinel error message verbatim — keeps the router and
        // the resumer in sync without a backward import.
        res := &stubResumer{retErr: errors.New("wait_event: target node is not waiting")}
        r := &Router{Correlations: corr, Resumer: res, Fallback: &stubFallback{}, Audit: &stubAudit{}}
        _, err := r.Route(context.Background(), RouteInput{Token: token})
        if !errors.Is(err, ErrExecutionNotWaiting) {
            t.Fatalf("err = %v, want ErrExecutionNotWaiting", err)
        }
    }
    ```
  - Run: `rtk cargo test -p imcards` — green.

- [x] Step 3.5 — HTTP handler `POST /api/v1/im/card-actions`
  - File: `src-go/internal/handler/im_card_actions_handler.go`
    ```go
    package handler

    import (
        "errors"
        "net/http"

        "github.com/google/uuid"
        "github.com/labstack/echo/v4"
        "github.com/react-go-quick-starter/server/internal/imcards"
    )

    type IMCardActionsHandler struct{ Router *imcards.Router }

    func NewIMCardActionsHandler(r *imcards.Router) *IMCardActionsHandler {
        return &IMCardActionsHandler{Router: r}
    }

    type imCardActionRequest struct {
        CorrelationToken string                 `json:"correlation_token"`
        ActionID         string                 `json:"action_id"`
        Value            map[string]any         `json:"value"`
        ReplyTarget      map[string]any         `json:"replyTarget"`
        UserID           string                 `json:"user_id"`
        TenantID         string                 `json:"tenant_id"`
    }

    type imCardActionResponse struct {
        Outcome     string `json:"outcome"`
        ExecutionID string `json:"execution_id,omitempty"`
        NodeID      string `json:"node_id,omitempty"`
    }

    // Handle is the single Echo handler. Status mapping mirrors the spec §10
    // matrix: 410 expired, 409 consumed/not-waiting, 200 resumed/fallback.
    func (h *IMCardActionsHandler) Handle(c echo.Context) error {
        var req imCardActionRequest
        if err := c.Bind(&req); err != nil {
            return c.JSON(http.StatusBadRequest, map[string]string{
                "error": "invalid request body",
            })
        }
        token, err := uuid.Parse(req.CorrelationToken)
        if err != nil {
            return c.JSON(http.StatusBadRequest, map[string]string{
                "error": "invalid correlation_token",
            })
        }
        out, err := h.Router.Route(c.Request().Context(), imcards.RouteInput{
            Token:       token,
            ActionID:    req.ActionID,
            Value:       req.Value,
            ReplyTarget: req.ReplyTarget,
            UserID:      req.UserID,
            TenantID:    req.TenantID,
        })
        switch {
        case errors.Is(err, imcards.ErrCardActionExpired):
            return c.JSON(http.StatusGone, map[string]string{
                "code": "card_action:expired",
            })
        case errors.Is(err, imcards.ErrCardActionConsumed):
            return c.JSON(http.StatusConflict, map[string]string{
                "code": "card_action:consumed",
            })
        case errors.Is(err, imcards.ErrExecutionNotWaiting):
            return c.JSON(http.StatusConflict, map[string]string{
                "code": "card_action:execution_not_waiting",
            })
        case err != nil:
            return c.JSON(http.StatusInternalServerError, map[string]string{
                "error": err.Error(),
            })
        }
        resp := imCardActionResponse{Outcome: string(out.Outcome)}
        if out.ExecutionID != uuid.Nil {
            resp.ExecutionID = out.ExecutionID.String()
        }
        resp.NodeID = out.NodeID
        return c.JSON(http.StatusOK, resp)
    }
    ```
  - Wire in `src-go/internal/handler/server.go` (or wherever `RegisterRoutes` mounts handlers) — add inside the `/api/v1/im` group:
    ```go
    imGroup.POST("/card-actions", cardActions.Handle)
    ```
  - Add a handler-level test `im_card_actions_handler_test.go` covering: 200 resumed, 200 fallback, 409 consumed, 410 expired, 409 not-waiting, 400 bad UUID.
  - Run: `rtk cargo test -p handler -run IMCardActions` — green.

---

## Task E4 — `http_call` node type (handler + applier)

- [x] Step 4.1 — write failing handler test
  - File: `src-go/internal/workflow/nodetypes/http_call_test.go`
    ```go
    package nodetypes

    import (
        "context"
        "encoding/json"
        "testing"

        "github.com/google/uuid"
        "github.com/react-go-quick-starter/server/internal/model"
    )

    func TestHTTPCallHandler_EmitsExecuteEffect(t *testing.T) {
        h := HTTPCallHandler{}
        result, err := h.Execute(context.Background(), &NodeExecRequest{
            Execution: &model.WorkflowExecution{ID: uuid.New(), ProjectID: uuid.New()},
            Node:      &model.WorkflowNode{ID: "http-1"},
            Config: map[string]any{
                "method": "POST",
                "url":    "https://api.example.com/v1/things",
                "headers": map[string]any{
                    "Authorization": "Bearer {{secrets.GITHUB_TOKEN}}",
                    "Content-Type":  "application/json",
                },
                "body":            `{"hello":"world"}`,
                "timeout_seconds": 10.0,
            },
            DataStore: map[string]any{},
        })
        if err != nil { t.Fatalf("Execute: %v", err) }
        if len(result.Effects) != 1 { t.Fatalf("effects=%d", len(result.Effects)) }
        if result.Effects[0].Kind != EffectExecuteHTTPCall {
            t.Errorf("kind = %s, want execute_http_call", result.Effects[0].Kind)
        }
        var p ExecuteHTTPCallPayload
        if err := json.Unmarshal(result.Effects[0].Payload, &p); err != nil {
            t.Fatalf("unmarshal: %v", err)
        }
        if p.Method != "POST" { t.Errorf("method = %s", p.Method) }
        if p.URL != "https://api.example.com/v1/things" { t.Errorf("url = %s", p.URL) }
        if p.Headers["Authorization"] != "Bearer {{secrets.GITHUB_TOKEN}}" {
            t.Error("headers not preserved verbatim for applier-side resolution")
        }
    }

    func TestHTTPCallHandler_DefaultMethodAndTimeout(t *testing.T) {
        h := HTTPCallHandler{}
        result, _ := h.Execute(context.Background(), &NodeExecRequest{
            Execution: &model.WorkflowExecution{},
            Node:      &model.WorkflowNode{ID: "http-1"},
            Config:    map[string]any{"url": "https://api.example.com"},
            DataStore: map[string]any{},
        })
        var p ExecuteHTTPCallPayload
        _ = json.Unmarshal(result.Effects[0].Payload, &p)
        if p.Method != "GET" { t.Errorf("default method = %s", p.Method) }
        if p.TimeoutSeconds != 30 { t.Errorf("default timeout = %d", p.TimeoutSeconds) }
    }
    ```
  - Run: fails — `HTTPCallHandler` undefined.

- [x] Step 4.2 — implement handler + payload + new effect kind
  - File: `src-go/internal/workflow/nodetypes/effects.go` — append the new effect kind:
    ```go
    const (
        EffectExecuteHTTPCall EffectKind = "execute_http_call"
        EffectExecuteIMSend   EffectKind = "execute_im_send"
    )
    ```
    Note: these are NOT park effects — both run inline in the applier and return data into `dataStore[nodeID]`, then DAG advances normally. Do NOT add them to `IsPark()`.
  - In the same file, append payload structs:
    ```go
    type ExecuteHTTPCallPayload struct {
        Method          string            `json:"method"`
        URL             string            `json:"url"`
        Headers         map[string]string `json:"headers,omitempty"`
        URLQuery        map[string]string `json:"urlQuery,omitempty"`
        Body            string            `json:"body,omitempty"`
        TimeoutSeconds  int               `json:"timeoutSeconds"`
        TreatAsSuccess  []int             `json:"treatAsSuccess,omitempty"`
        // ProjectID is duplicated into the payload so the applier can resolve
        // {{secrets.X}} templates without reaching back into the execution.
        ProjectID string `json:"projectId"`
    }

    type ExecuteIMSendPayload struct {
        // RawCard is the templated ProviderNeutralCard JSON (camelCase tags
        // matching src-im-bridge/core/card_schema.ts). The applier renders
        // {{$dataStore.X}} templates into it and mints correlation tokens
        // for each callback action before dispatching to IM Bridge.
        RawCard       json.RawMessage `json:"rawCard"`
        Target        string          `json:"target"` // "reply_to_trigger" | "explicit"
        ExplicitChat  *IMSendExplicit `json:"explicit,omitempty"`
        TokenLifetime string          `json:"tokenLifetime,omitempty"` // Go duration; default 168h (7d)
    }

    type IMSendExplicit struct {
        Provider string `json:"provider"`
        ChatID   string `json:"chatId"`
        ThreadID string `json:"threadId,omitempty"`
    }
    ```
  - File: `src-go/internal/workflow/nodetypes/http_call.go`
    ```go
    package nodetypes

    import (
        "context"
        "encoding/json"
        "fmt"
    )

    // HTTPCallHandler implements the "http_call" node type. It is a pure
    // handler: it captures the templated config into an
    // EffectExecuteHTTPCall effect. The applier (E4 Step 4.4) is what
    // resolves {{secrets.X}}, dials the network, and writes the response into
    // dataStore.
    type HTTPCallHandler struct{}

    var allowedMethods = map[string]bool{
        "GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true,
    }

    func (HTTPCallHandler) Execute(_ context.Context, req *NodeExecRequest) (*NodeExecResult, error) {
        if req == nil { return nil, fmt.Errorf("nil request") }

        cfg := req.Config
        method := upperString(cfg["method"])
        if method == "" { method = "GET" }
        if !allowedMethods[method] {
            return nil, fmt.Errorf("http_call: unsupported method %q", method)
        }
        url, _ := cfg["url"].(string)
        if url == "" {
            return nil, fmt.Errorf("http_call: url is required")
        }
        timeout := 30
        if v, ok := cfg["timeout_seconds"].(float64); ok && v > 0 {
            timeout = int(v)
        }
        if timeout > 300 { timeout = 300 }

        headers := stringMap(cfg["headers"])
        urlQuery := stringMap(cfg["url_query"])
        body, _ := cfg["body"].(string)
        treatAsSuccess := intSlice(cfg["treat_as_success"])

        payload := ExecuteHTTPCallPayload{
            Method:         method,
            URL:            url,
            Headers:        headers,
            URLQuery:       urlQuery,
            Body:           body,
            TimeoutSeconds: timeout,
            TreatAsSuccess: treatAsSuccess,
            ProjectID:      req.ProjectID.String(),
        }
        raw, err := json.Marshal(payload)
        if err != nil { return nil, fmt.Errorf("marshal payload: %w", err) }

        return &NodeExecResult{
            Effects: []Effect{{Kind: EffectExecuteHTTPCall, Payload: raw}},
        }, nil
    }

    func (HTTPCallHandler) ConfigSchema() json.RawMessage {
        return json.RawMessage(`{
      "type":"object",
      "required":["url"],
      "properties":{
        "method":{"type":"string","enum":["GET","POST","PUT","PATCH","DELETE"]},
        "url":{"type":"string"},
        "headers":{"type":"object","additionalProperties":{"type":"string"}},
        "url_query":{"type":"object","additionalProperties":{"type":"string"}},
        "body":{"type":"string"},
        "timeout_seconds":{"type":"number","minimum":1,"maximum":300},
        "treat_as_success":{"type":"array","items":{"type":"integer"}}
      }
    }`)
    }

    func (HTTPCallHandler) Capabilities() []EffectKind {
        return []EffectKind{EffectExecuteHTTPCall}
    }

    // ── helpers ──
    func upperString(v any) string {
        s, _ := v.(string)
        out := make([]byte, 0, len(s))
        for i := 0; i < len(s); i++ {
            c := s[i]
            if c >= 'a' && c <= 'z' { c -= 32 }
            out = append(out, c)
        }
        return string(out)
    }
    func stringMap(v any) map[string]string {
        m, ok := v.(map[string]any)
        if !ok { return nil }
        out := make(map[string]string, len(m))
        for k, val := range m {
            if s, ok := val.(string); ok { out[k] = s }
        }
        return out
    }
    func intSlice(v any) []int {
        arr, ok := v.([]any)
        if !ok { return nil }
        out := make([]int, 0, len(arr))
        for _, x := range arr {
            switch n := x.(type) {
            case float64: out = append(out, int(n))
            case int: out = append(out, n)
            }
        }
        return out
    }
    ```
  - Run handler tests — green.

- [x] Step 4.3 — register `http_call` (and prepare for `im_send`) in bootstrap
  - File: `src-go/internal/workflow/nodetypes/bootstrap.go` — extend the `entries` slice:
    ```go
    {"http_call", HTTPCallHandler{}},
    {"im_send", IMSendHandler{}}, // implemented in E5
    ```
  - This must come AFTER E5 lands `IMSendHandler`. If executing tasks strictly in order, leave only the `http_call` line for now and add `im_send` in E5 Step 5.2.

- [x] Step 4.4 — write failing applier test
  - File: `src-go/internal/workflow/nodetypes/applier_http_call_test.go`
    ```go
    package nodetypes

    import (
        "context"
        "encoding/json"
        "fmt"
        "net/http"
        "net/http/httptest"
        "testing"

        "github.com/google/uuid"
        "github.com/react-go-quick-starter/server/internal/model"
    )

    type stubSecretResolver struct {
        valueByName map[string]string
        err         error
    }

    func (s *stubSecretResolver) Resolve(_ context.Context, _ uuid.UUID, _ string, template string) (string, error) {
        if s.err != nil { return "", s.err }
        out := template
        for name, val := range s.valueByName {
            out = replaceAll(out, "{{secrets."+name+"}}", val)
        }
        return out, nil
    }

    type stubDataStoreMerger struct{ wrote map[string]any }
    func (s *stubDataStoreMerger) MergeNodeResult(_ context.Context, _ uuid.UUID, nodeID string, payload map[string]any) error {
        if s.wrote == nil { s.wrote = map[string]any{} }
        s.wrote[nodeID] = payload
        return nil
    }

    func TestApplyExecuteHTTPCall_SecretResolutionAnd2xx(t *testing.T) {
        var gotAuth string
        srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            gotAuth = r.Header.Get("Authorization")
            w.Header().Set("Content-Type", "application/json")
            _, _ = fmt.Fprint(w, `{"ok":true}`)
        }))
        defer srv.Close()

        ds := &stubDataStoreMerger{}
        a := &EffectApplier{
            SecretResolver: &stubSecretResolver{valueByName: map[string]string{"GITHUB_TOKEN": "tok-abc"}},
            DataStoreMerger: ds,
        }

        payload := ExecuteHTTPCallPayload{
            Method: "GET", URL: srv.URL, TimeoutSeconds: 5,
            Headers: map[string]string{"Authorization": "Bearer {{secrets.GITHUB_TOKEN}}"},
            ProjectID: uuid.New().String(),
        }
        raw, _ := json.Marshal(payload)
        exec := &model.WorkflowExecution{ID: uuid.New(), ProjectID: uuid.New()}
        node := &model.WorkflowNode{ID: "http-1"}
        if err := a.applyExecuteHTTPCall(context.Background(), exec, node, raw); err != nil {
            t.Fatalf("applyExecuteHTTPCall: %v", err)
        }
        if gotAuth != "Bearer tok-abc" {
            t.Fatalf("server saw Authorization=%q, want Bearer tok-abc", gotAuth)
        }
        out := ds.wrote["http-1"].(map[string]any)
        if int(out["status"].(float64)) != 200 {
            t.Errorf("status = %v, want 200", out["status"])
        }
    }

    func TestApplyExecuteHTTPCall_Non2xxFailsByDefault(t *testing.T) {
        srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
            w.WriteHeader(http.StatusUnauthorized)
        }))
        defer srv.Close()
        a := &EffectApplier{
            SecretResolver: &stubSecretResolver{}, DataStoreMerger: &stubDataStoreMerger{},
        }
        raw, _ := json.Marshal(ExecuteHTTPCallPayload{
            Method: "GET", URL: srv.URL, TimeoutSeconds: 5, ProjectID: uuid.New().String(),
        })
        err := a.applyExecuteHTTPCall(context.Background(), &model.WorkflowExecution{}, &model.WorkflowNode{ID: "h"}, raw)
        if err == nil || err.Error() == "" {
            t.Fatalf("expected error, got nil")
        }
    }

    func TestApplyExecuteHTTPCall_TreatAsSuccessAllows401(t *testing.T) {
        srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
            w.WriteHeader(http.StatusUnauthorized)
        }))
        defer srv.Close()
        ds := &stubDataStoreMerger{}
        a := &EffectApplier{SecretResolver: &stubSecretResolver{}, DataStoreMerger: ds}
        raw, _ := json.Marshal(ExecuteHTTPCallPayload{
            Method: "GET", URL: srv.URL, TimeoutSeconds: 5,
            TreatAsSuccess: []int{401}, ProjectID: uuid.New().String(),
        })
        if err := a.applyExecuteHTTPCall(context.Background(), &model.WorkflowExecution{}, &model.WorkflowNode{ID: "h"}, raw); err != nil {
            t.Fatalf("treat_as_success should not fail: %v", err)
        }
        out := ds.wrote["h"].(map[string]any)
        if int(out["status"].(float64)) != 401 { t.Errorf("status = %v", out["status"]) }
    }

    // replaceAll is a tiny strings.ReplaceAll alias to keep test deps minimal.
    func replaceAll(s, old, new string) string {
        out := ""
        for {
            i := indexOf(s, old)
            if i < 0 { return out + s }
            out += s[:i] + new
            s = s[i+len(old):]
        }
    }
    func indexOf(s, sub string) int {
        if sub == "" { return 0 }
        for i := 0; i+len(sub) <= len(s); i++ {
            if s[i:i+len(sub)] == sub { return i }
        }
        return -1
    }
    ```
  - Run: fails (applier method + interface fields don't exist).

- [x] Step 4.5 — extend the applier
  - File: `src-go/internal/workflow/nodetypes/applier.go`
  - Add the two new applier-side seams to the `EffectApplier` struct:
    ```go
    // SecretResolver renders {{secrets.X}} templates on http_call config
    // strings. Wired in production by 1B's *secrets.Resolver. Nil disables
    // http_call (applier returns a structured error at dispatch time).
    SecretResolver SecretResolver

    // DataStoreMerger writes node results into the parent execution's
    // dataStore. Reuses WaitEventDataStoreAdapter so the http_call applier
    // and the wait_event resumer share one merge implementation.
    DataStoreMerger WaitEventDataStoreMerger

    // IMSendDispatcher posts the rendered card to IM Bridge /im/send.
    // Wired by the IM Bridge HTTP client at startup. Nil disables im_send
    // (applier returns a structured error at dispatch time).
    IMSendDispatcher IMSendDispatcher

    // CorrelationsCreator mints card_action_correlations rows for each
    // callback action on an im_send card. Wired by *imcards.CorrelationsRepo.
    CorrelationsCreator CorrelationsCreator

    // ExecutionMetaWriter merges system_metadata.im_dispatched=true after a
    // successful im_send. Wired in production by an adapter around execRepo
    // that runs `system_metadata = system_metadata || $1::jsonb`.
    ExecutionMetaWriter ExecutionMetaWriter
    ```
  - Add the supporting interfaces in `applier.go`:
    ```go
    type SecretResolver interface {
        Resolve(ctx context.Context, projectID uuid.UUID, fieldPath string, template string) (string, error)
    }

    type IMSendDispatcher interface {
        Send(ctx context.Context, replyTarget map[string]any, card json.RawMessage) (messageID string, err error)
    }

    type CorrelationsCreator interface {
        Create(ctx context.Context, in *CorrelationCreateInput) (uuid.UUID, error)
    }

    type CorrelationCreateInput struct {
        ExecutionID uuid.UUID
        NodeID      string
        ActionID    string
        Payload     map[string]any
        ExpiresAt   time.Time
    }

    type ExecutionMetaWriter interface {
        MergeSystemMetadata(ctx context.Context, executionID uuid.UUID, patch map[string]any) error
    }
    ```
  - Add the new switch cases in `Apply`:
    ```go
    case EffectExecuteHTTPCall:
        if err := a.applyExecuteHTTPCall(ctx, exec, node, e.Payload); err != nil {
            return false, fmt.Errorf("execute_http_call: %w", err)
        }
    case EffectExecuteIMSend:
        if err := a.applyExecuteIMSend(ctx, exec, node, e.Payload); err != nil {
            return false, fmt.Errorf("execute_im_send: %w", err)
        }
    ```
  - Implement `applyExecuteHTTPCall` in `applier.go` (separate file allowed):
    File: `src-go/internal/workflow/nodetypes/applier_http_call.go`
    ```go
    package nodetypes

    import (
        "bytes"
        "context"
        "encoding/json"
        "fmt"
        "io"
        "net/http"
        "net/url"
        "strings"
        "time"

        "github.com/google/uuid"
        "github.com/react-go-quick-starter/server/internal/model"
    )

    // applyExecuteHTTPCall resolves {{secrets.X}} templates in the payload's
    // headers / url / url_query / body fields, dials the URL, and writes the
    // response into dataStore[nodeID].
    //
    // Plaintext secret values exist only inside this function's scope. They
    // are NEVER copied into the request log, error message, or dataStore.
    func (a *EffectApplier) applyExecuteHTTPCall(ctx context.Context, exec *model.WorkflowExecution, node *model.WorkflowNode, raw json.RawMessage) error {
        if a.SecretResolver == nil {
            return fmt.Errorf("http_call: SecretResolver is not configured")
        }
        if a.DataStoreMerger == nil {
            return fmt.Errorf("http_call: DataStoreMerger is not configured")
        }
        var p ExecuteHTTPCallPayload
        if err := json.Unmarshal(raw, &p); err != nil {
            return fmt.Errorf("unmarshal payload: %w", err)
        }

        projectID, err := uuid.Parse(p.ProjectID)
        if err != nil {
            // Fall back to exec.ProjectID; the handler always stamps it but
            // be defensive against legacy payload shapes.
            projectID = exec.ProjectID
        }

        // Resolve every templated string. Field paths are passed through so
        // 1B's resolver can audit which field consumed which secret.
        urlResolved, err := a.SecretResolver.Resolve(ctx, projectID, "url", p.URL)
        if err != nil {
            return fmt.Errorf("http_call:secret_resolve url: %w", err)
        }
        bodyResolved, err := a.SecretResolver.Resolve(ctx, projectID, "body", p.Body)
        if err != nil {
            return fmt.Errorf("http_call:secret_resolve body: %w", err)
        }
        headersResolved := make(map[string]string, len(p.Headers))
        for k, v := range p.Headers {
            r, err := a.SecretResolver.Resolve(ctx, projectID, "headers."+k, v)
            if err != nil {
                return fmt.Errorf("http_call:secret_resolve header %q: %w", k, err)
            }
            headersResolved[k] = r
        }
        queryResolved := make(map[string]string, len(p.URLQuery))
        for k, v := range p.URLQuery {
            r, err := a.SecretResolver.Resolve(ctx, projectID, "url_query."+k, v)
            if err != nil {
                return fmt.Errorf("http_call:secret_resolve query %q: %w", k, err)
            }
            queryResolved[k] = r
        }

        // Append url_query as URL params.
        if len(queryResolved) > 0 {
            parsed, perr := url.Parse(urlResolved)
            if perr != nil {
                return fmt.Errorf("http_call: invalid url after resolve")
            }
            q := parsed.Query()
            for k, v := range queryResolved { q.Set(k, v) }
            parsed.RawQuery = q.Encode()
            urlResolved = parsed.String()
        }

        // Build request.
        var bodyReader io.Reader
        if bodyResolved != "" {
            bodyReader = bytes.NewReader([]byte(bodyResolved))
        }
        req, err := http.NewRequestWithContext(ctx, p.Method, urlResolved, bodyReader)
        if err != nil {
            return fmt.Errorf("http_call: build request: %w", err)
        }
        for k, v := range headersResolved {
            req.Header.Set(k, v)
        }

        client := &http.Client{Timeout: time.Duration(p.TimeoutSeconds) * time.Second}
        resp, err := client.Do(req)
        if err != nil {
            // Distinguish timeout for the spec §10 error matrix. We use
            // Go's url.Error.Timeout() which is set when the client.Timeout
            // fires, regardless of whether the inner cause was DNS / dial /
            // read.
            if ue, ok := err.(interface{ Timeout() bool }); ok && ue.Timeout() {
                return fmt.Errorf("http_call:timeout")
            }
            return fmt.Errorf("http_call: dial: %w", err)
        }
        defer resp.Body.Close()
        respBody, _ := io.ReadAll(resp.Body)

        // 2xx is always success. Otherwise treat_as_success whitelist.
        ok := resp.StatusCode >= 200 && resp.StatusCode < 300
        if !ok {
            for _, s := range p.TreatAsSuccess {
                if s == resp.StatusCode { ok = true; break }
            }
        }
        if !ok {
            return fmt.Errorf("http_call:non_2xx_status (got %d)", resp.StatusCode)
        }

        // Write response into dataStore. Body is parsed if it looks like JSON
        // so downstream nodes can do {{$dataStore.http-1.body.fieldName}}.
        var bodyVal any = string(respBody)
        if isJSONContentType(resp.Header.Get("Content-Type")) && json.Valid(respBody) {
            var parsed any
            _ = json.Unmarshal(respBody, &parsed)
            bodyVal = parsed
        }
        result := map[string]any{
            "status":  resp.StatusCode,
            "headers": flattenHeaders(resp.Header),
            "body":    bodyVal,
        }
        if err := a.DataStoreMerger.MergeNodeResult(ctx, exec.ID, node.ID, result); err != nil {
            return fmt.Errorf("http_call: merge result: %w", err)
        }

        // Audit log: NEVER include the URL with query (might contain secrets
        // a later iteration of url_query templates) or any header value.
        if a.AuditSink != nil {
            host := ""
            if u, _ := url.Parse(urlResolved); u != nil { host = u.Host }
            _ = a.AuditSink.Record(ctx, "http_call_executed", map[string]any{
                "executionId": exec.ID.String(),
                "nodeId":      node.ID,
                "method":      p.Method,
                "urlHost":     host,
                "status":      resp.StatusCode,
            })
        }
        return nil
    }

    func isJSONContentType(ct string) bool {
        ct = strings.ToLower(strings.TrimSpace(strings.SplitN(ct, ";", 2)[0]))
        return ct == "application/json" || strings.HasSuffix(ct, "+json")
    }
    func flattenHeaders(h http.Header) map[string]string {
        out := make(map[string]string, len(h))
        for k, v := range h {
            if len(v) > 0 { out[k] = v[0] }
        }
        return out
    }
    ```
  - Add `AuditSink` to the applier struct (only if absent):
    ```go
    AuditSink imcards.AuditSink // re-uses E3's AuditSink contract
    ```
    (Or, to avoid the cross-package dep, declare an identical interface in this package and let the wiring layer adapt — applier should not import imcards.) Use this version instead:
    ```go
    type AuditRecorder interface {
        Record(ctx context.Context, kind string, payload map[string]any) error
    }
    AuditSink AuditRecorder
    ```
  - Run applier tests — green.

---

## Task E5 — `im_send` node type (handler + applier)

- [x] Step 5.1 — write failing handler test
  - File: `src-go/internal/workflow/nodetypes/im_send_test.go`
    ```go
    package nodetypes

    import (
        "context"
        "encoding/json"
        "testing"

        "github.com/google/uuid"
        "github.com/react-go-quick-starter/server/internal/model"
    )

    func TestIMSendHandler_EmitsEffect(t *testing.T) {
        h := IMSendHandler{}
        result, err := h.Execute(context.Background(), &NodeExecRequest{
            Execution: &model.WorkflowExecution{ID: uuid.New(), ProjectID: uuid.New()},
            Node:      &model.WorkflowNode{ID: "im-1"},
            Config: map[string]any{
                "target": "reply_to_trigger",
                "card": map[string]any{
                    "title":   "Build complete",
                    "status":  "success",
                    "summary": "PR #42 merged",
                    "actions": []any{
                        map[string]any{
                            "id": "approve", "label": "Approve", "type": "callback",
                            "payload": map[string]any{"thread": "x"},
                        },
                    },
                },
            },
        })
        if err != nil { t.Fatalf("Execute: %v", err) }
        if len(result.Effects) != 1 { t.Fatalf("effects=%d", len(result.Effects)) }
        if result.Effects[0].Kind != EffectExecuteIMSend {
            t.Errorf("kind = %s", result.Effects[0].Kind)
        }
        var p ExecuteIMSendPayload
        if err := json.Unmarshal(result.Effects[0].Payload, &p); err != nil {
            t.Fatalf("unmarshal: %v", err)
        }
        if p.Target != "reply_to_trigger" { t.Errorf("target = %s", p.Target) }
        if len(p.RawCard) == 0 { t.Error("rawCard empty") }
    }
    ```
  - Run: fails.

- [x] Step 5.2 — implement handler + register in bootstrap
  - File: `src-go/internal/workflow/nodetypes/im_send.go`
    ```go
    package nodetypes

    import (
        "context"
        "encoding/json"
        "fmt"
    )

    // IMSendHandler implements the "im_send" node type. The handler emits a
    // single EffectExecuteIMSend carrying the templated card config. The
    // applier (E5 Step 5.4) is what mints correlation tokens, renders the
    // card, dispatches via IM Bridge, and stamps system_metadata.im_dispatched.
    type IMSendHandler struct{}

    func (IMSendHandler) Execute(_ context.Context, req *NodeExecRequest) (*NodeExecResult, error) {
        if req == nil { return nil, fmt.Errorf("nil request") }
        cfg := req.Config

        target, _ := cfg["target"].(string)
        if target == "" { target = "reply_to_trigger" }
        if target != "reply_to_trigger" && target != "explicit" {
            return nil, fmt.Errorf("im_send: invalid target %q", target)
        }

        cardRaw, ok := cfg["card"]
        if !ok {
            return nil, fmt.Errorf("im_send: card is required")
        }
        cardBytes, err := json.Marshal(cardRaw)
        if err != nil {
            return nil, fmt.Errorf("im_send: marshal card: %w", err)
        }

        payload := ExecuteIMSendPayload{
            RawCard: cardBytes,
            Target:  target,
        }
        if target == "explicit" {
            if exp, ok := cfg["explicit_target"].(map[string]any); ok {
                payload.ExplicitChat = &IMSendExplicit{
                    Provider: stringOf(exp["provider"]),
                    ChatID:   stringOf(exp["chat_id"]),
                    ThreadID: stringOf(exp["thread_id"]),
                }
            }
        }
        if v, ok := cfg["token_lifetime"].(string); ok {
            payload.TokenLifetime = v
        }

        raw, err := json.Marshal(payload)
        if err != nil { return nil, err }
        return &NodeExecResult{
            Effects: []Effect{{Kind: EffectExecuteIMSend, Payload: raw}},
        }, nil
    }

    func (IMSendHandler) ConfigSchema() json.RawMessage {
        return json.RawMessage(`{
      "type":"object",
      "required":["card"],
      "properties":{
        "target":{"type":"string","enum":["reply_to_trigger","explicit"]},
        "explicit_target":{"type":"object","properties":{
          "provider":{"type":"string"},
          "chat_id":{"type":"string"},
          "thread_id":{"type":"string"}
        }},
        "card":{"type":"object","required":["title"]},
        "token_lifetime":{"type":"string","description":"Go duration; default 168h"}
      }
    }`)
    }

    func (IMSendHandler) Capabilities() []EffectKind {
        return []EffectKind{EffectExecuteIMSend}
    }

    func stringOf(v any) string { s, _ := v.(string); return s }
    ```
  - File: `src-go/internal/workflow/nodetypes/bootstrap.go` — confirm or add the `{"im_send", IMSendHandler{}}` entry; remove the placeholder if E4 Step 4.3 left one out.

- [x] Step 5.3 — write failing applier test
  - File: `src-go/internal/workflow/nodetypes/applier_im_send_test.go`
    ```go
    package nodetypes

    import (
        "context"
        "encoding/json"
        "testing"
        "time"

        "github.com/google/uuid"
        "github.com/react-go-quick-starter/server/internal/model"
    )

    type stubDispatcher struct{ sentCards []json.RawMessage; replyTarget map[string]any }
    func (s *stubDispatcher) Send(_ context.Context, target map[string]any, card json.RawMessage) (string, error) {
        s.replyTarget = target
        s.sentCards = append(s.sentCards, card)
        return "msg-1", nil
    }

    type stubCorrelationsCreator struct{ created []*CorrelationCreateInput; nextToken uuid.UUID }
    func (s *stubCorrelationsCreator) Create(_ context.Context, in *CorrelationCreateInput) (uuid.UUID, error) {
        s.created = append(s.created, in)
        if s.nextToken == uuid.Nil { return uuid.New(), nil }
        return s.nextToken, nil
    }

    type stubMetaWriter struct{ patches []map[string]any }
    func (s *stubMetaWriter) MergeSystemMetadata(_ context.Context, _ uuid.UUID, p map[string]any) error {
        s.patches = append(s.patches, p); return nil
    }

    func TestApplyExecuteIMSend_ReplyToTriggerWithCallbackTokens(t *testing.T) {
        execID := uuid.New()
        // system_metadata.reply_target is preloaded by trigger_handler at
        // start time (per spec §6.3); the fixture inlines it.
        sysMeta := map[string]any{
            "reply_target": map[string]any{
                "provider": "feishu", "chat_id": "C1", "thread_id": "T1",
            },
        }
        sysMetaBytes, _ := json.Marshal(sysMeta)
        exec := &model.WorkflowExecution{ID: execID, SystemMetadata: sysMetaBytes}

        dispatch := &stubDispatcher{}
        creator := &stubCorrelationsCreator{}
        meta := &stubMetaWriter{}
        ds := &stubDataStoreMerger{}

        a := &EffectApplier{
            IMSendDispatcher: dispatch, CorrelationsCreator: creator,
            ExecutionMetaWriter: meta, DataStoreMerger: ds,
        }

        cardRaw := json.RawMessage(`{
          "title":"Done",
          "actions":[
            {"id":"approve","label":"Approve","type":"callback","payload":{"k":"v"}},
            {"id":"link","label":"View","type":"url","url":"https://x"}
          ]
        }`)
        payload := ExecuteIMSendPayload{RawCard: cardRaw, Target: "reply_to_trigger"}
        raw, _ := json.Marshal(payload)

        if err := a.applyExecuteIMSend(context.Background(), exec, &model.WorkflowNode{ID: "im-1"}, raw); err != nil {
            t.Fatalf("applyExecuteIMSend: %v", err)
        }
        if len(creator.created) != 1 {
            t.Fatalf("expected 1 callback correlation, got %d", len(creator.created))
        }
        if creator.created[0].ActionID != "approve" {
            t.Errorf("action id = %s", creator.created[0].ActionID)
        }
        // Token expires within 7 days +/- 1 minute.
        if creator.created[0].ExpiresAt.Sub(time.Now()) > 7*24*time.Hour+time.Minute {
            t.Error("default lifetime too long")
        }
        if len(dispatch.sentCards) != 1 { t.Fatal("card not dispatched") }
        if got := dispatch.replyTarget["chat_id"]; got != "C1" {
            t.Errorf("reply chat_id = %v", got)
        }
        if len(meta.patches) != 1 || meta.patches[0]["im_dispatched"] != true {
            t.Errorf("im_dispatched not stamped: %+v", meta.patches)
        }

        // The dispatched card MUST have the callback action's value rewritten
        // to {"correlation_token": "<uuid>", "action_id": "approve"} per spec
        // §14 (no inline payload — payload lives in the correlations row).
        var dispatched map[string]any
        _ = json.Unmarshal(dispatch.sentCards[0], &dispatched)
        actions := dispatched["actions"].([]any)
        cb := actions[0].(map[string]any)
        if cb["correlation_token"] == nil { t.Error("correlation_token not injected") }
    }

    func TestApplyExecuteIMSend_NoReplyTarget(t *testing.T) {
        a := &EffectApplier{IMSendDispatcher: &stubDispatcher{}, CorrelationsCreator: &stubCorrelationsCreator{}, ExecutionMetaWriter: &stubMetaWriter{}, DataStoreMerger: &stubDataStoreMerger{}}
        payload := ExecuteIMSendPayload{RawCard: json.RawMessage(`{"title":"x"}`), Target: "reply_to_trigger"}
        raw, _ := json.Marshal(payload)
        err := a.applyExecuteIMSend(context.Background(), &model.WorkflowExecution{}, &model.WorkflowNode{ID: "im-1"}, raw)
        if err == nil || err.Error() != "im_send:no_reply_target" {
            t.Fatalf("err = %v, want im_send:no_reply_target", err)
        }
    }
    ```
  - Run: fails.

- [x] Step 5.4 — implement applier
  - File: `src-go/internal/workflow/nodetypes/applier_im_send.go`
    ```go
    package nodetypes

    import (
        "context"
        "encoding/json"
        "fmt"
        "time"

        "github.com/react-go-quick-starter/server/internal/model"
    )

    // applyExecuteIMSend performs:
    //   1. Resolve target (reply_to_trigger | explicit).
    //   2. Walk card.actions[]; for each {type:"callback"}, mint a
    //      correlation row and rewrite the action to carry only
    //      {correlation_token, action_id} per spec §14.
    //   3. POST the rendered card to IM Bridge via IMSendDispatcher.
    //   4. Stamp system_metadata.im_dispatched=true so 1D's outbound
    //      dispatcher skips the default回帖.
    //   5. Write {sent:true, message_id} into dataStore[nodeID].
    func (a *EffectApplier) applyExecuteIMSend(ctx context.Context, exec *model.WorkflowExecution, node *model.WorkflowNode, raw json.RawMessage) error {
        if a.IMSendDispatcher == nil { return fmt.Errorf("im_send: IMSendDispatcher not configured") }
        if a.CorrelationsCreator == nil { return fmt.Errorf("im_send: CorrelationsCreator not configured") }
        if a.ExecutionMetaWriter == nil { return fmt.Errorf("im_send: ExecutionMetaWriter not configured") }
        if a.DataStoreMerger == nil { return fmt.Errorf("im_send: DataStoreMerger not configured") }

        var p ExecuteIMSendPayload
        if err := json.Unmarshal(raw, &p); err != nil {
            return fmt.Errorf("unmarshal payload: %w", err)
        }

        // --- 1. Resolve target ---
        var replyTarget map[string]any
        switch p.Target {
        case "reply_to_trigger":
            sysMeta := map[string]any{}
            if len(exec.SystemMetadata) > 0 {
                _ = json.Unmarshal(exec.SystemMetadata, &sysMeta)
            }
            rt, ok := sysMeta["reply_target"].(map[string]any)
            if !ok || len(rt) == 0 {
                return fmt.Errorf("im_send:no_reply_target")
            }
            replyTarget = rt
        case "explicit":
            if p.ExplicitChat == nil {
                return fmt.Errorf("im_send: explicit target requires explicit_target")
            }
            replyTarget = map[string]any{
                "provider":  p.ExplicitChat.Provider,
                "chat_id":   p.ExplicitChat.ChatID,
                "thread_id": p.ExplicitChat.ThreadID,
            }
        default:
            return fmt.Errorf("im_send: invalid target %q", p.Target)
        }

        // --- 2. Walk + mint correlations ---
        var card map[string]any
        if err := json.Unmarshal(p.RawCard, &card); err != nil {
            return fmt.Errorf("im_send: invalid card: %w", err)
        }
        lifetime := 7 * 24 * time.Hour
        if p.TokenLifetime != "" {
            if d, err := time.ParseDuration(p.TokenLifetime); err == nil { lifetime = d }
        }
        if actions, ok := card["actions"].([]any); ok {
            for i, raw := range actions {
                act, ok := raw.(map[string]any)
                if !ok { continue }
                if act["type"] != "callback" { continue }
                actionID, _ := act["id"].(string)
                if actionID == "" {
                    return fmt.Errorf("im_send: callback action missing id")
                }
                payloadFromAuthor := map[string]any{}
                if pl, ok := act["payload"].(map[string]any); ok { payloadFromAuthor = pl }
                token, err := a.CorrelationsCreator.Create(ctx, &CorrelationCreateInput{
                    ExecutionID: exec.ID,
                    NodeID:      node.ID,
                    ActionID:    actionID,
                    Payload:     payloadFromAuthor,
                    ExpiresAt:   time.Now().Add(lifetime),
                })
                if err != nil {
                    return fmt.Errorf("im_send: mint correlation %q: %w", actionID, err)
                }
                // Rewrite the action: drop the author payload from the wire
                // (it lives in the correlations row), inject correlation_token.
                act["correlation_token"] = token.String()
                delete(act, "payload")
                actions[i] = act
            }
            card["actions"] = actions
        }
        renderedCard, err := json.Marshal(card)
        if err != nil { return fmt.Errorf("im_send: re-marshal card: %w", err) }

        // --- 3. Dispatch ---
        messageID, err := a.IMSendDispatcher.Send(ctx, replyTarget, renderedCard)
        if err != nil {
            return fmt.Errorf("im_send: dispatch: %w", err)
        }

        // --- 4. Stamp im_dispatched ---
        if err := a.ExecutionMetaWriter.MergeSystemMetadata(ctx, exec.ID, map[string]any{
            "im_dispatched": true,
        }); err != nil {
            // Soft-fail: the dispatch already happened. Logging via audit so
            // operators can detect the rare double-send path.
            if a.AuditSink != nil {
                _ = a.AuditSink.Record(ctx, "im_send_meta_stamp_failed", map[string]any{
                    "executionId": exec.ID.String(), "nodeId": node.ID, "error": err.Error(),
                })
            }
        }

        // --- 5. Write result ---
        result := map[string]any{"sent": true}
        if messageID != "" { result["message_id"] = messageID }
        if err := a.DataStoreMerger.MergeNodeResult(ctx, exec.ID, node.ID, result); err != nil {
            return fmt.Errorf("im_send: merge result: %w", err)
        }
        return nil
    }

    var _ = model.WorkflowExecStatusRunning // avoid unused-import surprises
    ```
  - File: `src-go/internal/model/workflow_definition.go` — extend the struct (per spec §6.3):
    ```go
    SystemMetadata json.RawMessage `db:"system_metadata" json:"systemMetadata,omitempty" gorm:"type:jsonb"`
    ```
    Plus matching migration `src-go/migrations/068_workflow_executions_system_metadata.{up,down}.sql`:
    ```sql
    -- 068_workflow_executions_system_metadata.up.sql
    ALTER TABLE workflow_executions
      ADD COLUMN IF NOT EXISTS system_metadata jsonb NOT NULL DEFAULT '{}'::jsonb;
    ```
    ```sql
    -- 068_...down.sql
    ALTER TABLE workflow_executions DROP COLUMN IF EXISTS system_metadata;
    ```
    Note: 1D may also touch this column (it reads `im_dispatched`); coordinate so the migration lands exactly once. If 1D has already added it, drop this migration file from this plan.

- [x] Step 5.5 — implement `ExecutionMetaWriter` adapter
  - File: `src-go/internal/repository/workflow_execution_meta_repo.go`
    ```go
    package repository

    import (
        "context"
        "database/sql"
        "encoding/json"
        "fmt"

        "github.com/google/uuid"
    )

    type WorkflowExecutionMetaRepo struct{ db *sql.DB }

    func NewWorkflowExecutionMetaRepo(db *sql.DB) *WorkflowExecutionMetaRepo {
        return &WorkflowExecutionMetaRepo{db: db}
    }

    // MergeSystemMetadata performs an idempotent jsonb-merge so concurrent
    // writers (e.g. trigger_handler stamping reply_target while im_send
    // stamps im_dispatched) do not clobber each other.
    func (r *WorkflowExecutionMetaRepo) MergeSystemMetadata(ctx context.Context, executionID uuid.UUID, patch map[string]any) error {
        b, err := json.Marshal(patch)
        if err != nil { return fmt.Errorf("marshal patch: %w", err) }
        _, err = r.db.ExecContext(ctx, `
            UPDATE workflow_executions
               SET system_metadata = COALESCE(system_metadata, '{}'::jsonb) || $1::jsonb,
                   updated_at = now()
             WHERE id = $2`, b, executionID)
        return err
    }
    ```
  - Add a small repo-level test file covering the merge and verifying that a second call preserves a previously stamped key.
  - Run: `rtk cargo test -p repository -run WorkflowExecutionMetaRepo` — green.

---

## Task E6 — IM Bridge: Feishu inbound `card_action` → backend forward

> **Where to insert.** 1D will have removed `renderInteractiveCard` / `renderStructuredMessage` from `live.go`. The inbound parsing helpers (`normalizeCardActionRequest` etc., currently lines ~835-1010 of `live.go`) stay. We attach the new forwarder in the existing `handleCardAction` callback path.

- [x] Step 6.1 — write failing TS test
  - File: `src-im-bridge/platform/feishu/card_action_forward.test.ts`
    ```ts
    import { describe, expect, it, mock } from "bun:test";
    import { extractCorrelationToken, forwardCardActionToBackend } from "./card_action_forward";

    describe("extractCorrelationToken", () => {
      it("reads correlation_token from action.value", () => {
        const tok = extractCorrelationToken({
          value: { correlation_token: "11111111-1111-1111-1111-111111111111", action_id: "approve" },
        });
        expect(tok).toBe("11111111-1111-1111-1111-111111111111");
      });
      it("returns null when missing", () => {
        expect(extractCorrelationToken({ value: { foo: "bar" } })).toBeNull();
      });
    });

    describe("forwardCardActionToBackend", () => {
      it("posts the structured payload and maps backend status to a toast", async () => {
        const fetchSpy = mock(async (_url: string, init?: RequestInit) => {
          // Backend says expired
          return new Response(JSON.stringify({ code: "card_action:expired" }), {
            status: 410, headers: { "Content-Type": "application/json" },
          });
        });
        const out = await forwardCardActionToBackend({
          backendURL: "http://backend",
          token: "11111111-1111-1111-1111-111111111111",
          actionId: "approve",
          value: { foo: "bar" },
          replyTarget: { chat_id: "C1" },
          userId: "U1",
          tenantId: "T1",
          fetchImpl: fetchSpy as any,
        });
        expect(out.toastText).toContain("操作已过期");
        expect(fetchSpy).toHaveBeenCalledTimes(1);
      });
    });
    ```
  - Run: `rtk bun test src-im-bridge/platform/feishu/card_action_forward.test.ts` — fails.

- [x] Step 6.2 — implement the forwarder helper
  - File: `src-im-bridge/platform/feishu/card_action_forward.ts`
    ```ts
    /**
     * Inbound bridge → backend forwarder for Feishu card_action callbacks.
     *
     * Lifecycle:
     *   1. live.go → handleCardAction → normalizeCardActionRequest produces an
     *      ActionRequest whose `.Value` is the structured value sent inside
     *      the original button payload. Workflow-minted buttons place
     *      `{correlation_token, action_id, ...optionalUserValue}` here.
     *   2. extractCorrelationToken pulls the token. If absent, the click is a
     *      legacy / hardcoded card and falls back to the existing notify
     *      handler — DO NOT call this forwarder.
     *   3. forwardCardActionToBackend POSTs to /api/v1/im/card-actions and
     *      maps the response status to the user-facing toast text.
     */

    export type CardActionValue = Record<string, unknown> | undefined;

    export interface ForwardInput {
      backendURL: string;
      token: string;
      actionId: string;
      value: Record<string, unknown>;
      replyTarget: Record<string, unknown>;
      userId: string;
      tenantId: string;
      fetchImpl?: typeof fetch;
    }

    export interface ForwardResult {
      ok: boolean;
      toastText?: string;
    }

    export function extractCorrelationToken(action: { value?: CardActionValue }): string | null {
      const v = action?.value;
      if (!v || typeof v !== "object") return null;
      const tok = (v as Record<string, unknown>)["correlation_token"];
      if (typeof tok === "string" && tok.length > 0) return tok;
      return null;
    }

    export async function forwardCardActionToBackend(input: ForwardInput): Promise<ForwardResult> {
      const f = input.fetchImpl ?? fetch;
      const body = {
        correlation_token: input.token,
        action_id: input.actionId,
        value: input.value,
        replyTarget: input.replyTarget,
        user_id: input.userId,
        tenant_id: input.tenantId,
      };
      let res: Response;
      try {
        res = await f(`${input.backendURL}/api/v1/im/card-actions`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(body),
        });
      } catch (err) {
        // Network failure — surface a generic toast but do not throw; the
        // user-facing path must always return *something* renderable.
        return { ok: false, toastText: "操作失败，请稍后再试" };
      }

      if (res.status === 200) return { ok: true };

      // Map backend error codes to localized toasts per spec §10.
      let code = "";
      try {
        const j = (await res.json()) as { code?: string };
        code = j.code ?? "";
      } catch {
        code = "";
      }
      switch (code) {
        case "card_action:expired":
          return { ok: false, toastText: "操作已过期" };
        case "card_action:consumed":
          return { ok: false, toastText: "操作已处理" };
        case "card_action:execution_not_waiting":
          return { ok: false, toastText: "工作流已结束" };
        default:
          return { ok: false, toastText: "操作失败" };
      }
    }
    ```
  - Run TS test — green.

- [x] Step 6.3 — wire into `handleCardAction`
  - File: `src-im-bridge/platform/feishu/live.go` — locate `handleCardAction` (~line 835). After `normalizeCardActionRequest` succeeds and BEFORE the existing `l.actionHandler.HandleAction(...)` call, insert a token-bearing branch:
    ```go
    // Workflow-minted buttons carry a correlation_token in act.Value. When
    // present, forward to backend /api/v1/im/card-actions instead of routing
    // to the legacy notify handler. (E6 wiring.)
    if token := extractCorrelationTokenGo(event.Event.Action); token != "" {
        result, err := l.cardActionForwarder.Forward(ctx, cardActionForwardInput{
            Token:       token,
            ActionID:    req.Action,         // already normalized above
            Value:       req.Metadata,        // free-form value bag
            ReplyTarget: replyTargetFromEvent(event),
            UserID:      userIDFromEvent(event),
            TenantID:    tenantIDFromEvent(event),
        })
        if err != nil { return nil, err }
        return cardActionToastResponse(result), nil
    }
    ```
    Add the helpers in a new file `src-im-bridge/platform/feishu/card_action_forward.go` mirroring the TS forwarder shape. Backend URL comes from the same env var the rest of the bridge uses (see `BridgeConfig.BackendURL`).
  - For Slack / DingTalk inbound: per the brief, add empty stubs only.
    Files: `src-im-bridge/platform/slack/card_action_stub.ts`, `src-im-bridge/platform/dingtalk/card_action_stub.ts`:
    ```ts
    // TODO(spec1-followup): implement inbound card_action forwarding for this
    // provider. Until then, log + drop so tests can spot the missing path.
    export function dropCardAction(provider: string, action: unknown) {
      console.warn(`[bridge] ${provider} inbound card_action dropped`, action);
    }
    ```

---

## Task E7 — Frontend: `http_call` + `im_send` config panels + palette entries

- [x] Step 7.1 — write failing config-panel test for http_call
  - File: `components/workflow-editor/config-panel/node-configs/http-call-config.test.tsx`
    ```tsx
    import { render, screen, fireEvent } from "@testing-library/react";
    import { HTTPCallConfig } from "./http-call-config";

    describe("HTTPCallConfig", () => {
      it("renders method, url, and a help blurb on secrets templating", () => {
        render(<HTTPCallConfig config={{}} onChange={() => {}} />);
        expect(screen.getByLabelText(/Method/i)).toBeInTheDocument();
        expect(screen.getByLabelText(/URL/i)).toBeInTheDocument();
        expect(screen.getByText(/\{\{secrets\.\w+\}\}/)).toBeInTheDocument();
      });
      it("serializes a header row to config.headers", () => {
        const onChange = jest.fn();
        render(<HTTPCallConfig config={{}} onChange={onChange} />);
        fireEvent.click(screen.getByRole("button", { name: /Add header/i }));
        const keyInput = screen.getByPlaceholderText(/Header name/i);
        fireEvent.change(keyInput, { target: { value: "Authorization" } });
        const valueInput = screen.getByPlaceholderText(/Header value/i);
        fireEvent.change(valueInput, { target: { value: "Bearer {{secrets.X}}" } });
        const last = onChange.mock.calls.at(-1)![0];
        expect(last.headers).toEqual({ Authorization: "Bearer {{secrets.X}}" });
      });
    });
    ```
  - Run: `rtk vitest run components/workflow-editor/config-panel/node-configs/http-call-config.test.tsx` — fails.

- [x] Step 7.2 — implement http-call-config.tsx
  - File: `components/workflow-editor/config-panel/node-configs/http-call-config.tsx`
    ```tsx
    "use client";

    import { useState } from "react";
    import { Plus, Trash2 } from "lucide-react";
    import { Input } from "@/components/ui/input";
    import { Textarea } from "@/components/ui/textarea";
    import { Label } from "@/components/ui/label";
    import { Button } from "@/components/ui/button";
    import {
      Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
    } from "@/components/ui/select";

    type KV = { k: string; v: string };
    type Method = "GET" | "POST" | "PUT" | "PATCH" | "DELETE";

    interface Props {
      config: Record<string, unknown>;
      onChange: (c: Record<string, unknown>) => void;
    }

    const SECRETS_HELP = "Templating: {{secrets.NAME}} is allowed only in URL, headers, url_query, and body. Other fields reject it at save time.";

    function fromObject(o: unknown): KV[] {
      if (!o || typeof o !== "object") return [];
      return Object.entries(o as Record<string, string>).map(([k, v]) => ({ k, v }));
    }
    function toObject(rows: KV[]): Record<string, string> {
      const out: Record<string, string> = {};
      for (const r of rows) if (r.k.trim()) out[r.k] = r.v;
      return out;
    }

    export function HTTPCallConfig({ config, onChange }: Props) {
      const method = (config.method as Method | undefined) ?? "GET";
      const url = (config.url as string | undefined) ?? "";
      const body = (config.body as string | undefined) ?? "";
      const timeout = (config.timeout_seconds as number | undefined) ?? 30;
      const treatAsSuccess = ((config.treat_as_success as number[] | undefined) ?? []).join(",");

      const [headers, setHeaders] = useState<KV[]>(fromObject(config.headers));
      const [query, setQuery] = useState<KV[]>(fromObject(config.url_query));

      function update(patch: Record<string, unknown>) {
        onChange({ ...config, ...patch });
      }

      function updateHeaders(next: KV[]) {
        setHeaders(next);
        update({ headers: toObject(next) });
      }
      function updateQuery(next: KV[]) {
        setQuery(next);
        update({ url_query: toObject(next) });
      }

      return (
        <div className="flex flex-col gap-4">
          <div className="flex flex-col gap-1.5">
            <Label className="text-xs">Method</Label>
            <Select value={method} onValueChange={(v) => update({ method: v })}>
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                {(["GET","POST","PUT","PATCH","DELETE"] as Method[]).map((m) => (
                  <SelectItem key={m} value={m}>{m}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="flex flex-col gap-1.5">
            <Label className="text-xs">URL</Label>
            <Input value={url} onChange={(e) => update({ url: e.target.value })}
              placeholder="https://api.example.com/v1/things" />
            <p className="text-[11px] text-muted-foreground">{SECRETS_HELP}</p>
          </div>

          <KVEditor label="Headers" rows={headers} onChange={updateHeaders}
            keyPlaceholder="Header name" valuePlaceholder="Header value (e.g. Bearer {{secrets.X}})" />

          <KVEditor label="URL Query" rows={query} onChange={updateQuery}
            keyPlaceholder="Query name" valuePlaceholder="Value" />

          <div className="flex flex-col gap-1.5">
            <Label className="text-xs">Body</Label>
            <Textarea rows={5} value={body}
              onChange={(e) => update({ body: e.target.value })}
              placeholder='{"hello":"world"} — supports {{secrets.X}} and {{$dataStore.X.Y}}'
              className="font-mono text-xs" />
          </div>

          <div className="flex flex-col gap-1.5">
            <Label className="text-xs">Timeout (seconds, max 300)</Label>
            <Input type="number" value={timeout}
              onChange={(e) => update({ timeout_seconds: Number(e.target.value) })} />
          </div>

          <div className="flex flex-col gap-1.5">
            <Label className="text-xs">Treat as success (comma-separated status codes)</Label>
            <Input value={treatAsSuccess}
              onChange={(e) => {
                const arr = e.target.value.split(",").map((s) => Number(s.trim())).filter((n) => !Number.isNaN(n));
                update({ treat_as_success: arr });
              }}
              placeholder="e.g. 401,404" />
          </div>
        </div>
      );
    }

    function KVEditor({
      label, rows, onChange, keyPlaceholder, valuePlaceholder,
    }: {
      label: string; rows: KV[]; onChange: (rows: KV[]) => void;
      keyPlaceholder: string; valuePlaceholder: string;
    }) {
      return (
        <div className="flex flex-col gap-1.5">
          <Label className="text-xs">{label}</Label>
          <div className="flex flex-col gap-1.5">
            {rows.map((r, i) => (
              <div key={i} className="flex gap-1.5">
                <Input className="flex-1" placeholder={keyPlaceholder} value={r.k}
                  onChange={(e) => {
                    const next = rows.slice();
                    next[i] = { ...r, k: e.target.value };
                    onChange(next);
                  }} />
                <Input className="flex-1" placeholder={valuePlaceholder} value={r.v}
                  onChange={(e) => {
                    const next = rows.slice();
                    next[i] = { ...r, v: e.target.value };
                    onChange(next);
                  }} />
                <Button variant="ghost" size="icon" onClick={() => onChange(rows.filter((_, idx) => idx !== i))}>
                  <Trash2 className="h-3.5 w-3.5" />
                </Button>
              </div>
            ))}
            <Button variant="ghost" size="sm" className="self-start"
              onClick={() => onChange([...rows, { k: "", v: "" }])}>
              <Plus className="mr-1 h-3.5 w-3.5" /> Add {label.toLowerCase().replace(/s$/, "")}
            </Button>
          </div>
        </div>
      );
    }
    ```
  - Run http-call-config tests — green.

- [x] Step 7.3 — write failing config-panel test for im_send
  - File: `components/workflow-editor/config-panel/node-configs/im-send-config.test.tsx`
    ```tsx
    import { render, screen, fireEvent } from "@testing-library/react";
    import { IMSendConfig } from "./im-send-config";

    describe("IMSendConfig", () => {
      it("defaults to reply_to_trigger with no explicit form", () => {
        render(<IMSendConfig config={{}} onChange={() => {}} />);
        expect(screen.getByLabelText(/Target/i)).toBeInTheDocument();
        expect(screen.queryByLabelText(/Chat ID/i)).not.toBeInTheDocument();
      });
      it("flips to explicit and shows provider + chat id inputs", () => {
        const onChange = jest.fn();
        render(<IMSendConfig config={{}} onChange={onChange} />);
        fireEvent.click(screen.getByRole("combobox", { name: /Target/i }));
        fireEvent.click(screen.getByText(/Explicit/i));
        expect(screen.getByLabelText(/Chat ID/i)).toBeInTheDocument();
      });
      it("appends a callback action and serializes it into config.card.actions", () => {
        const onChange = jest.fn();
        render(<IMSendConfig config={{}} onChange={onChange} />);
        fireEvent.click(screen.getByRole("button", { name: /Add action/i }));
        const idInput = screen.getByPlaceholderText(/Action id/i);
        fireEvent.change(idInput, { target: { value: "approve" } });
        const last = onChange.mock.calls.at(-1)![0];
        expect(last.card.actions[0].id).toBe("approve");
      });
    });
    ```
  - Run: fails.

- [x] Step 7.4 — implement im-send-config.tsx
  - File: `components/workflow-editor/config-panel/node-configs/im-send-config.tsx`
    ```tsx
    "use client";

    import { useMemo } from "react";
    import { Plus, Trash2 } from "lucide-react";
    import { Input } from "@/components/ui/input";
    import { Textarea } from "@/components/ui/textarea";
    import { Label } from "@/components/ui/label";
    import { Button } from "@/components/ui/button";
    import {
      Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
    } from "@/components/ui/select";

    type Target = "reply_to_trigger" | "explicit";
    type ActionType = "url" | "callback";

    interface CardAction {
      id: string;
      label: string;
      style?: "primary" | "danger" | "default";
      type: ActionType;
      url?: string;
      payload?: Record<string, unknown>;
    }

    interface CardConfig {
      title?: string;
      status?: "success" | "failed" | "running" | "pending" | "info";
      summary?: string;
      fields?: Array<{ label: string; value: string }>;
      actions?: CardAction[];
      footer?: string;
    }

    interface Props {
      config: Record<string, unknown>;
      onChange: (c: Record<string, unknown>) => void;
    }

    const DATASTORE_HELP = "Templating: {{$dataStore.<nodeId>.<field>}} resolves at execution time.";

    export function IMSendConfig({ config, onChange }: Props) {
      const target = (config.target as Target | undefined) ?? "reply_to_trigger";
      const explicit = (config.explicit_target as Record<string, string> | undefined) ?? {};
      const card = useMemo<CardConfig>(() => (config.card as CardConfig | undefined) ?? {}, [config.card]);

      function update(patch: Record<string, unknown>) {
        onChange({ ...config, ...patch });
      }
      function updateCard(patch: Partial<CardConfig>) {
        update({ card: { ...card, ...patch } });
      }
      function updateExplicit(patch: Record<string, string>) {
        update({ explicit_target: { ...explicit, ...patch } });
      }

      const actions = card.actions ?? [];
      const fields = card.fields ?? [];

      return (
        <div className="flex flex-col gap-4">
          <div className="flex flex-col gap-1.5">
            <Label className="text-xs">Target</Label>
            <Select value={target} onValueChange={(v) => update({ target: v })}>
              <SelectTrigger aria-label="Target"><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="reply_to_trigger">Reply to triggering message</SelectItem>
                <SelectItem value="explicit">Explicit chat / thread</SelectItem>
              </SelectContent>
            </Select>
          </div>

          {target === "explicit" && (
            <div className="grid grid-cols-2 gap-2">
              <div className="flex flex-col gap-1.5">
                <Label className="text-xs">Provider</Label>
                <Input value={explicit.provider ?? ""}
                  onChange={(e) => updateExplicit({ provider: e.target.value })} placeholder="feishu" />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label className="text-xs">Chat ID</Label>
                <Input value={explicit.chat_id ?? ""}
                  onChange={(e) => updateExplicit({ chat_id: e.target.value })} placeholder="oc_xxx" />
              </div>
              <div className="flex flex-col gap-1.5 col-span-2">
                <Label className="text-xs">Thread ID (optional)</Label>
                <Input value={explicit.thread_id ?? ""}
                  onChange={(e) => updateExplicit({ thread_id: e.target.value })} />
              </div>
            </div>
          )}

          <div className="border-t pt-3 flex flex-col gap-3">
            <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Card</p>
            <div className="flex flex-col gap-1.5">
              <Label className="text-xs">Title</Label>
              <Input value={card.title ?? ""} onChange={(e) => updateCard({ title: e.target.value })} />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label className="text-xs">Status</Label>
              <Select value={card.status ?? "info"}
                onValueChange={(v) => updateCard({ status: v as CardConfig["status"] })}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  {["info", "success", "failed", "running", "pending"].map((s) => (
                    <SelectItem key={s} value={s}>{s}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="flex flex-col gap-1.5">
              <Label className="text-xs">Summary</Label>
              <Textarea rows={3} value={card.summary ?? ""}
                onChange={(e) => updateCard({ summary: e.target.value })} />
              <p className="text-[11px] text-muted-foreground">{DATASTORE_HELP}</p>
            </div>

            {/* Fields editor */}
            <div className="flex flex-col gap-1.5">
              <Label className="text-xs">Fields</Label>
              {fields.map((f, i) => (
                <div key={i} className="flex gap-1.5">
                  <Input className="flex-1" value={f.label} placeholder="Label"
                    onChange={(e) => {
                      const next = fields.slice(); next[i] = { ...f, label: e.target.value };
                      updateCard({ fields: next });
                    }} />
                  <Input className="flex-1" value={f.value} placeholder="Value"
                    onChange={(e) => {
                      const next = fields.slice(); next[i] = { ...f, value: e.target.value };
                      updateCard({ fields: next });
                    }} />
                  <Button variant="ghost" size="icon"
                    onClick={() => updateCard({ fields: fields.filter((_, idx) => idx !== i) })}>
                    <Trash2 className="h-3.5 w-3.5" />
                  </Button>
                </div>
              ))}
              <Button variant="ghost" size="sm" className="self-start"
                onClick={() => updateCard({ fields: [...fields, { label: "", value: "" }] })}>
                <Plus className="mr-1 h-3.5 w-3.5" /> Add field
              </Button>
            </div>

            {/* Actions editor */}
            <div className="flex flex-col gap-1.5">
              <Label className="text-xs">Actions (buttons)</Label>
              {actions.map((a, i) => (
                <div key={i} className="border rounded p-2 flex flex-col gap-1.5">
                  <div className="flex gap-1.5">
                    <Input className="flex-1" placeholder="Action id (e.g. approve)" value={a.id}
                      onChange={(e) => {
                        const next = actions.slice(); next[i] = { ...a, id: e.target.value };
                        updateCard({ actions: next });
                      }} />
                    <Input className="flex-1" placeholder="Label" value={a.label}
                      onChange={(e) => {
                        const next = actions.slice(); next[i] = { ...a, label: e.target.value };
                        updateCard({ actions: next });
                      }} />
                    <Button variant="ghost" size="icon"
                      onClick={() => updateCard({ actions: actions.filter((_, idx) => idx !== i) })}>
                      <Trash2 className="h-3.5 w-3.5" />
                    </Button>
                  </div>
                  <div className="flex gap-1.5">
                    <Select value={a.type} onValueChange={(v) => {
                      const next = actions.slice(); next[i] = { ...a, type: v as ActionType };
                      updateCard({ actions: next });
                    }}>
                      <SelectTrigger className="flex-1"><SelectValue /></SelectTrigger>
                      <SelectContent>
                        <SelectItem value="callback">Callback</SelectItem>
                        <SelectItem value="url">URL</SelectItem>
                      </SelectContent>
                    </Select>
                    {a.type === "url" ? (
                      <Input className="flex-2" placeholder="https://…" value={a.url ?? ""}
                        onChange={(e) => {
                          const next = actions.slice(); next[i] = { ...a, url: e.target.value };
                          updateCard({ actions: next });
                        }} />
                    ) : (
                      <Input className="flex-2" placeholder='Payload JSON: {"k":"v"}'
                        value={a.payload ? JSON.stringify(a.payload) : ""}
                        onChange={(e) => {
                          const next = actions.slice();
                          try {
                            next[i] = { ...a, payload: JSON.parse(e.target.value || "{}") };
                          } catch {
                            // keep stale; user is mid-typing
                            return;
                          }
                          updateCard({ actions: next });
                        }} />
                    )}
                  </div>
                </div>
              ))}
              <Button variant="ghost" size="sm" className="self-start"
                onClick={() => updateCard({
                  actions: [...actions, { id: "", label: "", type: "callback", payload: {} }],
                })}>
                <Plus className="mr-1 h-3.5 w-3.5" /> Add action
              </Button>
            </div>

            <div className="flex flex-col gap-1.5">
              <Label className="text-xs">Footer (optional)</Label>
              <Input value={card.footer ?? ""} onChange={(e) => updateCard({ footer: e.target.value })} />
            </div>
          </div>
        </div>
      );
    }
    ```
  - Run im-send-config tests — green.

- [x] Step 7.5 — register node icons + types in `nodes/node-types.tsx`
  - Add icons + node types:
    ```tsx
    import { Globe, MessageSquare } from "lucide-react"; // add to existing import block

    // NODE_ICONS map — append:
    http_call: Globe,
    im_send: MessageSquare,
    ```
  - Below the existing memo blocks, append:
    ```tsx
    export const HTTPCallNode = memo(function HTTPCallNode(props: NodeProps) {
      return <BaseWorkflowNode data={props.data as unknown as WorkflowNodeBase} nodeType="http_call" selected={props.selected} />;
    });
    export const IMSendNode = memo(function IMSendNode(props: NodeProps) {
      return <BaseWorkflowNode data={props.data as unknown as WorkflowNodeBase} nodeType="im_send" selected={props.selected} />;
    });
    ```
  - Add to `workflowNodeTypes` and `NODE_TYPE_LABELS`:
    ```tsx
    http_call: HTTPCallNode,
    im_send: IMSendNode,
    // labels:
    http_call: "HTTP Call",
    im_send: "IM Send",
    ```
  - Add a matching style entry in `nodes/node-styles.ts`. Mirror the `notification` entry's shape; recommended colors: `#0ea5e9` for http_call, `#14b8a6` for im_send.

- [x] Step 7.6 — register palette entries in `nodes/node-registry.ts`
  - In the `// ── Action ──` section append:
    ```ts
    {
      type: "http_call",
      label: "HTTP Call",
      category: "action",
      icon: Globe,
      color: "#0ea5e9",
      description: "Call an external HTTP API; supports {{secrets.X}} in url, headers, query, body",
      configSchema: [], // custom override below
      defaultConfig: { method: "GET", timeout_seconds: 30 },
    },
    {
      type: "im_send",
      label: "IM Send",
      category: "action",
      icon: MessageSquare,
      color: "#14b8a6",
      description: "Send a rich card with action buttons that can resume wait_event nodes",
      configSchema: [],
      defaultConfig: { target: "reply_to_trigger", card: { title: "" } },
    },
    ```
  - And import `Globe, MessageSquare` at the top.
- [x] Step 7.7 — wire custom override in `node-config-panel.tsx`
  - Extend the `hasCustomOverride` check + the conditional render:
    ```tsx
    const hasCustomOverride =
      node.type === "llm_agent" ||
      node.type === "condition" ||
      node.type === "sub_workflow" ||
      node.type === "http_call" ||
      node.type === "im_send";
    ```
  - And inside the `<AccordionContent>`:
    ```tsx
    {node.type === "http_call" && (
      <HTTPCallConfig config={config} onChange={handleConfigChange} />
    )}
    {node.type === "im_send" && (
      <IMSendConfig config={config} onChange={handleConfigChange} />
    )}
    ```
  - Add the imports at the top:
    ```tsx
    import { HTTPCallConfig } from "./node-configs/http-call-config";
    import { IMSendConfig } from "./node-configs/im-send-config";
    ```
  - Run all FE node-config tests + the existing palette test — green: `rtk vitest run components/workflow-editor`.

---

## Task E8 — Integration test + E2E smoke fixture (Trace B)

- [x] Step 8.1 — Go integration: end-to-end Trace B with real DB and stub IM Bridge
  - File: `src-go/internal/service/dag_workflow_card_action_e2e_test.go`
    ```go
    package service

    import (
        "context"
        "encoding/json"
        "net/http"
        "net/http/httptest"
        "testing"

        "github.com/google/uuid"
        "github.com/react-go-quick-starter/server/internal/imcards"
        "github.com/react-go-quick-starter/server/internal/model"
        "github.com/react-go-quick-starter/server/internal/repository"
        "github.com/react-go-quick-starter/server/internal/testutil"
        "github.com/react-go-quick-starter/server/internal/workflow/nodetypes"
    )

    // TestTraceB exercises the full chain documented in spec §9 Trace B:
    //   trigger → http_call (with {{secrets.X}}) → im_send (callback button)
    //   → wait_event parks → POST /api/v1/im/card-actions
    //   → wait_event resumes → continue → end (im_dispatched=true so the
    //   default outbound dispatcher does not fire a second card).
    func TestTraceB_HTTPThenIMSendThenButtonResume(t *testing.T) {
        db := testutil.PostgresForTest(t)
        // Stub external HTTP target — returns 200 + a small JSON body so the
        // http_call node has something to expose via dataStore.
        ext := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            if r.Header.Get("Authorization") != "Bearer tok-fixture" {
                w.WriteHeader(http.StatusUnauthorized); return
            }
            w.Header().Set("Content-Type", "application/json")
            _, _ = w.Write([]byte(`{"pr":42,"merged":false}`))
        }))
        defer ext.Close()

        // Stub IM Bridge — captures the dispatched card so we can extract the
        // correlation_token the im_send applier minted.
        var dispatchedCard json.RawMessage
        bridge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            body, _ := io.ReadAll(r.Body)
            dispatchedCard = json.RawMessage(body)
            _, _ = w.Write([]byte(`{"message_id":"m-1"}`))
        }))
        defer bridge.Close()

        // Wire the system end-to-end: registry, applier, services, repos.
        // (Full assembly omitted here for brevity; mirror the assembly used
        // by dag_workflow_service_test.go's TestStartExecution_HappyPath but
        // additionally inject SecretResolver, IMSendDispatcher, etc.)
        env := buildTraceBEnv(t, db, ext.URL, bridge.URL)

        // 1. Create execution with reply_target preloaded into system_metadata.
        execID := env.startExecutionFor(t, "trace-b-fixture")

        // 2. Drive http_call + im_send nodes.
        if err := env.dagSvc.AdvanceExecution(context.Background(), execID); err != nil {
            t.Fatalf("AdvanceExecution: %v", err)
        }

        // Assert the card was dispatched and contains a correlation_token.
        if len(dispatchedCard) == 0 { t.Fatal("no card dispatched to bridge") }
        var card map[string]any
        _ = json.Unmarshal(dispatchedCard, &card)
        action := card["actions"].([]any)[0].(map[string]any)
        token, _ := action["correlation_token"].(string)
        if token == "" { t.Fatal("missing correlation_token in dispatched card") }

        // 3. Simulate the user clicking Approve.
        out, err := env.router.Route(context.Background(), imcards.RouteInput{
            Token: uuid.MustParse(token), ActionID: "approve",
            UserID: "U1", TenantID: "T1",
        })
        if err != nil { t.Fatalf("Route: %v", err) }
        if out.Outcome != imcards.OutcomeResumed {
            t.Fatalf("outcome = %s", out.Outcome)
        }

        // 4. Final execution status must be 'completed' and im_dispatched stays true.
        finalExec, _ := env.execRepo.GetExecution(context.Background(), execID)
        if finalExec.Status != model.WorkflowExecStatusCompleted {
            t.Errorf("final status = %s", finalExec.Status)
        }
        var meta map[string]any
        _ = json.Unmarshal(finalExec.SystemMetadata, &meta)
        if meta["im_dispatched"] != true {
            t.Error("im_dispatched not set; default outbound dispatcher would double-fire")
        }
    }
    ```
  - The `buildTraceBEnv(...)` helper assembles registry + repos + applier + DAG svc + router pointing at the test DB. Mirror the wiring pattern already used in `dag_workflow_service_test.go`.

- [x] Step 8.2 — IM Bridge smoke fixture
  - File: `src-im-bridge/scripts/smoke/fixtures/feishu-workflow-button-resume.json`
    ```json
    {
      "name": "Trace B: workflow with HTTP + IM card + button resume",
      "scenario": [
        {
          "kind": "im_event",
          "platform": "feishu",
          "command": "/build",
          "expect": { "card_dispatched": true, "card_has_correlation_token": true }
        },
        {
          "kind": "card_action_callback",
          "platform": "feishu",
          "use_token_from": "previous.card.actions[0].correlation_token",
          "action_id": "approve",
          "expect": {
            "backend_status": 200,
            "execution_completed": true,
            "outbound_default_skipped": true
          }
        },
        {
          "kind": "card_action_callback_replay",
          "comment": "Replays the previous click; backend should respond 409 with code card_action:consumed",
          "expect": { "backend_status": 409, "code": "card_action:consumed" }
        }
      ]
    }
    ```
  - Add a runner stub note in `Invoke-StubSmoke.ps1` referencing the new fixture (no behavior change required if the runner already iterates the directory).

---

## Self-review pass (run before declaring complete)

- [ ] Re-read the **Coordination notes** at the top. Confirm:
  - The `ProviderNeutralCard` JSON tags in `ExecuteIMSendPayload.RawCard` exactly match `src-im-bridge/core/card_schema.ts` (camelCase). If 1D landed `snake_case` instead, swap accordingly in E5 and E7.
  - `secret_resolver.Resolve` signature matches 1B's actual export (4 args, returns `(string, error)`). If 1B chose a different shape, refactor `applyExecuteHTTPCall` and `stubSecretResolver` to match.
  - The `system_metadata` migration was added by exactly one of 1D / this plan — never both.
- [ ] Run the full Go test suite: `rtk cargo test -p workflow/nodetypes -p service -p handler -p imcards -p repository`.
- [ ] Run frontend tests: `rtk vitest run components/workflow-editor`.
- [ ] Run IM Bridge tests: `rtk bun test src-im-bridge/platform/feishu`.
- [ ] Verify zero new lint errors: `rtk lint` and `rtk tsc --noEmit`.
- [ ] Manually trace through spec §11 Security: ensure no plaintext secret value can appear in audit payloads, error messages bubbled to the IM Bridge, or dataStore writes.
- [ ] Confirm spec §10 error matrix: 410 expired / 409 consumed / 409 not_waiting / 200 fallback all observable in `im_card_actions_handler_test.go`.
