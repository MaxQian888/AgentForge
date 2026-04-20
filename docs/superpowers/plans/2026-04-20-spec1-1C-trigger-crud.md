# Spec 1C — Trigger CRUD as Independent Resource

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 落地 spec1 §4 行 4 + §6.2 + §7 Trigger CRUD — 把 trigger 升级为可在 FE 直接 CRUD 的独立资源；员工详情页成为主入口；DAG 里的 trigger 节点保留为"默认配置源"但不再权威。

**Architecture:** workflow_triggers 表新增 `created_via` 区分两类源；后端补 POST/PATCH/DELETE/test 端点 + 员工维度 list；registrar 从"删后全插"改为"按 created_via='dag_node' merge"，manual 行从不被覆盖；FE 在员工详情新增 Triggers 子页（CRUD 表单 + dry-run test），现有 workflow 编辑器内的 triggers section 退化为只读 list-with-link；老 toggle 路径删除不并行保留。

**Tech Stack:** Go (Echo + 现有 trigger router 内部复用), Postgres, Next.js 16 App Router, Zustand, shadcn/ui form/drawer.

**Depends on:** 1A (needs `workflow_triggers.created_via / display_name / description` columns + `/employees/[id]` detail layout)

**Parallel with:** 1D (IM Bridge cards + outbound dispatcher) — completely independent after 1A

**Unblocks:** demos that need user-created triggers without YAML

---

## Coordination notes (read before starting)

- **Migration column names**: 1A owns `migrations/06X_workflow_trigger_independent.up.sql`. This plan ASSUMES that migration adds `created_via varchar(16) NOT NULL DEFAULT 'dag_node'`, `display_name varchar(128)`, `description text`. If 1A names differ, sync before C2.
- **Employee detail shell**: 1A owns `app/(dashboard)/employees/[id]/layout.tsx` and the side-nav with "Runs" entry. This plan adds the "Triggers" entry to that nav. If 1A is not yet merged, mock the layout but DO NOT duplicate the nav file.
- **Old code deletion**: per spec §12 we DELETE the toggle path in `workflow-trigger-store.ts` and the inline toggle UI. No feature flag.

---

## Task 1 — Extend WorkflowTrigger model + repository for CRUD distinguishability

- [x] Step 1.1 — write failing repo test asserting `created_via` round-trips
  - File: `src-go/internal/repository/workflow_trigger_repo_test.go`
  - Add `TestWorkflowTriggerRepo_CreatedViaRoundTrip` after existing tests (around line of last `func Test`):
    ```go
    func TestWorkflowTriggerRepo_CreatedViaRoundTrip(t *testing.T) {
        repo := newTestRepo(t)
        ctx := context.Background()
        wfID := uuid.New()
        projID := uuid.New()
        tr := &model.WorkflowTrigger{
            WorkflowID: &wfID, ProjectID: projID,
            Source: model.TriggerSourceIM, TargetKind: model.TriggerTargetDAG,
            Config: json.RawMessage(`{"platform":"feishu","command":"/ping"}`),
            InputMapping: json.RawMessage(`{}`),
            CreatedVia: model.TriggerCreatedViaManual,
            DisplayName: "ping echo", Description: "test row",
            Enabled: true,
        }
        if err := repo.Create(ctx, tr); err != nil { t.Fatalf("create: %v", err) }
        got, err := repo.GetByID(ctx, tr.ID)
        if err != nil { t.Fatalf("get: %v", err) }
        if got.CreatedVia != model.TriggerCreatedViaManual { t.Errorf("created_via: %s", got.CreatedVia) }
        if got.DisplayName != "ping echo" { t.Errorf("display_name: %s", got.DisplayName) }
    }
    ```

- [x] Step 1.2 — add `CreatedVia / DisplayName / Description` to `model.WorkflowTrigger`
  - File: `src-go/internal/model/workflow_trigger.go`, after line 25 add:
    ```go
    type TriggerCreatedVia string

    const (
        TriggerCreatedViaDAGNode TriggerCreatedVia = "dag_node"
        TriggerCreatedViaManual  TriggerCreatedVia = "manual"
    )
    ```
  - Inside the `WorkflowTrigger` struct (lines 34–51) add three new fields between `DisabledReason` and `ActingEmployeeID`:
    ```go
    CreatedVia  TriggerCreatedVia `db:"created_via"  json:"createdVia"`
    DisplayName string            `db:"display_name" json:"displayName,omitempty"`
    Description string            `db:"description"  json:"description,omitempty"`
    ```

