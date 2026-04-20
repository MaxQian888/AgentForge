package trigger_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/react-go-quick-starter/server/internal/employee"
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
	deletedIDs  []uuid.UUID
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
		if r.WorkflowID != nil && *r.WorkflowID == workflowID {
			cp := *r
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (m *mockTriggerRepo) Delete(_ context.Context, id uuid.UUID) error {
	m.deleteCount++
	m.deletedIDs = append(m.deletedIDs, id)
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

	if _, err := reg.SyncFromDefinition(context.Background(), wfID, projID, nodes, nil); err != nil {
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
	wfRef := wfID
	repo.rows[staleID] = &model.WorkflowTrigger{
		ID:         staleID,
		WorkflowID: &wfRef,
		ProjectID:  projID,
		Source:     model.TriggerSourceIM,
		TargetKind: model.TriggerTargetDAG,
		Config:     json.RawMessage(`{}`),
	}

	// Sync with empty node list — nothing to keep.
	if _, err := reg.SyncFromDefinition(context.Background(), wfID, projID, nil, nil); err != nil {
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

	if _, err := reg.SyncFromDefinition(context.Background(), wfID, projID, nodes, nil); err != nil {
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

	_, err := reg.SyncFromDefinition(context.Background(), wfID, projID, nodes, nil)
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

	if _, err := reg.SyncFromDefinition(context.Background(), wfID, projID, nodes, nil); err != nil {
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

	if _, err := reg.SyncFromDefinition(context.Background(), wfID, projID, nodes, nil); err != nil {
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

// ---------------------------------------------------------------------------
// Target-kind aware tests (Task 3.4)
// ---------------------------------------------------------------------------

type stubDAGResolver struct {
	defs map[uuid.UUID]*model.WorkflowDefinition
}

func (s *stubDAGResolver) GetByID(_ context.Context, id uuid.UUID) (*model.WorkflowDefinition, error) {
	if def, ok := s.defs[id]; ok {
		return def, nil
	}
	return nil, errors.New("not found")
}

type stubPluginResolver struct {
	records map[string]*model.PluginRecord
}

func (s *stubPluginResolver) GetByID(_ context.Context, id string) (*model.PluginRecord, error) {
	if rec, ok := s.records[id]; ok {
		return rec, nil
	}
	return nil, errors.New("not found")
}

// Plugin-target trigger syncs with plugin_id and defaults Enabled=true when
// no resolver is wired.
func TestRegistrar_SyncFromDefinition_PluginTargetNoResolver(t *testing.T) {
	repo := newMockTriggerRepo()
	reg := trigger.NewRegistrar(repo)

	wfID := uuid.New()
	projID := uuid.New()

	nodes := []model.WorkflowNode{
		makeNode("n1", model.NodeTypeTrigger, map[string]any{
			"source":      "im",
			"target_kind": "plugin",
			"plugin_id":   "workflow-plugin-x",
			"im":          map[string]any{"command": "/review"},
		}),
	}

	outcomes, err := reg.SyncFromDefinition(context.Background(), wfID, projID, nodes, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(outcomes) != 1 {
		t.Fatalf("expected 1 outcome, got %d", len(outcomes))
	}
	if outcomes[0].TargetKind != model.TriggerTargetPlugin {
		t.Errorf("expected target_kind=plugin, got %s", outcomes[0].TargetKind)
	}
	if !outcomes[0].Enabled {
		t.Errorf("expected enabled=true when no resolver validates")
	}
	for _, row := range repo.rows {
		if row.PluginID != "workflow-plugin-x" {
			t.Errorf("plugin_id not propagated: %q", row.PluginID)
		}
		if row.WorkflowID != nil {
			t.Errorf("plugin-target row should not carry workflow_id")
		}
	}
}

// Plugin-target trigger missing plugin_id is persisted with a disabled reason.
func TestRegistrar_SyncFromDefinition_PluginTargetMissingPluginID(t *testing.T) {
	repo := newMockTriggerRepo()
	reg := trigger.NewRegistrar(repo)

	wfID := uuid.New()
	projID := uuid.New()

	nodes := []model.WorkflowNode{
		makeNode("n1", model.NodeTypeTrigger, map[string]any{
			"source":      "im",
			"target_kind": "plugin",
			// plugin_id deliberately omitted
			"im": map[string]any{"command": "/review"},
		}),
	}

	outcomes, err := reg.SyncFromDefinition(context.Background(), wfID, projID, nodes, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(outcomes) != 1 || outcomes[0].Enabled {
		t.Fatalf("expected 1 disabled outcome, got %+v", outcomes)
	}
	if outcomes[0].DisabledReason != trigger.DisabledReasonPluginMissingID {
		t.Errorf("expected disabled_reason=%s, got %s",
			trigger.DisabledReasonPluginMissingID, outcomes[0].DisabledReason)
	}
}

// Plugin-target trigger with a disabled plugin is persisted disabled.
func TestRegistrar_SyncFromDefinition_PluginTargetDisabledPlugin(t *testing.T) {
	repo := newMockTriggerRepo()
	resolver := &stubPluginResolver{records: map[string]*model.PluginRecord{
		"plug-a": {
			PluginManifest: model.PluginManifest{
				Kind:     model.PluginKindWorkflow,
				Metadata: model.PluginMetadata{ID: "plug-a"},
				Spec:     model.PluginSpec{Workflow: &model.WorkflowPluginSpec{}},
			},
			LifecycleState: model.PluginStateDisabled,
		},
	}}
	reg := trigger.NewRegistrar(repo).WithPluginResolver(resolver)

	wfID := uuid.New()
	projID := uuid.New()

	nodes := []model.WorkflowNode{
		makeNode("n1", model.NodeTypeTrigger, map[string]any{
			"source":      "im",
			"target_kind": "plugin",
			"plugin_id":   "plug-a",
			"im":          map[string]any{"command": "/review"},
		}),
	}

	outcomes, err := reg.SyncFromDefinition(context.Background(), wfID, projID, nodes, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(outcomes) != 1 || outcomes[0].Enabled {
		t.Fatalf("expected 1 disabled outcome, got %+v", outcomes)
	}
	if outcomes[0].DisabledReason != trigger.DisabledReasonPluginDisabled {
		t.Errorf("expected disabled_reason=%s, got %s",
			trigger.DisabledReasonPluginDisabled, outcomes[0].DisabledReason)
	}
}

// DAG-target trigger referencing a missing workflow is persisted disabled.
func TestRegistrar_SyncFromDefinition_DAGTargetMissingWorkflow(t *testing.T) {
	repo := newMockTriggerRepo()
	resolver := &stubDAGResolver{defs: map[uuid.UUID]*model.WorkflowDefinition{}}
	reg := trigger.NewRegistrar(repo).WithDAGResolver(resolver)

	wfID := uuid.New() // not registered in resolver
	projID := uuid.New()

	nodes := []model.WorkflowNode{
		makeNode("n1", model.NodeTypeTrigger, map[string]any{
			"source": "im",
			"im":     map[string]any{"command": "/review"},
		}),
	}

	outcomes, err := reg.SyncFromDefinition(context.Background(), wfID, projID, nodes, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(outcomes) != 1 || outcomes[0].Enabled {
		t.Fatalf("expected 1 disabled outcome, got %+v", outcomes)
	}
	if outcomes[0].DisabledReason != trigger.DisabledReasonDAGWorkflowMissing {
		t.Errorf("expected disabled_reason=%s, got %s",
			trigger.DisabledReasonDAGWorkflowMissing, outcomes[0].DisabledReason)
	}
}

// ---------------------------------------------------------------------------
// Acting-employee author-time validation tests (Section 4.3)
// ---------------------------------------------------------------------------

type stubActingEmployeeGuard struct {
	// key: employeeID. Rows belong to expectedProject; everything else is
	// treated as cross-project. A zero-value expectedProject disables
	// cross-project checks and only employee lookup succeeds/fails.
	known          map[uuid.UUID]uuid.UUID // employeeID → projectID
	archivedSet    map[uuid.UUID]struct{}
}

func (s *stubActingEmployeeGuard) ValidateForProject(_ context.Context, employeeID uuid.UUID, projectID uuid.UUID) error {
	proj, ok := s.known[employeeID]
	if !ok {
		return employee.ErrEmployeeNotFound
	}
	if _, archived := s.archivedSet[employeeID]; archived {
		return employee.ErrEmployeeArchived
	}
	if proj != projectID {
		return employee.ErrEmployeeCrossProject
	}
	return nil
}

// Cross-project employee reference: registrar disables the row with the
// acting_employee_cross_project reason.
func TestRegistrar_SyncFromDefinition_CrossProjectEmployeeDisables(t *testing.T) {
	repo := newMockTriggerRepo()

	empID := uuid.New()
	empProject := uuid.New()
	workflowProject := uuid.New()

	guard := &stubActingEmployeeGuard{
		known: map[uuid.UUID]uuid.UUID{empID: empProject},
	}
	reg := trigger.NewRegistrar(repo).WithAttributionGuard(guard)

	wfID := uuid.New()
	nodes := []model.WorkflowNode{
		makeNode("n1", model.NodeTypeTrigger, map[string]any{
			"source":             "im",
			"im":                 map[string]any{"command": "/review"},
			"acting_employee_id": empID.String(),
		}),
	}

	outcomes, err := reg.SyncFromDefinition(context.Background(), wfID, workflowProject, nodes, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(outcomes) != 1 {
		t.Fatalf("expected 1 outcome, got %d", len(outcomes))
	}
	if outcomes[0].Enabled {
		t.Errorf("expected row disabled for cross-project employee, got enabled")
	}
	if outcomes[0].DisabledReason != trigger.DisabledReasonActingEmployeeCross {
		t.Errorf("expected reason %q, got %q",
			trigger.DisabledReasonActingEmployeeCross, outcomes[0].DisabledReason)
	}
}

// Unknown employee reference: registrar disables the row with the
// acting_employee_not_found reason.
func TestRegistrar_SyncFromDefinition_UnknownEmployeeDisables(t *testing.T) {
	repo := newMockTriggerRepo()

	guard := &stubActingEmployeeGuard{
		known: map[uuid.UUID]uuid.UUID{}, // empty: every lookup returns not-found
	}
	reg := trigger.NewRegistrar(repo).WithAttributionGuard(guard)

	wfID := uuid.New()
	projID := uuid.New()
	nodes := []model.WorkflowNode{
		makeNode("n1", model.NodeTypeTrigger, map[string]any{
			"source":             "im",
			"im":                 map[string]any{"command": "/review"},
			"acting_employee_id": uuid.New().String(),
		}),
	}

	outcomes, err := reg.SyncFromDefinition(context.Background(), wfID, projID, nodes, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(outcomes) != 1 {
		t.Fatalf("expected 1 outcome, got %d", len(outcomes))
	}
	if outcomes[0].Enabled {
		t.Errorf("expected row disabled for unknown employee, got enabled")
	}
	if outcomes[0].DisabledReason != trigger.DisabledReasonActingEmployeeMissing {
		t.Errorf("expected reason %q, got %q",
			trigger.DisabledReasonActingEmployeeMissing, outcomes[0].DisabledReason)
	}
}

// Same-project active employee: row is enabled and carries the acting id.
func TestRegistrar_SyncFromDefinition_ActiveSameProjectEmployeeEnables(t *testing.T) {
	repo := newMockTriggerRepo()

	empID := uuid.New()
	projID := uuid.New()
	guard := &stubActingEmployeeGuard{
		known: map[uuid.UUID]uuid.UUID{empID: projID},
	}
	reg := trigger.NewRegistrar(repo).WithAttributionGuard(guard)

	wfID := uuid.New()
	nodes := []model.WorkflowNode{
		makeNode("n1", model.NodeTypeTrigger, map[string]any{
			"source":             "im",
			"im":                 map[string]any{"command": "/review"},
			"acting_employee_id": empID.String(),
		}),
	}

	outcomes, err := reg.SyncFromDefinition(context.Background(), wfID, projID, nodes, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(outcomes) != 1 || !outcomes[0].Enabled {
		t.Fatalf("expected enabled outcome, got %+v", outcomes)
	}
	// Verify acting_employee_id round-trips through the upserted row.
	for _, row := range repo.rows {
		if row.ActingEmployeeID == nil || *row.ActingEmployeeID != empID {
			t.Errorf("expected row.ActingEmployeeID=%s, got %v", empID, row.ActingEmployeeID)
		}
	}
}

// Malformed UUID string: row disabled with missing reason, row still persisted.
func TestRegistrar_SyncFromDefinition_InvalidEmployeeUUID(t *testing.T) {
	repo := newMockTriggerRepo()

	reg := trigger.NewRegistrar(repo) // no guard: invalid UUID must be caught regardless

	wfID := uuid.New()
	projID := uuid.New()
	nodes := []model.WorkflowNode{
		makeNode("n1", model.NodeTypeTrigger, map[string]any{
			"source":             "im",
			"im":                 map[string]any{"command": "/review"},
			"acting_employee_id": "not-a-uuid",
		}),
	}

	outcomes, err := reg.SyncFromDefinition(context.Background(), wfID, projID, nodes, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(outcomes) != 1 || outcomes[0].Enabled {
		t.Fatalf("expected disabled outcome, got %+v", outcomes)
	}
	if outcomes[0].DisabledReason != trigger.DisabledReasonActingEmployeeMissing {
		t.Errorf("expected reason %q, got %q",
			trigger.DisabledReasonActingEmployeeMissing, outcomes[0].DisabledReason)
	}
}

// ---------------------------------------------------------------------------
// Spec 1C §6.2: registrar merges by created_via instead of delete-and-insert.
// ---------------------------------------------------------------------------

// TestRegistrar_SyncFromDefinition_PreservesManualRows verifies that a
// re-save of the DAG (with no remaining trigger nodes) deletes stale
// 'dag_node' rows but leaves any 'manual' rows for the same workflow alone.
// The 'manual' rows are owned by the FE trigger-CRUD surface (Spec 1C).
func TestRegistrar_SyncFromDefinition_PreservesManualRows(t *testing.T) {
	repo := newMockTriggerRepo()
	reg := trigger.NewRegistrar(repo)

	wfID := uuid.New()
	projID := uuid.New()
	wfRef := wfID

	// Pre-seed a manual row for this workflow — owned by the FE CRUD.
	manualID := uuid.New()
	repo.rows[manualID] = &model.WorkflowTrigger{
		ID:         manualID,
		WorkflowID: &wfRef,
		ProjectID:  projID,
		Source:     model.TriggerSourceIM,
		TargetKind: model.TriggerTargetDAG,
		Config:     json.RawMessage(`{"platform":"feishu","command":"/manual"}`),
		CreatedVia: model.TriggerCreatedViaManual,
		Enabled:    true,
	}
	// Pre-seed a stale dag_node row — should be deleted on sync.
	staleID := uuid.New()
	repo.rows[staleID] = &model.WorkflowTrigger{
		ID:         staleID,
		WorkflowID: &wfRef,
		ProjectID:  projID,
		Source:     model.TriggerSourceIM,
		TargetKind: model.TriggerTargetDAG,
		Config:     json.RawMessage(`{"platform":"feishu","command":"/old"}`),
		CreatedVia: model.TriggerCreatedViaDAGNode,
		Enabled:    true,
	}

	// Sync with empty DAG node list → stale dag_node deleted, manual kept.
	if _, err := reg.SyncFromDefinition(context.Background(), wfID, projID, nil, nil); err != nil {
		t.Fatalf("sync: %v", err)
	}
	if _, ok := repo.rows[manualID]; !ok {
		t.Error("manual row was deleted by sync; must be preserved")
	}
	if _, ok := repo.rows[staleID]; ok {
		t.Error("stale dag_node row was not deleted")
	}
	if repo.deleteCount != 1 {
		t.Errorf("expected exactly 1 delete (stale dag_node), got %d", repo.deleteCount)
	}
	if len(repo.deletedIDs) != 1 || repo.deletedIDs[0] != staleID {
		t.Errorf("unexpected deletedIDs: %v (want [%s])", repo.deletedIDs, staleID)
	}
}

// TestRegistrar_SyncFromDefinition_DAGRowsAddedUpdatedRemoved exercises the
// merge logic across all three transitions in a single pass: a new dag_node
// node is upserted, an existing dag_node node matches and is preserved, an
// extra orphan dag_node is deleted, and a manual row mixed in for the same
// workflow is left untouched.
func TestRegistrar_SyncFromDefinition_DAGRowsAddedUpdatedRemoved(t *testing.T) {
	repo := newMockTriggerRepo()
	reg := trigger.NewRegistrar(repo)

	wfID := uuid.New()
	projID := uuid.New()
	wfRef := wfID

	// Pre-seed a manual row (must survive untouched).
	manualID := uuid.New()
	repo.rows[manualID] = &model.WorkflowTrigger{
		ID:         manualID,
		WorkflowID: &wfRef,
		ProjectID:  projID,
		Source:     model.TriggerSourceIM,
		TargetKind: model.TriggerTargetDAG,
		Config:     json.RawMessage(`{"platform":"feishu","command":"/manual"}`),
		CreatedVia: model.TriggerCreatedViaManual,
		Enabled:    true,
	}
	// Pre-seed a stale dag_node row absent from the new DAG (must be deleted).
	orphanDAGID := uuid.New()
	repo.rows[orphanDAGID] = &model.WorkflowTrigger{
		ID:         orphanDAGID,
		WorkflowID: &wfRef,
		ProjectID:  projID,
		Source:     model.TriggerSourceIM,
		TargetKind: model.TriggerTargetDAG,
		Config:     json.RawMessage(`{"platform":"feishu","command":"/orphan"}`),
		CreatedVia: model.TriggerCreatedViaDAGNode,
		Enabled:    true,
	}

	nodes := []model.WorkflowNode{
		makeNode("n1", model.NodeTypeTrigger, map[string]any{
			"source": "im",
			"im":     map[string]any{"platform": "feishu", "command": "/n1"},
		}),
		makeNode("n2", model.NodeTypeTrigger, map[string]any{
			"source": "im",
			"im":     map[string]any{"platform": "feishu", "command": "/n2"},
		}),
	}

	if _, err := reg.SyncFromDefinition(context.Background(), wfID, projID, nodes, nil); err != nil {
		t.Fatalf("sync: %v", err)
	}

	// Manual row preserved.
	if _, ok := repo.rows[manualID]; !ok {
		t.Error("manual row deleted; must be preserved across DAG re-save")
	}
	// Orphan dag_node row deleted.
	if _, ok := repo.rows[orphanDAGID]; ok {
		t.Error("orphan dag_node row not deleted")
	}
	// All upserts stamped CreatedVia=dag_node.
	dagNodeRows := 0
	for id, row := range repo.rows {
		if id == manualID {
			continue
		}
		if row.CreatedVia != model.TriggerCreatedViaDAGNode {
			t.Errorf("row %s upserted with unexpected created_via=%q", id, row.CreatedVia)
		}
		dagNodeRows++
	}
	if dagNodeRows != 2 {
		t.Errorf("expected 2 dag_node rows after sync, got %d", dagNodeRows)
	}
	if repo.upsertCount != 2 {
		t.Errorf("expected 2 upserts, got %d", repo.upsertCount)
	}
	if repo.deleteCount != 1 || repo.deletedIDs[0] != orphanDAGID {
		t.Errorf("expected single delete of orphan dag_node, got deletes=%d ids=%v",
			repo.deleteCount, repo.deletedIDs)
	}
}

// Happy path: DAG resolver finds an active definition → row enabled, no reason.
func TestRegistrar_SyncFromDefinition_DAGTargetActiveEnables(t *testing.T) {
	repo := newMockTriggerRepo()
	wfID := uuid.New()
	resolver := &stubDAGResolver{defs: map[uuid.UUID]*model.WorkflowDefinition{
		wfID: {ID: wfID, Status: model.WorkflowDefStatusActive},
	}}
	reg := trigger.NewRegistrar(repo).WithDAGResolver(resolver)

	projID := uuid.New()

	nodes := []model.WorkflowNode{
		makeNode("n1", model.NodeTypeTrigger, map[string]any{
			"source": "im",
			"im":     map[string]any{"command": "/review"},
		}),
	}

	outcomes, err := reg.SyncFromDefinition(context.Background(), wfID, projID, nodes, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(outcomes) != 1 || !outcomes[0].Enabled {
		t.Fatalf("expected enabled outcome, got %+v", outcomes)
	}
	if outcomes[0].DisabledReason != "" {
		t.Errorf("expected no disabled reason, got %q", outcomes[0].DisabledReason)
	}
}
