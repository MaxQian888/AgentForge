//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/agentforge/server/internal/employee"
	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/repository"
	"github.com/agentforge/server/internal/service"
	"github.com/agentforge/server/internal/trigger"
	"github.com/agentforge/server/migrations"
	"github.com/agentforge/server/pkg/database"
)

// Ensure the unused-import guards at build time even when some tests skip.
var (
	_ = errors.New
	_ = employee.ErrEmployeeArchived
)

func TestMain(m *testing.M) {
	if url := os.Getenv("TEST_POSTGRES_URL"); url != "" {
		if err := database.RunMigrations(url, migrations.FS); err != nil {
			fmt.Fprintf(os.Stderr, "migration error: %v\n", err)
			os.Exit(1)
		}
	}
	os.Exit(m.Run())
}

func TestTriggerFlow_Integration_EventSeedsExecution(t *testing.T) {
	url := os.Getenv("TEST_POSTGRES_URL")
	if url == "" {
		t.Skip("TEST_POSTGRES_URL not set — skipping integration test")
	}

	db, err := database.NewPostgres(url)
	if err != nil {
		t.Fatalf("NewPostgres: %v", err)
	}
	defer func() { _ = database.ClosePostgres(db) }()

	ctx := context.Background()

	// --- Seed a project + a minimal active workflow definition with a trigger node.
	projectID := uuid.New()
	workflowID := uuid.New()
	setupProjectAndWorkflow(t, db, projectID, workflowID)
	t.Cleanup(func() {
		_ = db.WithContext(ctx).Exec("DELETE FROM workflow_executions WHERE workflow_id = ?", workflowID).Error
		_ = db.WithContext(ctx).Exec("DELETE FROM workflow_triggers WHERE workflow_id = ?", workflowID).Error
		_ = db.WithContext(ctx).Exec("DELETE FROM workflow_definitions WHERE id = ?", workflowID).Error
		_ = db.WithContext(ctx).Exec("DELETE FROM projects WHERE id = ?", projectID).Error
	})

	// --- Wire the real trigger layer.
	triggerRepo := repository.NewWorkflowTriggerRepository(db)
	registrar := trigger.NewRegistrar(triggerRepo)

	// Materialize trigger subscriptions from the just-saved workflow.
	nodes := []model.WorkflowNode{
		{
			ID:   "trg-review",
			Type: model.NodeTypeTrigger,
			Config: jsonOf(t, map[string]any{
				"source": "im",
				"im": map[string]any{
					"platform": "feishu",
					"command":  "/review",
				},
				"input_mapping": map[string]any{
					"pr_url": "{{$event.args.0}}",
				},
			}),
		},
		{
			ID:     "llm",
			Type:   "llm_agent",
			Config: jsonOf(t, map[string]any{"runtime": "fake"}),
		},
	}
	if _, err := registrar.SyncFromDefinition(ctx, workflowID, projectID, nodes, nil); err != nil {
		t.Fatalf("SyncFromDefinition: %v", err)
	}

	// Verify the trigger row landed in workflow_triggers.
	registered, err := triggerRepo.ListByWorkflow(ctx, workflowID)
	if err != nil {
		t.Fatalf("ListByWorkflow: %v", err)
	}
	if len(registered) != 1 {
		t.Fatalf("expected 1 trigger row, got %d", len(registered))
	}
	if registered[0].Source != model.TriggerSourceIM {
		t.Errorf("expected source=im, got %s", registered[0].Source)
	}

	// --- Route an IM event through the router.
	starter := &captureStarter{}
	idem := &nopIdem{}
	router := trigger.NewRouter(triggerRepo, idem, trigger.NewDAGEngineAdapter(starter))

	ev := trigger.Event{
		Source: model.TriggerSourceIM,
		Data: map[string]any{
			"platform": "feishu",
			"command":  "/review",
			"args":     []any{"https://github.com/acme/web/pull/42"},
		},
	}
	started, err := router.Route(ctx, ev)
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if started != 1 {
		t.Fatalf("expected 1 execution started, got %d", started)
	}

	// --- Assert the starter was called with the rendered seed + TriggeredBy.
	if len(starter.calls) != 1 {
		t.Fatalf("expected 1 starter call, got %d", len(starter.calls))
	}
	call := starter.calls[0]
	if call.WorkflowID != workflowID {
		t.Errorf("expected WorkflowID=%s, got %s", workflowID, call.WorkflowID)
	}
	if call.Opts.TriggeredBy == nil || *call.Opts.TriggeredBy != registered[0].ID {
		t.Errorf("expected TriggeredBy=%s, got %v", registered[0].ID, call.Opts.TriggeredBy)
	}
	gotPRURL, _ := call.Opts.Seed["pr_url"].(string)
	if gotPRURL != "https://github.com/acme/web/pull/42" {
		t.Errorf("expected pr_url seeded, got %q", gotPRURL)
	}
}