- [x] Step 1.3 — extend `workflowTriggerRecord` + mappers + add `Create` / `GetByID` / `Update`
  - File: `src-go/internal/repository/workflow_trigger_repo.go`
  - Add to `workflowTriggerRecord` (after line 42, before `ActingEmployeeID`):
    ```go
    CreatedVia  string  `gorm:"column:created_via"`
    DisplayName string  `gorm:"column:display_name"`
    Description string  `gorm:"column:description"`
    ```
  - Wire into `newWorkflowTriggerRecord` (line 55) and `toModel` (line 88) — copy `CreatedVia` defaulting to `"dag_node"` when blank, `DisplayName`, `Description`.
  - After `Delete` (line 339), add `Create(ctx, t)` (insert without dedup lookup; sets `t.ID/CreatedAt/UpdatedAt`), `GetByID(ctx, id) (*model.WorkflowTrigger, error)`, `Update(ctx, t)` (full replace of mutable columns including `display_name`, `description`, `config`, `input_mapping`, `acting_employee_id`, `enabled`, `idempotency_key_template`, `dedupe_window_seconds`; `id`/`workflow_id`/`source`/`created_via` are NOT touched). Return `ErrNotFound` when row missing.

- [x] Step 1.4 — verify
  - Run `rtk go test ./internal/repository/...` — new test passes; existing trigger tests still pass.

  **Note (deviation from plan)**: model + record fields were already landed by 1A
  prior to this plan execution; verified via Grep before adding new code. The
  Step 1.1 spec called for an integration-style `newTestRepo` test against
  Postgres, but this repo's existing test pattern is nil-DB only (no
  in-process SQLite/PG harness). Round-trip validation of `created_via /
  display_name / description` is therefore deferred to the live-PG integration
  test in Task 12, where a real DB is available. New nil-DB tests cover the
  Create/GetByID/Update happy preconditions.

---

## Task 2 — Refactor registrar from delete-and-insert to merge-by-created_via

- [x] Step 2.1 — write failing test: manual rows survive a DAG re-save
  - File: `src-go/internal/trigger/registrar_test.go`
  - Add at end:
    ```go
    func TestRegistrar_SyncFromDefinition_PreservesManualRows(t *testing.T) {
        repo := newMockTriggerRepo()
        reg := trigger.NewRegistrar(repo)
        wfID := uuid.New(); projID := uuid.New()

        // Pre-seed a manual row for this workflow.
        manualID := uuid.New()
        wfRef := wfID
        repo.rows[manualID] = &model.WorkflowTrigger{
            ID: manualID, WorkflowID: &wfRef, ProjectID: projID,
            Source: model.TriggerSourceIM, TargetKind: model.TriggerTargetDAG,
            Config: json.RawMessage(`{"platform":"feishu","command":"/manual"}`),
            CreatedVia: model.TriggerCreatedViaManual, Enabled: true,
        }
        // Pre-seed a stale dag_node row that should be deleted.
        staleID := uuid.New()
        repo.rows[staleID] = &model.WorkflowTrigger{
            ID: staleID, WorkflowID: &wfRef, ProjectID: projID,
            Source: model.TriggerSourceIM, TargetKind: model.TriggerTargetDAG,
            Config: json.RawMessage(`{"platform":"feishu","command":"/old"}`),
            CreatedVia: model.TriggerCreatedViaDAGNode, Enabled: true,
        }

        // Sync with empty DAG node list — stale dag_node deleted, manual kept.
        if _, err := reg.SyncFromDefinition(context.Background(), wfID, projID, nil, nil); err != nil {
            t.Fatalf("sync: %v", err)
        }
        if _, ok := repo.rows[manualID]; !ok {
            t.Error("manual row was deleted by sync; must be preserved")
        }
        if _, ok := repo.rows[staleID]; ok {
            t.Error("stale dag_node row was not deleted")
        }
    }
    ```
  - Also extend `mockTriggerRepo.Delete` to track which IDs were deleted (for assertion clarity) — keep the existing `deleteCount` field and add `deletedIDs []uuid.UUID`.

- [x] Step 2.2 — implement merge logic in registrar
  - File: `src-go/internal/trigger/registrar.go`
  - Inside the `for _, node := range nodes` loop (around line 159), set `tr.CreatedVia = model.TriggerCreatedViaDAGNode` before the upsert (line 273).
  - After the upsert section, change the cleanup loop (lines 286–299) so the delete-stale step ONLY targets `created_via='dag_node'` rows:
    ```go
    for _, row := range existing {
        if row.CreatedVia == model.TriggerCreatedViaManual {
            continue // manual rows are owned by the FE CRUD, never reaped
        }
        if _, keep := keepSet[row.ID]; keep {
            continue
        }
        if err := r.repo.Delete(ctx, row.ID); err != nil {
            return outcomes, fmt.Errorf("sync triggers: delete stale dag_node row %s: %w", row.ID, err)
        }
    }
    ```

