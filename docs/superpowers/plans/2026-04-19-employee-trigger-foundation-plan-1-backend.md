# Employee & Trigger Foundation — Plan 1: Backend

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the Go backend foundation for persistent Employee entities and the Event→Workflow trigger pathway — schema, domain services, workflow engine integration, and the HTTP seam for external event ingestion — verified by integration tests against Postgres + Redis.

**Architecture:** New `employee` and `trigger` internal packages; additive schema (3 new tables, 4 column additions, one non-breaking migration); existing `dag_workflow_service.StartExecution` extended with a `StartOptions` struct (seed + triggered_by); existing `llm_agent` node and `applier.applySpawnAgent` re-routed through `EmployeeService.Invoke` when a config carries `employeeId`. Adapters (IM, Schedule), review refactor, and frontend are explicitly out of scope for Plan 1 — see Plan 2.

**Tech Stack:** Go 1.22, Echo, pgx/Postgres, Redis, golang-migrate, existing `workflow/nodetypes` effect system.

**Scope out of this plan:**
- IM `/workflow` command + `POST /api/v1/triggers/im/events` handler — Plan 2
- Scheduler `workflow_job` handler + job registration — Plan 2
- Review refactor and `system:code-review` template — Plan 2
- Frontend (Employee CRUD, trigger inspector, Triggers tab) — Plan 2
- Smoke fixtures, `CLAUDE.md` / `PRD.md` text updates — Plan 2

**Design doc:** `docs/superpowers/specs/2026-04-19-employee-and-workflow-trigger-foundation-design.md`

---

## File Structure

**New files:**
- `src-go/migrations/062_employee_trigger_foundation.up.sql`
- `src-go/migrations/062_employee_trigger_foundation.down.sql`
- `src-go/internal/model/employee.go` — `Employee`, `EmployeeSkill`, `EmployeeState` enum
- `src-go/internal/model/workflow_trigger.go` — `WorkflowTrigger`, `TriggerSource` enum
- `src-go/internal/repository/employee_repository.go`
- `src-go/internal/repository/employee_repository_test.go`
- `src-go/internal/repository/workflow_trigger_repository.go`
- `src-go/internal/repository/workflow_trigger_repository_test.go`
- `src-go/internal/employee/service.go`
- `src-go/internal/employee/service_test.go`
- `src-go/internal/employee/errors.go`
- `src-go/internal/employee/registry.go` — YAML loader + seed
- `src-go/internal/employee/registry_test.go`
- `src-go/internal/trigger/registrar.go`
- `src-go/internal/trigger/registrar_test.go`
- `src-go/internal/trigger/router.go`
- `src-go/internal/trigger/router_test.go`
- `src-go/internal/trigger/idempotency.go`
- `src-go/internal/trigger/idempotency_test.go`
- `src-go/internal/handler/employee_handler.go`
- `src-go/internal/handler/employee_handler_test.go`
- `src-go/internal/integration/employee_trigger_integration_test.go`

**Modified files:**
- `src-go/internal/service/dag_workflow_service.go` — `StartExecution` signature (+ `StartOptions`)
- `src-go/internal/handler/workflow_handler.go` — `StartExecution` handler + new `HandleExternalEvent` route
- `src-go/internal/service/workflow_def_service.go` (or equivalent that handles workflow save) — call `trigger.Registrar.SyncFromDefinition`
- `src-go/internal/workflow/nodetypes/llm_agent.go` — read `employeeId`; include in payload
- `src-go/internal/workflow/nodetypes/effects.go` — add `EmployeeID` to `SpawnAgentPayload`
- `src-go/internal/workflow/nodetypes/applier.go` — extend `AgentSpawner` interface and `applySpawnAgent` routing
- `src-go/internal/workflow/nodetypes/applier_test.go` — extend fakes
- `src-go/internal/service/agent_service.go` — implement new spawner method `SpawnForEmployee`
- `src-go/internal/model/agent_run.go` — add `EmployeeID *uuid.UUID`
- `src-go/internal/model/workflow_execution.go` — add `TriggeredBy *uuid.UUID`
- `src-go/internal/model/agent_memory.go` — add `EmployeeID *uuid.UUID` + extend scope enum
- `src-go/cmd/server/main.go` — wire EmployeeService, TriggerRegistrar, HTTP routes, seed call on startup

---

## Task 1 — Database migration

**Files:**
- Create: `src-go/migrations/062_employee_trigger_foundation.up.sql`
- Create: `src-go/migrations/062_employee_trigger_foundation.down.sql`

- [ ] **Step 1: Write the up migration**

Create `src-go/migrations/062_employee_trigger_foundation.up.sql`:

```sql
-- Employees: persistent capability carriers scoped per project.
CREATE TABLE employees (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id    UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    display_name  TEXT,
    role_id       TEXT NOT NULL,
    runtime_prefs JSONB NOT NULL DEFAULT '{}'::jsonb,
    config        JSONB NOT NULL DEFAULT '{}'::jsonb,
    state         TEXT NOT NULL DEFAULT 'active'
                     CHECK (state IN ('active','paused','archived')),
    created_by    UUID REFERENCES members(id) ON DELETE SET NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (project_id, name)
);
CREATE INDEX employees_project_state_idx ON employees(project_id, state);

CREATE TABLE employee_skills (
    employee_id UUID NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
    skill_path  TEXT NOT NULL,
    auto_load   BOOLEAN NOT NULL DEFAULT true,
    overrides   JSONB NOT NULL DEFAULT '{}'::jsonb,
    added_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (employee_id, skill_path)
);

-- Workflow triggers: materialized registration rows that map external events to workflow starts.
CREATE TABLE workflow_triggers (
    id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id              UUID NOT NULL REFERENCES workflow_definitions(id) ON DELETE CASCADE,
    project_id               UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    source                   TEXT NOT NULL CHECK (source IN ('im','schedule')),
    config                   JSONB NOT NULL,
    input_mapping            JSONB NOT NULL DEFAULT '{}'::jsonb,
    idempotency_key_template TEXT,
    dedupe_window_seconds    INTEGER NOT NULL DEFAULT 0,
    enabled                  BOOLEAN NOT NULL DEFAULT true,
    created_by               UUID REFERENCES members(id) ON DELETE SET NULL,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- Unique on (workflow, source, config hash) so the same config can't be registered twice.
CREATE UNIQUE INDEX workflow_triggers_config_hash_uniq
    ON workflow_triggers (workflow_id, source, md5(config::text));
CREATE INDEX workflow_triggers_source_enabled_idx
    ON workflow_triggers (source) WHERE enabled = true;

-- agent_runs: optional employee binding.
ALTER TABLE agent_runs
    ADD COLUMN employee_id UUID REFERENCES employees(id) ON DELETE SET NULL;
CREATE INDEX agent_runs_employee_idx ON agent_runs(employee_id) WHERE employee_id IS NOT NULL;

-- agent_memory: support an employee scope with dedicated employee_id column.
ALTER TABLE agent_memory
    DROP CONSTRAINT IF EXISTS agent_memory_scope_check;
ALTER TABLE agent_memory
    ADD CONSTRAINT agent_memory_scope_check
    CHECK (scope IN ('global','project','role','employee'));
ALTER TABLE agent_memory
    ADD COLUMN employee_id UUID REFERENCES employees(id) ON DELETE CASCADE;
CREATE INDEX agent_memory_employee_idx
    ON agent_memory(employee_id) WHERE employee_id IS NOT NULL;

-- workflow_executions: which trigger fired this run (NULL = manual/legacy).
ALTER TABLE workflow_executions
    ADD COLUMN triggered_by UUID REFERENCES workflow_triggers(id) ON DELETE SET NULL;
CREATE INDEX workflow_executions_triggered_by_idx
    ON workflow_executions(triggered_by) WHERE triggered_by IS NOT NULL;

-- reviews: link back to workflow execution (NULL for legacy rows).
ALTER TABLE reviews
    ADD COLUMN execution_id UUID REFERENCES workflow_executions(id) ON DELETE SET NULL;
CREATE INDEX reviews_execution_id_idx
    ON reviews(execution_id) WHERE execution_id IS NOT NULL;
```

- [ ] **Step 2: Write the down migration**

Create `src-go/migrations/062_employee_trigger_foundation.down.sql`:

```sql
ALTER TABLE reviews DROP COLUMN IF EXISTS execution_id;
ALTER TABLE workflow_executions DROP COLUMN IF EXISTS triggered_by;
ALTER TABLE agent_memory DROP COLUMN IF EXISTS employee_id;
ALTER TABLE agent_memory DROP CONSTRAINT IF EXISTS agent_memory_scope_check;
ALTER TABLE agent_memory
    ADD CONSTRAINT agent_memory_scope_check
    CHECK (scope IN ('global','project','role'));
ALTER TABLE agent_runs DROP COLUMN IF EXISTS employee_id;

DROP TABLE IF EXISTS workflow_triggers;
DROP TABLE IF EXISTS employee_skills;
DROP TABLE IF EXISTS employees;
```

- [ ] **Step 3: Apply and verify**

Run the local dev stack, then:

```bash
pnpm dev:backend:verify
```

Expected: startup succeeds; migration `062` applied. Verify with `psql`:

```bash
docker compose exec postgres psql -U postgres agentforge -c "\d employees"
docker compose exec postgres psql -U postgres agentforge -c "\d workflow_triggers"
docker compose exec postgres psql -U postgres agentforge -c "\d+ agent_runs" | grep employee_id
```

Expected: tables exist; `agent_runs.employee_id` column visible.

- [ ] **Step 4: Commit**

```bash
rtk git add src-go/migrations/062_employee_trigger_foundation.up.sql src-go/migrations/062_employee_trigger_foundation.down.sql
rtk git commit -m "feat(db): add employees, employee_skills, workflow_triggers tables (migration 062)"
```

---

## Task 2 — Domain model types

**Files:**
- Create: `src-go/internal/model/employee.go`
- Create: `src-go/internal/model/workflow_trigger.go`
- Modify: `src-go/internal/model/agent_run.go`
- Modify: `src-go/internal/model/workflow_execution.go`
- Modify: `src-go/internal/model/agent_memory.go`

- [ ] **Step 1: Create `employee.go`**

Write `src-go/internal/model/employee.go`:

```go
package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type EmployeeState string

const (
	EmployeeStateActive   EmployeeState = "active"
	EmployeeStatePaused   EmployeeState = "paused"
	EmployeeStateArchived EmployeeState = "archived"
)

// Employee is a persistent capability carrier scoped to one project.
// It binds a role manifest, optional extra skills, runtime preferences,
// and lifecycle state. AgentRuns executed on its behalf carry
// EmployeeID for memory isolation and history attribution.
type Employee struct {
	ID           uuid.UUID       `json:"id"`
	ProjectID    uuid.UUID       `json:"projectId"`
	Name         string          `json:"name"`
	DisplayName  string          `json:"displayName,omitempty"`
	RoleID       string          `json:"roleId"`
	RuntimePrefs json.RawMessage `json:"runtimePrefs"`
	Config       json.RawMessage `json:"config"`
	State        EmployeeState   `json:"state"`
	CreatedBy    *uuid.UUID      `json:"createdBy,omitempty"`
	CreatedAt    time.Time       `json:"createdAt"`
	UpdatedAt    time.Time       `json:"updatedAt"`

	Skills []EmployeeSkill `json:"skills,omitempty"`
}

// EmployeeSkill is an additional skill binding beyond the role manifest's declared skills.
type EmployeeSkill struct {
	EmployeeID uuid.UUID       `json:"employeeId"`
	SkillPath  string          `json:"skillPath"`
	AutoLoad   bool            `json:"autoLoad"`
	Overrides  json.RawMessage `json:"overrides,omitempty"`
	AddedAt    time.Time       `json:"addedAt"`
}

// RuntimePrefs is the typed shape we expect inside Employee.RuntimePrefs.
// Persisted as JSON to allow schema evolution without migrations.
type RuntimePrefs struct {
	Runtime   string  `json:"runtime,omitempty"`
	Provider  string  `json:"provider,omitempty"`
	Model     string  `json:"model,omitempty"`
	BudgetUsd float64 `json:"budgetUsd,omitempty"`
	MaxTurns  int     `json:"maxTurns,omitempty"`
}
```

- [ ] **Step 2: Create `workflow_trigger.go`**

Write `src-go/internal/model/workflow_trigger.go`:

```go
package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type TriggerSource string

const (
	TriggerSourceIM       TriggerSource = "im"
	TriggerSourceSchedule TriggerSource = "schedule"
)

// WorkflowTrigger is the materialized form of a trigger-node subscription.
// Rows are upserted when a workflow definition is saved (see trigger.Registrar.SyncFromDefinition)
// and consulted at runtime by trigger.EventRouter and the scheduler adapter.
type WorkflowTrigger struct {
	ID                     uuid.UUID       `json:"id"`
	WorkflowID             uuid.UUID       `json:"workflowId"`
	ProjectID              uuid.UUID       `json:"projectId"`
	Source                 TriggerSource   `json:"source"`
	Config                 json.RawMessage `json:"config"`
	InputMapping           json.RawMessage `json:"inputMapping"`
	IdempotencyKeyTemplate string          `json:"idempotencyKeyTemplate,omitempty"`
	DedupeWindowSeconds    int             `json:"dedupeWindowSeconds"`
	Enabled                bool            `json:"enabled"`
	CreatedBy              *uuid.UUID      `json:"createdBy,omitempty"`
	CreatedAt              time.Time       `json:"createdAt"`
	UpdatedAt              time.Time       `json:"updatedAt"`
}
```

- [ ] **Step 3: Extend `agent_run.go`**

Append an `EmployeeID *uuid.UUID` field to the existing `AgentRun` struct. Locate the struct in `src-go/internal/model/agent_run.go` and add the field:

```go
// Within the existing AgentRun struct:
EmployeeID *uuid.UUID `json:"employeeId,omitempty"`
```

Place it next to `RoleID` to preserve logical grouping.

- [ ] **Step 4: Extend `workflow_execution.go`**

Locate `WorkflowExecution` struct; add:

```go
TriggeredBy *uuid.UUID `json:"triggeredBy,omitempty"`
```

Place adjacent to existing metadata fields (e.g., right after the status/time fields).

- [ ] **Step 5: Extend `agent_memory.go`**

Within the existing memory struct (likely `AgentMemory`), add:

```go
EmployeeID *uuid.UUID `json:"employeeId,omitempty"`
```

Also add a new scope constant (find the block where `MemoryScopeGlobal/Project/Role` are defined):

```go
MemoryScopeEmployee MemoryScope = "employee"
```

- [ ] **Step 6: Verify compile**

```bash
cd src-go && go build ./...
```

Expected: clean build; no other code needs updates yet because fields are additive.