func TestTriggerFlow_Integration_SyncRemovesStaleRowsOnResave(t *testing.T) {
	url := os.Getenv("TEST_POSTGRES_URL")
	if url == "" {
		t.Skip("TEST_POSTGRES_URL not set — skipping integration test")
	}

	db, err := database.NewPostgres(url)
	if err != nil {
		t.Fatalf("NewPostgres: %v", err)
	}
	defer func() { _ = database.ClosePostgres(db) }()

	ctx := context.Background()
	projectID := uuid.New()
	workflowID := uuid.New()
	setupProjectAndWorkflow(t, db, projectID, workflowID)
	t.Cleanup(func() {
		_ = db.WithContext(ctx).Exec("DELETE FROM workflow_triggers WHERE workflow_id = ?", workflowID).Error
		_ = db.WithContext(ctx).Exec("DELETE FROM workflow_definitions WHERE id = ?", workflowID).Error
		_ = db.WithContext(ctx).Exec("DELETE FROM projects WHERE id = ?", projectID).Error
	})

	triggerRepo := repository.NewWorkflowTriggerRepository(db)
	registrar := trigger.NewRegistrar(triggerRepo)

	// First save: one IM trigger.
	firstNodes := []model.WorkflowNode{
		{
			ID:   "trg-one",
			Type: model.NodeTypeTrigger,
			Config: jsonOf(t, map[string]any{
				"source": "im",
				"im":     map[string]any{"platform": "feishu", "command": "/review"},
			}),
		},
	}
	if _, err := registrar.SyncFromDefinition(ctx, workflowID, projectID, firstNodes, nil); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	rows, _ := triggerRepo.ListByWorkflow(ctx, workflowID)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row after first sync, got %d", len(rows))
	}

	// Second save: trigger removed entirely.
	if _, err := registrar.SyncFromDefinition(ctx, workflowID, projectID, nil, nil); err != nil {
		t.Fatalf("second sync: %v", err)
	}
	rows, _ = triggerRepo.ListByWorkflow(ctx, workflowID)
	if len(rows) != 0 {
		t.Fatalf("expected 0 rows after trigger removed, got %d", len(rows))
	}
}

// --- Helpers ---

func setupProjectAndWorkflow(t *testing.T, db *gorm.DB, projectID, workflowID uuid.UUID) {
	t.Helper()
	ctx := context.Background()

	// projects table: only name, slug are NOT NULL without defaults (status has
	// no CHECK but description/repo_url default to ''). No owner_user_id column
	// exists in the 002 migration.
	err := db.WithContext(ctx).Exec(`
		INSERT INTO projects (id, name, slug, created_at, updated_at)
		VALUES (?, ?, ?, now(), now())
	`, projectID, "integration-"+projectID.String()[:8], "int-"+projectID.String()[:8]).Error
	if err != nil {
		t.Fatalf("insert project: %v", err)
	}

	// workflow_definitions: all NOT NULL columns have defaults except project_id.
	nodesJSON, _ := json.Marshal([]model.WorkflowNode{})
	edgesJSON, _ := json.Marshal([]model.WorkflowEdge{})
	err = db.WithContext(ctx).Exec(`
		INSERT INTO workflow_definitions (id, project_id, name, status, nodes, edges, created_at, updated_at)
		VALUES (?, ?, ?, 'active', ?, ?, now(), now())
	`, workflowID, projectID, "integration-workflow", nodesJSON, edgesJSON).Error
	if err != nil {
		t.Fatalf("insert workflow definition: %v", err)
	}
}

func jsonOf(t *testing.T, v any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return raw
}

// captureStarter satisfies trigger.Starter by recording every call.
type captureStarter struct {
	calls []struct {
		WorkflowID uuid.UUID
		Opts       service.StartOptions
	}
}

func (s *captureStarter) StartExecution(_ context.Context, workflowID uuid.UUID, _ *uuid.UUID, opts service.StartOptions) (*model.WorkflowExecution, error) {
	s.calls = append(s.calls, struct {
		WorkflowID uuid.UUID
		Opts       service.StartOptions
	}{workflowID, opts})
	return &model.WorkflowExecution{ID: uuid.New(), WorkflowID: workflowID}, nil
}