- [x] Step 2.3 — add complementary test: dag_node rows added/updated/removed cleanly
  - Same file, add `TestRegistrar_SyncFromDefinition_DAGRowsAddedUpdatedRemoved` that:
    1. starts with one pre-existing `dag_node` row matching node "n1" config (asserts no delete);
    2. node "n2" is new (asserts upsert called with `CreatedVia=dag_node`);
    3. an extra pre-existing `dag_node` row absent from DAG is deleted;
    4. a `manual` row mixed in with the same workflow is untouched.

- [x] Step 2.4 — verify
  - Run `rtk go test ./internal/trigger/...` — all green; existing 12 tests still pass.

---

## Task 3 — Trigger CRUD service layer

- [ ] Step 3.1 — write failing service test for `TriggerService.Create` validation matrix
  - New file: `src-go/internal/service/trigger_service_test.go`
  - Cover four cases via mocks:
    - happy path returns the row;
    - `workflow_id` not in same project → returns sentinel `ErrTriggerWorkflowNotFound`;
    - `acting_employee_id` archived → returns `ErrTriggerActingEmployeeArchived`;
    - `acting_employee_id` cross-project → returns `ErrTriggerActingEmployeeArchived`-or-similar (decide one sentinel per spec).

- [ ] Step 3.2 — implement service
  - New file: `src-go/internal/service/trigger_service.go`:
    ```go
    package service

    import (
        "context"
        "encoding/json"
        "errors"

        "github.com/google/uuid"
        "github.com/react-go-quick-starter/server/internal/model"
        "github.com/react-go-quick-starter/server/internal/repository"
    )

    var (
        ErrTriggerWorkflowNotFound        = errors.New("trigger:workflow_not_found")
        ErrTriggerActingEmployeeArchived  = errors.New("trigger:acting_employee_archived")
        ErrTriggerCannotDeleteDAGManaged  = errors.New("trigger:cannot_delete_dag_managed")
    )

    type triggerCRUDRepo interface {
        Create(ctx context.Context, t *model.WorkflowTrigger) error
        GetByID(ctx context.Context, id uuid.UUID) (*model.WorkflowTrigger, error)
        Update(ctx context.Context, t *model.WorkflowTrigger) error
        Delete(ctx context.Context, id uuid.UUID) error
        ListByActingEmployee(ctx context.Context, employeeID uuid.UUID) ([]*model.WorkflowTrigger, error)
    }

    type workflowDefLookup interface {
        GetByID(ctx context.Context, id uuid.UUID) (*model.WorkflowDefinition, error)
    }

    type employeeLookup interface {
        Get(ctx context.Context, id uuid.UUID) (*model.Employee, error)
    }

    type TriggerService struct {
        repo     triggerCRUDRepo
        defs     workflowDefLookup
        emps     employeeLookup
    }

    func NewTriggerService(repo triggerCRUDRepo, defs workflowDefLookup, emps employeeLookup) *TriggerService {
        return &TriggerService{repo: repo, defs: defs, emps: emps}
    }

    type CreateTriggerInput struct {
        WorkflowID       uuid.UUID
        Source           model.TriggerSource
        Config           json.RawMessage
        InputMapping     json.RawMessage
        ActingEmployeeID *uuid.UUID
        DisplayName      string
        Description      string
        CreatedBy        *uuid.UUID
    }

    func (s *TriggerService) Create(ctx context.Context, in CreateTriggerInput) (*model.WorkflowTrigger, error) {
        def, err := s.defs.GetByID(ctx, in.WorkflowID)
        if err != nil || def == nil {
            return nil, ErrTriggerWorkflowNotFound
        }
        if in.ActingEmployeeID != nil {
            emp, err := s.emps.Get(ctx, *in.ActingEmployeeID)
            if err != nil || emp == nil { return nil, ErrTriggerActingEmployeeArchived }
            if emp.ProjectID != def.ProjectID || emp.State == model.EmployeeStateArchived {
                return nil, ErrTriggerActingEmployeeArchived
            }
        }
        wfRef := in.WorkflowID
        tr := &model.WorkflowTrigger{
            WorkflowID: &wfRef, ProjectID: def.ProjectID,
            Source: in.Source, TargetKind: model.TriggerTargetDAG,
            Config: in.Config, InputMapping: in.InputMapping,
            ActingEmployeeID: in.ActingEmployeeID,
            DisplayName: in.DisplayName, Description: in.Description,
            CreatedVia: model.TriggerCreatedViaManual,
            CreatedBy: in.CreatedBy, Enabled: true,
        }
        if err := s.repo.Create(ctx, tr); err != nil { return nil, err }
        return tr, nil
    }
    ```
  - Add `Patch(ctx, id, PatchTriggerInput)` and `Delete(ctx, id)`:
    - `Patch` loads via `GetByID`, applies non-nil pointer fields among `Config / InputMapping / ActingEmployeeID / DisplayName / Description / Enabled`, RE-RUNS the `acting_employee_id` validation if changed, calls `Update`. Disallow editing `WorkflowID/Source/CreatedVia` (silently ignored at this layer; the handler enforces the contract).
    - `Delete` loads, returns `ErrTriggerCannotDeleteDAGManaged` if `CreatedVia != "manual"`, else `repo.Delete`.