- [ ] **Step 7: Commit**

```bash
rtk git add src-go/internal/model/
rtk git commit -m "feat(model): add Employee, EmployeeSkill, WorkflowTrigger types; extend AgentRun/WorkflowExecution/AgentMemory"
```

---

## Task 3 — Employee repository

**Files:**
- Create: `src-go/internal/repository/employee_repository.go`
- Create: `src-go/internal/repository/employee_repository_test.go`

- [ ] **Step 1: Write the test file (TDD)**

Write `src-go/internal/repository/employee_repository_test.go`:

```go
package repository_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"agentforge/internal/model"
	"agentforge/internal/repository"
	"agentforge/internal/testutil" // existing helper that opens a per-test Postgres tx
)

func TestEmployeeRepository_CreateAndGet(t *testing.T) {
	ctx := context.Background()
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	project := testutil.SeedProject(t, db)
	repo := repository.NewEmployeeRepository(db)

	emp := &model.Employee{
		ProjectID:    project.ID,
		Name:         "test-reviewer",
		DisplayName:  "Test Reviewer",
		RoleID:       "code-reviewer",
		RuntimePrefs: json.RawMessage(`{"runtime":"claude_code","model":"claude-opus-4-7"}`),
		Config:       json.RawMessage(`{}`),
		State:        model.EmployeeStateActive,
	}
	require.NoError(t, repo.Create(ctx, emp))
	require.NotEqual(t, uuid.Nil, emp.ID)

	got, err := repo.Get(ctx, emp.ID)
	require.NoError(t, err)
	require.Equal(t, emp.Name, got.Name)
	require.Equal(t, model.EmployeeStateActive, got.State)
}

func TestEmployeeRepository_UniqueProjectName(t *testing.T) {
	ctx := context.Background()
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	project := testutil.SeedProject(t, db)
	repo := repository.NewEmployeeRepository(db)

	first := &model.Employee{ProjectID: project.ID, Name: "dup", RoleID: "code-reviewer", State: model.EmployeeStateActive}
	require.NoError(t, repo.Create(ctx, first))

	second := &model.Employee{ProjectID: project.ID, Name: "dup", RoleID: "code-reviewer", State: model.EmployeeStateActive}
	err := repo.Create(ctx, second)
	require.Error(t, err)
	require.ErrorIs(t, err, repository.ErrEmployeeNameConflict)
}

func TestEmployeeRepository_ListByProject(t *testing.T) {
	ctx := context.Background()
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	project := testutil.SeedProject(t, db)
	repo := repository.NewEmployeeRepository(db)

	for _, name := range []string{"a", "b", "c"} {
		require.NoError(t, repo.Create(ctx, &model.Employee{
			ProjectID: project.ID, Name: name, RoleID: "code-reviewer", State: model.EmployeeStateActive,
		}))
	}

	list, err := repo.ListByProject(ctx, project.ID, repository.EmployeeFilter{})
	require.NoError(t, err)
	require.Len(t, list, 3)
}

func TestEmployeeRepository_SkillsCRUD(t *testing.T) {
	ctx := context.Background()
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	project := testutil.SeedProject(t, db)
	repo := repository.NewEmployeeRepository(db)
	emp := &model.Employee{ProjectID: project.ID, Name: "skilled", RoleID: "code-reviewer", State: model.EmployeeStateActive}
	require.NoError(t, repo.Create(ctx, emp))

	require.NoError(t, repo.AddSkill(ctx, emp.ID, model.EmployeeSkill{SkillPath: "skills/typescript", AutoLoad: true}))
	require.NoError(t, repo.AddSkill(ctx, emp.ID, model.EmployeeSkill{SkillPath: "skills/go", AutoLoad: false}))

	skills, err := repo.ListSkills(ctx, emp.ID)
	require.NoError(t, err)
	require.Len(t, skills, 2)

	require.NoError(t, repo.RemoveSkill(ctx, emp.ID, "skills/go"))
	skills, _ = repo.ListSkills(ctx, emp.ID)
	require.Len(t, skills, 1)
}
```

- [ ] **Step 2: Run the failing test**

```bash
cd src-go && go test ./internal/repository/ -run TestEmployeeRepository -v
```

Expected: FAIL with `undefined: repository.NewEmployeeRepository` (and related).

- [ ] **Step 3: Implement the repository**

Write `src-go/internal/repository/employee_repository.go`:

```go
package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"agentforge/internal/model"
)

var ErrEmployeeNameConflict = errors.New("employee name already exists in project")

type EmployeeFilter struct {
	State *model.EmployeeState // nil = any
}

type EmployeeRepository struct {
	db *pgxpool.Pool
}

func NewEmployeeRepository(db *pgxpool.Pool) *EmployeeRepository {
	return &EmployeeRepository{db: db}
}

func (r *EmployeeRepository) Create(ctx context.Context, e *model.Employee) error {
	row := r.db.QueryRow(ctx, `
        INSERT INTO employees (project_id, name, display_name, role_id, runtime_prefs, config, state, created_by)
        VALUES ($1,$2,$3,$4,COALESCE($5,'{}'::jsonb),COALESCE($6,'{}'::jsonb),$7,$8)
        RETURNING id, created_at, updated_at
    `, e.ProjectID, e.Name, e.DisplayName, e.RoleID, e.RuntimePrefs, e.Config, e.State, e.CreatedBy)

	if err := row.Scan(&e.ID, &e.CreatedAt, &e.UpdatedAt); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrEmployeeNameConflict
		}
		return fmt.Errorf("insert employee: %w", err)
	}
	return nil
}

func (r *EmployeeRepository) Get(ctx context.Context, id uuid.UUID) (*model.Employee, error) {
	const q = `
        SELECT id, project_id, name, COALESCE(display_name,''), role_id,
               runtime_prefs, config, state, created_by, created_at, updated_at
        FROM employees WHERE id = $1`
	row := r.db.QueryRow(ctx, q, id)
	e := &model.Employee{}
	err := row.Scan(&e.ID, &e.ProjectID, &e.Name, &e.DisplayName, &e.RoleID,
		&e.RuntimePrefs, &e.Config, &e.State, &e.CreatedBy, &e.CreatedAt, &e.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get employee: %w", err)
	}
	return e, nil
}

func (r *EmployeeRepository) ListByProject(ctx context.Context, projectID uuid.UUID, f EmployeeFilter) ([]*model.Employee, error) {
	q := `SELECT id, project_id, name, COALESCE(display_name,''), role_id,
                 runtime_prefs, config, state, created_by, created_at, updated_at
          FROM employees WHERE project_id = $1`
	args := []any{projectID}
	if f.State != nil {
		q += " AND state = $2"
		args = append(args, *f.State)
	}
	q += " ORDER BY created_at DESC"

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query employees: %w", err)
	}
	defer rows.Close()
	var out []*model.Employee
	for rows.Next() {
		e := &model.Employee{}
		if err := rows.Scan(&e.ID, &e.ProjectID, &e.Name, &e.DisplayName, &e.RoleID,
			&e.RuntimePrefs, &e.Config, &e.State, &e.CreatedBy, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *EmployeeRepository) Update(ctx context.Context, e *model.Employee) error {
	ct, err := r.db.Exec(ctx, `
        UPDATE employees SET
            display_name = $2, role_id = $3,
            runtime_prefs = COALESCE($4,'{}'::jsonb),
            config = COALESCE($5,'{}'::jsonb),
            updated_at = now()
        WHERE id = $1
    `, e.ID, e.DisplayName, e.RoleID, e.RuntimePrefs, e.Config)
	if err != nil {
		return fmt.Errorf("update employee: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *EmployeeRepository) SetState(ctx context.Context, id uuid.UUID, state model.EmployeeState) error {
	ct, err := r.db.Exec(ctx, `UPDATE employees SET state = $2, updated_at = now() WHERE id = $1`, id, state)
	if err != nil {
		return fmt.Errorf("set employee state: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *EmployeeRepository) Delete(ctx context.Context, id uuid.UUID) error {
	ct, err := r.db.Exec(ctx, `DELETE FROM employees WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete employee: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *EmployeeRepository) AddSkill(ctx context.Context, employeeID uuid.UUID, s model.EmployeeSkill) error {
	_, err := r.db.Exec(ctx, `
        INSERT INTO employee_skills (employee_id, skill_path, auto_load, overrides)
        VALUES ($1,$2,$3,COALESCE($4,'{}'::jsonb))
        ON CONFLICT (employee_id, skill_path) DO UPDATE
        SET auto_load = EXCLUDED.auto_load, overrides = EXCLUDED.overrides
    `, employeeID, s.SkillPath, s.AutoLoad, s.Overrides)
	return err
}

func (r *EmployeeRepository) RemoveSkill(ctx context.Context, employeeID uuid.UUID, skillPath string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM employee_skills WHERE employee_id=$1 AND skill_path=$2`, employeeID, skillPath)
	return err
}

func (r *EmployeeRepository) ListSkills(ctx context.Context, employeeID uuid.UUID) ([]model.EmployeeSkill, error) {
	rows, err := r.db.Query(ctx, `
        SELECT employee_id, skill_path, auto_load, overrides, added_at
        FROM employee_skills WHERE employee_id = $1 ORDER BY added_at
    `, employeeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.EmployeeSkill
	for rows.Next() {
		var s model.EmployeeSkill
		if err := rows.Scan(&s.EmployeeID, &s.SkillPath, &s.AutoLoad, &s.Overrides, &s.AddedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// helper for timestamping existing-employee updates in one round-trip if needed later
var _ = time.Time{}
```

If `ErrNotFound` does not already exist in the `repository` package, add to `src-go/internal/repository/errors.go` (create if missing):

```go
package repository

import "errors"

var ErrNotFound = errors.New("not found")
```

- [ ] **Step 4: Run the tests to verify pass**

```bash
cd src-go && go test ./internal/repository/ -run TestEmployeeRepository -v
```

Expected: PASS (4 tests).

- [ ] **Step 5: Commit**

```bash
rtk git add src-go/internal/repository/employee_repository.go src-go/internal/repository/employee_repository_test.go src-go/internal/repository/errors.go
rtk git commit -m "feat(repo): EmployeeRepository with skill CRUD"
```

---

## Task 4 — Workflow-trigger repository

**Files:**
- Create: `src-go/internal/repository/workflow_trigger_repository.go`
- Create: `src-go/internal/repository/workflow_trigger_repository_test.go`

- [ ] **Step 1: Write the failing test**

Write `src-go/internal/repository/workflow_trigger_repository_test.go`:

```go
package repository_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"agentforge/internal/model"
	"agentforge/internal/repository"
	"agentforge/internal/testutil"
)

func TestWorkflowTriggerRepository_UpsertAndList(t *testing.T) {
	ctx := context.Background()
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	project := testutil.SeedProject(t, db)
	wf := testutil.SeedWorkflowDefinition(t, db, project.ID)

	repo := repository.NewWorkflowTriggerRepository(db)
	t1 := &model.WorkflowTrigger{
		WorkflowID:   wf.ID,
		ProjectID:    project.ID,
		Source:       model.TriggerSourceIM,
		Config:       json.RawMessage(`{"platform":"feishu","command":"/review"}`),
		InputMapping: json.RawMessage(`{"pr_url":"{{$event.args[0]}}"}`),
		Enabled:      true,
	}
	require.NoError(t, repo.Upsert(ctx, t1))
	require.NotEmpty(t, t1.ID)

	// Second upsert with identical config should no-op (same row).
	t2 := *t1
	t2.ID = [16]byte{}
	require.NoError(t, repo.Upsert(ctx, &t2))
	require.Equal(t, t1.ID, t2.ID)

	// Different config → new row.
	t3 := &model.WorkflowTrigger{
		WorkflowID:   wf.ID,
		ProjectID:    project.ID,
		Source:       model.TriggerSourceIM,
		Config:       json.RawMessage(`{"platform":"slack","command":"/review"}`),
		InputMapping: json.RawMessage(`{}`),
		Enabled:      true,
	}
	require.NoError(t, repo.Upsert(ctx, t3))
	require.NotEqual(t, t1.ID, t3.ID)

	all, err := repo.ListEnabledBySource(ctx, model.TriggerSourceIM)
	require.NoError(t, err)
	require.Len(t, all, 2)
}

func TestWorkflowTriggerRepository_Disable(t *testing.T) {
	ctx := context.Background()
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	project := testutil.SeedProject(t, db)
	wf := testutil.SeedWorkflowDefinition(t, db, project.ID)
	repo := repository.NewWorkflowTriggerRepository(db)
	tr := &model.WorkflowTrigger{
		WorkflowID: wf.ID, ProjectID: project.ID, Source: model.TriggerSourceSchedule,
		Config: json.RawMessage(`{"cron":"0 9 * * *","timezone":"UTC"}`),
		InputMapping: json.RawMessage(`{}`), Enabled: true,
	}
	require.NoError(t, repo.Upsert(ctx, tr))

	require.NoError(t, repo.SetEnabled(ctx, tr.ID, false))
	list, _ := repo.ListEnabledBySource(ctx, model.TriggerSourceSchedule)
	require.Len(t, list, 0)
}
```

- [ ] **Step 2: Run the failing test**

```bash
cd src-go && go test ./internal/repository/ -run TestWorkflowTriggerRepository -v
```

Expected: FAIL (undefined).

- [ ] **Step 3: Implement the repository**

Write `src-go/internal/repository/workflow_trigger_repository.go`:

```go
package repository

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"agentforge/internal/model"
)

type WorkflowTriggerRepository struct {
	db *pgxpool.Pool
}

func NewWorkflowTriggerRepository(db *pgxpool.Pool) *WorkflowTriggerRepository {
	return &WorkflowTriggerRepository{db: db}
}

func (r *WorkflowTriggerRepository) Upsert(ctx context.Context, t *model.WorkflowTrigger) error {
	// Compute deterministic hash of config for uniqueness.
	sum := md5.Sum(t.Config)
	configHash := hex.EncodeToString(sum[:])

	row := r.db.QueryRow(ctx, `
        WITH upsert AS (
          SELECT id FROM workflow_triggers
           WHERE workflow_id = $1 AND source = $2 AND md5(config::text) = $3
        )
        INSERT INTO workflow_triggers
            (workflow_id, project_id, source, config, input_mapping,
             idempotency_key_template, dedupe_window_seconds, enabled, created_by)
        SELECT $1,$4,$2,$5,$6,$7,$8,$9,$10
         WHERE NOT EXISTS (SELECT 1 FROM upsert)
        RETURNING id, created_at, updated_at
    `,
		t.WorkflowID, t.Source, configHash,
		t.ProjectID, t.Config, t.InputMapping,
		nullString(t.IdempotencyKeyTemplate), t.DedupeWindowSeconds, t.Enabled, t.CreatedBy,
	)
	err := row.Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt)
	if err == nil {
		return nil
	}

	// No new row inserted → fetch existing.
	if err.Error() == "no rows in result set" {
		q := `SELECT id, created_at, updated_at FROM workflow_triggers
              WHERE workflow_id=$1 AND source=$2 AND md5(config::text)=$3`
		row := r.db.QueryRow(ctx, q, t.WorkflowID, t.Source, configHash)
		if err2 := row.Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt); err2 != nil {
			return fmt.Errorf("read existing trigger: %w", err2)
		}
		// Apply non-key updates (enabled/input_mapping/etc.).
		_, err = r.db.Exec(ctx, `
            UPDATE workflow_triggers SET
              input_mapping = $2, idempotency_key_template = $3,
              dedupe_window_seconds = $4, enabled = $5, updated_at = now()
            WHERE id = $1`,
			t.ID, t.InputMapping, nullString(t.IdempotencyKeyTemplate), t.DedupeWindowSeconds, t.Enabled)
		if err != nil {
			return fmt.Errorf("update existing trigger: %w", err)
		}
		return nil
	}
	return fmt.Errorf("upsert trigger: %w", err)
}

func (r *WorkflowTriggerRepository) ListEnabledBySource(ctx context.Context, src model.TriggerSource) ([]*model.WorkflowTrigger, error) {
	rows, err := r.db.Query(ctx, `
        SELECT id, workflow_id, project_id, source, config, input_mapping,
               COALESCE(idempotency_key_template,''), dedupe_window_seconds,
               enabled, created_by, created_at, updated_at
        FROM workflow_triggers WHERE source = $1 AND enabled = true
    `, src)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*model.WorkflowTrigger
	for rows.Next() {
		t := &model.WorkflowTrigger{}
		if err := rows.Scan(&t.ID, &t.WorkflowID, &t.ProjectID, &t.Source, &t.Config,
			&t.InputMapping, &t.IdempotencyKeyTemplate, &t.DedupeWindowSeconds,
			&t.Enabled, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *WorkflowTriggerRepository) ListByWorkflow(ctx context.Context, workflowID uuid.UUID) ([]*model.WorkflowTrigger, error) {
	rows, err := r.db.Query(ctx, `
        SELECT id, workflow_id, project_id, source, config, input_mapping,
               COALESCE(idempotency_key_template,''), dedupe_window_seconds,
               enabled, created_by, created_at, updated_at
        FROM workflow_triggers WHERE workflow_id = $1
    `, workflowID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.WorkflowTrigger
	for rows.Next() {
		t := &model.WorkflowTrigger{}
		if err := rows.Scan(&t.ID, &t.WorkflowID, &t.ProjectID, &t.Source, &t.Config,
			&t.InputMapping, &t.IdempotencyKeyTemplate, &t.DedupeWindowSeconds,
			&t.Enabled, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *WorkflowTriggerRepository) SetEnabled(ctx context.Context, id uuid.UUID, enabled bool) error {
	_, err := r.db.Exec(ctx, `UPDATE workflow_triggers SET enabled=$2, updated_at=now() WHERE id=$1`, id, enabled)
	return err
}

func (r *WorkflowTriggerRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM workflow_triggers WHERE id = $1`, id)
	return err
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
```

- [ ] **Step 4: Run tests**

```bash
cd src-go && go test ./internal/repository/ -run TestWorkflowTriggerRepository -v
```

Expected: PASS (2 tests).

- [ ] **Step 5: Commit**

```bash
rtk git add src-go/internal/repository/workflow_trigger_repository.go src-go/internal/repository/workflow_trigger_repository_test.go
rtk git commit -m "feat(repo): WorkflowTriggerRepository with hash-based upsert"
```

---

## Task 5 — Idempotency store (Redis-backed)

**Files:**
- Create: `src-go/internal/trigger/idempotency.go`
- Create: `src-go/internal/trigger/idempotency_test.go`

- [ ] **Step 1: Write the failing test**

Write `src-go/internal/trigger/idempotency_test.go`:

```go
package trigger_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"agentforge/internal/testutil"
	"agentforge/internal/trigger"
)