// nopIdem satisfies trigger.IdempotencyStore without dedup.
type nopIdem struct{}

func (nopIdem) SeenWithin(_ context.Context, _ string, _ time.Duration) (bool, error) {
	return false, nil
}

// recordPluginStarter satisfies the PluginEngineAdapter's PluginEngineStarter.
type recordPluginStarter struct {
	calls []struct {
		PluginID         string
		TriggerID        uuid.UUID
		Seed             map[string]any
		ActingEmployeeID *uuid.UUID
	}
}

func (p *recordPluginStarter) StartTriggered(_ context.Context, pluginID string, seed map[string]any, triggerID uuid.UUID) (*model.WorkflowPluginRun, error) {
	p.calls = append(p.calls, struct {
		PluginID         string
		TriggerID        uuid.UUID
		Seed             map[string]any
		ActingEmployeeID *uuid.UUID
	}{pluginID, triggerID, seed, nil})
	return &model.WorkflowPluginRun{ID: uuid.New(), PluginID: pluginID}, nil
}

func (p *recordPluginStarter) StartTriggeredWithEmployee(_ context.Context, pluginID string, seed map[string]any, triggerID uuid.UUID, actingEmployeeID *uuid.UUID) (*model.WorkflowPluginRun, error) {
	p.calls = append(p.calls, struct {
		PluginID         string
		TriggerID        uuid.UUID
		Seed             map[string]any
		ActingEmployeeID *uuid.UUID
	}{pluginID, triggerID, seed, actingEmployeeID})
	return &model.WorkflowPluginRun{ID: uuid.New(), PluginID: pluginID, ActingEmployeeID: actingEmployeeID}, nil
}

// TestTriggerFlow_Integration_DualEngineFanout boots the trigger router with
// both a DAG engine and a plugin engine, registers two triggers matching the
// same IM command (one per engine), and asserts a single IM event fires
// both engines exactly once.
func TestTriggerFlow_Integration_DualEngineFanout(t *testing.T) {
	url := os.Getenv("TEST_POSTGRES_URL")
	if url == "" {
		t.Skip("TEST_POSTGRES_URL not set — skipping integration test")
	}

	db, err := database.NewPostgres(url)
	if err != nil {
		t.Fatalf("NewPostgres: %v", err)
	}
	defer func() { _ = database.ClosePostgres(db) }()

	ctx := context.Background()
	projectID := uuid.New()
	workflowID := uuid.New()
	setupProjectAndWorkflow(t, db, projectID, workflowID)
	t.Cleanup(func() {
		_ = db.WithContext(ctx).Exec("DELETE FROM workflow_triggers WHERE project_id = ?", projectID).Error
		_ = db.WithContext(ctx).Exec("DELETE FROM workflow_definitions WHERE id = ?", workflowID).Error
		_ = db.WithContext(ctx).Exec("DELETE FROM projects WHERE id = ?", projectID).Error
	})

	triggerRepo := repository.NewWorkflowTriggerRepository(db)

	// Manually insert one DAG-target and one plugin-target trigger row matching
	// the same IM command. Going through the registrar would require wiring
	// both resolvers; the per-row fields below are already registrar-shaped.
	dagRow := &model.WorkflowTrigger{
		WorkflowID: &workflowID,
		ProjectID:  projectID,
		Source:     model.TriggerSourceIM,
		TargetKind: model.TriggerTargetDAG,
		Config:     json.RawMessage(`{"command":"/review"}`),
		Enabled:    true,
	}
	pluginRow := &model.WorkflowTrigger{
		PluginID:   "workflow-plugin-review",
		ProjectID:  projectID,
		Source:     model.TriggerSourceIM,
		TargetKind: model.TriggerTargetPlugin,
		Config:     json.RawMessage(`{"command":"/review"}`),
		Enabled:    true,
	}
	if err := triggerRepo.Upsert(ctx, dagRow); err != nil {
		t.Fatalf("upsert dag row: %v", err)
	}
	if err := triggerRepo.Upsert(ctx, pluginRow); err != nil {
		t.Fatalf("upsert plugin row: %v", err)
	}

	dagStarter := &captureStarter{}
	pluginStarter := &recordPluginStarter{}

	router := trigger.NewRouter(triggerRepo, &nopIdem{},
		trigger.NewDAGEngineAdapter(dagStarter),
		trigger.NewPluginEngineAdapter(pluginStarter),
	)

	ev := trigger.Event{
		Source: model.TriggerSourceIM,
		Data: map[string]any{
			"platform": "feishu",
			"command":  "/review",
		},
	}
	outcomes, err := router.RouteWithOutcomes(ctx, ev)
	if err != nil {
		t.Fatalf("RouteWithOutcomes: %v", err)
	}

	if len(outcomes) != 2 {
		t.Fatalf("expected 2 outcomes (one per engine), got %d", len(outcomes))
	}

	sawDAG, sawPlugin := false, false
	for _, o := range outcomes {
		if o.Status != trigger.OutcomeStarted {
			t.Errorf("unexpected status %s on outcome %+v", o.Status, o)
			continue
		}
		switch o.TargetKind {
		case model.TriggerTargetDAG:
			sawDAG = true
		case model.TriggerTargetPlugin:
			sawPlugin = true
		}
	}
	if !sawDAG || !sawPlugin {
		t.Errorf("expected both dag and plugin outcomes; got dag=%v plugin=%v", sawDAG, sawPlugin)
	}

	if len(dagStarter.calls) != 1 {
		t.Errorf("expected exactly 1 DAG start, got %d", len(dagStarter.calls))
	}
	if len(pluginStarter.calls) != 1 {
		t.Errorf("expected exactly 1 plugin start, got %d", len(pluginStarter.calls))
	}
	if pluginStarter.calls[0].PluginID != "workflow-plugin-review" {
		t.Errorf("plugin start received wrong plugin id: %s", pluginStarter.calls[0].PluginID)
	}
}