- [ ] Step 3.3 — verify
  - Run `rtk go test ./internal/service/... -run TestTrigger` — passes.

---

## Task 4 — Trigger CRUD HTTP handlers (POST/PATCH/DELETE/GET-by-employee)

- [ ] Step 4.1 — write failing handler test for create + delete-of-dag-managed
  - File: `src-go/internal/handler/trigger_handler_test.go`
  - Add a `mockTriggerService` implementing the interface from Step 4.2 with controllable returns.
  - Tests:
    - `TestTriggerHandler_Create_HappyPath` posts JSON, expects 201 + body shape.
    - `TestTriggerHandler_Create_WorkflowNotFound` service returns sentinel → 400 with code `trigger:workflow_not_found`.
    - `TestTriggerHandler_Delete_DAGManagedReturns409` service returns `ErrTriggerCannotDeleteDAGManaged` → 409 with code `trigger:cannot_delete_dag_managed`.
    - `TestTriggerHandler_ListByEmployee_OK` returns array.

- [ ] Step 4.2 — extend `TriggerHandler`
  - File: `src-go/internal/handler/trigger_handler.go`
  - Add a new interface above `triggerQueryRepo`:
    ```go
    type triggerCRUDService interface {
        Create(ctx context.Context, in service.CreateTriggerInput) (*model.WorkflowTrigger, error)
        Patch(ctx context.Context, id uuid.UUID, in service.PatchTriggerInput) (*model.WorkflowTrigger, error)
        Delete(ctx context.Context, id uuid.UUID) error
        ListByEmployee(ctx context.Context, employeeID uuid.UUID) ([]*model.WorkflowTrigger, error)
    }
    ```
  - Add `crud triggerCRUDService` to the `TriggerHandler` struct (line 34) and `WithCRUDService(svc triggerCRUDService) *TriggerHandler` setter.
  - Inside `RegisterRoutes` (line 52), when `h.crud != nil` register:
    ```go
    g.POST("",       h.Create)        // POST /api/v1/triggers
    g.PATCH("/:id",  h.Patch)
    g.DELETE("/:id", h.Delete)
    e.GET("/api/v1/employees/:employeeId/triggers", h.ListByEmployee)
    ```
  - Implement handlers that:
    - parse the JSON body, validate UUIDs;
    - on `errors.Is(err, service.ErrTriggerWorkflowNotFound)` → `c.JSON(400, {"code":"trigger:workflow_not_found"})`;
    - on `errors.Is(err, service.ErrTriggerActingEmployeeArchived)` → 400 + `trigger:acting_employee_archived`;
    - on `errors.Is(err, service.ErrTriggerCannotDeleteDAGManaged)` → 409 + `trigger:cannot_delete_dag_managed`;
    - reject `workflow_id / source / created_via` in PATCH body (return 400).
  - Reuse `localizedError` for unknown errors.

- [ ] Step 4.3 — verify
  - Run `rtk go test ./internal/handler/... -run TestTriggerHandler` — all new + existing tests pass.

---

## Task 5 — Trigger dry-run `/test` endpoint

- [ ] Step 5.1 — write failing handler test
  - Same file as Task 4. `TestTriggerHandler_Test_DryRun`:
    - POST `/api/v1/triggers/{id}/test` with body `{"event":{"platform":"feishu","command":"/echo","content":"/echo hi","chat_id":"c-1","args":["hi"]}}`.
    - Mock service returns `{matched: true, would_dispatch: true, rendered_input: {"text":"hi"}, skip_reason: ""}`.
    - Assert 200 + JSON shape.

