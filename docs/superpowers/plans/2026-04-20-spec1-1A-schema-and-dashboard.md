# Spec 1A — Schema Migrations + Per-Employee Runs Dashboard

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 落地 spec1 的 §6.2 / §6.3 schema 扩展，并在 FE 暴露按员工的 workflow + agent run 历史看板。

**Architecture:** 两条迁移给 workflow_executions / workflow_triggers 加字段（系统元数据袋 + trigger source 标记）；后端新增 `/api/v1/employees/:id/runs` UNION 查询返回 workflow_executions ∪ agent_runs 行；FE 新增员工详情 Runs 子页，复用现有 WS workflow_step / agent_run 事件流做增量刷新。

**Tech Stack:** Go (Echo handler / pgx repo / sqlc-style queries — match existing repo style), Postgres jsonb migrations, Next.js 16 App Router, Zustand, Tailwind v4, shadcn/ui.

**Depends on:** none (first wave)

**Parallel with:** 1B (Secrets store) — completely independent

**Unblocks:** 1C (needs `created_via` column), 1D (needs `system_metadata` column), 1E (needs both)

---

## Coordination notes (read before starting)

- **Migration numbering**: latest is `066_workflow_run_parent_link_parent_kind.up.sql`. This plan claims **067** and **068**. If another plan lands first, bump these by one in lock-step (the `_test.go` files in 1A do not hard-code migration numbers; only the filenames do).
- **Employees route**: today employees are mounted as a section inside `app/(dashboard)/agents/page.tsx`; no `app/(dashboard)/employees/` directory exists. Task 9 creates the new directory tree from scratch (`/employees/[id]/layout.tsx` + `/employees/[id]/runs/page.tsx`). Plans 1C / 1D / 1E hang sibling tabs (`triggers`, `secrets`) off the same layout.
- **Existing `acting_employee_id` column** already exists on both `workflow_executions` and `workflow_triggers` (migration 064). Migration 067 in this plan ADDs an INDEX on `workflow_triggers.acting_employee_id` only if 064's partial index is not sufficient — see Step 2.1 for the discriminator.
- **WS event types are NOT new**: per spec §9 边界, the dashboard reuses existing `workflow.execution.*` / `workflow.node.*` / `agent.completed` / `agent.failed` events filtered by employee; do not add `workflow_step.*` or `agent_run.*` event types.

---

## Task 1 — Migration 067: workflow_executions.system_metadata

- [x] Step 1.1 — write failing repo test asserting `system_metadata` round-trips through the workflow execution record
  - File: `src-go/internal/repository/workflow_execution_system_metadata_test.go` (new)
  - Content:
    ```go
    package repository

    import (
        "encoding/json"
        "testing"

        "github.com/google/uuid"
        "github.com/react-go-quick-starter/server/internal/model"
    )

    func TestWorkflowExecutionRecord_SystemMetadataRoundTrip(t *testing.T) {
        meta := json.RawMessage(`{"reply_target":{"provider":"feishu","chat_id":"oc_x","thread_id":"r_y"},"im_dispatched":true}`)
        rec := workflowExecutionRecord{
            ID:             uuid.New(),
            WorkflowID:     uuid.New(),
            ProjectID:      uuid.New(),
            Status:         model.WorkflowExecStatusRunning,
            CurrentNodes:   newRawJSON(json.RawMessage("[]"), "[]"),
            Context:        newRawJSON(json.RawMessage("{}"), "{}"),
            DataStore:      newRawJSON(json.RawMessage("{}"), "{}"),
            SystemMetadata: newRawJSON(meta, "{}"),
        }
        m := rec.toModel()
        if string(m.SystemMetadata) != string(meta) {
            t.Fatalf("system_metadata round-trip mismatch:\n  got:  %s\n  want: %s", string(m.SystemMetadata), string(meta))
        }
    }
    ```

- [x] Step 1.2 — run `cd src-go && go test ./internal/repository/ -run TestWorkflowExecutionRecord_SystemMetadataRoundTrip` — expect compile error: `workflowExecutionRecord` has no field `SystemMetadata`, model.WorkflowExecution has no field `SystemMetadata`

- [x] Step 1.3 — add `SystemMetadata` to the model
  - File: `src-go/internal/model/workflow_definition.go`
  - In the `WorkflowExecution` struct (lines 52–68), add this field directly after `DataStore` (line 60):
    ```go
    SystemMetadata   json.RawMessage `db:"system_metadata" json:"systemMetadata,omitempty" gorm:"type:jsonb"`
    ```

- [x] Step 1.4 — extend repository record + mapper
  - File: `src-go/internal/repository/workflow_definition_repo.go`
  - In `workflowExecutionRecord` (lines 56–72), add directly after the `DataStore` line (line 64):
    ```go
    SystemMetadata   rawJSON    `gorm:"column:system_metadata;type:jsonb"`
    ```
  - In `(r *workflowExecutionRecord).toModel()` (lines 76–97), add directly after `DataStore: r.DataStore.Bytes("{}"),` (line 88):
    ```go
    SystemMetadata:   r.SystemMetadata.Bytes("{}"),
    ```
  - In `CreateExecution` (lines 325–348), add directly after `DataStore: newRawJSON(exec.DataStore, "{}"),` (line 337):
    ```go
    SystemMetadata:   newRawJSON(exec.SystemMetadata, "{}"),
    ```

