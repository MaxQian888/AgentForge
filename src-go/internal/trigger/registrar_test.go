package trigger_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/trigger"
)

// ---------------------------------------------------------------------------
// Mock repository
// ---------------------------------------------------------------------------

type mockTriggerRepo struct {
	rows        map[uuid.UUID]*model.WorkflowTrigger
	upsertCount int
	deleteCount int
}

func newMockTriggerRepo() *mockTriggerRepo {
	return &mockTriggerRepo{rows: make(map[uuid.UUID]*model.WorkflowTrigger)}
}

func (m *mockTriggerRepo) Upsert(_ context.Context, t *model.WorkflowTrigger) error {
	m.upsertCount++
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	// Copy to avoid aliasing issues.
	cp := *t
	m.rows[t.ID] = &cp
	return nil
}

func (m *mockTriggerRepo) ListByWorkflow(_ context.Context, workflowID uuid.UUID) ([]*model.WorkflowTrigger, error) {
	var out []*model.WorkflowTrigger
	for _, r := range m.rows {
		if r.WorkflowID == workflowID {
			cp := *r
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (m *mockTriggerRepo) Delete(_ context.Context, id uuid.UUID) error {
	m.deleteCount++
	delete(m.rows, id)
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func mustRawMessage(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return json.RawMessage(b)
}

func makeNode(id, nodeType string, cfg any) model.WorkflowNode {
	node := model.WorkflowNode{
		ID:   id,
		Type: nodeType,
	}
	if cfg != nil {
		node.Config = mustRawMessage(cfg)
	}
	return node
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestRegistrar_SyncFromDefinition_UpsertsIMAndScheduleTriggers verifies that
// two trigger nodes (one IM, one schedule) produce two persisted rows with the
// right sources.
func TestRegistrar_SyncFromDefinition_UpsertsIMAndScheduleTriggers(t *testing.T) {
	repo := newMockTriggerRepo()
	reg := trigger.NewRegistrar(repo)

	wfID := uuid.New()
	projID := uuid.New()

	nodes := []model.WorkflowNode{
		makeNode("n1", model.NodeTypeTrigger, map[string]any{
			"source": "im",
			"im":     map[string]any{"channel": "#general"},
		}),
		makeNode("n2", model.NodeTypeTrigger, map[string]any{
			"source":   "schedule",
			"schedule": map[string]any{"cron": "0 9 * * 1-5"},
		}),
	}

	if err := reg.SyncFromDefinition(context.Background(), wfID, projID, nodes, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if repo.upsertCount != 2 {
		t.Errorf("expected 2 upserts, got %d", repo.upsertCount)
	}
	if len(repo.rows) != 2 {
		t.Errorf("expected 2 rows in store, got %d", len(repo.rows))
	}

	sources := make(map[model.TriggerSource]bool)
	for _, row := range repo.rows {
		sources[row.Source] = true
	}
	if !sources[model.TriggerSourceIM] {
		t.Error("expected an IM trigger row")
	}
	if !sources[model.TriggerSourceSchedule] {
		t.Error("expected a schedule trigger row")
	}
}

// TestRegistrar_SyncFromDefinition_DeletesStaleTriggers verifies that rows
// pre-existing in the repo but absent from the node list are deleted.
func TestRegistrar_SyncFromDefinition_DeletesStaleTriggers(t *testing.T) {
	repo := newMockTriggerRepo()
	reg := trigger.NewRegistrar(repo)

	wfID := uuid.New()
	projID := uuid.New()

	// Pre-populate a stale row.
	staleID := uuid.New()
	repo.rows[staleID] = &model.WorkflowTrigger{
		ID:         staleID,
		WorkflowID: wfID,
		ProjectID:  projID,
		Source:     model.TriggerSourceIM,
		Config:     json.RawMessage(`{}`),
	}

	// Sync with empty node list — nothing to keep.
	if err := reg.SyncFromDefinition(context.Background(), wfID, projID, nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repo.rows) != 0 {
		t.Errorf("expected 0 rows after deleting stale trigger, got %d", len(repo.rows))
	}
	if repo.deleteCount != 1 {
		t.Errorf("expected 1 delete call, got %d", repo.deleteCount)
	}
}

// TestRegistrar_SyncFromDefinition_SkipsManualAndMissingConfig verifies that
// manual-source and empty-config trigger nodes produce no rows and no deletions.
func TestRegistrar_SyncFromDefinition_SkipsManualAndMissingConfig(t *testing.T) {
	repo := newMockTriggerRepo()
	reg := trigger.NewRegistrar(repo)

	wfID := uuid.New()
	projID := uuid.New()

	nodes := []model.WorkflowNode{
		// Explicit manual source — should be skipped.
		makeNode("n1", model.NodeTypeTrigger, map[string]any{"source": "manual"}),
		// Empty config — should be skipped.
		{ID: "n2", Type: model.NodeTypeTrigger},
	}

	if err := reg.SyncFromDefinition(context.Background(), wfID, projID, nodes, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if repo.upsertCount != 0 {
		t.Errorf("expected 0 upserts, got %d", repo.upsertCount)
	}
	if repo.deleteCount != 0 {
		t.Errorf("expected 0 deletes, got %d", repo.deleteCount)
	}
	if len(repo.rows) != 0 {
		t.Errorf("expected 0 rows, got %d", len(repo.rows))
	}
}

// TestRegistrar_SyncFromDefinition_ErrorsOnUnsupportedSource verifies that a
// trigger node with source="webhook" causes SyncFromDefinition to return an
// error and no row to be created.
func TestRegistrar_SyncFromDefinition_ErrorsOnUnsupportedSource(t *testing.T) {
	repo := newMockTriggerRepo()
	reg := trigger.NewRegistrar(repo)

	wfID := uuid.New()
	projID := uuid.New()

	nodes := []model.WorkflowNode{
		makeNode("n1", model.NodeTypeTrigger, map[string]any{"source": "webhook"}),
	}

	err := reg.SyncFromDefinition(context.Background(), wfID, projID, nodes, nil)
	if err == nil {
		t.Fatal("expected an error for unsupported source, got nil")
	}

	if repo.upsertCount != 0 {
		t.Errorf("expected 0 upserts on error path, got %d", repo.upsertCount)
	}
}

// TestRegistrar_SyncFromDefinition_EnabledDefaultsTrue verifies that when the
// trigger node config omits the "enabled" field, the upserted row has Enabled==true.
func TestRegistrar_SyncFromDefinition_EnabledDefaultsTrue(t *testing.T) {
	repo := newMockTriggerRepo()
	reg := trigger.NewRegistrar(repo)

	wfID := uuid.New()
	projID := uuid.New()

	nodes := []model.WorkflowNode{
		makeNode("n1", model.NodeTypeTrigger, map[string]any{
			"source": "im",
			"im":     map[string]any{},
			// "enabled" deliberately omitted
		}),
	}

	if err := reg.SyncFromDefinition(context.Background(), wfID, projID, nodes, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repo.rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(repo.rows))
	}
	for _, row := range repo.rows {
		if !row.Enabled {
			t.Errorf("expected Enabled=true (default), got false")
		}
	}
}

// TestRegistrar_SyncFromDefinition_PropagatesInputMappingAndIdempotency verifies
// that input_mapping, idempotency_key_template, and dedupe_window_seconds are
// all persisted correctly on the upserted row.
func TestRegistrar_SyncFromDefinition_PropagatesInputMappingAndIdempotency(t *testing.T) {
	repo := newMockTriggerRepo()
	reg := trigger.NewRegistrar(repo)

	wfID := uuid.New()
	projID := uuid.New()

	nodes := []model.WorkflowNode{
		makeNode("n1", model.NodeTypeTrigger, map[string]any{
			"source": "schedule",
			"schedule": map[string]any{"cron": "0 0 * * *"},
			"input_mapping": map[string]any{
				"task_id": "{{event.task_id}}",
			},
			"idempotency_key_template": "{{workflow_id}}-{{run_date}}",
			"dedupe_window_seconds":    30,
		}),
	}

	if err := reg.SyncFromDefinition(context.Background(), wfID, projID, nodes, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repo.rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(repo.rows))
	}

	for _, row := range repo.rows {
		if row.IdempotencyKeyTemplate != "{{workflow_id}}-{{run_date}}" {
			t.Errorf("IdempotencyKeyTemplate: got %q, want %q",
				row.IdempotencyKeyTemplate, "{{workflow_id}}-{{run_date}}")
		}
		if row.DedupeWindowSeconds != 30 {
			t.Errorf("DedupeWindowSeconds: got %d, want 30", row.DedupeWindowSeconds)
		}

		// Verify input_mapping contains the expected key.
		var im map[string]any
		if err := json.Unmarshal(row.InputMapping, &im); err != nil {
			t.Fatalf("failed to unmarshal InputMapping: %v", err)
		}
		if im["task_id"] != "{{event.task_id}}" {
			t.Errorf("InputMapping[task_id]: got %v, want {{event.task_id}}", im["task_id"])
		}
	}
}