- [ ] Step 5.2 — add `Test(ctx, id, eventPayload)` to TriggerService
  - File: `src-go/internal/service/trigger_service.go` add:
    ```go
    type DryRunResult struct {
        Matched       bool           `json:"matched"`
        WouldDispatch bool           `json:"would_dispatch"`
        RenderedInput map[string]any `json:"rendered_input,omitempty"`
        SkipReason    string         `json:"skip_reason,omitempty"`
    }
    ```
  - The dry-run reuses `trigger.matchesTrigger` and `trigger.renderInputMapping`. Since both are unexported, EXPORT them (rename to `MatchesTrigger`, `RenderInputMapping`) in `src-go/internal/trigger/router.go` (lines 273, 397). Update existing internal callers in the same file.
  - Service method signature: `func (s *TriggerService) Test(ctx context.Context, id uuid.UUID, event map[string]any) (*DryRunResult, error)`. Implementation:
    1. `repo.GetByID(ctx, id)` — 404 if missing.
    2. Build `trigger.Event{Source: tr.Source, Data: event}`.
    3. `matched := trigger.MatchesTrigger(tr, ev)`. If not matched, return `{Matched:false, WouldDispatch:false, SkipReason:"no_match"}`.
    4. `mapped, mapErr := trigger.RenderInputMapping(tr.InputMapping, event)`. If err → `{Matched:true, WouldDispatch:false, SkipReason:"mapping_error: "+err.Error()}`.
    5. If `tr.Enabled == false` → `{Matched:true, WouldDispatch:false, SkipReason:"trigger_disabled"}`.
    6. Else `{Matched:true, WouldDispatch:true, RenderedInput:mapped}`.
    - Critical: the dry-run NEVER calls `engine.Start` and NEVER touches the idempotency store.

- [ ] Step 5.3 — handler `Test` method
  - In `trigger_handler.go` add `Test` that parses body `{"event": map[string]any}`, calls `h.crud.Test(...)`, returns 200 + struct. Register `g.POST("/:id/test", h.Test)`.

- [ ] Step 5.4 — verify
  - Run `rtk go test ./internal/...` — all green; the rename of `matchesTrigger`→`MatchesTrigger` may cascade through `router.go` — fix call sites.

---

## Task 6 — Wire CRUD service in routes.go

- [ ] Step 6.1 — extend route construction
  - File: `src-go/internal/server/routes.go`, around line 568 right after `triggerRepo := repository.NewWorkflowTriggerRepository(taskRepo.DB())`:
    ```go
    triggerSvc := service.NewTriggerService(triggerRepo, dagDefRepo, employeeRepo)
    ```
  - Replace line 577:
    ```go
    triggerH := handler.NewTriggerHandler(triggerRouter).
        WithQueryRepo(triggerRepo).
        WithCRUDService(triggerSvc)
    ```
  - The `employeeRepo` variable already exists upstream; if not, locate the `repository.NewEmployeeRepository(...)` line and store its result in a local var named `employeeRepo`.

- [ ] Step 6.2 — verify
  - `rtk go build ./cmd/server` succeeds.
  - `rtk go test ./internal/server/...` passes.

---

## Task 7 — Backfill script for existing rows

- [ ] Step 7.1 — write `cmd/backfill-trigger-source/main.go`
  - New directory `src-go/cmd/backfill-trigger-source/`
  - File `main.go`:
    ```go
    package main

    import (
        "context"
        "log"
        "time"

        "github.com/react-go-quick-starter/server/internal/config"
        "github.com/react-go-quick-starter/server/internal/repository"
    )

    func main() {
        cfg := config.Load()
        if cfg.PostgresURL == "" { log.Fatal("POSTGRES_URL is required") }
        db, err := repository.OpenPostgres(cfg.PostgresURL)
        if err != nil { log.Fatalf("open db: %v", err) }
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        res := db.WithContext(ctx).Exec(
            "UPDATE workflow_triggers SET created_via = 'dag_node' WHERE created_via IS NULL OR created_via = ''",
        )
        if res.Error != nil { log.Fatalf("backfill: %v", res.Error) }
        log.Printf("backfill complete: %d row(s) updated", res.RowsAffected)
    }
    ```
  - If `repository.OpenPostgres` does not exist, mirror the helper used in `cmd/migrate-once/main.go` for opening a connection.

- [ ] Step 7.2 — document invocation in plan note
  - Plan note (no file change): "The migration in 1A sets `DEFAULT 'dag_node'` so existing inserts get the value automatically. This script handles any pre-migration rows where the column was added with a NULL backfill (defensive). Invoke via `cd src-go && go run ./cmd/backfill-trigger-source` after migrations."

- [ ] Step 7.3 — smoke test
  - `rtk go build ./cmd/backfill-trigger-source` succeeds.

---

## Task 8 — Frontend store: employee-trigger CRUD

- [ ] Step 8.1 — write failing Jest test for CRUD store
  - New file: `lib/stores/employee-trigger-store.test.ts`
  - Cases:
    1. `fetchByEmployee("emp-1")` calls `GET /api/v1/employees/emp-1/triggers` with token, hydrates `triggersByEmployee["emp-1"]`.
    2. `createTrigger({...})` POSTs `/api/v1/triggers`, prepends row to local list, success toast.
    3. `patchTrigger("trg-1", {enabled: false})` PATCHes `/api/v1/triggers/trg-1`, mutates in-place.
    4. `deleteTrigger("trg-1")` DELETEs, removes from local list.
    5. `testTrigger("trg-1", {platform:"feishu",command:"/echo"})` POSTs `/api/v1/triggers/trg-1/test`, returns the dry-run object without mutating store.