func TestIdempotencyStore_FirstSeenAllows(t *testing.T) {
	redis, cleanup := testutil.NewTestRedis(t)
	defer cleanup()

	store := trigger.NewRedisIdempotencyStore(redis)
	ctx := context.Background()

	seen, err := store.SeenWithin(ctx, "key-A", 60*time.Second)
	require.NoError(t, err)
	require.False(t, seen, "first occurrence must not be seen")
}

func TestIdempotencyStore_SecondWithinWindow(t *testing.T) {
	redis, cleanup := testutil.NewTestRedis(t)
	defer cleanup()

	store := trigger.NewRedisIdempotencyStore(redis)
	ctx := context.Background()

	_, _ = store.SeenWithin(ctx, "key-B", 60*time.Second)
	seen, err := store.SeenWithin(ctx, "key-B", 60*time.Second)
	require.NoError(t, err)
	require.True(t, seen, "second occurrence within window must be seen")
}

func TestIdempotencyStore_ZeroWindowDisabled(t *testing.T) {
	redis, cleanup := testutil.NewTestRedis(t)
	defer cleanup()

	store := trigger.NewRedisIdempotencyStore(redis)
	ctx := context.Background()

	seen1, _ := store.SeenWithin(ctx, "key-C", 0)
	seen2, _ := store.SeenWithin(ctx, "key-C", 0)
	require.False(t, seen1)
	require.False(t, seen2, "zero window disables dedupe entirely")
}
```

- [ ] **Step 2: Run failing test**

```bash
cd src-go && go test ./internal/trigger/ -run TestIdempotencyStore -v
```

Expected: FAIL (undefined).

- [ ] **Step 3: Implement the store**

Write `src-go/internal/trigger/idempotency.go`:

```go
package trigger

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// IdempotencyStore remembers which trigger-firing keys were seen recently,
// so duplicate events within a dedupe window can be skipped.
type IdempotencyStore interface {
	// SeenWithin reports whether key was seen in the last `window` duration.
	// On a first-sight, it also records the key for future checks.
	// A zero window disables deduplication: the key is never recorded and
	// the method always reports false.
	SeenWithin(ctx context.Context, key string, window time.Duration) (bool, error)
}

type RedisIdempotencyStore struct {
	rdb *redis.Client
}

func NewRedisIdempotencyStore(rdb *redis.Client) *RedisIdempotencyStore {
	return &RedisIdempotencyStore{rdb: rdb}
}

func (s *RedisIdempotencyStore) SeenWithin(ctx context.Context, key string, window time.Duration) (bool, error) {
	if window <= 0 {
		return false, nil
	}
	redisKey := "af:trigger:idem:" + key
	// SET NX — if key already existed, this returns false (Go redis "Bool()" returns whether set).
	ok, err := s.rdb.SetNX(ctx, redisKey, "1", window).Result()
	if err != nil {
		return false, err
	}
	return !ok, nil
}
```

- [ ] **Step 4: Run tests**

```bash
cd src-go && go test ./internal/trigger/ -run TestIdempotencyStore -v
```

Expected: PASS (3 tests). If the `testutil.NewTestRedis` helper doesn't exist yet, add it alongside `NewTestDB` in the existing `testutil` package. Search its current shape with `Grep` and mirror the pattern; the helper should return a `*redis.Client` connected to the dev-stack Redis.

- [ ] **Step 5: Commit**

```bash
rtk git add src-go/internal/trigger/idempotency.go src-go/internal/trigger/idempotency_test.go src-go/internal/testutil/
rtk git commit -m "feat(trigger): Redis-backed idempotency store"
```

---

## Task 6 — EmployeeService: CRUD + SetState

**Files:**
- Create: `src-go/internal/employee/service.go`
- Create: `src-go/internal/employee/service_test.go`
- Create: `src-go/internal/employee/errors.go`

- [ ] **Step 1: Write the errors file**

Write `src-go/internal/employee/errors.go`:

```go
package employee

import "errors"

var (
	ErrEmployeeArchived    = errors.New("employee is archived")
	ErrEmployeePaused      = errors.New("employee is paused")
	ErrRoleNotFound        = errors.New("role manifest not found for employee.role_id")
	ErrEmployeeNameExists  = errors.New("employee name already exists in project")
)
```

- [ ] **Step 2: Write failing tests for CRUD + SetState**

Write `src-go/internal/employee/service_test.go`:

```go
package employee_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"agentforge/internal/employee"
	"agentforge/internal/model"
	"agentforge/internal/repository"
	"agentforge/internal/testutil"
)

// fakeRoleRegistry is a minimal stand-in for role.Registry for unit tests.
type fakeRoleRegistry struct {
	known map[string]bool
}

func (f *fakeRoleRegistry) Has(roleID string) bool { return f.known[roleID] }

func TestEmployeeService_Create_RejectsUnknownRole(t *testing.T) {
	ctx := context.Background()
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	project := testutil.SeedProject(t, db)

	svc := employee.NewService(
		repository.NewEmployeeRepository(db),
		&fakeRoleRegistry{known: map[string]bool{"code-reviewer": true}},
		nil, // agentSvc — not exercised here
	)

	_, err := svc.Create(ctx, employee.CreateInput{
		ProjectID: project.ID,
		Name:      "e1",
		RoleID:    "nonexistent-role",
	})
	require.ErrorIs(t, err, employee.ErrRoleNotFound)
}

func TestEmployeeService_CRUD(t *testing.T) {
	ctx := context.Background()
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	project := testutil.SeedProject(t, db)

	svc := employee.NewService(
		repository.NewEmployeeRepository(db),
		&fakeRoleRegistry{known: map[string]bool{"code-reviewer": true}},
		nil,
	)

	emp, err := svc.Create(ctx, employee.CreateInput{
		ProjectID:    project.ID,
		Name:         "reviewer-1",
		RoleID:       "code-reviewer",
		RuntimePrefs: json.RawMessage(`{"runtime":"claude_code"}`),
	})
	require.NoError(t, err)
	require.Equal(t, model.EmployeeStateActive, emp.State)

	got, err := svc.Get(ctx, emp.ID)
	require.NoError(t, err)
	require.Equal(t, emp.ID, got.ID)

	require.NoError(t, svc.SetState(ctx, emp.ID, model.EmployeeStatePaused))
	got, _ = svc.Get(ctx, emp.ID)
	require.Equal(t, model.EmployeeStatePaused, got.State)

	require.NoError(t, svc.Delete(ctx, emp.ID))
	_, err = svc.Get(ctx, emp.ID)
	require.ErrorIs(t, err, repository.ErrNotFound)
}

func TestEmployeeService_NameConflictError(t *testing.T) {
	ctx := context.Background()
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	project := testutil.SeedProject(t, db)

	svc := employee.NewService(
		repository.NewEmployeeRepository(db),
		&fakeRoleRegistry{known: map[string]bool{"code-reviewer": true}},
		nil,
	)
	_, err := svc.Create(ctx, employee.CreateInput{ProjectID: project.ID, Name: "dup", RoleID: "code-reviewer"})
	require.NoError(t, err)
	_, err = svc.Create(ctx, employee.CreateInput{ProjectID: project.ID, Name: "dup", RoleID: "code-reviewer"})
	require.ErrorIs(t, err, employee.ErrEmployeeNameExists)

	// Silence unused import for uuid in case of future edits.
	_ = uuid.Nil
}
```

- [ ] **Step 3: Run failing tests**

```bash
cd src-go && go test ./internal/employee/ -run TestEmployeeService -v
```

Expected: FAIL (undefined).

- [ ] **Step 4: Implement the service (CRUD half)**

Write `src-go/internal/employee/service.go` (Invoke added in Task 7; stub it for now):

```go
package employee

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"agentforge/internal/model"
	"agentforge/internal/repository"
)

// RoleRegistry is the minimum role-lookup dependency the service needs.
// In production this is satisfied by role.Registry.
type RoleRegistry interface {
	Has(roleID string) bool
}

// AgentSpawner is the spawn seam; unit tests pass nil and don't call Invoke.
// In production this is the extended AgentService that accepts EmployeeID.
type AgentSpawner interface {
	SpawnForEmployee(ctx context.Context, in SpawnForEmployeeInput) (*model.AgentRun, error)
}

type SpawnForEmployeeInput struct {
	EmployeeID uuid.UUID
	TaskID     uuid.UUID
	MemberID   uuid.UUID
	Runtime    string
	Provider   string
	Model      string
	RoleID     string
	BudgetUsd  float64
	SystemPromptOverride string
	ExtraSkills          []model.EmployeeSkill
}

type CreateInput struct {
	ProjectID    uuid.UUID
	Name         string
	DisplayName  string
	RoleID       string
	RuntimePrefs json.RawMessage
	Config       json.RawMessage
	CreatedBy    *uuid.UUID
	Skills       []model.EmployeeSkill
}

type UpdateInput struct {
	DisplayName  *string
	RoleID       *string
	RuntimePrefs json.RawMessage
	Config       json.RawMessage
}

type InvokeInput struct {
	EmployeeID     uuid.UUID
	TaskID         uuid.UUID
	ExecutionID    uuid.UUID
	NodeID         string
	Prompt         string
	Context        map[string]any
	BudgetOverride *float64
}

type InvokeResult struct {
	AgentRunID uuid.UUID
}

type Service struct {
	repo         *repository.EmployeeRepository
	roles        RoleRegistry
	agentSpawner AgentSpawner
}

func NewService(repo *repository.EmployeeRepository, roles RoleRegistry, spawner AgentSpawner) *Service {
	return &Service{repo: repo, roles: roles, agentSpawner: spawner}
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*model.Employee, error) {
	if !s.roles.Has(in.RoleID) {
		return nil, ErrRoleNotFound
	}
	e := &model.Employee{
		ProjectID:    in.ProjectID,
		Name:         in.Name,
		DisplayName:  in.DisplayName,
		RoleID:       in.RoleID,
		RuntimePrefs: in.RuntimePrefs,
		Config:       in.Config,
		State:        model.EmployeeStateActive,
		CreatedBy:    in.CreatedBy,
	}
	if err := s.repo.Create(ctx, e); err != nil {
		if errors.Is(err, repository.ErrEmployeeNameConflict) {
			return nil, ErrEmployeeNameExists
		}
		return nil, err
	}
	for _, sk := range in.Skills {
		if err := s.repo.AddSkill(ctx, e.ID, sk); err != nil {
			return nil, fmt.Errorf("add skill %s: %w", sk.SkillPath, err)
		}
	}
	return s.withSkills(ctx, e)
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*model.Employee, error) {
	e, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.withSkills(ctx, e)
}

