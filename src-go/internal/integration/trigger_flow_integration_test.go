//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
	"github.com/react-go-quick-starter/server/internal/service"
	"github.com/react-go-quick-starter/server/internal/trigger"
	"github.com/react-go-quick-starter/server/migrations"
	"github.com/react-go-quick-starter/server/pkg/database"
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
	if err := registrar.SyncFromDefinition(ctx, workflowID, projectID, nodes, nil); err != nil {
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
	router := trigger.NewRouter(triggerRepo, starter, idem)

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
	if err := registrar.SyncFromDefinition(ctx, workflowID, projectID, firstNodes, nil); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	rows, _ := triggerRepo.ListByWorkflow(ctx, workflowID)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row after first sync, got %d", len(rows))
	}

	// Second save: trigger removed entirely.
	if err := registrar.SyncFromDefinition(ctx, workflowID, projectID, nil, nil); err != nil {
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