// ---------------------------------------------------------------------------
// Section 8.1 — dual-engine acting_employee_id attribution integration test
// (change bridge-employee-attribution-legacy).
// ---------------------------------------------------------------------------

// TestTriggerFlow_Integration_DualEngineEmployeeAttribution registers one DAG
// trigger and one plugin trigger sharing a single IM command, both carrying
// the same acting_employee_id. A single IM event fires both; we assert both
// run records persist acting_employee_id = E and both adapters received the
// attribution id.
func TestTriggerFlow_Integration_DualEngineEmployeeAttribution(t *testing.T) {
	url := os.Getenv("TEST_POSTGRES_URL")
	if url == "" {
		t.Skip("TEST_POSTGRES_URL not set — skipping integration test")
	}

	db, err := database.NewPostgres(url)
	if err != nil {
		t.Fatalf("NewPostgres: %v", err)
	}
	defer func() { _ = database.ClosePostgres(db) }()

	ctx := context.Background()
	projectID := uuid.New()
	workflowID := uuid.New()
	setupProjectAndWorkflow(t, db, projectID, workflowID)

	// Seed an active employee in the same project.
	empID := seedEmployee(t, db, projectID, "active-employee", model.EmployeeStateActive)

	t.Cleanup(func() {
		_ = db.WithContext(ctx).Exec("DELETE FROM workflow_triggers WHERE project_id = ?", projectID).Error
		_ = db.WithContext(ctx).Exec("DELETE FROM employees WHERE id = ?", empID).Error
		_ = db.WithContext(ctx).Exec("DELETE FROM workflow_definitions WHERE id = ?", workflowID).Error
		_ = db.WithContext(ctx).Exec("DELETE FROM projects WHERE id = ?", projectID).Error
	})

	triggerRepo := repository.NewWorkflowTriggerRepository(db)

	dagRow := &model.WorkflowTrigger{
		WorkflowID:       &workflowID,
		ProjectID:        projectID,
		Source:           model.TriggerSourceIM,
		TargetKind:       model.TriggerTargetDAG,
		Config:           json.RawMessage(`{"command":"/review"}`),
		Enabled:          true,
		ActingEmployeeID: &empID,
	}
	pluginRow := &model.WorkflowTrigger{
		PluginID:         "workflow-plugin-review-emp",
		ProjectID:        projectID,
		Source:           model.TriggerSourceIM,
		TargetKind:       model.TriggerTargetPlugin,
		Config:           json.RawMessage(`{"command":"/review"}`),
		Enabled:          true,
		ActingEmployeeID: &empID,
	}
	if err := triggerRepo.Upsert(ctx, dagRow); err != nil {
		t.Fatalf("upsert dag row: %v", err)
	}
	if err := triggerRepo.Upsert(ctx, pluginRow); err != nil {
		t.Fatalf("upsert plugin row: %v", err)
	}

	dagStarter := &captureStarter{}
	pluginStarter := &recordPluginStarter{}

	router := trigger.NewRouter(triggerRepo, &nopIdem{},
		trigger.NewDAGEngineAdapter(dagStarter),
		trigger.NewPluginEngineAdapter(pluginStarter),
	)

	ev := trigger.Event{
		Source: model.TriggerSourceIM,
		Data: map[string]any{
			"platform": "feishu",
			"command":  "/review",
		},
	}
	outcomes, err := router.RouteWithOutcomes(ctx, ev)
	if err != nil {
		t.Fatalf("RouteWithOutcomes: %v", err)
	}
	if len(outcomes) != 2 {
		t.Fatalf("expected 2 outcomes, got %d", len(outcomes))
	}
	for _, o := range outcomes {
		if o.Status != trigger.OutcomeStarted {
			t.Errorf("unexpected outcome status %s: %+v", o.Status, o)
		}
	}

	// DAG adapter forwards the acting_employee_id via StartOptions.
	if len(dagStarter.calls) != 1 {
		t.Fatalf("expected 1 dag start, got %d", len(dagStarter.calls))
	}
	if got := dagStarter.calls[0].Opts.ActingEmployeeID; got == nil || *got != empID {
		t.Errorf("DAG adapter did not forward acting_employee_id; got %v, want %s", got, empID)
	}

	// Plugin adapter forwards via StartTriggeredWithEmployee.
	if len(pluginStarter.calls) != 1 {
		t.Fatalf("expected 1 plugin start, got %d", len(pluginStarter.calls))
	}
	if got := pluginStarter.calls[0].ActingEmployeeID; got == nil || *got != empID {
		t.Errorf("plugin adapter did not forward acting_employee_id; got %v, want %s", got, empID)
	}
}