func (s *Service) ListByProject(ctx context.Context, projectID uuid.UUID, f repository.EmployeeFilter) ([]*model.Employee, error) {
	return s.repo.ListByProject(ctx, projectID, f)
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (*model.Employee, error) {
	e, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if in.DisplayName != nil {
		e.DisplayName = *in.DisplayName
	}
	if in.RoleID != nil {
		if !s.roles.Has(*in.RoleID) {
			return nil, ErrRoleNotFound
		}
		e.RoleID = *in.RoleID
	}
	if in.RuntimePrefs != nil {
		e.RuntimePrefs = in.RuntimePrefs
	}
	if in.Config != nil {
		e.Config = in.Config
	}
	if err := s.repo.Update(ctx, e); err != nil {
		return nil, err
	}
	return s.withSkills(ctx, e)
}

func (s *Service) SetState(ctx context.Context, id uuid.UUID, state model.EmployeeState) error {
	return s.repo.SetState(ctx, id, state)
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *Service) AddSkill(ctx context.Context, employeeID uuid.UUID, sk model.EmployeeSkill) error {
	return s.repo.AddSkill(ctx, employeeID, sk)
}

func (s *Service) RemoveSkill(ctx context.Context, employeeID uuid.UUID, skillPath string) error {
	return s.repo.RemoveSkill(ctx, employeeID, skillPath)
}

// Invoke is implemented in Task 7. Leaving the stub here keeps the package compiling
// while the repo/CRUD paths are being tested.
func (s *Service) Invoke(ctx context.Context, in InvokeInput) (*InvokeResult, error) {
	return nil, errors.New("employee.Service.Invoke: not implemented in Task 6; see Task 7")
}

func (s *Service) withSkills(ctx context.Context, e *model.Employee) (*model.Employee, error) {
	skills, err := s.repo.ListSkills(ctx, e.ID)
	if err != nil {
		return nil, err
	}
	e.Skills = skills
	return e, nil
}
```

- [ ] **Step 5: Run tests**

```bash
cd src-go && go test ./internal/employee/ -run TestEmployeeService -v
```

Expected: PASS (3 tests).

- [ ] **Step 6: Commit**

```bash
rtk git add src-go/internal/employee/
rtk git commit -m "feat(employee): Service with CRUD, skill bindings, state management (Invoke stub)"
```

---

## Task 7 — EmployeeService.Invoke + AgentService.SpawnForEmployee

**Files:**
- Modify: `src-go/internal/employee/service.go`
- Modify: `src-go/internal/employee/service_test.go`
- Modify: `src-go/internal/service/agent_service.go`
- Modify: `src-go/internal/model/agent_run.go` (already done in Task 2)

- [ ] **Step 1: Add Invoke tests**

Append to `src-go/internal/employee/service_test.go`:

```go
// --- Invoke tests ---

type fakeSpawner struct {
	called bool
	last   employee.SpawnForEmployeeInput
	result *model.AgentRun
	err    error
}

func (f *fakeSpawner) SpawnForEmployee(_ context.Context, in employee.SpawnForEmployeeInput) (*model.AgentRun, error) {
	f.called = true
	f.last = in
	if f.err != nil {
		return nil, f.err
	}
	return f.result, nil
}

func TestEmployeeService_Invoke_ArchivedRejected(t *testing.T) {
	ctx := context.Background()
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	project := testutil.SeedProject(t, db)

	spawner := &fakeSpawner{}
	svc := employee.NewService(
		repository.NewEmployeeRepository(db),
		&fakeRoleRegistry{known: map[string]bool{"code-reviewer": true}},
		spawner,
	)
	emp, _ := svc.Create(ctx, employee.CreateInput{ProjectID: project.ID, Name: "arc", RoleID: "code-reviewer"})
	require.NoError(t, svc.SetState(ctx, emp.ID, model.EmployeeStateArchived))

	_, err := svc.Invoke(ctx, employee.InvokeInput{EmployeeID: emp.ID, TaskID: uuid.New()})
	require.ErrorIs(t, err, employee.ErrEmployeeArchived)
	require.False(t, spawner.called)
}

func TestEmployeeService_Invoke_DelegatesToSpawner(t *testing.T) {
	ctx := context.Background()
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	project := testutil.SeedProject(t, db)

	runID := uuid.New()
	spawner := &fakeSpawner{result: &model.AgentRun{ID: runID}}
	svc := employee.NewService(
		repository.NewEmployeeRepository(db),
		&fakeRoleRegistry{known: map[string]bool{"code-reviewer": true}},
		spawner,
	)

	emp, _ := svc.Create(ctx, employee.CreateInput{
		ProjectID:    project.ID,
		Name:         "active",
		RoleID:       "code-reviewer",
		RuntimePrefs: json.RawMessage(`{"runtime":"claude_code","provider":"anthropic","model":"claude-opus-4-7","budgetUsd":7.5}`),
	})
	_ = svc.AddSkill(ctx, emp.ID, model.EmployeeSkill{SkillPath: "skills/typescript", AutoLoad: true})

	taskID := uuid.New()
	out, err := svc.Invoke(ctx, employee.InvokeInput{EmployeeID: emp.ID, TaskID: taskID})
	require.NoError(t, err)
	require.Equal(t, runID, out.AgentRunID)

	require.True(t, spawner.called)
	require.Equal(t, emp.ID, spawner.last.EmployeeID)
	require.Equal(t, taskID, spawner.last.TaskID)
	require.Equal(t, "claude_code", spawner.last.Runtime)
	require.Equal(t, 7.5, spawner.last.BudgetUsd)
	require.Equal(t, "code-reviewer", spawner.last.RoleID)
	require.Len(t, spawner.last.ExtraSkills, 1)
}

func TestEmployeeService_Invoke_BudgetOverride(t *testing.T) {
	ctx := context.Background()
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	project := testutil.SeedProject(t, db)

	spawner := &fakeSpawner{result: &model.AgentRun{ID: uuid.New()}}
	svc := employee.NewService(
		repository.NewEmployeeRepository(db),
		&fakeRoleRegistry{known: map[string]bool{"code-reviewer": true}},
		spawner,
	)
	emp, _ := svc.Create(ctx, employee.CreateInput{
		ProjectID:    project.ID, Name: "b", RoleID: "code-reviewer",
		RuntimePrefs: json.RawMessage(`{"budgetUsd":5}`),
	})
	budget := 10.0
	_, err := svc.Invoke(ctx, employee.InvokeInput{EmployeeID: emp.ID, TaskID: uuid.New(), BudgetOverride: &budget})
	require.NoError(t, err)
	require.Equal(t, 10.0, spawner.last.BudgetUsd)
}
```

- [ ] **Step 2: Replace the Invoke stub**

Replace the stub in `src-go/internal/employee/service.go` with:

```go
func (s *Service) Invoke(ctx context.Context, in InvokeInput) (*InvokeResult, error) {
	emp, err := s.repo.Get(ctx, in.EmployeeID)
	if err != nil {
		return nil, err
	}
	switch emp.State {
	case model.EmployeeStateArchived:
		return nil, ErrEmployeeArchived
	case model.EmployeeStatePaused:
		return nil, ErrEmployeePaused
	}

	prefs, err := decodePrefs(emp.RuntimePrefs)
	if err != nil {
		return nil, fmt.Errorf("decode runtime prefs: %w", err)
	}

	skills, err := s.repo.ListSkills(ctx, emp.ID)
	if err != nil {
		return nil, fmt.Errorf("list employee skills: %w", err)
	}

	systemPromptOverride := extractSystemPromptOverride(emp.Config)

	budget := prefs.BudgetUsd
	if budget <= 0 {
		budget = 5.0
	}
	if in.BudgetOverride != nil && *in.BudgetOverride > 0 {
		budget = *in.BudgetOverride
	}

	run, err := s.agentSpawner.SpawnForEmployee(ctx, SpawnForEmployeeInput{
		EmployeeID:           emp.ID,
		TaskID:               in.TaskID,
		MemberID:             uuid.Nil, // employee-sourced runs aren't attributed to a human member
		Runtime:              prefs.Runtime,
		Provider:             prefs.Provider,
		Model:                prefs.Model,
		RoleID:               emp.RoleID,
		BudgetUsd:            budget,
		SystemPromptOverride: systemPromptOverride,
		ExtraSkills:          skills,
	})
	if err != nil {
		return nil, err
	}
	return &InvokeResult{AgentRunID: run.ID}, nil
}

func decodePrefs(raw json.RawMessage) (model.RuntimePrefs, error) {
	var p model.RuntimePrefs
	if len(raw) == 0 {
		return p, nil
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return p, err
	}
	return p, nil
}

func extractSystemPromptOverride(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		return ""
	}
	if v, ok := m["system_prompt_override"].(string); ok {
		return v
	}
	return ""
}
```

- [ ] **Step 3: Extend AgentService with SpawnForEmployee**

In `src-go/internal/service/agent_service.go`, add after the existing `SpawnForTeam` function:

```go
// SpawnForEmployee dispatches a run on behalf of a persistent Employee.
// The resulting agent_run row carries employee_id so downstream memory
// and history can be isolated per-employee. ExtraSkills and
// SystemPromptOverride are forwarded to the Bridge runtime context.
func (s *AgentService) SpawnForEmployee(ctx context.Context, in employee.SpawnForEmployeeInput) (*model.AgentRun, error) {
	run, err := s.spawnWithContext(ctx, in.TaskID, in.MemberID, in.Runtime, in.Provider, in.Model, in.BudgetUsd, in.RoleID, &bridgeExecutionContext{
		EmployeeID:           &in.EmployeeID,
		ExtraSkills:          in.ExtraSkills,
		SystemPromptOverride: in.SystemPromptOverride,
	})
	if err != nil {
		return nil, err
	}
	// Persist employee_id on the run row. spawnWithContext inserts the row with
	// employee_id=NULL by default; patch after creation to avoid churn in the
	// shared spawn code path.
	if err := s.runRepo.SetEmployeeID(ctx, run.ID, in.EmployeeID); err != nil {
		return nil, fmt.Errorf("set employee_id on run %s: %w", run.ID, err)
	}
	run.EmployeeID = &in.EmployeeID
	return run, nil
}
```

Add to the existing `bridgeExecutionContext` struct (locate and extend):

```go
EmployeeID           *uuid.UUID
ExtraSkills          []model.EmployeeSkill
SystemPromptOverride string
```

Add a new import at the top:

```go
"agentforge/internal/employee"
```

Forward the execution-context fields into whatever payload the bridge call already builds. Look for where `execCtx` is consumed inside `spawnWithContext` and serialize the new fields into the run-context JSON that the bridge already accepts. If a dedicated field for employee metadata doesn't exist in the bridge payload yet, add it as a `context.employee` sub-object (non-breaking, extra properties).

Then add `SetEmployeeID` to the run repository:

```go
// In src-go/internal/repository/agent_run_repository.go — append a new method.
func (r *AgentRunRepository) SetEmployeeID(ctx context.Context, id uuid.UUID, employeeID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `UPDATE agent_runs SET employee_id = $2 WHERE id = $1`, id, employeeID)
	return err
}
```

- [ ] **Step 4: Run employee tests**

```bash
cd src-go && go test ./internal/employee/ -v
```

Expected: PASS (all Invoke tests).

- [ ] **Step 5: Build the full tree to catch fallout**

```bash
cd src-go && go build ./...
```

Expected: clean build.

- [ ] **Step 6: Commit**

```bash
rtk git add src-go/internal/employee/ src-go/internal/service/agent_service.go src-go/internal/repository/agent_run_repository.go
rtk git commit -m "feat(employee): Invoke that merges role+skills+prefs and calls AgentService.SpawnForEmployee"
```

---

## Task 8 — Employee YAML loader + Registry.SeedFromDir

**Files:**
- Create: `src-go/internal/employee/registry.go`
- Create: `src-go/internal/employee/registry_test.go`
- Create: `employees/default-code-reviewer.yaml` (sample seed, used by tests + startup)

- [ ] **Step 1: Write the failing test**

Write `src-go/internal/employee/registry_test.go`:

```go
package employee_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"agentforge/internal/employee"
	"agentforge/internal/model"
	"agentforge/internal/repository"
	"agentforge/internal/testutil"
)

func TestRegistry_SeedFromDir_UpsertsAcrossProjects(t *testing.T) {
	ctx := context.Background()
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	projectA := testutil.SeedProject(t, db)
	projectB := testutil.SeedProject(t, db)

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "default-code-reviewer.yaml"), []byte(`
apiVersion: agentforge/v1
kind: Employee
metadata:
  id: default-code-reviewer
  name: 默认代码评审员
role_id: code-reviewer
runtime_prefs:
  runtime: claude_code
  provider: anthropic
  model: claude-opus-4-7
extra_skills:
  - path: skills/typescript
    auto_load: true
`), 0o644))

	svc := employee.NewService(
		repository.NewEmployeeRepository(db),
		&fakeRoleRegistry{known: map[string]bool{"code-reviewer": true}},
		nil,
	)
	registry := employee.NewRegistry(svc)

	// Seed into every project: test helper list.
	report, err := registry.SeedFromDir(ctx, dir, []uuid.UUID{projectA.ID, projectB.ID})
	require.NoError(t, err)
	require.Equal(t, 2, report.Upserted)

	// Second run is a no-op (idempotent).
	report2, err := registry.SeedFromDir(ctx, dir, []uuid.UUID{projectA.ID, projectB.ID})
	require.NoError(t, err)
	require.Equal(t, 0, report2.Upserted)

	got, _ := svc.ListByProject(ctx, projectA.ID, repository.EmployeeFilter{})
	require.Len(t, got, 1)
	require.Equal(t, "default-code-reviewer", got[0].Name)
	require.Equal(t, model.EmployeeStateActive, got[0].State)
}
```

Remember to add the required import in the test file:

```go
import "github.com/google/uuid"
```

- [ ] **Step 2: Run failing test**

```bash
cd src-go && go test ./internal/employee/ -run TestRegistry_SeedFromDir -v
```

Expected: FAIL (undefined).

- [ ] **Step 3: Implement the loader + registry**

Write `src-go/internal/employee/registry.go`:

```go
package employee

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"

	"agentforge/internal/model"
	"agentforge/internal/repository"
)

// Manifest is the YAML shape for a seeded employee template.
type Manifest struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		ID          string `yaml:"id"`
		Name        string `yaml:"name"`
		DisplayName string `yaml:"displayName"`
	} `yaml:"metadata"`
	RoleID       string         `yaml:"role_id"`
	RuntimePrefs map[string]any `yaml:"runtime_prefs"`
	Config       map[string]any `yaml:"config"`
	ExtraSkills  []struct {
		Path     string `yaml:"path"`
		AutoLoad bool   `yaml:"auto_load"`
	} `yaml:"extra_skills"`
}

type SeedReport struct {
	Upserted int
	Skipped  int
	Errors   []error
}

type Registry struct {
	svc *Service
}