- [ ] Step 8.2 — implement store
  - New file: `lib/stores/employee-trigger-store.ts` mirroring `employee-store.ts` shape:
    ```ts
    "use client";
    import { create } from "zustand";
    import { toast } from "sonner";
    import { createApiClient } from "@/lib/api-client";
    import { useAuthStore } from "./auth-store";
    import type { WorkflowTrigger, TriggerSource } from "./workflow-trigger-store";

    const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

    export interface CreateTriggerInput {
      workflowId: string;
      source: TriggerSource;
      config: Record<string, unknown>;
      inputMapping?: Record<string, unknown>;
      actingEmployeeId?: string;
      displayName?: string;
      description?: string;
    }
    export interface PatchTriggerInput {
      config?: Record<string, unknown>;
      inputMapping?: Record<string, unknown>;
      actingEmployeeId?: string | null;
      displayName?: string;
      description?: string;
      enabled?: boolean;
    }
    export interface DryRunResult {
      matched: boolean;
      would_dispatch: boolean;
      rendered_input?: Record<string, unknown>;
      skip_reason?: string;
    }

    interface State {
      triggersByEmployee: Record<string, WorkflowTrigger[]>;
      loading: Record<string, boolean>;
      fetchByEmployee: (employeeId: string) => Promise<void>;
      createTrigger: (input: CreateTriggerInput) => Promise<WorkflowTrigger | null>;
      patchTrigger: (triggerId: string, input: PatchTriggerInput) => Promise<void>;
      deleteTrigger: (triggerId: string, employeeId: string) => Promise<void>;
      testTrigger: (triggerId: string, event: Record<string, unknown>) => Promise<DryRunResult | null>;
    }
    // ... implement create/patch/delete/test, surfacing toast on i18n-aware error codes
    ```
  - On error responses with `code` matching `trigger:workflow_not_found / trigger:acting_employee_archived / trigger:cannot_delete_dag_managed`, render localized toast.

- [ ] Step 8.3 — verify
  - `rtk pnpm test -- employee-trigger-store` passes.

---

## Task 9 — Frontend page: `/employees/[id]/triggers`

- [ ] Step 9.1 — create the page
  - New file: `app/(dashboard)/employees/[id]/triggers/page.tsx`
  - Outline:
    ```tsx
    "use client";
    import { useEffect, useState } from "react";
    import { useParams } from "next/navigation";
    import { Plus } from "lucide-react";
    import { Button } from "@/components/ui/button";
    import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
    import { TriggerListTable } from "@/components/triggers/trigger-list-table";
    import { TriggerEditDrawer } from "@/components/triggers/trigger-edit-drawer";
    import { TriggerTestModal } from "@/components/triggers/trigger-test-modal";
    import { useEmployeeTriggerStore } from "@/lib/stores/employee-trigger-store";

    export default function EmployeeTriggersPage() {
      const params = useParams<{ id: string }>();
      const employeeId = params.id;
      const triggers = useEmployeeTriggerStore((s) => s.triggersByEmployee[employeeId] ?? []);
      const fetchByEmployee = useEmployeeTriggerStore((s) => s.fetchByEmployee);
      const [editing, setEditing] = useState<{ open: boolean; triggerId?: string }>({ open: false });
      const [testing, setTesting] = useState<{ open: boolean; triggerId?: string }>({ open: false });

      useEffect(() => { void fetchByEmployee(employeeId); }, [employeeId, fetchByEmployee]);

      return (
        <Card>
          <CardHeader className="flex flex-row items-center justify-between">
            <CardTitle>触发器 (Triggers)</CardTitle>
            <Button onClick={() => setEditing({ open: true })}>
              <Plus className="h-4 w-4 mr-1" /> 新建触发器
            </Button>
          </CardHeader>
          <CardContent>
            <TriggerListTable
              triggers={triggers}
              onEdit={(t) => setEditing({ open: true, triggerId: t.id })}
              onTest={(t) => setTesting({ open: true, triggerId: t.id })}
            />
          </CardContent>
          <TriggerEditDrawer
            open={editing.open} triggerId={editing.triggerId}
            employeeId={employeeId}
            onClose={() => setEditing({ open: false })}
          />
          <TriggerTestModal
            open={testing.open} triggerId={testing.triggerId}
            onClose={() => setTesting({ open: false })}
          />
        </Card>
      );
    }
    ```