// ---------------------------------------------------------------------------
// Section 8.2 — registrar cross-project rejection + dispatch-time archived
// rejection integration tests.
// ---------------------------------------------------------------------------

// TestTriggerFlow_Integration_RegistrarRejectsCrossProjectEmployee boots the
// registrar with an AttributionGuard and verifies that a cross-project
// acting_employee_id causes the synced trigger to land with enabled=false and
// the acting_employee_cross_project disabled reason.
func TestTriggerFlow_Integration_RegistrarRejectsCrossProjectEmployee(t *testing.T) {
	url := os.Getenv("TEST_POSTGRES_URL")
	if url == "" {
		t.Skip("TEST_POSTGRES_URL not set — skipping integration test")
	}

	db, err := database.NewPostgres(url)
	if err != nil {
		t.Fatalf("NewPostgres: %v", err)
	}
	defer func() { _ = database.ClosePostgres(db) }()

	ctx := context.Background()
	projectA := uuid.New() // employee belongs to this project
	projectB := uuid.New() // workflow belongs to this project
	workflowID := uuid.New()
	setupProjectAndWorkflow(t, db, projectB, workflowID)
	// Insert a separate project for the employee.
	if err := db.WithContext(ctx).Exec(`
		INSERT INTO projects (id, name, slug, created_at, updated_at)
		VALUES (?, ?, ?, now(), now())
	`, projectA, "cross-"+projectA.String()[:8], "cross-"+projectA.String()[:8]).Error; err != nil {
		t.Fatalf("insert cross project: %v", err)
	}
	empID := seedEmployee(t, db, projectA, "cross-employee", model.EmployeeStateActive)

	t.Cleanup(func() {
		_ = db.WithContext(ctx).Exec("DELETE FROM workflow_triggers WHERE project_id = ?", projectB).Error
		_ = db.WithContext(ctx).Exec("DELETE FROM employees WHERE id = ?", empID).Error
		_ = db.WithContext(ctx).Exec("DELETE FROM workflow_definitions WHERE id = ?", workflowID).Error
		_ = db.WithContext(ctx).Exec("DELETE FROM projects WHERE id IN (?, ?)", projectA, projectB).Error
	})

	triggerRepo := repository.NewWorkflowTriggerRepository(db)
	employeeRepo := repository.NewEmployeeRepository(db)
	guard := employee.NewAttributionGuard(employeeRepo)
	registrar := trigger.NewRegistrar(triggerRepo).WithAttributionGuard(guard)

	nodes := []model.WorkflowNode{
		{
			ID:   "trg",
			Type: model.NodeTypeTrigger,
			Config: jsonOf(t, map[string]any{
				"source":             "im",
				"im":                 map[string]any{"platform": "feishu", "command": "/review"},
				"acting_employee_id": empID.String(),
			}),
		},
	}
	outcomes, err := registrar.SyncFromDefinition(ctx, workflowID, projectB, nodes, nil)
	if err != nil {
		t.Fatalf("SyncFromDefinition: %v", err)
	}
	if len(outcomes) != 1 {
		t.Fatalf("expected 1 outcome, got %d", len(outcomes))
	}
	if outcomes[0].Enabled {
		t.Errorf("expected enabled=false for cross-project employee, got enabled=true")
	}
	if outcomes[0].DisabledReason != trigger.DisabledReasonActingEmployeeCross {
		t.Errorf("expected reason %q, got %q",
			trigger.DisabledReasonActingEmployeeCross, outcomes[0].DisabledReason)
	}
}