func NewRegistry(svc *Service) *Registry { return &Registry{svc: svc} }

func (r *Registry) SeedFromDir(ctx context.Context, dir string, projectIDs []uuid.UUID) (SeedReport, error) {
	var rep SeedReport
	entries, err := os.ReadDir(dir)
	if err != nil {
		return rep, fmt.Errorf("read dir %s: %w", dir, err)
	}

	for _, ent := range entries {
		if ent.IsDir() || !isYAML(ent.Name()) {
			continue
		}
		m, err := loadManifest(filepath.Join(dir, ent.Name()))
		if err != nil {
			rep.Errors = append(rep.Errors, fmt.Errorf("%s: %w", ent.Name(), err))
			continue
		}
		for _, pid := range projectIDs {
			existing, err := r.findByName(ctx, pid, m.Metadata.ID)
			if err == nil && existing != nil {
				rep.Skipped++
				continue
			}
			_, err = r.svc.Create(ctx, CreateInput{
				ProjectID:    pid,
				Name:         m.Metadata.ID,
				DisplayName:  firstNonEmpty(m.Metadata.DisplayName, m.Metadata.Name),
				RoleID:       m.RoleID,
				RuntimePrefs: mustJSON(m.RuntimePrefs),
				Config:       mustJSON(m.Config),
				Skills:       toSkills(m.ExtraSkills),
			})
			if err != nil {
				rep.Errors = append(rep.Errors, fmt.Errorf("project=%s %s: %w", pid, m.Metadata.ID, err))
				continue
			}
			rep.Upserted++
		}
	}
	return rep, nil
}

func (r *Registry) findByName(ctx context.Context, projectID uuid.UUID, name string) (*model.Employee, error) {
	list, err := r.svc.ListByProject(ctx, projectID, repository.EmployeeFilter{})
	if err != nil {
		return nil, err
	}
	for _, e := range list {
		if e.Name == name {
			return e, nil
		}
	}
	return nil, nil
}

func loadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	if m.Kind != "Employee" {
		return nil, fmt.Errorf("kind must be Employee, got %q", m.Kind)
	}
	if m.Metadata.ID == "" || m.RoleID == "" {
		return nil, fmt.Errorf("manifest missing metadata.id or role_id")
	}
	return &m, nil
}

func toSkills(specs []struct {
	Path     string `yaml:"path"`
	AutoLoad bool   `yaml:"auto_load"`
}) []model.EmployeeSkill {
	out := make([]model.EmployeeSkill, 0, len(specs))
	for _, s := range specs {
		out = append(out, model.EmployeeSkill{SkillPath: s.Path, AutoLoad: s.AutoLoad})
	}
	return out
}

func mustJSON(v any) json.RawMessage {
	if v == nil {
		return json.RawMessage("{}")
	}
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("{}")
	}
	return b
}

func isYAML(name string) bool {
	n := strings.ToLower(name)
	return strings.HasSuffix(n, ".yaml") || strings.HasSuffix(n, ".yml")
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
```

- [ ] **Step 4: Write the sample manifest**

Create `employees/default-code-reviewer.yaml`:

```yaml
apiVersion: agentforge/v1
kind: Employee
metadata:
  id: default-code-reviewer
  name: 默认代码评审员
  displayName: Default Code Reviewer
role_id: code-reviewer
runtime_prefs:
  runtime: claude_code
  provider: anthropic
  model: claude-opus-4-7
  budgetUsd: 7.5
config: {}
extra_skills:
  - path: skills/typescript
    auto_load: true
```

- [ ] **Step 5: Run tests**

```bash
cd src-go && go test ./internal/employee/ -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
rtk git add src-go/internal/employee/registry.go src-go/internal/employee/registry_test.go employees/
rtk git commit -m "feat(employee): YAML registry + seed-from-dir with default-code-reviewer manifest"
```

---

## Task 9 — Employee HTTP handler + routes

**Files:**
- Create: `src-go/internal/handler/employee_handler.go`
- Create: `src-go/internal/handler/employee_handler_test.go`
- Modify: `src-go/internal/handler/routes.go` (or wherever route registration happens)

- [ ] **Step 1: Write the test**

Write `src-go/internal/handler/employee_handler_test.go`:

```go
package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"agentforge/internal/employee"
	"agentforge/internal/handler"
	"agentforge/internal/repository"
	"agentforge/internal/testutil"
)

func TestEmployeeHandler_CreateListGet(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	project := testutil.SeedProject(t, db)
	svc := employee.NewService(
		repository.NewEmployeeRepository(db),
		&fakeRoleRegistry{known: map[string]bool{"code-reviewer": true}},
		nil,
	)
	h := handler.NewEmployeeHandler(svc)
	e := testutil.NewEcho(t, handler.WithAuthAs(project.OwnerMemberID))
	h.Register(e)

	// POST create
	body := `{"name":"bobby","roleId":"code-reviewer"}`
	rec := doJSON(t, e, http.MethodPost, "/api/v1/projects/"+project.ID.String()+"/employees", body)
	require.Equal(t, http.StatusCreated, rec.Code)

	var created map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	require.Equal(t, "bobby", created["name"])

	// GET list
	rec = doJSON(t, e, http.MethodGet, "/api/v1/projects/"+project.ID.String()+"/employees", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var list []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &list))
	require.Len(t, list, 1)
}

// fakeRoleRegistry satisfies employee.RoleRegistry for handler tests.
type fakeRoleRegistry struct{ known map[string]bool }

func (f *fakeRoleRegistry) Has(r string) bool { return f.known[r] }

func doJSON(t *testing.T, e *echo.Echo, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	_ = context.Background()
	return rec
}
```

Add the required imports: `"github.com/labstack/echo/v4"`. Use the repo's existing `testutil.NewEcho` helper; if no such helper exists, search existing handler_test files for the setup pattern and mirror it.

- [ ] **Step 2: Run failing test**

```bash
cd src-go && go test ./internal/handler/ -run TestEmployeeHandler -v
```

Expected: FAIL (undefined).

- [ ] **Step 3: Implement the handler**

Write `src-go/internal/handler/employee_handler.go`:

```go
package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"agentforge/internal/employee"
	"agentforge/internal/model"
	"agentforge/internal/repository"
)

type EmployeeHandler struct {
	svc *employee.Service
}

func NewEmployeeHandler(svc *employee.Service) *EmployeeHandler {
	return &EmployeeHandler{svc: svc}
}

func (h *EmployeeHandler) Register(e *echo.Echo) {
	g := e.Group("/api/v1/projects/:projectId/employees")
	g.GET("", h.list)
	g.POST("", h.create)
	g.GET("/:id", h.get)
	g.PATCH("/:id", h.update)
	g.DELETE("/:id", h.delete)
	g.POST("/:id/state", h.setState)

	sg := e.Group("/api/v1/projects/:projectId/employees/:id/skills")
	sg.GET("", h.listSkills)
	sg.POST("", h.addSkill)
	sg.DELETE("/:skillPath", h.removeSkill)
}

type createReq struct {
	Name         string          `json:"name"`
	DisplayName  string          `json:"displayName"`
	RoleID       string          `json:"roleId"`
	RuntimePrefs json.RawMessage `json:"runtimePrefs"`
	Config       json.RawMessage `json:"config"`
	Skills       []struct {
		SkillPath string `json:"skillPath"`
		AutoLoad  bool   `json:"autoLoad"`
	} `json:"skills"`
}

func (h *EmployeeHandler) create(c echo.Context) error {
	projectID, err := uuid.Parse(c.Param("projectId"))
	if err != nil {
		return badRequest(c, "invalid projectId")
	}
	var req createReq
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid body")
	}
	if req.Name == "" || req.RoleID == "" {
		return badRequest(c, "name and roleId are required")
	}
	createdBy := memberIDFromContext(c) // existing helper used by other handlers

	skills := make([]model.EmployeeSkill, 0, len(req.Skills))
	for _, s := range req.Skills {
		skills = append(skills, model.EmployeeSkill{SkillPath: s.SkillPath, AutoLoad: s.AutoLoad})
	}

	emp, err := h.svc.Create(c.Request().Context(), employee.CreateInput{
		ProjectID:    projectID,
		Name:         req.Name,
		DisplayName:  req.DisplayName,
		RoleID:       req.RoleID,
		RuntimePrefs: req.RuntimePrefs,
		Config:       req.Config,
		CreatedBy:    createdBy,
		Skills:       skills,
	})
	if err != nil {
		switch {
		case errors.Is(err, employee.ErrRoleNotFound):
			return badRequest(c, "unknown roleId")
		case errors.Is(err, employee.ErrEmployeeNameExists):
			return c.JSON(http.StatusConflict, errResp{"employee name already exists"})
		}
		return internalError(c, err)
	}
	return c.JSON(http.StatusCreated, emp)
}

func (h *EmployeeHandler) list(c echo.Context) error {
	projectID, err := uuid.Parse(c.Param("projectId"))
	if err != nil {
		return badRequest(c, "invalid projectId")
	}
	list, err := h.svc.ListByProject(c.Request().Context(), projectID, repository.EmployeeFilter{})
	if err != nil {
		return internalError(c, err)
	}
	return c.JSON(http.StatusOK, list)
}

func (h *EmployeeHandler) get(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return badRequest(c, "invalid id")
	}
	emp, err := h.svc.Get(c.Request().Context(), id)
	if errors.Is(err, repository.ErrNotFound) {
		return c.NoContent(http.StatusNotFound)
	}
	if err != nil {
		return internalError(c, err)
	}
	return c.JSON(http.StatusOK, emp)
}

type updateReq struct {
	DisplayName  *string         `json:"displayName"`
	RoleID       *string         `json:"roleId"`
	RuntimePrefs json.RawMessage `json:"runtimePrefs"`
	Config       json.RawMessage `json:"config"`
}

func (h *EmployeeHandler) update(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return badRequest(c, "invalid id")
	}
	var req updateReq
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid body")
	}
	emp, err := h.svc.Update(c.Request().Context(), id, employee.UpdateInput{
		DisplayName: req.DisplayName, RoleID: req.RoleID,
		RuntimePrefs: req.RuntimePrefs, Config: req.Config,
	})
	if err != nil {
		if errors.Is(err, employee.ErrRoleNotFound) {
			return badRequest(c, "unknown roleId")
		}
		if errors.Is(err, repository.ErrNotFound) {
			return c.NoContent(http.StatusNotFound)
		}
		return internalError(c, err)
	}
	return c.JSON(http.StatusOK, emp)
}