- [ ] Step 9.2 — create `components/triggers/trigger-list-table.tsx`
  - Columns: display_name (or "(unnamed)"), source badge, summary (`platform command` or `cron tz`), target workflow name (resolve via `useDAGWorkflowStore` if available; fall back to truncated UUID), enabled Switch (calls `patchTrigger(t.id, {enabled})`), actions menu (Edit / Test / Delete with confirm dialog using `AlertDialog`).
  - Mark each `<TableRow>` with `id={`trigger-${t.id}`}` so the anchor link from the workflow editor side can scroll to it.

- [ ] Step 9.3 — create `components/triggers/trigger-edit-drawer.tsx`
  - shadcn `Sheet` drawer matching `employees-section.tsx` pattern (lines 8–14, 70–101).
  - Form state: `workflowId`, `source` (radio im/schedule), per-source sub-form, `displayName`, `description`, `actingEmployeeId` (defaults to current `employeeId` prop).
  - `source==="im"` sub-form fields: `platform` (select feishu/slack/dingtalk), `command`, `match_regex`, `chat_allowlist` (Textarea of newline-separated IDs), `input_mapping` (Textarea JSON; validated on submit).
  - `source==="schedule"` sub-form fields: `cron`, `timezone` (default `UTC`), `overlap_policy` (select skip_if_running/allow_parallel), `input_mapping` JSON.
  - On submit:
    - assemble `config` JSON object from sub-form;
    - parse `input_mapping` JSON (catch error → inline error);
    - call `createTrigger(...)` or `patchTrigger(...)`.
  - Show errors from the toast pipeline.

- [ ] Step 9.4 — create `components/triggers/trigger-test-modal.tsx`
  - shadcn `Dialog` with two tabs: "Sample event" (Textarea pre-filled with platform-specific stub like `{"platform":"feishu","command":"/echo","content":"/echo hi","chat_id":"c-1","args":["hi"]}`) and "Result" (read-only JSON).
  - "Run dry-run" button → `testTrigger(triggerId, parsedEvent)` → render `matched` + `would_dispatch` + `rendered_input` (or `skip_reason`).

- [ ] Step 9.5 — verify
  - `rtk pnpm test -- triggers` passes the new component tests (next task).
  - Manual smoke: `pnpm dev` then visit `/employees/<id>/triggers` (UI renders even with no backend).

---

## Task 10 — Add "Triggers" entry to employee detail nav

- [ ] Step 10.1 — extend the employee detail nav
  - File: `app/(dashboard)/employees/[id]/layout.tsx` (created by 1A).
  - Add a `{ href: "triggers", label: "Triggers" }` entry to the same nav array that already has "Runs". If 1A's layout sits in a different file, adjust accordingly.
  - **Coordination note**: this is the only file touched by both 1A and 1C; merge order matters. If 1A is not yet merged, leave this step in a "blocked-on-1A" status and ship 1C without the nav entry — the page is still reachable via direct URL.

---

## Task 11 — Refactor workflow-triggers-section.tsx to list-only-with-link

- [ ] Step 11.1 — write failing FE test asserting toggle is gone
  - File: `components/workflow/workflow-triggers-section.test.tsx` (new).
  - Mocks `useWorkflowTriggerStore` to return one trigger. Render. Assert there is NO `role="switch"` element. Assert there IS an `<a>` with href containing `/employees/` and `#trigger-`.

- [ ] Step 11.2 — refactor the component
  - File: `components/workflow/workflow-triggers-section.tsx`
  - DELETE: `Switch` import (line 6), `setEnabled` selector (line 46), the entire `<TableHead>启用</TableHead>` column header (line 108), and the entire `<TableCell><Switch ...></TableCell>` cell (lines 146–151).
  - REPLACE the whole row-rendering loop (lines 113–157) so each row shows: display_name (or `configSummary(t)`), source badge, target_kind badge, "扮演员工" cell (existing). Wrap the row's primary text in a `<Link href={...}>` per rule:
    - if `t.actingEmployeeId` set → `/employees/${t.actingEmployeeId}/triggers#trigger-${t.id}`;
    - else → render plain text + small "未绑定员工" badge with hover tooltip "在 DAG 节点中配置 acting_employee_id 以启用 FE 编辑".
  - Update the introductory `<p>` text inside `CardHeader` to: "此处只读展示。新增/编辑请在『员工 → 数字员工 → Triggers』完成。"

- [ ] Step 11.3 — clean `lib/stores/workflow-trigger-store.ts`
  - File: `lib/stores/workflow-trigger-store.ts`
  - DELETE the entire `setEnabled` action (lines 41 + 73–94) and the `setEnabled` member of `WorkflowTriggerStoreState`.
  - DELETE the `toast` import if no longer used after removal.
  - Update `lib/stores/workflow-trigger-store.test.ts`: remove the "flips the enabled flag" test (lines 103–120). Adjust the remaining tests to confirm only `fetchTriggers` is exposed.