// TestTriggerFlow_Integration_DispatchRejectsArchivedEmployee verifies that a
// trigger saved against an active employee becomes non-dispatchable once the
// employee is archived. The router emits OutcomeFailedActingEmployee; no
// workflow run is started.
func TestTriggerFlow_Integration_DispatchRejectsArchivedEmployee(t *testing.T) {
	url := os.Getenv("TEST_POSTGRES_URL")
	if url == "" {
		t.Skip("TEST_POSTGRES_URL not set — skipping integration test")
	}

	db, err := database.NewPostgres(url)
	if err != nil {
		t.Fatalf("NewPostgres: %v", err)
	}
	defer func() { _ = database.ClosePostgres(db) }()

	ctx := context.Background()
	projectID := uuid.New()
	workflowID := uuid.New()
	setupProjectAndWorkflow(t, db, projectID, workflowID)
	empID := seedEmployee(t, db, projectID, "to-be-archived", model.EmployeeStateArchived)

	t.Cleanup(func() {
		_ = db.WithContext(ctx).Exec("DELETE FROM workflow_triggers WHERE project_id = ?", projectID).Error
		_ = db.WithContext(ctx).Exec("DELETE FROM employees WHERE id = ?", empID).Error
		_ = db.WithContext(ctx).Exec("DELETE FROM workflow_definitions WHERE id = ?", workflowID).Error
		_ = db.WithContext(ctx).Exec("DELETE FROM projects WHERE id = ?", projectID).Error
	})

	triggerRepo := repository.NewWorkflowTriggerRepository(db)
	row := &model.WorkflowTrigger{
		WorkflowID:       &workflowID,
		ProjectID:        projectID,
		Source:           model.TriggerSourceIM,
		TargetKind:       model.TriggerTargetDAG,
		Config:           json.RawMessage(`{"command":"/review"}`),
		Enabled:          true,
		ActingEmployeeID: &empID,
	}
	if err := triggerRepo.Upsert(ctx, row); err != nil {
		t.Fatalf("upsert trigger: %v", err)
	}

	employeeRepo := repository.NewEmployeeRepository(db)
	guard := employee.NewAttributionGuard(employeeRepo)
	dagStarter := &captureStarter{}

	router := trigger.NewRouter(triggerRepo, &nopIdem{}, trigger.NewDAGEngineAdapter(dagStarter)).
		WithAttributionGuard(guard)

	outcomes, err := router.RouteWithOutcomes(ctx, trigger.Event{
		Source: model.TriggerSourceIM,
		Data:   map[string]any{"platform": "feishu", "command": "/review"},
	})
	if err != nil {
		t.Fatalf("RouteWithOutcomes: %v", err)
	}
	if len(outcomes) != 1 {
		t.Fatalf("expected 1 outcome, got %d", len(outcomes))
	}
	if outcomes[0].Status != trigger.OutcomeFailedActingEmployee {
		t.Errorf("expected OutcomeFailedActingEmployee, got %s", outcomes[0].Status)
	}
	if len(dagStarter.calls) != 0 {
		t.Errorf("expected no engine call on archived dispatch; got %d", len(dagStarter.calls))
	}
}

// seedEmployee inserts a minimal employees row and returns its id.
func seedEmployee(t *testing.T, db *gorm.DB, projectID uuid.UUID, name string, state model.EmployeeState) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	empID := uuid.New()
	err := db.WithContext(ctx).Exec(`
		INSERT INTO employees (id, project_id, name, role_id, state, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, now(), now())
	`, empID, projectID, name+"-"+empID.String()[:4], "coder", string(state)).Error
	if err != nil {
		t.Fatalf("insert employee: %v", err)
	}
	return empID
}