func (h *EmployeeHandler) setState(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return badRequest(c, "invalid id")
	}
	var req struct {
		State model.EmployeeState `json:"state"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid body")
	}
	if req.State != model.EmployeeStateActive && req.State != model.EmployeeStatePaused && req.State != model.EmployeeStateArchived {
		return badRequest(c, "invalid state")
	}
	if err := h.svc.SetState(c.Request().Context(), id, req.State); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return c.NoContent(http.StatusNotFound)
		}
		return internalError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *EmployeeHandler) delete(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return badRequest(c, "invalid id")
	}
	if err := h.svc.Delete(c.Request().Context(), id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return c.NoContent(http.StatusNotFound)
		}
		return internalError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *EmployeeHandler) listSkills(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return badRequest(c, "invalid id")
	}
	emp, err := h.svc.Get(c.Request().Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return c.NoContent(http.StatusNotFound)
		}
		return internalError(c, err)
	}
	return c.JSON(http.StatusOK, emp.Skills)
}

func (h *EmployeeHandler) addSkill(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return badRequest(c, "invalid id")
	}
	var req model.EmployeeSkill
	if err := c.Bind(&req); err != nil || req.SkillPath == "" {
		return badRequest(c, "invalid skill body")
	}
	if err := h.svc.AddSkill(c.Request().Context(), id, req); err != nil {
		return internalError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *EmployeeHandler) removeSkill(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return badRequest(c, "invalid id")
	}
	skillPath := c.Param("skillPath")
	if err := h.svc.RemoveSkill(c.Request().Context(), id, skillPath); err != nil {
		return internalError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

// Helpers (if not already present in this package)
type errResp struct {
	Error string `json:"error"`
}

func badRequest(c echo.Context, msg string) error {
	return c.JSON(http.StatusBadRequest, errResp{msg})
}
func internalError(c echo.Context, err error) error {
	c.Logger().Error(err)
	return c.JSON(http.StatusInternalServerError, errResp{"internal error"})
}
```

If `memberIDFromContext` doesn't exist, mirror how other handlers read the authed member from `echo.Context`. If no auth helper exists, use `nil` for `CreatedBy` in tests.

- [ ] **Step 4: Run tests**

```bash
cd src-go && go test ./internal/handler/ -run TestEmployeeHandler -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
rtk git add src-go/internal/handler/employee_handler.go src-go/internal/handler/employee_handler_test.go
rtk git commit -m "feat(handler): Employee CRUD + state + skills HTTP API"
```

---

## Task 10 — Extend StartExecution with StartOptions

**Files:**
- Modify: `src-go/internal/service/dag_workflow_service.go`
- Modify: `src-go/internal/handler/workflow_handler.go`
- Modify callers across the repo (grep will find them)

- [ ] **Step 1: Find existing callers**

```bash
rtk grep "dagSvc.StartExecution\|s.dagSvc.StartExecution\|DAGWorkflowService) StartExecution" src-go/
```

Record every callsite; all will need the new argument.

- [ ] **Step 2: Add the options struct + change signature**

Edit `src-go/internal/service/dag_workflow_service.go`. Find the existing function:

```go
func (s *DAGWorkflowService) StartExecution(ctx context.Context, workflowID uuid.UUID, taskID *uuid.UUID) (*model.WorkflowExecution, error) {
```

Replace with:

```go
// StartOptions is the set of optional controls for StartExecution.
// Seed pre-populates WorkflowExecution.DataStore, so the first advancement
// can consume external event data (IM payload, cron context, etc.).
// TriggeredBy stamps the execution with the WorkflowTrigger that fired it,
// enabling dashboards and execution filters.
type StartOptions struct {
	Seed        map[string]any
	TriggeredBy *uuid.UUID
}

func (s *DAGWorkflowService) StartExecution(ctx context.Context, workflowID uuid.UUID, taskID *uuid.UUID, opts StartOptions) (*model.WorkflowExecution, error) {
```

Within the body, after the existing `WorkflowExecution` is built but before `AdvanceExecution` runs, apply the options:

```go
	if len(opts.Seed) > 0 {
		if exec.DataStore == nil {
			exec.DataStore = map[string]any{}
		}
		// Seed goes under a dedicated "$event" namespace so template references
		// like "{{$event.pr_url}}" resolve cleanly and never collide with node outputs.
		exec.DataStore["$event"] = opts.Seed
	}
	if opts.TriggeredBy != nil {
		exec.TriggeredBy = opts.TriggeredBy
	}
```

Persist the updated fields via the existing execution-upsert path. If the current code inserts the execution row before this point, add an explicit `execRepo.SetTriggeredBy(ctx, exec.ID, *opts.TriggeredBy)` call or replace the prior `SaveExecution`/`Update` with one that includes these columns — see the repository layer for the pattern.

Also extend the repository:

```go
// In workflow_execution_repository.go
func (r *WorkflowExecutionRepository) SetTriggeredBy(ctx context.Context, id uuid.UUID, triggerID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `UPDATE workflow_executions SET triggered_by = $2 WHERE id = $1`, id, triggerID)
	return err
}
```

And the initial insert SQL (wherever `workflow_executions` rows are inserted) must accept `data_store` with the seed. Locate the insert and ensure the `DataStore` field (JSONB) is marshaled and included.

- [ ] **Step 3: Fix every caller**

For each hit from Step 1, replace the call with the new signature. Most call sites pass no options:

```go
// Before
exec, err := s.dagSvc.StartExecution(ctx, wfID, &taskID)

// After
exec, err := s.dagSvc.StartExecution(ctx, wfID, &taskID, service.StartOptions{})
```

In `handler/workflow_handler.go`, update the HTTP handler similarly — it currently has no seed or trigger, so an empty `StartOptions{}` is correct:

```go
exec, err := h.dagSvc.StartExecution(ctx, workflowID, req.TaskID, service.StartOptions{})
```

- [ ] **Step 4: Build & run existing tests to catch regressions**

```bash
cd src-go && go build ./...
cd src-go && go test ./internal/service/ -run "TestDAG|TestWorkflow" -v
```

Expected: clean build; existing DAG/workflow tests still pass (no test changes needed since defaults reproduce old behavior).

- [ ] **Step 5: Commit**

```bash
rtk git add src-go/internal/service/ src-go/internal/handler/workflow_handler.go src-go/internal/repository/
rtk git commit -m "feat(workflow): extend StartExecution with StartOptions (seed + triggered_by)"
```

---

## Task 11 — llm_agent node accepts employeeId

**Files:**
- Modify: `src-go/internal/workflow/nodetypes/effects.go`
- Modify: `src-go/internal/workflow/nodetypes/llm_agent.go`
- Modify: `src-go/internal/workflow/nodetypes/llm_agent_test.go`

- [ ] **Step 1: Extend `SpawnAgentPayload`**

In `src-go/internal/workflow/nodetypes/effects.go`, extend the struct:

```go
type SpawnAgentPayload struct {
	Runtime    string  `json:"runtime"`
	Provider   string  `json:"provider"`
	Model      string  `json:"model"`
	RoleID     string  `json:"roleId"`
	MemberID   string  `json:"memberId,omitempty"`
	EmployeeID string  `json:"employeeId,omitempty"`
	BudgetUsd  float64 `json:"budgetUsd"`
}
```

- [ ] **Step 2: Extend `llm_agent.go` to read `employeeId`**

In `src-go/internal/workflow/nodetypes/llm_agent.go`, inside `Execute`, after the `memberID` parse block, add:

```go
	employeeID := ""
	if eid, ok := req.Config["employeeId"].(string); ok && eid != "" {
		if _, err := uuid.Parse(eid); err == nil {
			employeeID = eid
		}
	}
```

Include `EmployeeID: employeeID` in the `SpawnAgentPayload` literal that's marshaled to the effect.

Update `ConfigSchema` to list the new property:

```go
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "runtime":    {"type": "string"},
    "provider":   {"type": "string"},
    "model":      {"type": "string"},
    "roleId":     {"type": "string"},
    "memberId":   {"type": "string", "format": "uuid"},
    "employeeId": {"type": "string", "format": "uuid"},
    "budgetUsd":  {"type": "number", "minimum": 0}
  }
}`)
```

- [ ] **Step 3: Extend the existing test**

Append to `src-go/internal/workflow/nodetypes/llm_agent_test.go`:

```go
func TestLLMAgentHandler_EmployeeIDPropagated(t *testing.T) {
	empID := uuid.New()
	req := &NodeExecRequest{
		Execution: &model.WorkflowExecution{TaskID: uuidPtr(uuid.New())},
		Config: map[string]any{
			"runtime":    "claude_code",
			"roleId":     "code-reviewer",
			"employeeId": empID.String(),
		},
	}
	res, err := LLMAgentHandler{}.Execute(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, res.Effects, 1)

	var p SpawnAgentPayload
	require.NoError(t, json.Unmarshal(res.Effects[0].Payload, &p))
	require.Equal(t, empID.String(), p.EmployeeID)
}

func uuidPtr(id uuid.UUID) *uuid.UUID { return &id }
```

- [ ] **Step 4: Run tests**

```bash
cd src-go && go test ./internal/workflow/nodetypes/ -run TestLLMAgentHandler -v
```

Expected: PASS (existing + new).

- [ ] **Step 5: Commit**

```bash
rtk git add src-go/internal/workflow/nodetypes/effects.go src-go/internal/workflow/nodetypes/llm_agent.go src-go/internal/workflow/nodetypes/llm_agent_test.go
rtk git commit -m "feat(workflow): llm_agent node accepts optional employeeId in config"
```

---

## Task 12 — Applier routes Employee-based spawns via EmployeeService

**Files:**
- Modify: `src-go/internal/workflow/nodetypes/applier.go`
- Modify: `src-go/internal/workflow/nodetypes/applier_test.go`

- [ ] **Step 1: Add EmployeeSpawner seam**

In `src-go/internal/workflow/nodetypes/applier.go`, extend:

```go
// EmployeeSpawner dispatches agent runs on behalf of persistent Employees.
// When a spawn_agent effect carries a non-empty EmployeeID, the applier
// prefers this seam over the raw AgentSpawner so employee_id is persisted
// on the resulting agent_run row.
type EmployeeSpawner interface {
	Invoke(ctx context.Context, in EmployeeInvokeInput) (*EmployeeInvokeResult, error)
}

type EmployeeInvokeInput struct {
	EmployeeID  uuid.UUID
	TaskID      uuid.UUID
	ExecutionID uuid.UUID
	NodeID      string
	BudgetUsd   float64
}

type EmployeeInvokeResult struct {
	AgentRunID uuid.UUID
}
```

Extend `EffectApplier`:

```go
type EffectApplier struct {
	AgentSpawner    AgentSpawner
	EmployeeSpawner EmployeeSpawner
	MappingRepo     RunMappingRepo
	ReviewRepo      ReviewRepo
}
```

Modify `applySpawnAgent`: after decoding the payload, branch on `EmployeeID`:

```go
	if p.EmployeeID != "" {
		if a.EmployeeSpawner == nil {
			return fmt.Errorf("EmployeeSpawner is nil but employeeId set")
		}
		empID, err := uuid.Parse(p.EmployeeID)
		if err != nil {
			return fmt.Errorf("invalid employeeId: %w", err)
		}
		res, err := a.EmployeeSpawner.Invoke(ctx, EmployeeInvokeInput{
			EmployeeID:  empID,
			TaskID:      *exec.TaskID,
			ExecutionID: exec.ID,
			NodeID:      node.ID,
			BudgetUsd:   p.BudgetUsd,
		})
		if err != nil {
			return fmt.Errorf("employee invoke: %w", err)
		}
		// Persist mapping so completion handlers can wake this node.
		if a.MappingRepo != nil {
			if err := a.MappingRepo.Create(ctx, &model.WorkflowRunMapping{
				ExecutionID: exec.ID, NodeID: node.ID, AgentRunID: res.AgentRunID,
			}); err != nil {
				// Existing behavior: warn but don't fail the spawn.
				log.Printf("warn: create run mapping: %v", err)
			}
		}
		return nil
	}
```

Keep the existing fallback (the `AgentSpawner.Spawn(...)` path below).

- [ ] **Step 2: Extend the test doubles**

In `src-go/internal/workflow/nodetypes/applier_test.go`:

```go
type fakeEmployeeSpawner struct {
	called   bool
	lastIn   EmployeeInvokeInput
	runID    uuid.UUID
	returnErr error
}

func (f *fakeEmployeeSpawner) Invoke(_ context.Context, in EmployeeInvokeInput) (*EmployeeInvokeResult, error) {
	f.called = true
	f.lastIn = in
	if f.returnErr != nil {
		return nil, f.returnErr
	}
	return &EmployeeInvokeResult{AgentRunID: f.runID}, nil
}
```

Add:

```go
func TestApplier_SpawnAgent_RoutesThroughEmployeeSpawner(t *testing.T) {
	runID := uuid.New()
	empID := uuid.New()

	empSpawner := &fakeEmployeeSpawner{runID: runID}
	agtSpawner := &fakeAgentSpawner{}
	mappingRepo := &fakeRunMappingRepo{}

	applier := &EffectApplier{
		AgentSpawner:    agtSpawner,
		EmployeeSpawner: empSpawner,
		MappingRepo:     mappingRepo,
	}

	exec := newExecWithTask()
	payload, _ := json.Marshal(SpawnAgentPayload{
		Runtime: "claude_code", RoleID: "code-reviewer",
		EmployeeID: empID.String(), BudgetUsd: 5,
	})
	effects := []Effect{{Kind: EffectSpawnAgent, Payload: payload}}
	node := &model.WorkflowNode{ID: "n1"}

	parked, err := applier.Apply(context.Background(), exec, node, effects)
	require.NoError(t, err)
	require.True(t, parked)
	require.True(t, empSpawner.called)
	require.False(t, agtSpawner.called, "must not fall through to AgentSpawner when employeeId set")
	require.Equal(t, empID, empSpawner.lastIn.EmployeeID)
	require.Equal(t, runID, mappingRepo.last.AgentRunID)
}
```

- [ ] **Step 3: Run tests**

```bash
cd src-go && go test ./internal/workflow/nodetypes/ -run TestApplier -v
```

Expected: PASS (existing tests still green + new test green).

- [ ] **Step 4: Wire the Employee seam**

At the startup wiring site (likely `cmd/server/main.go` where `EffectApplier` is constructed), inject the employee service through a thin adapter:

```go
// In cmd/server/main.go wiring code.
applier := &nodetypes.EffectApplier{
	AgentSpawner: agentSvc, // existing
	EmployeeSpawner: employeeSpawnerAdapter{svc: employeeSvc},
	MappingRepo: runMappingRepo,
	ReviewRepo: reviewRepo,
}
```

Adapter file: `src-go/internal/employee/applier_adapter.go`:

```go
package employee

import (
	"context"

	"agentforge/internal/workflow/nodetypes"
)

// ApplierAdapter adapts *Service to the nodetypes.EmployeeSpawner interface.
type ApplierAdapter struct {
	Svc *Service
}

func (a ApplierAdapter) Invoke(ctx context.Context, in nodetypes.EmployeeInvokeInput) (*nodetypes.EmployeeInvokeResult, error) {
	res, err := a.Svc.Invoke(ctx, InvokeInput{
		EmployeeID: in.EmployeeID, TaskID: in.TaskID,
		ExecutionID: in.ExecutionID, NodeID: in.NodeID,
		BudgetOverride: &in.BudgetUsd,
	})
	if err != nil {
		return nil, err
	}
	return &nodetypes.EmployeeInvokeResult{AgentRunID: res.AgentRunID}, nil
}
```

- [ ] **Step 5: Commit**

```bash
rtk git add src-go/internal/workflow/nodetypes/ src-go/internal/employee/applier_adapter.go src-go/cmd/server/main.go
rtk git commit -m "feat(workflow): applier routes employee-backed spawns through EmployeeService"
```

---

## Task 13 — TriggerRegistrar: DB ↔ in-memory index + SyncFromDefinition

**Files:**
- Create: `src-go/internal/trigger/registrar.go`
- Create: `src-go/internal/trigger/registrar_test.go`

- [ ] **Step 1: Write the test**

Write `src-go/internal/trigger/registrar_test.go`:

```go
package trigger_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"agentforge/internal/model"
	"agentforge/internal/repository"
	"agentforge/internal/testutil"
	"agentforge/internal/trigger"
)

func TestRegistrar_SyncFromDefinition(t *testing.T) {
	ctx := context.Background()
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	project := testutil.SeedProject(t, db)
	wf := testutil.SeedWorkflowDefinition(t, db, project.ID)

	repo := repository.NewWorkflowTriggerRepository(db)
	reg := trigger.NewRegistrar(repo)

	// Definition with two trigger nodes, one IM and one schedule.
	defNodes := []model.WorkflowNode{
		{
			ID: "trg1", Type: "trigger",
			Config: map[string]any{
				"source": "im",
				"im": map[string]any{
					"platform": "feishu", "command": "/review",
				},
				"input_mapping": map[string]any{"pr_url": "{{$event.args[0]}}"},
			},
		},
		{
			ID: "trg2", Type: "trigger",
			Config: map[string]any{
				"source": "schedule",
				"schedule": map[string]any{
					"cron": "0 9 * * *", "timezone": "Asia/Shanghai",
				},
				"input_mapping": map[string]any{"target_count": 20},
			},
		},
	}

	require.NoError(t, reg.SyncFromDefinition(ctx, wf.ID, project.ID, defNodes, nil))

	got, err := repo.ListByWorkflow(ctx, wf.ID)
	require.NoError(t, err)
	require.Len(t, got, 2)

	sources := map[model.TriggerSource]bool{}
	for _, t := range got {
		sources[t.Source] = true
	}
	require.True(t, sources[model.TriggerSourceIM])
	require.True(t, sources[model.TriggerSourceSchedule])
}

func TestRegistrar_SyncRemovesStaleTriggers(t *testing.T) {
	ctx := context.Background()
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	project := testutil.SeedProject(t, db)
	wf := testutil.SeedWorkflowDefinition(t, db, project.ID)
	repo := repository.NewWorkflowTriggerRepository(db)
	reg := trigger.NewRegistrar(repo)

	firstNodes := []model.WorkflowNode{
		{ID: "trg1", Type: "trigger", Config: map[string]any{
			"source": "im",
			"im":     map[string]any{"platform": "feishu", "command": "/review"},
		}},
	}
	require.NoError(t, reg.SyncFromDefinition(ctx, wf.ID, project.ID, firstNodes, nil))
	all, _ := repo.ListByWorkflow(ctx, wf.ID)
	require.Len(t, all, 1)

	// Remove the trigger node from the definition; sync again.
	require.NoError(t, reg.SyncFromDefinition(ctx, wf.ID, project.ID, []model.WorkflowNode{}, nil))
	all, _ = repo.ListByWorkflow(ctx, wf.ID)
	require.Len(t, all, 0)

	// Silence unused
	_ = json.Marshal
}
```

- [ ] **Step 2: Run the failing test**

```bash
cd src-go && go test ./internal/trigger/ -run TestRegistrar -v
```

Expected: FAIL (undefined).

- [ ] **Step 3: Implement the registrar**

Write `src-go/internal/trigger/registrar.go`:

```go
package trigger

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"agentforge/internal/model"
	"agentforge/internal/repository"
)