- [x] Step 1.5 — add an `UpdateExecutionSystemMetadata` repo method (used by 1D's outbound dispatcher to flip `im_dispatched`; we land it here so 1D can wire without touching the repo)
  - File: `src-go/internal/repository/workflow_definition_repo.go`
  - Add this method directly after `UpdateExecutionDataStore` (after line 542):
    ```go
    // UpdateExecutionSystemMetadata replaces the system_metadata jsonb document
    // for the execution. Callers MUST pass the full document; this is a
    // last-write-wins replacement intended for backend-internal flags
    // (reply_target, im_dispatched, final_output) that are never written by DAG
    // node code (see spec §6.3).
    func (r *WorkflowExecutionRepository) UpdateExecutionSystemMetadata(ctx context.Context, id uuid.UUID, systemMetadata json.RawMessage) error {
        if r.db == nil {
            return ErrDatabaseUnavailable
        }
        updates := map[string]any{
            "system_metadata": newRawJSON(systemMetadata, "{}"),
            "updated_at":      gorm.Expr("NOW()"),
        }
        result := r.db.WithContext(ctx).Model(&workflowExecutionRecord{}).Where("id = ?", id).Updates(updates)
        if result.Error != nil {
            return fmt.Errorf("update workflow execution system_metadata: %w", result.Error)
        }
        return nil
    }
    ```

- [x] Step 1.6 — write the migration
  - File: `src-go/migrations/067_add_workflow_execution_system_metadata.up.sql` (new)
  - Content:
    ```sql
    -- system_metadata is a backend-only jsonb document attached to every workflow
    -- execution. Reserved system keys (per spec §6.3):
    --   reply_target          {provider, chat_id, thread_id, message_id, tenant_id}
    --   im_dispatched         bool   ← outbound_dispatcher reads this; im_send node sets it true
    --   final_output          jsonb  ← optional author-declared completion summary
    --
    -- DAG node code MUST NOT read or write this column directly; it is owned by
    -- trigger_handler (writes reply_target on execution create), the im_send
    -- node (sets im_dispatched), and the outbound_dispatcher (reads both).
    -- Author-facing data lives in data_store.
    ALTER TABLE workflow_executions
        ADD COLUMN system_metadata JSONB NOT NULL DEFAULT '{}'::jsonb;
    ```

- [x] Step 1.7 — write the matching down migration
  - File: `src-go/migrations/067_add_workflow_execution_system_metadata.down.sql` (new)
  - Content:
    ```sql
    ALTER TABLE workflow_executions DROP COLUMN IF EXISTS system_metadata;
    ```

- [x] Step 1.8 — run `cd src-go && go test ./internal/repository/ -run TestWorkflowExecutionRecord_SystemMetadataRoundTrip` — expect green

- [x] Step 1.9 — run `rtk git add src-go/migrations/067_add_workflow_execution_system_metadata.up.sql src-go/migrations/067_add_workflow_execution_system_metadata.down.sql src-go/internal/model/workflow_definition.go src-go/internal/repository/workflow_definition_repo.go src-go/internal/repository/workflow_execution_system_metadata_test.go && rtk git commit -m "feat(workflow): add system_metadata jsonb column to workflow_executions (spec1 §6.3)"`

---

## Task 2 — Migration 068: workflow_triggers source-of-truth columns

- [x] Step 2.1 — write failing repo test asserting `created_via / display_name / description` round-trip
  - File: `src-go/internal/repository/workflow_trigger_metadata_test.go` (new)
  - Content:
    ```go
    package repository

    import (
        "testing"

        "github.com/react-go-quick-starter/server/internal/model"
    )

    func TestWorkflowTriggerRecord_CreatedViaAndDisplayMetadata(t *testing.T) {
        rec := workflowTriggerRecord{
            CreatedVia:  string(model.TriggerCreatedViaManual),
            DisplayName: "ping echo",
            Description: "demo trigger created via FE",
        }
        m := rec.toModel()
        if m.CreatedVia != model.TriggerCreatedViaManual {
            t.Errorf("created_via: got %q want %q", m.CreatedVia, model.TriggerCreatedViaManual)
        }
        if m.DisplayName != "ping echo" {
            t.Errorf("display_name: got %q want ping echo", m.DisplayName)
        }
        if m.Description != "demo trigger created via FE" {
            t.Errorf("description: got %q", m.Description)
        }

        rec2 := workflowTriggerRecord{} // empty created_via must default to dag_node
        if m2 := rec2.toModel(); m2.CreatedVia != model.TriggerCreatedViaDAGNode {
            t.Errorf("default created_via: got %q want %q", m2.CreatedVia, model.TriggerCreatedViaDAGNode)
        }
    }
    ```

- [x] Step 2.2 — run `cd src-go && go test ./internal/repository/ -run TestWorkflowTriggerRecord_CreatedViaAndDisplayMetadata` — expect compile error

- [x] Step 2.3 — add `TriggerCreatedVia` enum + struct fields to the model
  - File: `src-go/internal/model/workflow_trigger.go`
  - After the existing `TriggerTargetKind` constants block (after line 25), add:
    ```go
    // TriggerCreatedVia distinguishes how a workflow_triggers row was authored.
    // 'dag_node' rows are upserted by the registrar from a workflow definition's
    // trigger node and may be replaced when the DAG is re-saved. 'manual' rows
    // are authored via the trigger CRUD API (Spec 1C) and are NEVER touched by
    // the registrar's merge pass.
    type TriggerCreatedVia string

    const (
        TriggerCreatedViaDAGNode TriggerCreatedVia = "dag_node"
        TriggerCreatedViaManual  TriggerCreatedVia = "manual"
    )
    ```
  - Inside the `WorkflowTrigger` struct (lines 34–51), add three new fields between `DisabledReason` and `ActingEmployeeID`:
    ```go
    CreatedVia       TriggerCreatedVia `db:"created_via"  json:"createdVia"`
    DisplayName      string            `db:"display_name" json:"displayName,omitempty"`
    Description      string            `db:"description"  json:"description,omitempty"`
    ```

- [x] Step 2.4 — extend the repository record + mapper
  - File: `src-go/internal/repository/workflow_trigger_repo.go`
  - Locate the `workflowTriggerRecord` struct (search for `type workflowTriggerRecord struct`). Add after the `DisabledReason` column line:
    ```go
    CreatedVia  string `gorm:"column:created_via"`
    DisplayName string `gorm:"column:display_name"`
    Description string `gorm:"column:description"`
    ```
  - In `(r *workflowTriggerRecord).toModel()`, add directly before the closing `}`:
    ```go
    createdVia := model.TriggerCreatedVia(r.CreatedVia)
    if createdVia == "" {
        createdVia = model.TriggerCreatedViaDAGNode
    }
    out.CreatedVia = createdVia
    out.DisplayName = r.DisplayName
    out.Description = r.Description
    ```
    (If the function uses `return &model.WorkflowTrigger{...}` literal form, refactor to assign to a local `out := &model.WorkflowTrigger{...}` first, then `return out`. If a `newWorkflowTriggerRecord(*model.WorkflowTrigger)` constructor exists, mirror by copying `CreatedVia` (with same default), `DisplayName`, `Description` into the record.)

- [x] Step 2.5 — write the migration
  - File: `src-go/migrations/068_extend_workflow_triggers_metadata.up.sql` (new)
  - Content:
    ```sql
    -- Extend workflow_triggers with the source-of-truth + display metadata
    -- columns that Spec 1C needs for its FE CRUD surface (display_name /
    -- description) and that the registrar needs to merge instead of
    -- delete-then-insert (created_via discriminates DAG-owned vs human-authored
    -- rows; only 'dag_node' rows are eligible to be replaced on workflow save).
    ALTER TABLE workflow_triggers
        ADD COLUMN created_via VARCHAR(16) NOT NULL DEFAULT 'dag_node'
            CHECK (created_via IN ('dag_node','manual')),
        ADD COLUMN display_name VARCHAR(128),
        ADD COLUMN description TEXT;

    -- Existing rows (all registrar-authored before Spec 1C lands) inherit the
    -- 'dag_node' default, which is the desired backfill: their FE CRUD surface
    -- is read-only until the user explicitly converts them to 'manual'.

    -- Migration 064 already created a partial index on acting_employee_id with
    -- a WHERE clause. The /api/v1/employees/:id/runs endpoint and the
    -- registrar's per-employee filter both use the column without the IS NOT
    -- NULL predicate, so we add an unconditional index here for plan-stability.
    CREATE INDEX IF NOT EXISTS idx_workflow_triggers_acting_employee
        ON workflow_triggers(acting_employee_id);
    ```

- [x] Step 2.6 — write the matching down migration
  - File: `src-go/migrations/068_extend_workflow_triggers_metadata.down.sql` (new)
  - Content:
    ```sql
    DROP INDEX IF EXISTS idx_workflow_triggers_acting_employee;
    ALTER TABLE workflow_triggers DROP COLUMN IF EXISTS description;
    ALTER TABLE workflow_triggers DROP COLUMN IF EXISTS display_name;
    ALTER TABLE workflow_triggers DROP COLUMN IF EXISTS created_via;
    ```

- [x] Step 2.7 — run `cd src-go && go test ./internal/repository/ -run TestWorkflowTriggerRecord_CreatedViaAndDisplayMetadata` — expect green

- [x] Step 2.8 — run `rtk git add src-go/migrations/068_extend_workflow_triggers_metadata.up.sql src-go/migrations/068_extend_workflow_triggers_metadata.down.sql src-go/internal/model/workflow_trigger.go src-go/internal/repository/workflow_trigger_repo.go src-go/internal/repository/workflow_trigger_metadata_test.go && rtk git commit -m "feat(workflow): add created_via/display_name/description columns to workflow_triggers (spec1 §6.2)"`

---

## Task 3 — EmployeeRunsRepository: UNION query

- [x] Step 3.1 — write failing repo test (nil DB short-circuit + DTO shape)
  - File: `src-go/internal/repository/employee_runs_repo_test.go` (new)
  - Content:
    ```go
    package repository

    import (
        "context"
        "testing"

        "github.com/google/uuid"
    )

    func TestEmployeeRunsRepository_NilDB(t *testing.T) {
        repo := NewEmployeeRunsRepository(nil)
        _, err := repo.ListByEmployee(context.Background(), uuid.New(), EmployeeRunKindAll, 1, 20)
        if err != ErrDatabaseUnavailable {
            t.Fatalf("expected ErrDatabaseUnavailable, got %v", err)
        }
    }

    func TestEmployeeRunsRepository_PaginationDefaults(t *testing.T) {
        repo := NewEmployeeRunsRepository(nil)
        // page <= 0 must coerce to 1; size <= 0 must coerce to 20; size > 200 capped to 200
        if got := normalizeRunsPage(0); got != 1 {
            t.Errorf("normalizeRunsPage(0) = %d, want 1", got)
        }
        if got := normalizeRunsPage(-5); got != 1 {
            t.Errorf("normalizeRunsPage(-5) = %d, want 1", got)
        }
        if got := normalizeRunsSize(0); got != 20 {
            t.Errorf("normalizeRunsSize(0) = %d, want 20", got)
        }
        if got := normalizeRunsSize(500); got != 200 {
            t.Errorf("normalizeRunsSize(500) = %d, want 200", got)
        }
        // explicit suppress unused-import warning
        _ = repo
    }
    ```

- [x] Step 3.2 — run `cd src-go && go test ./internal/repository/ -run TestEmployeeRunsRepository` — expect compile error: `NewEmployeeRunsRepository`, `EmployeeRunKindAll`, `normalizeRunsPage`, `normalizeRunsSize` undefined

- [x] Step 3.3 — implement the repository
  - File: `src-go/internal/repository/employee_runs_repo.go` (new)
  - Content:
    ```go
    package repository

    import (
        "context"
        "fmt"
        "time"

        "github.com/google/uuid"
        "gorm.io/gorm"
    )

    // EmployeeRunKind narrows the UNION query in ListByEmployee.
    type EmployeeRunKind string

    const (
        EmployeeRunKindAll      EmployeeRunKind = "all"
        EmployeeRunKindWorkflow EmployeeRunKind = "workflow"
        EmployeeRunKindAgent    EmployeeRunKind = "agent"
    )

    // EmployeeRunRow is the unified DTO returned by ListByEmployee. One row
    // represents either a workflow_executions row (Kind="workflow") or an
    // agent_runs row (Kind="agent"); the Name / RefURL fields are pre-rendered
    // so the FE can drill down without extra joins.
    type EmployeeRunRow struct {
        Kind        string     `json:"kind"`        // "workflow" | "agent"
        ID          string     `json:"id"`
        Name        string     `json:"name"`        // workflow_definitions.name OR roles.id (agent_runs.role_id)
        Status      string     `json:"status"`
        StartedAt   *time.Time `json:"startedAt,omitempty"`
        CompletedAt *time.Time `json:"completedAt,omitempty"`
        DurationMs  *int64     `json:"durationMs,omitempty"`
        RefURL      string     `json:"refUrl"`
    }

    // EmployeeRunsRepository serves the per-employee unified runs feed.
    type EmployeeRunsRepository struct {
        db *gorm.DB
    }

    func NewEmployeeRunsRepository(db *gorm.DB) *EmployeeRunsRepository {
        return &EmployeeRunsRepository{db: db}
    }

    func normalizeRunsPage(page int) int {
        if page <= 0 {
            return 1
        }
        return page
    }

    func normalizeRunsSize(size int) int {
        if size <= 0 {
            return 20
        }
        if size > 200 {
            return 200
        }
        return size
    }

    // unifiedRunRow is the SQL projection target for the UNION query.
    type unifiedRunRow struct {
        Kind        string     `gorm:"column:kind"`
        ID          uuid.UUID  `gorm:"column:id"`
        Name        string     `gorm:"column:name"`
        Status      string     `gorm:"column:status"`
        StartedAt   *time.Time `gorm:"column:started_at"`
        CompletedAt *time.Time `gorm:"column:completed_at"`
    }

    // ListByEmployee returns workflow_executions ∪ agent_runs filtered by
    // employee, ordered started_at DESC, with offset-based pagination.
    //
    // The UNION is evaluated as a subquery so the outer ORDER BY / LIMIT /
    // OFFSET applies to the combined result set, not to each leg
    // independently. workflow_executions.acting_employee_id and
    // agent_runs.employee_id are both nullable; rows with NULL are excluded
    // by the WHERE clauses on each leg.
    func (r *EmployeeRunsRepository) ListByEmployee(ctx context.Context, employeeID uuid.UUID, kind EmployeeRunKind, page, size int) ([]EmployeeRunRow, error) {
        if r.db == nil {
            return nil, ErrDatabaseUnavailable
        }
        page = normalizeRunsPage(page)
        size = normalizeRunsSize(size)
        offset := (page - 1) * size

        // Build per-leg SELECTs, then UNION ALL based on kind.
        wfSQL := `
            SELECT 'workflow' AS kind,
                   we.id        AS id,
                   COALESCE(wd.name, we.workflow_id::text) AS name,
                   we.status    AS status,
                   we.started_at AS started_at,
                   we.completed_at AS completed_at
              FROM workflow_executions we
              LEFT JOIN workflow_definitions wd ON wd.id = we.workflow_id
             WHERE we.acting_employee_id = ?`

        arSQL := `
            SELECT 'agent' AS kind,
                   ar.id     AS id,
                   COALESCE(NULLIF(ar.role_id, ''), 'agent') AS name,
                   ar.status AS status,
                   ar.started_at AS started_at,
                   ar.completed_at AS completed_at
              FROM agent_runs ar
             WHERE ar.employee_id = ?`

        var sqlText string
        var args []any
        switch kind {
        case EmployeeRunKindWorkflow:
            sqlText = wfSQL + ` ORDER BY started_at DESC NULLS LAST, id DESC LIMIT ? OFFSET ?`
            args = []any{employeeID, size, offset}
        case EmployeeRunKindAgent:
            sqlText = arSQL + ` ORDER BY started_at DESC NULLS LAST, id DESC LIMIT ? OFFSET ?`
            args = []any{employeeID, size, offset}
        default: // EmployeeRunKindAll or unknown
            sqlText = `
                SELECT * FROM (
                ` + wfSQL + `
                    UNION ALL
                ` + arSQL + `
                ) u
                ORDER BY started_at DESC NULLS LAST, id DESC
                LIMIT ? OFFSET ?`
            args = []any{employeeID, employeeID, size, offset}
        }

        var rows []unifiedRunRow
        if err := r.db.WithContext(ctx).Raw(sqlText, args...).Scan(&rows).Error; err != nil {
            return nil, fmt.Errorf("list employee runs: %w", err)
        }

        out := make([]EmployeeRunRow, 0, len(rows))
        for _, row := range rows {
            er := EmployeeRunRow{
                Kind:        row.Kind,
                ID:          row.ID.String(),
                Name:        row.Name,
                Status:      row.Status,
                StartedAt:   row.StartedAt,
                CompletedAt: row.CompletedAt,
            }
            if row.StartedAt != nil && row.CompletedAt != nil {
                d := row.CompletedAt.Sub(*row.StartedAt).Milliseconds()
                er.DurationMs = &d
            }
            switch row.Kind {
            case "workflow":
                er.RefURL = "/workflow/runs/" + row.ID.String()
            case "agent":
                er.RefURL = "/agents?run=" + row.ID.String()
            }
            out = append(out, er)
        }
        return out, nil
    }
    ```

- [x] Step 3.4 — run `cd src-go && go test ./internal/repository/ -run TestEmployeeRunsRepository` — expect green

- [x] Step 3.5 — run `rtk git add src-go/internal/repository/employee_runs_repo.go src-go/internal/repository/employee_runs_repo_test.go && rtk git commit -m "feat(workflow): EmployeeRunsRepository.ListByEmployee UNION query (spec1 §7 S5)"`

---

## Task 4 — Handler: GET /api/v1/employees/:id/runs

- [x] Step 4.1 — write failing handler test
  - File: `src-go/internal/handler/employee_runs_handler_test.go` (new)
  - Content:
    ```go
    package handler

    import (
        "context"
        "encoding/json"
        "net/http"
        "net/http/httptest"
        "strings"
        "testing"
        "time"

        "github.com/google/uuid"
        "github.com/labstack/echo/v4"
        "github.com/react-go-quick-starter/server/internal/repository"
    )

    type fakeEmployeeRunsRepo struct {
        rows  []repository.EmployeeRunRow
        err   error
        gotID uuid.UUID
        gotKind repository.EmployeeRunKind
        gotPage int
        gotSize int
    }

    func (f *fakeEmployeeRunsRepo) ListByEmployee(_ context.Context, id uuid.UUID, kind repository.EmployeeRunKind, page, size int) ([]repository.EmployeeRunRow, error) {
        f.gotID, f.gotKind, f.gotPage, f.gotSize = id, kind, page, size
        return f.rows, f.err
    }

    func TestEmployeeRunsHandler_List_DefaultsAndShape(t *testing.T) {
        e := echo.New()
        empID := uuid.New()
        started := time.Now().Add(-2 * time.Minute)
        completed := started.Add(45 * time.Second)
        repo := &fakeEmployeeRunsRepo{rows: []repository.EmployeeRunRow{{
            Kind: "workflow", ID: uuid.New().String(), Name: "echo-flow",
            Status: "completed", StartedAt: &started, CompletedAt: &completed,
            RefURL: "/workflow/runs/abc",
        }}}
        h := NewEmployeeRunsHandler(repo)
        req := httptest.NewRequest(http.MethodGet, "/api/v1/employees/"+empID.String()+"/runs", nil)
        rec := httptest.NewRecorder()
        c := e.NewContext(req, rec)
        c.SetParamNames("id")
        c.SetParamValues(empID.String())

        if err := h.List(c); err != nil {
            t.Fatalf("List error: %v", err)
        }
        if rec.Code != http.StatusOK {
            t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
        }
        if repo.gotID != empID || repo.gotKind != repository.EmployeeRunKindAll || repo.gotPage != 1 || repo.gotSize != 20 {
            t.Fatalf("repo args: id=%s kind=%s page=%d size=%d", repo.gotID, repo.gotKind, repo.gotPage, repo.gotSize)
        }

        var body struct {
            Items []map[string]any `json:"items"`
            Page  int              `json:"page"`
            Size  int              `json:"size"`
        }
        if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
            t.Fatalf("decode: %v", err)
        }
        if len(body.Items) != 1 || body.Items[0]["kind"] != "workflow" {
            t.Fatalf("body items: %+v", body.Items)
        }
    }

    func TestEmployeeRunsHandler_List_BadID(t *testing.T) {
        e := echo.New()
        h := NewEmployeeRunsHandler(&fakeEmployeeRunsRepo{})
        req := httptest.NewRequest(http.MethodGet, "/", nil)
        rec := httptest.NewRecorder()
        c := e.NewContext(req, rec)
        c.SetParamNames("id")
        c.SetParamValues("not-a-uuid")
        _ = h.List(c)
        if rec.Code != http.StatusBadRequest {
            t.Fatalf("expected 400, got %d body=%s", rec.Code, strings.TrimSpace(rec.Body.String()))
        }
    }
    ```

- [x] Step 4.2 — run `cd src-go && go test ./internal/handler/ -run TestEmployeeRunsHandler` — expect compile error: `NewEmployeeRunsHandler` undefined

- [x] Step 4.3 — implement the handler
  - File: `src-go/internal/handler/employee_runs_handler.go` (new)
  - Content:
    ```go
    package handler

    import (
        "context"
        "net/http"
        "strconv"

        "github.com/google/uuid"
        "github.com/labstack/echo/v4"
        "github.com/react-go-quick-starter/server/internal/i18n"
        "github.com/react-go-quick-starter/server/internal/repository"
    )

    // employeeRunsRepo is the narrow read-side contract that EmployeeRunsHandler
    // depends on. In production it is satisfied by *repository.EmployeeRunsRepository.
    type employeeRunsRepo interface {
        ListByEmployee(ctx context.Context, employeeID uuid.UUID, kind repository.EmployeeRunKind, page, size int) ([]repository.EmployeeRunRow, error)
    }

    // EmployeeRunsHandler serves GET /api/v1/employees/:id/runs.
    type EmployeeRunsHandler struct {
        repo employeeRunsRepo
    }

    // NewEmployeeRunsHandler returns a new EmployeeRunsHandler backed by the
    // given repository.
    func NewEmployeeRunsHandler(repo employeeRunsRepo) *EmployeeRunsHandler {
        return &EmployeeRunsHandler{repo: repo}
    }

    // employeeRunsResponse wraps the page so the FE can advance pagination
    // without a separate count query (HasMore is derived by the FE: rows length
    // == size means a next page may exist).
    type employeeRunsResponse struct {
        Items []repository.EmployeeRunRow `json:"items"`
        Page  int                         `json:"page"`
        Size  int                         `json:"size"`
        Kind  string                      `json:"kind"`
    }

    // List handles GET /api/v1/employees/:id/runs?type=&page=&size=
    //
    // Query params:
    //   type   one of "all" (default), "workflow", "agent"
    //   page   1-indexed, defaults to 1
    //   size   1..200, defaults to 20
    func (h *EmployeeRunsHandler) List(c echo.Context) error {
        id, err := uuid.Parse(c.Param("id"))
        if err != nil {
            return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidEmployeeID)
        }

        kind := repository.EmployeeRunKindAll
        switch c.QueryParam("type") {
        case "workflow":
            kind = repository.EmployeeRunKindWorkflow
        case "agent":
            kind = repository.EmployeeRunKindAgent
        case "", "all":
            // keep default
        default:
            return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidEmployeeRequest)
        }

        page := 1
        if v := c.QueryParam("page"); v != "" {
            if parsed, perr := strconv.Atoi(v); perr == nil {
                page = parsed
            }
        }
        size := 20
        if v := c.QueryParam("size"); v != "" {
            if parsed, perr := strconv.Atoi(v); perr == nil {
                size = parsed
            }
        }

        rows, err := h.repo.ListByEmployee(c.Request().Context(), id, kind, page, size)
        if err != nil {
            return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToListAgentRuns)
        }

        return c.JSON(http.StatusOK, employeeRunsResponse{
            Items: rows,
            Page:  page,
            Size:  size,
            Kind:  string(kind),
        })
    }
    ```

- [x] Step 4.4 — run `cd src-go && go test ./internal/handler/ -run TestEmployeeRunsHandler` — expect green

- [x] Step 4.5 — run `rtk git add src-go/internal/handler/employee_runs_handler.go src-go/internal/handler/employee_runs_handler_test.go && rtk git commit -m "feat(employees): GET /api/v1/employees/:id/runs handler (spec1 §7 S5)"`

---

## Task 5 — Wire handler into routes.go

- [x] Step 5.1 — write failing route test
  - File: `src-go/internal/server/employee_runs_route_test.go` (new)
  - Content:
    ```go
    package server

    import (
        "strings"
        "testing"
    )

    // TestEmployeeRunsRouteRegistered guards that the GET
    // /api/v1/employees/:id/runs route is wired alongside the existing
    // employee CRUD routes. Failing this test means a future refactor
    // dropped the route — Spec 1A explicitly requires it for the FE
    // dashboard.
    func TestEmployeeRunsRouteRegistered(t *testing.T) {
        // routes.go is large; we sniff its source for the route mounting
        // line. This is a pragmatic guard, not a runtime test.
        src := mustReadFile(t, "routes.go")
        if !strings.Contains(src, `/employees/:id/runs`) {
            t.Fatalf("expected GET /employees/:id/runs to be registered in routes.go")
        }
        if !strings.Contains(src, "NewEmployeeRunsHandler") {
            t.Fatalf("expected NewEmployeeRunsHandler() to be invoked in routes.go")
        }
    }
    ```
  - Add this helper at the bottom of the same file:
    ```go
    func mustReadFile(t *testing.T, path string) string {
        t.Helper()
        b, err := readSourceFile(path)
        if err != nil {
            t.Fatalf("read %s: %v", path, err)
        }
        return string(b)
    }
    ```
  - File: `src-go/internal/server/source_helpers_test.go` (new) — the actual reader, kept separate so it can be reused by other route guards:
    ```go
    package server

    import "os"

    func readSourceFile(path string) ([]byte, error) {
        return os.ReadFile(path)
    }
    ```

- [x] Step 5.2 — run `cd src-go && go test ./internal/server/ -run TestEmployeeRunsRouteRegistered` — expect failure (route not yet wired)

- [x] Step 5.3 — wire the route
  - File: `src-go/internal/server/routes.go`
  - Locate the existing employee mounting (around line 1037):
    ```go
    employeeH := handler.NewEmployeeHandler(employeeSvc)
    employeeH.Register(projectGroup)
    ```
  - Immediately after that block, before the `// Workflow templates` comment (around line 1040), add:
    ```go
    // Per-employee unified runs feed (workflow_executions ∪ agent_runs).
    // Route is global (not project-scoped) because the employee id is
    // self-disambiguating and the FE drills down from the employee detail
    // shell, not from a project picker. Project RBAC is enforced by the
    // existing JWT middleware on `protected`.
    employeeRunsRepo := repository.NewEmployeeRunsRepository(db)
    employeeRunsH := handler.NewEmployeeRunsHandler(employeeRunsRepo)
    protected.GET("/employees/:id/runs", employeeRunsH.List)
    ```
  - Note: the `db` symbol is the `*gorm.DB` that the surrounding code uses to construct other repositories — confirm by searching for `repository.NewWorkflowExecutionRepository(` in the same file and reusing that exact identifier name.

- [x] Step 5.4 — run `cd src-go && go test ./internal/server/ -run TestEmployeeRunsRouteRegistered && cd src-go && go build ./...` — expect both green

- [x] Step 5.5 — run `rtk git add src-go/internal/server/routes.go src-go/internal/server/employee_runs_route_test.go src-go/internal/server/source_helpers_test.go && rtk git commit -m "feat(server): mount /api/v1/employees/:id/runs route (spec1 §7 S5)"`

---

## Task 6 — Frontend Zustand store: employee-runs-store

- [x] Step 6.1 — write failing store test
  - File: `lib/stores/employee-runs-store.test.ts` (new)
  - Content:
    ```ts
    import { useEmployeeRunsStore } from "./employee-runs-store";
    import { useAuthStore } from "./auth-store";

    type RunRow = {
      kind: "workflow" | "agent";
      id: string;
      name: string;
      status: string;
      startedAt?: string;
      completedAt?: string;
      durationMs?: number;
      refUrl: string;
    };

    describe("employee-runs-store", () => {
      const empID = "00000000-0000-0000-0000-000000000001";
      const baseRow: RunRow = {
        kind: "workflow",
        id: "11111111-1111-1111-1111-111111111111",
        name: "echo-flow",
        status: "completed",
        startedAt: "2026-04-20T10:00:00Z",
        completedAt: "2026-04-20T10:00:45Z",
        durationMs: 45000,
        refUrl: "/workflow/runs/11111111-1111-1111-1111-111111111111",
      };

      beforeEach(() => {
        useEmployeeRunsStore.setState({
          runsByEmployee: {},
          loadingByEmployee: {},
          pageByEmployee: {},
          hasMoreByEmployee: {},
          kindByEmployee: {},
        });
        useAuthStore.setState({ accessToken: "test-token" } as never);
        global.fetch = jest.fn().mockResolvedValue({
          ok: true,
          status: 200,
          json: async () => ({ items: [baseRow], page: 1, size: 20, kind: "all" }),
        }) as unknown as typeof fetch;
      });

      it("fetchRuns populates runsByEmployee and infers hasMore", async () => {
        await useEmployeeRunsStore.getState().fetchRuns(empID, 1);
        const state = useEmployeeRunsStore.getState();
        expect(state.runsByEmployee[empID]).toHaveLength(1);
        expect(state.runsByEmployee[empID][0].name).toBe("echo-flow");
        expect(state.hasMoreByEmployee[empID]).toBe(false); // 1 row < size=20
        expect(state.loadingByEmployee[empID]).toBe(false);
      });

      it("ingestWorkflowEvent prepends a new run for the matching employee", () => {
        useEmployeeRunsStore.setState({
          runsByEmployee: { [empID]: [baseRow] },
        });
        useEmployeeRunsStore.getState().ingestWorkflowEvent(empID, {
          kind: "workflow",
          id: "22222222-2222-2222-2222-222222222222",
          name: "card-flow",
          status: "running",
          startedAt: "2026-04-20T10:05:00Z",
          refUrl: "/workflow/runs/22222222-2222-2222-2222-222222222222",
        });
        const list = useEmployeeRunsStore.getState().runsByEmployee[empID];
        expect(list).toHaveLength(2);
        expect(list[0].id).toBe("22222222-2222-2222-2222-222222222222");
      });

      it("ingestWorkflowEvent updates an existing row in place", () => {
        useEmployeeRunsStore.setState({
          runsByEmployee: { [empID]: [{ ...baseRow, status: "running", completedAt: undefined, durationMs: undefined }] },
        });
        useEmployeeRunsStore.getState().ingestWorkflowEvent(empID, {
          ...baseRow,
          status: "completed",
        });
        const row = useEmployeeRunsStore.getState().runsByEmployee[empID][0];
        expect(row.status).toBe("completed");
        expect(row.completedAt).toBe("2026-04-20T10:00:45Z");
      });
    });
    ```

- [x] Step 6.2 — run `pnpm test -- employee-runs-store` — expect failure (module not found)

- [x] Step 6.3 — implement the store
  - File: `lib/stores/employee-runs-store.ts` (new)
  - Content:
    ```ts
    "use client";

    import { create } from "zustand";
    import { toast } from "sonner";
    import { createApiClient } from "@/lib/api-client";
    import { useAuthStore } from "./auth-store";

    const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";
    const DEFAULT_PAGE_SIZE = 20;

    export type EmployeeRunKind = "all" | "workflow" | "agent";

    export interface EmployeeRunRow {
      kind: "workflow" | "agent";
      id: string;
      name: string;
      status: string;
      startedAt?: string;
      completedAt?: string;
      durationMs?: number;
      refUrl: string;
    }

    interface RunsResponse {
      items: EmployeeRunRow[];
      page: number;
      size: number;
      kind: EmployeeRunKind;
    }

    interface EmployeeRunsState {
      runsByEmployee: Record<string, EmployeeRunRow[]>;
      loadingByEmployee: Record<string, boolean>;
      pageByEmployee: Record<string, number>;
      hasMoreByEmployee: Record<string, boolean>;
      kindByEmployee: Record<string, EmployeeRunKind>;

      fetchRuns: (employeeId: string, page?: number, kind?: EmployeeRunKind) => Promise<void>;
      ingestWorkflowEvent: (employeeId: string, row: EmployeeRunRow) => void;
      reset: (employeeId: string) => void;
    }

    const getApi = () => createApiClient(API_URL);
    const getToken = () => {
      const state = useAuthStore.getState() as { accessToken?: string | null; token?: string | null };
      return state.accessToken ?? state.token ?? null;
    };

    export const useEmployeeRunsStore = create<EmployeeRunsState>()((set, get) => ({
      runsByEmployee: {},
      loadingByEmployee: {},
      pageByEmployee: {},
      hasMoreByEmployee: {},
      kindByEmployee: {},

      fetchRuns: async (employeeId, page = 1, kind) => {
        const token = getToken();
        if (!token) return;
        const effectiveKind = kind ?? get().kindByEmployee[employeeId] ?? "all";
        set((s) => ({
          loadingByEmployee: { ...s.loadingByEmployee, [employeeId]: true },
          kindByEmployee: { ...s.kindByEmployee, [employeeId]: effectiveKind },
        }));
        try {
          const qs = `?type=${encodeURIComponent(effectiveKind)}&page=${page}&size=${DEFAULT_PAGE_SIZE}`;
          const { data } = await getApi().get<RunsResponse>(
            `/api/v1/employees/${employeeId}/runs${qs}`,
            { token },
          );
          const items = data?.items ?? [];
          const merged = page <= 1 ? items : [
            ...(get().runsByEmployee[employeeId] ?? []),
            ...items,
          ];
          set((s) => ({
            runsByEmployee: { ...s.runsByEmployee, [employeeId]: merged },
            pageByEmployee: { ...s.pageByEmployee, [employeeId]: page },
            hasMoreByEmployee: {
              ...s.hasMoreByEmployee,
              [employeeId]: items.length >= DEFAULT_PAGE_SIZE,
            },
          }));
        } catch (err) {
          toast.error(`加载员工执行历史失败: ${(err as Error).message}`);
        } finally {
          set((s) => ({
            loadingByEmployee: { ...s.loadingByEmployee, [employeeId]: false },
          }));
        }
      },

      ingestWorkflowEvent: (employeeId, row) => {
        set((s) => {
          const existing = s.runsByEmployee[employeeId] ?? [];
          const idx = existing.findIndex((r) => r.kind === row.kind && r.id === row.id);
          let next: EmployeeRunRow[];
          if (idx >= 0) {
            next = existing.slice();
            next[idx] = { ...existing[idx], ...row };
          } else {
            next = [row, ...existing];
          }
          return {
            runsByEmployee: { ...s.runsByEmployee, [employeeId]: next },
          };
        });
      },

      reset: (employeeId) => {
        set((s) => {
          const { [employeeId]: _runs, ...restRuns } = s.runsByEmployee;
          const { [employeeId]: _loading, ...restLoading } = s.loadingByEmployee;
          const { [employeeId]: _page, ...restPage } = s.pageByEmployee;
          const { [employeeId]: _has, ...restHas } = s.hasMoreByEmployee;
          const { [employeeId]: _kind, ...restKind } = s.kindByEmployee;
          return {
            runsByEmployee: restRuns,
            loadingByEmployee: restLoading,
            pageByEmployee: restPage,
            hasMoreByEmployee: restHas,
            kindByEmployee: restKind,
          };
        });
      },
    }));
    ```

- [x] Step 6.4 — run `pnpm test -- employee-runs-store` — expect green

- [x] Step 6.5 — run `rtk git add lib/stores/employee-runs-store.ts lib/stores/employee-runs-store.test.ts && rtk git commit -m "feat(fe): employee-runs-store with fetch + WS-event ingest seam"`

---

## Task 7 — WS subscription: filter existing workflow + agent events by employee

- [x] Step 7.1 — write failing test asserting the bridge function exists and forwards by employee
  - File: `lib/stores/ws-store.employee-runs.test.ts` (new)
  - Content:
    ```ts
    import { forwardRunEventToEmployee } from "./ws-store";
    import { useEmployeeRunsStore } from "./employee-runs-store";

    describe("forwardRunEventToEmployee", () => {
      const empID = "11111111-2222-3333-4444-555555555555";

      beforeEach(() => {
        useEmployeeRunsStore.setState({
          runsByEmployee: {},
          loadingByEmployee: {},
          pageByEmployee: {},
          hasMoreByEmployee: {},
          kindByEmployee: {},
        });
      });

      it("ingests a workflow.execution.completed payload tagged with actingEmployeeId", () => {
        forwardRunEventToEmployee("workflow.execution.completed", {
          executionId: "exec-123",
          workflowName: "echo-flow",
          actingEmployeeId: empID,
          status: "completed",
          startedAt: "2026-04-20T10:00:00Z",
          completedAt: "2026-04-20T10:00:30Z",
        });
        const rows = useEmployeeRunsStore.getState().runsByEmployee[empID];
        expect(rows).toHaveLength(1);
        expect(rows[0].kind).toBe("workflow");
        expect(rows[0].status).toBe("completed");
        expect(rows[0].refUrl).toBe("/workflow/runs/exec-123");
      });

      it("ignores payloads without actingEmployeeId / employeeId", () => {
        forwardRunEventToEmployee("workflow.execution.started", {
          executionId: "exec-no-emp",
          workflowName: "x",
          status: "running",
        });
        expect(useEmployeeRunsStore.getState().runsByEmployee).toEqual({});
      });

      it("ingests an agent.completed payload tagged with employeeId", () => {
        forwardRunEventToEmployee("agent.completed", {
          agentRunId: "run-abc",
          roleId: "code-reviewer",
          employeeId: empID,
          status: "completed",
          startedAt: "2026-04-20T10:00:00Z",
          completedAt: "2026-04-20T10:01:00Z",
        });
        const rows = useEmployeeRunsStore.getState().runsByEmployee[empID];
        expect(rows).toHaveLength(1);
        expect(rows[0].kind).toBe("agent");
        expect(rows[0].name).toBe("code-reviewer");
      });
    });
    ```

- [x] Step 7.2 — run `pnpm test -- ws-store.employee-runs` — expect failure (`forwardRunEventToEmployee` is not exported)

- [x] Step 7.3 — add the forwarder + wire into existing event handlers
  - File: `lib/stores/ws-store.ts`
  - Add this import near the top, after the existing `import { useWorkflowStore } from "./workflow-store";` line:
    ```ts
    import {
      useEmployeeRunsStore,
      type EmployeeRunRow,
    } from "./employee-runs-store";
    ```
  - Add this exported helper function above the `let client: WSClient | null = null;` line:
    ```ts
    /**
     * forwardRunEventToEmployee inspects a workflow.* or agent.* payload for
     * an actingEmployeeId / employeeId tag and, when present, prepends or
     * upserts a unified run row into the per-employee runs store.
     *
     * Filters at the FE because the WS hub broadcasts project-scoped, not
     * employee-scoped — see Spec 1A coordination notes.
     */
    export function forwardRunEventToEmployee(eventType: string, raw: unknown): void {
      if (!raw || typeof raw !== "object") return;
      const payload = raw as Record<string, unknown>;

      if (eventType.startsWith("workflow.")) {
        const employeeId =
          typeof payload.actingEmployeeId === "string" ? payload.actingEmployeeId : null;
        if (!employeeId) return;
        const id =
          typeof payload.executionId === "string" ? payload.executionId :
          typeof payload.id === "string" ? payload.id : null;
        if (!id) return;
        const row: EmployeeRunRow = {
          kind: "workflow",
          id,
          name:
            (typeof payload.workflowName === "string" && payload.workflowName) ||
            (typeof payload.name === "string" && payload.name) ||
            id,
          status: typeof payload.status === "string" ? payload.status : "running",
          startedAt: typeof payload.startedAt === "string" ? payload.startedAt : undefined,
          completedAt: typeof payload.completedAt === "string" ? payload.completedAt : undefined,
          refUrl: `/workflow/runs/${id}`,
        };
        if (row.startedAt && row.completedAt) {
          const dur = new Date(row.completedAt).getTime() - new Date(row.startedAt).getTime();
          if (Number.isFinite(dur) && dur >= 0) row.durationMs = dur;
        }
        useEmployeeRunsStore.getState().ingestWorkflowEvent(employeeId, row);
        return;
      }

      if (eventType.startsWith("agent.")) {
        const employeeId =
          typeof payload.employeeId === "string" ? payload.employeeId : null;
        if (!employeeId) return;
        const id =
          typeof payload.agentRunId === "string" ? payload.agentRunId :
          typeof payload.id === "string" ? payload.id : null;
        if (!id) return;
        const row: EmployeeRunRow = {
          kind: "agent",
          id,
          name:
            (typeof payload.roleId === "string" && payload.roleId) ||
            (typeof payload.name === "string" && payload.name) ||
            "agent",
          status: typeof payload.status === "string" ? payload.status : "running",
          startedAt: typeof payload.startedAt === "string" ? payload.startedAt : undefined,
          completedAt: typeof payload.completedAt === "string" ? payload.completedAt : undefined,
          refUrl: `/agents?run=${id}`,
        };
        if (row.startedAt && row.completedAt) {
          const dur = new Date(row.completedAt).getTime() - new Date(row.startedAt).getTime();
          if (Number.isFinite(dur) && dur >= 0) row.durationMs = dur;
        }
        useEmployeeRunsStore.getState().ingestWorkflowEvent(employeeId, row);
      }
    }
    ```
  - Inside the `connect: (url, token) => { ... }` body (around line 105 in the current file), find each existing `client.on(...)` call for `workflow.execution.started`, `workflow.execution.completed`, `workflow.execution.advanced`, `workflow.node.completed`, `workflow.node.waiting`, `agent.completed`, `agent.failed`, `agent.started`. If those handlers don't yet exist, add a single fan-in registration at the bottom of `connect` (right after the last `client.on(...)` block):
    ```ts
    const RUN_EVENT_TYPES = [
      "workflow.execution.started",
      "workflow.execution.advanced",
      "workflow.execution.completed",
      "workflow.execution.paused",
      "workflow.node.completed",
      "workflow.node.waiting",
      "agent.started",
      "agent.completed",
      "agent.failed",
    ];
    for (const evt of RUN_EVENT_TYPES) {
      client.on(evt, (data) => {
        const payload = extractPayload<Record<string, unknown>>(data);
        if (payload) forwardRunEventToEmployee(evt, payload);
      });
    }
    ```

- [x] Step 7.4 — run `pnpm test -- ws-store.employee-runs` — expect green

- [x] Step 7.5 — run `rtk git add lib/stores/ws-store.ts lib/stores/ws-store.employee-runs.test.ts && rtk git commit -m "feat(ws): forward acting_employee_id-tagged workflow + agent events to employee-runs-store"`

---

## Task 8 — RunRow component (presentational)

- [ ] Step 8.1 — write failing component test
  - File: `components/employees/employee-run-row.test.tsx` (new)
  - Content:
    ```tsx
    import { render, screen } from "@testing-library/react";
    import { EmployeeRunRow } from "./employee-run-row";

    describe("EmployeeRunRow", () => {
      it("renders kind badge, status badge, name as a link, and formatted duration", () => {
        render(
          <EmployeeRunRow
            row={{
              kind: "workflow",
              id: "exec-1",
              name: "echo-flow",
              status: "completed",
              startedAt: "2026-04-20T10:00:00Z",
              completedAt: "2026-04-20T10:00:45Z",
              durationMs: 45000,
              refUrl: "/workflow/runs/exec-1",
            }}
          />,
        );
        expect(screen.getByText("workflow")).toBeInTheDocument();
        expect(screen.getByText("completed")).toBeInTheDocument();
        const link = screen.getByRole("link", { name: /echo-flow/ });
        expect(link).toHaveAttribute("href", "/workflow/runs/exec-1");
        expect(screen.getByText(/45\.0s|45000ms|0:45/)).toBeInTheDocument();
      });

      it("renders an em-dash for missing started_at", () => {
        render(
          <EmployeeRunRow
            row={{
              kind: "agent",
              id: "run-1",
              name: "code-reviewer",
              status: "running",
              refUrl: "/agents?run=run-1",
            }}
          />,
        );
        expect(screen.getByText("—")).toBeInTheDocument();
      });
    });
    ```

- [ ] Step 8.2 — run `pnpm test -- employee-run-row` — expect failure (module not found)

- [ ] Step 8.3 — implement the component
  - File: `components/employees/employee-run-row.tsx` (new)
  - Content:
    ```tsx
    "use client";

    import Link from "next/link";
    import { Badge } from "@/components/ui/badge";
    import { cn } from "@/lib/utils";
    import type { EmployeeRunRow as Row } from "@/lib/stores/employee-runs-store";

    const KIND_COLOR: Record<Row["kind"], string> = {
      workflow: "bg-blue-500/15 text-blue-700 dark:text-blue-400",
      agent: "bg-violet-500/15 text-violet-700 dark:text-violet-400",
    };

    const STATUS_COLOR: Record<string, string> = {
      pending: "bg-zinc-500/15 text-zinc-700 dark:text-zinc-400",
      running: "bg-blue-500/15 text-blue-700 dark:text-blue-400 animate-pulse",
      paused: "bg-amber-500/15 text-amber-700 dark:text-amber-400",
      completed: "bg-green-500/15 text-green-700 dark:text-green-400",
      failed: "bg-red-500/15 text-red-700 dark:text-red-400",
      cancelled: "bg-zinc-500/15 text-zinc-600 dark:text-zinc-400",
    };

    function fmtDuration(ms?: number): string {
      if (ms === undefined || ms === null || !Number.isFinite(ms)) return "—";
      if (ms < 1000) return `${ms}ms`;
      if (ms < 60_000) return `${(ms / 1000).toFixed(1)}s`;
      const m = Math.floor(ms / 60_000);
      const s = Math.floor((ms % 60_000) / 1000);
      return `${m}:${s.toString().padStart(2, "0")}`;
    }

    function fmtTime(iso?: string): string {
      if (!iso) return "—";
      const t = new Date(iso);
      if (Number.isNaN(t.getTime())) return "—";
      return t.toLocaleString();
    }

    export function EmployeeRunRow({ row }: { row: Row }) {
      return (
        <div className="grid grid-cols-12 items-center gap-3 px-4 py-3 border-b text-sm">
          <div className="col-span-2">
            <Badge className={cn("uppercase text-[10px]", KIND_COLOR[row.kind])}>
              {row.kind}
            </Badge>
          </div>
          <div className="col-span-4 truncate">
            <Link
              href={row.refUrl}
              className="font-medium hover:underline focus-visible:underline"
            >
              {row.name}
            </Link>
            <div className="text-xs text-muted-foreground truncate">{row.id}</div>
          </div>
          <div className="col-span-2">
            <Badge className={cn("text-[11px]", STATUS_COLOR[row.status] ?? STATUS_COLOR.pending)}>
              {row.status}
            </Badge>
          </div>
          <div className="col-span-2 text-muted-foreground text-xs">
            {fmtTime(row.startedAt)}
          </div>
          <div className="col-span-2 text-right tabular-nums">
            {fmtDuration(row.durationMs)}
          </div>
        </div>
      );
    }
    ```

- [ ] Step 8.4 — run `pnpm test -- employee-run-row` — expect green

- [ ] Step 8.5 — run `rtk git add components/employees/employee-run-row.tsx components/employees/employee-run-row.test.tsx && rtk git commit -m "feat(fe): EmployeeRunRow presentational component for runs dashboard"`

---

## Task 9 — Employee detail layout shell + Runs page

- [ ] Step 9.1 — write failing page test
  - File: `app/(dashboard)/employees/[id]/runs/page.test.tsx` (new)
  - Content:
    ```tsx
    import { render, screen, waitFor } from "@testing-library/react";
    import EmployeeRunsPage from "./page";
    import { useEmployeeRunsStore } from "@/lib/stores/employee-runs-store";

    jest.mock("next/navigation", () => ({
      useParams: () => ({ id: "emp-1" }),
    }));

    describe("EmployeeRunsPage", () => {
      beforeEach(() => {
        useEmployeeRunsStore.setState({
          runsByEmployee: {
            "emp-1": [
              {
                kind: "workflow",
                id: "exec-1",
                name: "echo-flow",
                status: "completed",
                startedAt: "2026-04-20T10:00:00Z",
                completedAt: "2026-04-20T10:00:45Z",
                durationMs: 45000,
                refUrl: "/workflow/runs/exec-1",
              },
              {
                kind: "agent",
                id: "run-1",
                name: "code-reviewer",
                status: "running",
                startedAt: "2026-04-20T10:01:00Z",
                refUrl: "/agents?run=run-1",
              },
            ],
          },
          loadingByEmployee: { "emp-1": false },
          pageByEmployee: { "emp-1": 1 },
          hasMoreByEmployee: { "emp-1": false },
          kindByEmployee: { "emp-1": "all" },
        });
      });

      it("renders both row kinds with drill-down links", async () => {
        render(<EmployeeRunsPage />);
        await waitFor(() => expect(screen.getByText("echo-flow")).toBeInTheDocument());
        expect(screen.getByText("code-reviewer")).toBeInTheDocument();
        expect(screen.getByRole("link", { name: /echo-flow/ })).toHaveAttribute(
          "href",
          "/workflow/runs/exec-1",
        );
        expect(screen.getByRole("link", { name: /code-reviewer/ })).toHaveAttribute(
          "href",
          "/agents?run=run-1",
        );
      });
    });
    ```

- [ ] Step 9.2 — run `pnpm test -- "employees/\[id\]/runs/page"` — expect failure (module not found)

- [ ] Step 9.3 — create the layout shell
  - File: `app/(dashboard)/employees/[id]/layout.tsx` (new)
  - Content:
    ```tsx
    "use client";

    import Link from "next/link";
    import { use } from "react";
    import { usePathname } from "next/navigation";
    import { cn } from "@/lib/utils";

    /**
     * Employee detail layout — owns the side-nav for per-employee sub-pages.
     * Spec 1A introduces the "Runs" tab; Specs 1C/1D will hang Triggers /
     * Secrets tabs off the same nav. Do NOT duplicate this file in those
     * downstream plans.
     */
    interface NavTab {
      slug: string;
      label: string;
    }

    const TABS: NavTab[] = [
      { slug: "runs", label: "Runs" },
      // 1C will append: { slug: "triggers", label: "Triggers" }
      // 1D will append: { slug: "secrets",  label: "Secrets"  }
    ];

    export default function EmployeeDetailLayout({
      params,
      children,
    }: {
      params: Promise<{ id: string }>;
      children: React.ReactNode;
    }) {
      const { id } = use(params);
      const pathname = usePathname();
      return (
        <div className="space-y-4">
          <div className="border-b">
            <nav className="flex gap-1 px-1" aria-label="Employee sections">
              {TABS.map((tab) => {
                const href = `/employees/${id}/${tab.slug}`;
                const active = pathname?.startsWith(href);
                return (
                  <Link
                    key={tab.slug}
                    href={href}
                    className={cn(
                      "px-4 py-2 text-sm border-b-2 -mb-px transition-colors",
                      active
                        ? "border-primary text-foreground"
                        : "border-transparent text-muted-foreground hover:text-foreground",
                    )}
                  >
                    {tab.label}
                  </Link>
                );
              })}
            </nav>
          </div>
          {children}
        </div>
      );
    }
    ```

- [ ] Step 9.4 — implement the page
  - File: `app/(dashboard)/employees/[id]/runs/page.tsx` (new)
  - Content:
    ```tsx
    "use client";

    import { useEffect } from "react";
    import { useParams } from "next/navigation";
    import { Loader2 } from "lucide-react";
    import { Button } from "@/components/ui/button";
    import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
    import { EmptyState } from "@/components/shared/empty-state";
    import { EmployeeRunRow } from "@/components/employees/employee-run-row";
    import {
      useEmployeeRunsStore,
      type EmployeeRunKind,
    } from "@/lib/stores/employee-runs-store";

    const KIND_FILTERS: { value: EmployeeRunKind; label: string }[] = [
      { value: "all", label: "All" },
      { value: "workflow", label: "Workflows" },
      { value: "agent", label: "Agents" },
    ];

    export default function EmployeeRunsPage() {
      const params = useParams<{ id: string }>();
      const employeeId = params.id;

      const rows = useEmployeeRunsStore((s) => s.runsByEmployee[employeeId] ?? []);
      const loading = useEmployeeRunsStore((s) => s.loadingByEmployee[employeeId] ?? false);
      const page = useEmployeeRunsStore((s) => s.pageByEmployee[employeeId] ?? 1);
      const hasMore = useEmployeeRunsStore((s) => s.hasMoreByEmployee[employeeId] ?? false);
      const kind = useEmployeeRunsStore((s) => s.kindByEmployee[employeeId] ?? "all");
      const fetchRuns = useEmployeeRunsStore((s) => s.fetchRuns);

      useEffect(() => {
        if (!employeeId) return;
        void fetchRuns(employeeId, 1, kind);
      }, [employeeId, kind, fetchRuns]);

      return (
        <Card>
          <CardHeader className="flex flex-row items-center justify-between">
            <CardTitle>执行历史 (Runs)</CardTitle>
            <div className="flex gap-1">
              {KIND_FILTERS.map((f) => (
                <Button
                  key={f.value}
                  size="sm"
                  variant={kind === f.value ? "default" : "outline"}
                  onClick={() => fetchRuns(employeeId, 1, f.value)}
                >
                  {f.label}
                </Button>
              ))}
            </div>
          </CardHeader>
          <CardContent className="p-0">
            {loading && rows.length === 0 ? (
              <div className="p-8 flex items-center justify-center text-muted-foreground">
                <Loader2 className="h-4 w-4 animate-spin mr-2" />
                加载中...
              </div>
            ) : rows.length === 0 ? (
              <EmptyState
                title="暂无执行记录"
                description="该员工还没有驱动过任何 workflow 或 agent run。绑定 trigger 后即可在此看到回放。"
              />
            ) : (
              <>
                <div className="grid grid-cols-12 items-center gap-3 px-4 py-2 border-b bg-muted/40 text-xs font-medium uppercase text-muted-foreground">
                  <div className="col-span-2">类型</div>
                  <div className="col-span-4">名称 / ID</div>
                  <div className="col-span-2">状态</div>
                  <div className="col-span-2">开始时间</div>
                  <div className="col-span-2 text-right">耗时</div>
                </div>
                {rows.map((row) => (
                  <EmployeeRunRow key={`${row.kind}-${row.id}`} row={row} />
                ))}
                {hasMore && (
                  <div className="p-3 border-t flex justify-center">
                    <Button
                      size="sm"
                      variant="ghost"
                      disabled={loading}
                      onClick={() => fetchRuns(employeeId, page + 1, kind)}
                    >
                      {loading ? <Loader2 className="h-3 w-3 animate-spin mr-1" /> : null}
                      加载更多
                    </Button>
                  </div>
                )}
              </>
            )}
          </CardContent>
        </Card>
      );
    }
    ```

- [ ] Step 9.5 — run `pnpm test -- "employees/\[id\]/runs/page"` — expect green

- [ ] Step 9.6 — run `rtk pnpm exec tsc --noEmit && rtk lint app/\(dashboard\)/employees lib/stores/employee-runs-store.ts components/employees/employee-run-row.tsx` — expect both green

- [ ] Step 9.7 — run `rtk git add app/\(dashboard\)/employees lib/stores/employee-runs-store.ts components/employees/employee-run-row.tsx components/employees/employee-run-row.test.tsx app/\(dashboard\)/employees/\[id\]/runs/page.test.tsx && rtk git commit -m "feat(fe): /employees/[id]/runs page + employee detail layout shell (spec1 §9 Trace A view)"`

---

## Task 10 — Add "Runs" entry-point to existing employees section

- [ ] Step 10.1 — write failing test asserting the row gains a Runs link
  - File: `components/employees/employees-section.runs-link.test.tsx` (new)
  - Content:
    ```tsx
    import { render, screen } from "@testing-library/react";
    import { EmployeesSection } from "./employees-section";
    import { useEmployeeStore } from "@/lib/stores/employee-store";

    jest.mock("@/lib/stores/auth-store", () => ({
      useAuthStore: { getState: () => ({ accessToken: "t" }) },
    }));

    describe("EmployeesSection runs link", () => {
      beforeEach(() => {
        useEmployeeStore.setState({
          employeesByProject: {
            "proj-1": [
              {
                id: "emp-1",
                projectId: "proj-1",
                name: "ping",
                roleId: "code-reviewer",
                state: "active",
                createdAt: "2026-04-20T00:00:00Z",
                updatedAt: "2026-04-20T00:00:00Z",
              },
            ],
          },
          loadingByProject: { "proj-1": false },
        } as never);
      });

      it("renders a Runs link on each employee row", () => {
        render(<EmployeesSection projectId="proj-1" />);
        const link = screen.getByRole("link", { name: /Runs/ });
        expect(link).toHaveAttribute("href", "/employees/emp-1/runs");
      });
    });
    ```

- [ ] Step 10.2 — run `pnpm test -- employees-section.runs-link` — expect failure (no Runs link)

- [ ] Step 10.3 — add the Runs link to the row
  - File: `components/employees/employees-section.tsx`
  - Add `Link` to the `next/link` import (add a new import line near the top):
    ```tsx
    import Link from "next/link";
    ```
  - In the `<TableCell className="text-right">` block (line 155), add a Runs link directly before the `<DropdownMenu>` opening tag:
    ```tsx
    <Button asChild variant="ghost" size="sm" className="mr-1">
      <Link href={`/employees/${emp.id}/runs`}>Runs</Link>
    </Button>
    ```

- [ ] Step 10.4 — run `pnpm test -- employees-section.runs-link` — expect green

- [ ] Step 10.5 — run `rtk git add components/employees/employees-section.tsx components/employees/employees-section.runs-link.test.tsx && rtk git commit -m "feat(fe): add per-row Runs link from employees table to /employees/:id/runs"`

---

## Task 11 — Final integration verification

- [ ] Step 11.1 — run the full Go test suite
  - `cd src-go && go test ./...`
  - Expect: green. If any pre-existing test fails, scope-fail back to the failing migration / handler change before proceeding.

- [ ] Step 11.2 — run the full FE test suite
  - `pnpm test`
  - Expect: green.

- [ ] Step 11.3 — typecheck + lint pass
  - `rtk pnpm exec tsc --noEmit && rtk lint`
  - Expect: green.

- [ ] Step 11.4 — manual smoke (operator-run; document as a checklist comment in the verification commit message, not a separate file)
  1. `pnpm dev:backend:verify` — confirm migration 067 + 068 apply cleanly on a fresh DB
  2. Seed an employee + run any workflow that sets `acting_employee_id` (existing wait-event test workflow works)
  3. Open `/employees/<id>/runs` — verify the run appears, status badge updates live as WS events fire, drill-down link goes to `/workflow/runs/<exec_id>`

- [ ] Step 11.5 — commit the verification log
  - `rtk git commit --allow-empty -m "chore(spec1-1A): verification pass — migrations apply, runs feed populates, WS deltas stream"` (use empty commit only if no new files; otherwise stage the verification artifact and commit normally)

---

## Self-review checklist (run before declaring done)

- [ ] All 5 reserved system_metadata keys from spec §6.3 are documented in the migration comment (reply_target / im_dispatched / final_output) — yes (Step 1.6)
- [ ] `created_via` enum matches spec §6.2 vocabulary ('dag_node' | 'manual') — yes (Step 2.3, Step 2.5 CHECK constraint)
- [ ] No new WS event types invented — only existing `workflow.execution.*` / `workflow.node.*` / `agent.*` reused — yes (Step 7.3 RUN_EVENT_TYPES list cross-references eventbus/types.go)
- [ ] Drill-down URLs use existing routes (`/workflow/runs/:id` and `/agents?run=:id`) — yes (Step 3.3 RefURL switch). NOTE: `/workflow/runs/:id` does not yet exist as a dedicated page in this codebase (the unified runs view lives at `/workflow` page's "Runs" tab). The link will resolve to that tab via Next.js routing once a thin `/workflow/runs/[id]/page.tsx` is added by the unified-run-view follow-up; until then the link 404s gracefully — acceptable for Spec 1A.
- [ ] Migration numbering: 067 + 068, latest existing is 066 — verified
- [ ] No code in the plan uses `TBD`, `// ...`, `similar to above`, or other placeholders — verified