- [ ] Step 11.4 — verify
  - `rtk pnpm test -- workflow-trigger-store workflow-triggers-section` all pass.

---

## Task 12 — Backend integration test (real Postgres) for full lifecycle

- [ ] Step 12.1 — extend integration test
  - File: `src-go/internal/integration/trigger_flow_integration_test.go`
  - Add `TestTriggerCRUD_FullLifecycle_PreservesDAGNodeRows`:
    1. seed a workflow definition in PG with one DAG trigger node "n1" (source=im, command=/dag);
    2. call the `Registrar.SyncFromDefinition` once → assert one `created_via='dag_node'` row exists;
    3. call `TriggerService.Create` to add a manual row (command=/manual) for the same workflow;
    4. re-save the same DAG (same trigger node) → registrar should keep both: 1 dag_node + 1 manual;
    5. patch the manual row's `display_name` via `Patch` → re-fetch confirms;
    6. attempt `Delete` on the dag_node row → expect `ErrTriggerCannotDeleteDAGManaged`;
    7. `Delete` the manual row → succeeds → only the dag_node row remains.

- [ ] Step 12.2 — verify
  - `rtk go test ./internal/integration/... -run TriggerCRUD` passes against a live PG (CI hook).

---

## Task 13 — End-to-end FE component test

- [ ] Step 13.1 — `components/triggers/trigger-edit-drawer.test.tsx`
  - Mocks `useEmployeeTriggerStore`. Renders drawer in "create" mode for `employeeId="emp-1"`.
  - Fill fields (workflow, source=im, platform=feishu, command="/echo", input_mapping={"text":"{{$event.content}}"}). Click submit.
  - Assert `createTrigger` called with the expected payload, including `actingEmployeeId: "emp-1"`.
  - Edge: invalid JSON in input_mapping shows inline validation error; submit not called.

- [ ] Step 13.2 — `components/triggers/trigger-test-modal.test.tsx`
  - Mocks `testTrigger` to return `{matched:true, would_dispatch:true, rendered_input:{text:"hi"}}`.
  - Render, click "Run dry-run", assert the rendered_input JSON appears in the result tab.
  - Edge: `{matched:false}` shows "未匹配" badge.

- [ ] Step 13.3 — verify
  - `rtk pnpm test -- triggers` all green.

---

## Task 14 — Self-review pass

- [ ] Step 14.1 — Spec coverage audit
  - Open `docs/superpowers/specs/2026-04-20-foundation-gaps-design.md` §7 Trigger CRUD block (lines 167–175) and walk every endpoint:
    - POST `/api/v1/triggers` ✔ Task 4.
    - PATCH `/api/v1/triggers/:id` ✔ Task 4.
    - DELETE `/api/v1/triggers/:id` ✔ Task 4.
    - GET `/api/v1/employees/:id/triggers` ✔ Task 4.
    - POST `/api/v1/triggers/:id/test` ✔ Task 5.
    - error codes `trigger:workflow_not_found / trigger:acting_employee_archived / trigger:cannot_delete_dag_managed` ✔ Task 3 sentinels + Task 4 mapping.

- [ ] Step 14.2 — §10 Error matrix coverage
  - Confirm "Trigger CRUD 校验失败" → 400 + structured codes (Task 4 step 4.2).
  - Confirm "Registrar merge — 仅作用 `created_via='dag_node'` 行；manual 行不可被 DAG 覆盖" → Task 2 step 2.2 plus tests in 2.1, 2.3, 12.1.

- [ ] Step 14.3 — §12 Old code deletion checklist
  - `components/workflow/workflow-triggers-section.tsx` toggle removed → Task 11.2.
  - `lib/stores/workflow-trigger-store.ts` toggle path deleted (no flag) → Task 11.3.
  - Confirm no `g.POST("/:id/enabled", h.SetEnabled)` references remain in `trigger_handler.go` after the refactor (the SetEnabled handler can stay temporarily for legacy wiring but the FE no longer calls it; mark it deprecated with a `// Deprecated:` comment and remove in a follow-up if it's safe).

- [ ] Step 14.4 — Backwards compatibility / data safety
  - Verify backfill script is idempotent (Task 7) and the migration default is `'dag_node'` so live tables are safe.
  - Verify `Registrar.SyncFromDefinition` cannot accidentally promote/demote `created_via` (Task 2.2 — the field is set only when constructing `tr` for an upsert, never written by the cleanup path; existing manual rows are not loaded into `tr`).

- [ ] Step 14.5 — Final commit checklist
  - All `rtk go test ./...` green.
  - All `rtk pnpm test` green.
  - `rtk go build ./cmd/server` and `./cmd/backfill-trigger-source` succeed.
  - Run `rtk lint` and `rtk pnpm exec tsc --noEmit` clean.