// Registrar materializes the subscription config authored on workflow
// trigger nodes into rows in workflow_triggers. It also keeps an in-memory
// source→triggers index so hot paths (EventRouter) don't hit the DB on
// every dispatch.
type Registrar struct {
	repo *repository.WorkflowTriggerRepository
}

func NewRegistrar(repo *repository.WorkflowTriggerRepository) *Registrar {
	return &Registrar{repo: repo}
}

// SyncFromDefinition reconciles the persisted workflow_triggers for a
// workflow against the trigger nodes in its current definition.
//
// For each trigger node in nodes: build a WorkflowTrigger row and upsert.
// Any existing row whose config hash is not in the current set is deleted.
// createdBy is stamped on any newly-inserted row; nil is acceptable for
// system-initiated syncs (e.g., template seed).
func (r *Registrar) SyncFromDefinition(ctx context.Context, workflowID, projectID uuid.UUID, nodes []model.WorkflowNode, createdBy *uuid.UUID) error {
	keep := make(map[uuid.UUID]struct{})

	for _, n := range nodes {
		if n.Type != "trigger" {
			continue
		}
		cfg, ok := n.Config.(map[string]any)
		if !ok {
			continue
		}
		sourceStr, _ := cfg["source"].(string)
		if sourceStr == "manual" || sourceStr == "" {
			continue
		}

		tr, err := buildTriggerFromNode(workflowID, projectID, cfg, createdBy)
		if err != nil {
			return fmt.Errorf("node %s: %w", n.ID, err)
		}
		if err := r.repo.Upsert(ctx, tr); err != nil {
			return err
		}
		keep[tr.ID] = struct{}{}
	}

	// Garbage-collect triggers that are no longer referenced.
	existing, err := r.repo.ListByWorkflow(ctx, workflowID)
	if err != nil {
		return err
	}
	for _, e := range existing {
		if _, ok := keep[e.ID]; ok {
			continue
		}
		if err := r.repo.Delete(ctx, e.ID); err != nil {
			return fmt.Errorf("delete stale trigger %s: %w", e.ID, err)
		}
	}
	return nil
}

func buildTriggerFromNode(workflowID, projectID uuid.UUID, cfg map[string]any, createdBy *uuid.UUID) (*model.WorkflowTrigger, error) {
	sourceStr, _ := cfg["source"].(string)
	src := model.TriggerSource(sourceStr)
	if src != model.TriggerSourceIM && src != model.TriggerSourceSchedule {
		return nil, errors.New("unsupported trigger source: " + sourceStr)
	}

	// Per-source subconfig and input_mapping live under dedicated keys to
	// keep the JSON shape deterministic (hash stability matters for upsert).
	var sourceCfg any
	if src == model.TriggerSourceIM {
		sourceCfg = cfg["im"]
	} else {
		sourceCfg = cfg["schedule"]
	}
	configJSON, err := json.Marshal(sourceCfg)
	if err != nil {
		return nil, fmt.Errorf("marshal source config: %w", err)
	}
	inputMapping := cfg["input_mapping"]
	if inputMapping == nil {
		inputMapping = map[string]any{}
	}
	inputJSON, err := json.Marshal(inputMapping)
	if err != nil {
		return nil, fmt.Errorf("marshal input mapping: %w", err)
	}

	idemTpl, _ := cfg["idempotency_key_template"].(string)
	dedupe := 0
	if v, ok := cfg["dedupe_window_seconds"].(float64); ok {
		dedupe = int(v)
	}
	enabled := true
	if v, ok := cfg["enabled"].(bool); ok {
		enabled = v
	}

	return &model.WorkflowTrigger{
		WorkflowID:             workflowID,
		ProjectID:              projectID,
		Source:                 src,
		Config:                 configJSON,
		InputMapping:           inputJSON,
		IdempotencyKeyTemplate: idemTpl,
		DedupeWindowSeconds:    dedupe,
		Enabled:                enabled,
		CreatedBy:              createdBy,
	}, nil
}
```

- [ ] **Step 4: Run tests**

```bash
cd src-go && go test ./internal/trigger/ -run TestRegistrar -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
rtk git add src-go/internal/trigger/registrar.go src-go/internal/trigger/registrar_test.go
rtk git commit -m "feat(trigger): Registrar.SyncFromDefinition materializes trigger-node config to workflow_triggers"
```

---

## Task 14 — EventRouter.Route (match, render, start)

**Files:**
- Create: `src-go/internal/trigger/router.go`
- Create: `src-go/internal/trigger/router_test.go`

- [ ] **Step 1: Write the test**

Write `src-go/internal/trigger/router_test.go`:

```go
package trigger_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"agentforge/internal/model"
	"agentforge/internal/repository"
	"agentforge/internal/service"
	"agentforge/internal/testutil"
	"agentforge/internal/trigger"
)

type fakeStarter struct {
	calls []struct {
		WorkflowID uuid.UUID
		Opts       service.StartOptions
	}
	err error
}

func (f *fakeStarter) StartExecution(_ context.Context, workflowID uuid.UUID, _ *uuid.UUID, opts service.StartOptions) (*model.WorkflowExecution, error) {
	f.calls = append(f.calls, struct {
		WorkflowID uuid.UUID
		Opts       service.StartOptions
	}{workflowID, opts})
	if f.err != nil {
		return nil, f.err
	}
	return &model.WorkflowExecution{ID: uuid.New(), WorkflowID: workflowID}, nil
}

type nopIdem struct{ seen bool }

func (n *nopIdem) SeenWithin(_ context.Context, _ string, _ time.Duration) (bool, error) {
	s := n.seen
	n.seen = true
	return s, nil
}

func TestRouter_IM_MatchAndStart(t *testing.T) {
	ctx := context.Background()
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	project := testutil.SeedProject(t, db)
	wf := testutil.SeedWorkflowDefinition(t, db, project.ID)
	repo := repository.NewWorkflowTriggerRepository(db)
	reg := trigger.NewRegistrar(repo)
	require.NoError(t, reg.SyncFromDefinition(ctx, wf.ID, project.ID, []model.WorkflowNode{
		{ID: "trg", Type: "trigger", Config: map[string]any{
			"source": "im",
			"im":     map[string]any{"platform": "feishu", "command": "/review"},
			"input_mapping": map[string]any{"pr_url": "{{$event.args.0}}"},
		}},
	}, nil))

	starter := &fakeStarter{}
	router := trigger.NewRouter(repo, starter, &nopIdem{})

	ev := trigger.Event{
		Source: model.TriggerSourceIM,
		Data: map[string]any{
			"platform": "feishu",
			"command":  "/review",
			"args":     []any{"https://github.com/acme/web/pull/42"},
		},
	}
	n, err := router.Route(ctx, ev)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.Len(t, starter.calls, 1)
	require.Equal(t, wf.ID, starter.calls[0].WorkflowID)
	require.Equal(t, "https://github.com/acme/web/pull/42", starter.calls[0].Opts.Seed["pr_url"])

	// Unused silencer
	_ = json.Marshal
	_ = errors.New
}

func TestRouter_IM_NoMatchReturnsZero(t *testing.T) {
	ctx := context.Background()
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	_ = testutil.SeedProject(t, db)
	repo := repository.NewWorkflowTriggerRepository(db)
	router := trigger.NewRouter(repo, &fakeStarter{}, &nopIdem{})

	n, err := router.Route(ctx, trigger.Event{
		Source: model.TriggerSourceIM,
		Data:   map[string]any{"platform": "feishu", "command": "/something-unknown"},
	})
	require.NoError(t, err)
	require.Equal(t, 0, n)
}
```

Add `"time"` to imports.

- [ ] **Step 2: Run the failing test**

```bash
cd src-go && go test ./internal/trigger/ -run TestRouter -v
```

Expected: FAIL (undefined).

- [ ] **Step 3: Implement the router**

Write `src-go/internal/trigger/router.go`:

```go
package trigger

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"agentforge/internal/model"
	"agentforge/internal/repository"
	"agentforge/internal/service"
)

// Starter is the minimum dependency Router needs from the workflow engine.
type Starter interface {
	StartExecution(ctx context.Context, workflowID uuid.UUID, taskID *uuid.UUID, opts service.StartOptions) (*model.WorkflowExecution, error)
}

type Event struct {
	Source model.TriggerSource
	Data   map[string]any // source-specific payload (IM command args, schedule $trigger, etc.)
}

type Router struct {
	repo    *repository.WorkflowTriggerRepository
	starter Starter
	idem    IdempotencyStore
}

func NewRouter(repo *repository.WorkflowTriggerRepository, starter Starter, idem IdempotencyStore) *Router {
	return &Router{repo: repo, starter: starter, idem: idem}
}

// Route fans the event out to every matching, enabled trigger and starts
// one workflow execution per match. Returns the number of executions started.
// Errors while starting one execution do NOT abort the others; the method
// returns the last observed error after iterating.
func (r *Router) Route(ctx context.Context, ev Event) (int, error) {
	triggers, err := r.repo.ListEnabledBySource(ctx, ev.Source)
	if err != nil {
		return 0, fmt.Errorf("list triggers: %w", err)
	}

	var started int
	var lastErr error
	for _, tr := range triggers {
		if !matches(tr, ev) {
			continue
		}
		key, keyErr := renderIdempotencyKey(tr, ev)
		if keyErr != nil {
			lastErr = fmt.Errorf("idem key render %s: %w", tr.ID, keyErr)
			continue
		}
		if key != "" && tr.DedupeWindowSeconds > 0 {
			seen, err := r.idem.SeenWithin(ctx, key, time.Duration(tr.DedupeWindowSeconds)*time.Second)
			if err != nil {
				lastErr = fmt.Errorf("idem check %s: %w", tr.ID, err)
				continue
			}
			if seen {
				continue
			}
		}

		seed, err := renderInputMapping(tr.InputMapping, ev.Data)
		if err != nil {
			lastErr = fmt.Errorf("render input %s: %w", tr.ID, err)
			continue
		}

		triggerID := tr.ID
		if _, err := r.starter.StartExecution(ctx, tr.WorkflowID, nil, service.StartOptions{
			Seed: seed, TriggeredBy: &triggerID,
		}); err != nil {
			lastErr = fmt.Errorf("start execution for trigger %s: %w", tr.ID, err)
			continue
		}
		started++
	}
	return started, lastErr
}

func matches(tr *model.WorkflowTrigger, ev Event) bool {
	if tr.Source != ev.Source {
		return false
	}
	switch ev.Source {
	case model.TriggerSourceIM:
		return matchesIM(tr.Config, ev.Data)
	case model.TriggerSourceSchedule:
		// Schedule triggers are dispatched by cron handler referencing the
		// trigger ID directly; by the time Route is called, membership is
		// implied, so there's nothing more to check here.
		return true
	}
	return false
}

func matchesIM(configJSON json.RawMessage, data map[string]any) bool {
	var cfg struct {
		Platform      string   `json:"platform"`
		Command       string   `json:"command"`
		MatchRegex    string   `json:"match_regex"`
		ChatAllowlist []string `json:"chat_allowlist"`
	}
	_ = json.Unmarshal(configJSON, &cfg)

	if cfg.Platform != "" {
		if plat, _ := data["platform"].(string); plat != cfg.Platform {
			return false
		}
	}
	if cfg.Command != "" {
		if cmd, _ := data["command"].(string); cmd != cfg.Command {
			return false
		}
	}
	if cfg.MatchRegex != "" {
		re, err := regexp.Compile(cfg.MatchRegex)
		if err != nil {
			return false
		}
		content, _ := data["content"].(string)
		if !re.MatchString(content) {
			return false
		}
	}
	if len(cfg.ChatAllowlist) > 0 {
		chat, _ := data["chat_id"].(string)
		found := false
		for _, c := range cfg.ChatAllowlist {
			if c == chat {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func renderIdempotencyKey(tr *model.WorkflowTrigger, ev Event) (string, error) {
	if tr.IdempotencyKeyTemplate == "" {
		return "", nil
	}
	return renderTemplate(tr.IdempotencyKeyTemplate, ev.Data), nil
}

func renderInputMapping(mappingJSON json.RawMessage, data map[string]any) (map[string]any, error) {
	var mapping map[string]any
	if err := json.Unmarshal(mappingJSON, &mapping); err != nil {
		return nil, err
	}
	out := make(map[string]any, len(mapping))
	for k, v := range mapping {
		if s, ok := v.(string); ok {
			out[k] = renderTemplate(s, data)
		} else {
			out[k] = v
		}
	}
	return out, nil
}

// renderTemplate resolves references like "{{$event.args.0}}" or
// "{{$event.pr_url}}" against ev.Data. Unresolvable paths render as empty.
// Whole-string template ("{{ $event.args.0 }}") preserves the referenced
// value's type; embedded templates ("prefix-{{$event.id}}") stringify.
var templateExpr = regexp.MustCompile(`\{\{\s*\$event\.([a-zA-Z0-9_.\[\]]+)\s*\}\}`)

func renderTemplate(tmpl string, data map[string]any) any {
	// Whole-template case: no wrapping chars → return the resolved value
	// directly (preserves numbers/arrays).
	if m := templateExpr.FindStringSubmatch(tmpl); m != nil && m[0] == strings.TrimSpace(tmpl) {
		return lookupPath(data, m[1])
	}
	// Embedded case: stringify every substitution.
	return templateExpr.ReplaceAllStringFunc(tmpl, func(match string) string {
		m := templateExpr.FindStringSubmatch(match)
		v := lookupPath(data, m[1])
		if v == nil {
			return ""
		}
		return fmt.Sprint(v)
	})
}

// lookupPath walks "a.b.0.c" against data; integer segments index arrays.
func lookupPath(root map[string]any, path string) any {
	var cur any = root
	for _, seg := range strings.Split(path, ".") {
		switch v := cur.(type) {
		case map[string]any:
			cur = v[seg]
		case []any:
			var idx int
			_, err := fmt.Sscanf(seg, "%d", &idx)
			if err != nil || idx < 0 || idx >= len(v) {
				return nil
			}
			cur = v[idx]
		default:
			return nil
		}
		if cur == nil {
			return nil
		}
	}
	return cur
}
```

- [ ] **Step 4: Run tests**

```bash
cd src-go && go test ./internal/trigger/ -run TestRouter -v
```

Expected: PASS (2 tests).

- [ ] **Step 5: Commit**

```bash
rtk git add src-go/internal/trigger/router.go src-go/internal/trigger/router_test.go
rtk git commit -m "feat(trigger): EventRouter matches triggers, renders input mapping, starts executions"
```

---

## Task 15 — Trigger materialization on workflow save

**Files:**
- Modify: wherever workflow definitions are saved (grep for `defRepo.Save` / `defRepo.Upsert` / `UpdateDefinition` in `internal/service/`)

- [ ] **Step 1: Locate the save seam**

```bash
rtk grep "func.*(Save|Upsert|Create|Update).*Workflow.*(Definition|Def\b)" src-go/internal/service/
```

Identify the single method where a `WorkflowDefinition` is persisted after edits. The goal is to call `registrar.SyncFromDefinition(...)` in the same transaction as the save.

- [ ] **Step 2: Inject the registrar**

Thread `*trigger.Registrar` into the workflow service that owns the save. Add a field, add a constructor argument, update the wiring in `cmd/server/main.go`. Follow the existing DI pattern in this package.

- [ ] **Step 3: Call SyncFromDefinition on save**

Inside the save function, after the definition row is successfully written:

```go
if err := s.triggerRegistrar.SyncFromDefinition(ctx, def.ID, def.ProjectID, def.Nodes, ownerMemberID); err != nil {
    // Best-effort policy: log and return the error so UI surfaces it.
    return nil, fmt.Errorf("sync triggers: %w", err)
}
```

- [ ] **Step 4: Add integration test**

Create `src-go/internal/service/workflow_trigger_sync_test.go`:

```go
package service_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"agentforge/internal/model"
	"agentforge/internal/repository"
	"agentforge/internal/testutil"
)

func TestWorkflowDefinitionSave_MaterializesTriggers(t *testing.T) {
	ctx := context.Background()
	stack, cleanup := testutil.NewServiceStack(t) // existing helper that spins up services with DB
	defer cleanup()

	project := testutil.SeedProject(t, stack.DB)
	def := &model.WorkflowDefinition{
		ProjectID: project.ID,
		Name:      "m",
		Status:    "active",
		Nodes: []model.WorkflowNode{
			{ID: "trg1", Type: "trigger", Config: map[string]any{
				"source": "im",
				"im":     map[string]any{"platform": "feishu", "command": "/review"},
			}},
		},
	}
	_, err := stack.WorkflowDefSvc.Save(ctx, def, nil)
	require.NoError(t, err)

	triggers, err := repository.NewWorkflowTriggerRepository(stack.DB).ListByWorkflow(ctx, def.ID)
	require.NoError(t, err)
	require.Len(t, triggers, 1)
	require.Equal(t, model.TriggerSourceIM, triggers[0].Source)
}
```

If `testutil.NewServiceStack` doesn't exist, build the minimal service composition inline within the test, mirroring how production wires `WorkflowDefService` with its registrar.

- [ ] **Step 5: Run test**

```bash
cd src-go && go test ./internal/service/ -run TestWorkflowDefinitionSave_MaterializesTriggers -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
rtk git add src-go/internal/service/
rtk git commit -m "feat(workflow): call TriggerRegistrar.SyncFromDefinition on workflow save"
```

---

## Task 16 — HandleExternalEvent HTTP endpoint

**Files:**
- Modify: `src-go/internal/handler/workflow_handler.go`

- [ ] **Step 1: Add the route in the workflow handler**

Locate `Register` (or equivalent) in `src-go/internal/handler/workflow_handler.go` and add:

```go
e.POST("/api/v1/workflow-executions/:executionId/events", h.HandleExternalEvent)
```

If the handler interface doesn't yet declare this, add:

```go
func (h *WorkflowHandler) HandleExternalEvent(c echo.Context) error {
	execID, err := uuid.Parse(c.Param("executionId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid executionId"})
	}
	var req struct {
		NodeID  string          `json:"nodeId"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
	}
	if req.NodeID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "nodeId is required"})
	}
	if err := h.dagSvc.HandleExternalEvent(c.Request().Context(), execID, req.NodeID, req.Payload); err != nil {
		c.Logger().Error(err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to handle event"})
	}
	return c.NoContent(http.StatusAccepted)
}
```

- [ ] **Step 2: Add handler test**

Create `src-go/internal/handler/workflow_external_event_test.go`:

```go
package handler_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"agentforge/internal/testutil"
)

func TestHandleExternalEvent_BadExecutionID(t *testing.T) {
	e := testutil.NewEcho(t)
	// Register the workflow handler as production does.
	testutil.RegisterWorkflowHandler(t, e)

	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/workflow-executions/not-a-uuid/events",
		strings.NewReader(`{"nodeId":"n1","payload":{}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleExternalEvent_MissingNodeID(t *testing.T) {
	e := testutil.NewEcho(t)
	testutil.RegisterWorkflowHandler(t, e)

	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/workflow-executions/"+uuid.New().String()+"/events",
		strings.NewReader(`{"payload":{}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}
```

The full happy-path integration test lives in Task 17.

- [ ] **Step 3: Run the tests**

```bash
cd src-go && go test ./internal/handler/ -run TestHandleExternalEvent -v
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
rtk git add src-go/internal/handler/workflow_handler.go src-go/internal/handler/workflow_external_event_test.go
rtk git commit -m "feat(workflow): HTTP POST /api/v1/workflow-executions/:id/events → HandleExternalEvent"
```

---

## Task 17 — End-to-end integration test

**Files:**
- Create: `src-go/internal/integration/employee_trigger_integration_test.go`

- [ ] **Step 1: Write the integration test**

Write `src-go/internal/integration/employee_trigger_integration_test.go`:

```go
package integration_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"agentforge/internal/employee"
	"agentforge/internal/model"
	"agentforge/internal/repository"
	"agentforge/internal/service"
	"agentforge/internal/testutil"
	"agentforge/internal/trigger"
)

// End-to-end check:
//   1. Seed project, role, employee.
//   2. Save a workflow with a trigger node (source=im, command=/review)
//      and one llm_agent node bound to the employee.
//   3. Fire an IM event through the router.
//   4. Assert: execution started with seed, llm_agent node dispatched through
//      EmployeeService.Invoke, and the resulting agent_run carries employee_id.
func TestEmployeeTrigger_EndToEnd(t *testing.T) {
	ctx := context.Background()
	stack, cleanup := testutil.NewServiceStack(t) // provides DB, Redis, DAGService, WorkflowDefService, EmployeeService, TriggerRegistrar, Router — all wired as production
	defer cleanup()

	project := testutil.SeedProject(t, stack.DB)

	// 1. Seed an employee (role must exist in the YAML role registry used by stack).
	emp, err := stack.EmployeeSvc.Create(ctx, employee.CreateInput{
		ProjectID:    project.ID,
		Name:         "e2e-reviewer",
		RoleID:       "code-reviewer",
		RuntimePrefs: json.RawMessage(`{"runtime":"fake","provider":"fake","model":"fake","budgetUsd":1}`),
	})
	require.NoError(t, err)

	// 2. Save a minimal workflow: [trigger] -> [llm_agent bound to employee]
	def := &model.WorkflowDefinition{
		ProjectID: project.ID,
		Name:      "e2e-review",
		Status:    "active",
		Nodes: []model.WorkflowNode{
			{ID: "trg", Type: "trigger", Config: map[string]any{
				"source": "im",
				"im":     map[string]any{"platform": "feishu", "command": "/review"},
				"input_mapping": map[string]any{"pr_url": "{{$event.args.0}}"},
			}},
			{ID: "llm", Type: "llm_agent", Config: map[string]any{
				"employeeId": emp.ID.String(),
				"runtime":    "fake",
			}},
		},
		Edges: []model.WorkflowEdge{{Source: "trg", Target: "llm"}},
	}
	_, err = stack.WorkflowDefSvc.Save(ctx, def, nil)
	require.NoError(t, err)

	// 3. Fire IM event through the router.
	n, err := stack.Router.Route(ctx, trigger.Event{
		Source: model.TriggerSourceIM,
		Data: map[string]any{
			"platform": "feishu",
			"command":  "/review",
			"args":     []any{"https://github.com/acme/web/pull/42"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, 1, n)

	// 4. Find the execution, advance until llm_agent parks, verify agent_run has employee_id.
	// In the stack, Router starts the execution via dagSvc.StartExecution, which
	// immediately fires AdvanceExecution; the fake spawner in testutil
	// records a synthetic agent_run with employee_id set.
	runs := testutil.GetLastAgentRuns(t, stack.DB, project.ID, 1)
	require.Len(t, runs, 1)
	require.NotNil(t, runs[0].EmployeeID)
	require.Equal(t, emp.ID, *runs[0].EmployeeID)

	// Execution DataStore must carry the seed under $event.
	exec := testutil.GetLastExecution(t, stack.DB, def.ID)
	require.Equal(t, "https://github.com/acme/web/pull/42",
		exec.DataStore["$event"].(map[string]any)["pr_url"])
	require.NotNil(t, exec.TriggeredBy)

	// Silence unused
	_ = uuid.Nil
	_ = service.StartOptions{}
	_ = repository.ErrNotFound
}
```

The `testutil` helpers referenced (`NewServiceStack`, `GetLastAgentRuns`, `GetLastExecution`) collect the production wiring in one place. If those helpers don't yet exist, create them in `src-go/internal/testutil/` as part of this task — mirror the real `cmd/server/main.go` composition, substituting a fake bridge runtime so `AgentService.SpawnForEmployee` completes synchronously in tests (write a row to `agent_runs` with status=`completed`, then return it). The bridge fake can be a direct repo insert; no real HTTP needed.

- [ ] **Step 2: Run the test**

```bash
cd src-go && go test ./internal/integration/ -run TestEmployeeTrigger_EndToEnd -v
```

Expected: PASS.

- [ ] **Step 3: Run the entire backend test suite**

```bash
cd src-go && go test ./...
```

Expected: all tests green. Any failures stem from signature changes in Task 10 and 12; fix those callers the same way (pass `StartOptions{}`; inject `EmployeeSpawner` where an applier is constructed).

- [ ] **Step 4: Commit**

```bash
rtk git add src-go/internal/integration/ src-go/internal/testutil/
rtk git commit -m "test(integration): end-to-end employee+trigger path (event → workflow → agent run with employee_id)"
```

---

## Task 18 — Startup wiring & seed hook

**Files:**
- Modify: `src-go/cmd/server/main.go`

- [ ] **Step 1: Wire repositories, services, registrar, router**

In `main.go`, after the DB pool and Redis client are constructed, add:

```go
employeeRepo := repository.NewEmployeeRepository(dbPool)
triggerRepo := repository.NewWorkflowTriggerRepository(dbPool)
idemStore := trigger.NewRedisIdempotencyStore(rdb)

employeeSvc := employee.NewService(employeeRepo, roleRegistry, agentSvc /* implements SpawnForEmployee */)
triggerRegistrar := trigger.NewRegistrar(triggerRepo)
triggerRouter := trigger.NewRouter(triggerRepo, dagSvc, idemStore)
```

Inject `triggerRegistrar` into the workflow definition service (Task 15).

Inject `employee.ApplierAdapter{Svc: employeeSvc}` into `nodetypes.EffectApplier.EmployeeSpawner`.

Register the Employee HTTP handler:

```go
handler.NewEmployeeHandler(employeeSvc).Register(e)
```

- [ ] **Step 2: Seed `employees/*.yaml` on startup**

After the workflow template seeding step, add:

```go
projects, err := projectRepo.ListActive(ctx)
if err != nil {
    return fmt.Errorf("list active projects for employee seed: %w", err)
}
projectIDs := make([]uuid.UUID, 0, len(projects))
for _, p := range projects {
    projectIDs = append(projectIDs, p.ID)
}
employeeRegistry := employee.NewRegistry(employeeSvc)
if report, err := employeeRegistry.SeedFromDir(ctx, "employees", projectIDs); err != nil {
    log.Printf("warn: employee seed: %v", err)
} else {
    log.Printf("employee seed: upserted=%d skipped=%d errors=%d", report.Upserted, report.Skipped, len(report.Errors))
}
```

If `projectRepo.ListActive` doesn't exist, add a minimal method to the project repository:

```go
func (r *ProjectRepository) ListActive(ctx context.Context) ([]*model.Project, error) {
	rows, err := r.db.Query(ctx, `SELECT id FROM projects WHERE archived_at IS NULL`)
	// scan into []*model.Project with only ID populated — enough for the seed path.
}
```

- [ ] **Step 3: Verify boot**

```bash
pnpm dev:backend:verify
```

Expected: Go orchestrator comes up, migration 062 already applied from Task 1, employee seed log line appears (e.g., `upserted=0 skipped=N` on second boot, `upserted=N skipped=0` on first boot after migration).

- [ ] **Step 4: Commit**

```bash
rtk git add src-go/cmd/server/main.go src-go/internal/repository/project_repository.go
rtk git commit -m "feat(server): wire EmployeeService, TriggerRegistrar, Router; seed employees on startup"
```

---

## Self-Review Checklist

Before handing off, verify every section of `docs/superpowers/specs/2026-04-19-employee-and-workflow-trigger-foundation-design.md` is represented by at least one task:

| Spec section | Implemented by |
|---|---|
| §6.1 new tables (employees, employee_skills, workflow_triggers) | Task 1 |
| §6.2 agent_runs.employee_id, agent_memory scope/employee_id, workflow_executions.triggered_by, reviews.execution_id | Task 1 + Task 2 |
| §7.1 EmployeeService CRUD + SetState | Task 6 |
| §7.1 EmployeeService.Invoke + AgentService.SpawnForEmployee | Task 7 |
| §7.1 Employee YAML seed + registry | Task 8 |
| §7.1 llm_agent integration via applier | Tasks 11 + 12 |
| §7.2 TriggerRegistrar.SyncFromDefinition | Task 13 |
| §7.2 EventRouter.Route | Task 14 |
| §7.2 Trigger materialization on workflow save | Task 15 |
| §7.2 HandleExternalEvent HTTP endpoint | Task 16 |
| §7.3 StartOptions (seed + triggered_by) | Task 10 |
| §7.4 (frontend) | **Plan 2** (out of scope) |
| §7.2 IM adapter (`/api/v1/triggers/im/events`, `/workflow` command) | **Plan 2** |
| §7.2 Schedule adapter | **Plan 2** |
| §7.3 Review adaptation + `system:code-review` template | **Plan 2** |
| §10 manual verification checklist items | **Plan 2** (requires adapters + UI) |
| §11.4 Seed: project-creation hook for `default-code-reviewer` | **Plan 2** (requires adapters to demo) |

Plan 1 delivers the *verifiable backend foundation*: all Go-side plumbing that Plan 2 will wire into adapters, review refactor, and frontend. Success criterion for Plan 1: the integration test in Task 17 passes against live Postgres + Redis, and `pnpm dev:backend:verify` comes up cleanly.
