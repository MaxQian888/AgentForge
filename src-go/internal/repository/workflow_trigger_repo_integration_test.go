//go:build integration

package repository_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/repository"
	"github.com/agentforge/server/pkg/database"
	"github.com/google/uuid"
)

// Note: TestMain is defined in user_repo_integration_test.go and runs
// migrations once for the entire package. Do NOT redefine it here.

func TestWorkflowTriggerRepository_Integration_UpsertIdempotent(t *testing.T) {
	url := os.Getenv("TEST_POSTGRES_URL")
	if url == "" {
		t.Skip("TEST_POSTGRES_URL not set — skipping integration test")
	}

	db, err := database.NewPostgres(url)
	if err != nil {
		t.Fatalf("NewPostgres() error: %v", err)
	}
	// Registered first so it runs LAST in LIFO: DELETE cleanups must hit
	// the live connection, not a closed one (otherwise workflow_triggers
	// rows leak into cross-package integration tests like TriggerFlow).
	t.Cleanup(func() { _ = database.ClosePostgres(db) })

	ctx := context.Background()
	projectID := uuid.New()
	workflowID := uuid.New()

	if err := db.WithContext(ctx).Exec(
		"INSERT INTO projects (id, name, slug) VALUES (?, ?, ?)",
		projectID, "trig-proj-"+projectID.String(), "trig-slug-"+projectID.String(),
	).Error; err != nil {
		t.Fatalf("insert project: %v", err)
	}
	if err := db.WithContext(ctx).Exec(
		"INSERT INTO workflow_definitions (id, project_id) VALUES (?, ?)",
		workflowID, projectID,
	).Error; err != nil {
		t.Fatalf("insert workflow_definition: %v", err)
	}

	t.Cleanup(func() {
		_ = db.WithContext(ctx).Exec("DELETE FROM workflow_triggers WHERE workflow_id = ?", workflowID).Error
		_ = db.WithContext(ctx).Exec("DELETE FROM workflow_definitions WHERE id = ?", workflowID).Error
		_ = db.WithContext(ctx).Exec("DELETE FROM projects WHERE id = ?", projectID).Error
	})

	repo := repository.NewWorkflowTriggerRepository(db)

	trig := &model.WorkflowTrigger{
		WorkflowID: &workflowID,
		ProjectID:  projectID,
		Source:     model.TriggerSourceIM,
		Config:     []byte(`{"channel":"#general"}`),
		Enabled:    true,
	}

	// First upsert — should INSERT.
	if err := repo.Upsert(ctx, trig); err != nil {
		t.Fatalf("Upsert() first: %v", err)
	}
	firstID := trig.ID
	if firstID == uuid.Nil {
		t.Fatal("expected non-nil ID after first Upsert")
	}

	// Second upsert — same config, different enabled value → should UPDATE and
	// return the same row ID.
	trig2 := &model.WorkflowTrigger{
		WorkflowID: &workflowID,
		ProjectID:  projectID,
		Source:     model.TriggerSourceIM,
		Config:     []byte(`{"channel":"#general"}`),
		Enabled:    false,
	}
	if err := repo.Upsert(ctx, trig2); err != nil {
		t.Fatalf("Upsert() second: %v", err)
	}
	if trig2.ID != firstID {
		t.Errorf("expected same ID on second upsert: want %s, got %s", firstID, trig2.ID)
	}

	// Verify only one row exists.
	list, err := repo.ListByWorkflow(ctx, workflowID)
	if err != nil {
		t.Fatalf("ListByWorkflow() error: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 trigger row, got %d", len(list))
	}
	if list[0].Enabled != false {
		t.Errorf("expected Enabled=false after second upsert, got %v", list[0].Enabled)
	}
}

func TestWorkflowTriggerRepository_Integration_UpsertDifferentConfigCreatesNewRow(t *testing.T) {
	url := os.Getenv("TEST_POSTGRES_URL")
	if url == "" {
		t.Skip("TEST_POSTGRES_URL not set — skipping integration test")
	}

	db, err := database.NewPostgres(url)
	if err != nil {
		t.Fatalf("NewPostgres() error: %v", err)
	}
	// Registered first so it runs LAST in LIFO: DELETE cleanups must hit
	// the live connection, not a closed one (otherwise workflow_triggers
	// rows leak into cross-package integration tests like TriggerFlow).
	t.Cleanup(func() { _ = database.ClosePostgres(db) })

	ctx := context.Background()
	projectID := uuid.New()
	workflowID := uuid.New()

	if err := db.WithContext(ctx).Exec(
		"INSERT INTO projects (id, name, slug) VALUES (?, ?, ?)",
		projectID, "trig-proj2-"+projectID.String(), "trig-slug2-"+projectID.String(),
	).Error; err != nil {
		t.Fatalf("insert project: %v", err)
	}
	if err := db.WithContext(ctx).Exec(
		"INSERT INTO workflow_definitions (id, project_id) VALUES (?, ?)",
		workflowID, projectID,
	).Error; err != nil {
		t.Fatalf("insert workflow_definition: %v", err)
	}

	t.Cleanup(func() {
		_ = db.WithContext(ctx).Exec("DELETE FROM workflow_triggers WHERE workflow_id = ?", workflowID).Error
		_ = db.WithContext(ctx).Exec("DELETE FROM workflow_definitions WHERE id = ?", workflowID).Error
		_ = db.WithContext(ctx).Exec("DELETE FROM projects WHERE id = ?", projectID).Error
	})

	repo := repository.NewWorkflowTriggerRepository(db)

	t1 := &model.WorkflowTrigger{
		WorkflowID: &workflowID,
		ProjectID:  projectID,
		Source:     model.TriggerSourceIM,
		Config:     []byte(`{"channel":"#alpha"}`),
		Enabled:    true,
	}
	t2 := &model.WorkflowTrigger{
		WorkflowID: &workflowID,
		ProjectID:  projectID,
		Source:     model.TriggerSourceIM,
		Config:     []byte(`{"channel":"#beta"}`),
		Enabled:    true,
	}

	if err := repo.Upsert(ctx, t1); err != nil {
		t.Fatalf("Upsert() t1: %v", err)
	}
	if err := repo.Upsert(ctx, t2); err != nil {
		t.Fatalf("Upsert() t2: %v", err)
	}
	if t1.ID == t2.ID {
		t.Errorf("expected distinct IDs for different configs, both got %s", t1.ID)
	}

	list, err := repo.ListByWorkflow(ctx, workflowID)
	if err != nil {
		t.Fatalf("ListByWorkflow() error: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 trigger rows for distinct configs, got %d", len(list))
	}
}

func TestWorkflowTriggerRepository_Integration_ListEnabledBySource_FiltersDisabled(t *testing.T) {
	url := os.Getenv("TEST_POSTGRES_URL")
	if url == "" {
		t.Skip("TEST_POSTGRES_URL not set — skipping integration test")
	}

	db, err := database.NewPostgres(url)
	if err != nil {
		t.Fatalf("NewPostgres() error: %v", err)
	}
	// Registered first so it runs LAST in LIFO: DELETE cleanups must hit
	// the live connection, not a closed one (otherwise workflow_triggers
	// rows leak into cross-package integration tests like TriggerFlow).
	t.Cleanup(func() { _ = database.ClosePostgres(db) })

	ctx := context.Background()
	projectID := uuid.New()
	workflowID := uuid.New()

	if err := db.WithContext(ctx).Exec(
		"INSERT INTO projects (id, name, slug) VALUES (?, ?, ?)",
		projectID, "trig-proj3-"+projectID.String(), "trig-slug3-"+projectID.String(),
	).Error; err != nil {
		t.Fatalf("insert project: %v", err)
	}
	if err := db.WithContext(ctx).Exec(
		"INSERT INTO workflow_definitions (id, project_id) VALUES (?, ?)",
		workflowID, projectID,
	).Error; err != nil {
		t.Fatalf("insert workflow_definition: %v", err)
	}

	t.Cleanup(func() {
		_ = db.WithContext(ctx).Exec("DELETE FROM workflow_triggers WHERE workflow_id = ?", workflowID).Error
		_ = db.WithContext(ctx).Exec("DELETE FROM workflow_definitions WHERE id = ?", workflowID).Error
		_ = db.WithContext(ctx).Exec("DELETE FROM projects WHERE id = ?", projectID).Error
	})

	repo := repository.NewWorkflowTriggerRepository(db)

	enabledTrig := &model.WorkflowTrigger{
		WorkflowID: &workflowID,
		ProjectID:  projectID,
		Source:     model.TriggerSourceSchedule,
		Config:     []byte(`{"cron":"0 * * * *"}`),
		Enabled:    true,
	}
	disabledTrig := &model.WorkflowTrigger{
		WorkflowID: &workflowID,
		ProjectID:  projectID,
		Source:     model.TriggerSourceSchedule,
		Config:     []byte(`{"cron":"30 * * * *"}`),
		Enabled:    false,
	}

	if err := repo.Upsert(ctx, enabledTrig); err != nil {
		t.Fatalf("Upsert() enabled: %v", err)
	}
	if err := repo.Upsert(ctx, disabledTrig); err != nil {
		t.Fatalf("Upsert() disabled: %v", err)
	}

	list, err := repo.ListEnabledBySource(ctx, model.TriggerSourceSchedule)
	if err != nil {
		t.Fatalf("ListEnabledBySource() error: %v", err)
	}

	// The list may contain enabled rows from other tests, so just verify our
	// enabled row is present and our disabled row is absent.
	var foundEnabled, foundDisabled bool
	for _, tr := range list {
		if tr.ID == enabledTrig.ID {
			foundEnabled = true
		}
		if tr.ID == disabledTrig.ID {
			foundDisabled = true
		}
	}
	if !foundEnabled {
		t.Errorf("expected to find enabled trigger in list")
	}
	if foundDisabled {
		t.Errorf("did not expect disabled trigger in enabled list")
	}
}

func TestWorkflowTriggerRepository_Integration_SetEnabledAndDelete(t *testing.T) {
	url := os.Getenv("TEST_POSTGRES_URL")
	if url == "" {
		t.Skip("TEST_POSTGRES_URL not set — skipping integration test")
	}

	db, err := database.NewPostgres(url)
	if err != nil {
		t.Fatalf("NewPostgres() error: %v", err)
	}
	// Registered first so it runs LAST in LIFO: DELETE cleanups must hit
	// the live connection, not a closed one (otherwise workflow_triggers
	// rows leak into cross-package integration tests like TriggerFlow).
	t.Cleanup(func() { _ = database.ClosePostgres(db) })

	ctx := context.Background()
	projectID := uuid.New()
	workflowID := uuid.New()

	if err := db.WithContext(ctx).Exec(
		"INSERT INTO projects (id, name, slug) VALUES (?, ?, ?)",
		projectID, "trig-proj4-"+projectID.String(), "trig-slug4-"+projectID.String(),
	).Error; err != nil {
		t.Fatalf("insert project: %v", err)
	}
	if err := db.WithContext(ctx).Exec(
		"INSERT INTO workflow_definitions (id, project_id) VALUES (?, ?)",
		workflowID, projectID,
	).Error; err != nil {
		t.Fatalf("insert workflow_definition: %v", err)
	}

	t.Cleanup(func() {
		_ = db.WithContext(ctx).Exec("DELETE FROM workflow_triggers WHERE workflow_id = ?", workflowID).Error
		_ = db.WithContext(ctx).Exec("DELETE FROM workflow_definitions WHERE id = ?", workflowID).Error
		_ = db.WithContext(ctx).Exec("DELETE FROM projects WHERE id = ?", projectID).Error
	})

	repo := repository.NewWorkflowTriggerRepository(db)

	trig := &model.WorkflowTrigger{
		WorkflowID: &workflowID,
		ProjectID:  projectID,
		Source:     model.TriggerSourceIM,
		Config:     []byte(`{"channel":"#ops"}`),
		Enabled:    true,
	}
	if err := repo.Upsert(ctx, trig); err != nil {
		t.Fatalf("Upsert() error: %v", err)
	}

	// SetEnabled: disable the row.
	if err := repo.SetEnabled(ctx, trig.ID, false); err != nil {
		t.Fatalf("SetEnabled(false) error: %v", err)
	}

	// Verify via ListByWorkflow.
	list, err := repo.ListByWorkflow(ctx, workflowID)
	if err != nil {
		t.Fatalf("ListByWorkflow() error: %v", err)
	}
	if len(list) != 1 || list[0].Enabled != false {
		t.Errorf("expected 1 disabled trigger, got %v", list)
	}

	// SetEnabled on a non-existent ID must return ErrNotFound.
	err = repo.SetEnabled(ctx, uuid.New(), true)
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound for unknown ID, got %v", err)
	}

	// Delete the row.
	if err := repo.Delete(ctx, trig.ID); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	// Delete again must return ErrNotFound.
	err = repo.Delete(ctx, trig.ID)
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound on second Delete, got %v", err)
	}

	// ListByWorkflow should now be empty.
	list, err = repo.ListByWorkflow(ctx, workflowID)
	if err != nil {
		t.Fatalf("ListByWorkflow() after delete error: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("expected 0 triggers after delete, got %d", len(list))
	}
}

// TestWorkflowTriggerRepository_Integration_TargetKindDefaultsAndDistinct verifies
// that the target_kind column defaults to "dag" for legacy Upsert callers and that
// two triggers differing only in target_kind coexist as separate rows.
func TestWorkflowTriggerRepository_Integration_TargetKindDefaultsAndDistinct(t *testing.T) {
	url := os.Getenv("TEST_POSTGRES_URL")
	if url == "" {
		t.Skip("TEST_POSTGRES_URL not set — skipping integration test")
	}

	db, err := database.NewPostgres(url)
	if err != nil {
		t.Fatalf("NewPostgres() error: %v", err)
	}
	// Registered first so it runs LAST in LIFO: DELETE cleanups must hit
	// the live connection, not a closed one (otherwise workflow_triggers
	// rows leak into cross-package integration tests like TriggerFlow).
	t.Cleanup(func() { _ = database.ClosePostgres(db) })

	ctx := context.Background()
	projectID := uuid.New()
	workflowID := uuid.New()

	if err := db.WithContext(ctx).Exec(
		"INSERT INTO projects (id, name, slug) VALUES (?, ?, ?)",
		projectID, "trig-proj6-"+projectID.String(), "trig-slug6-"+projectID.String(),
	).Error; err != nil {
		t.Fatalf("insert project: %v", err)
	}
	if err := db.WithContext(ctx).Exec(
		"INSERT INTO workflow_definitions (id, project_id) VALUES (?, ?)",
		workflowID, projectID,
	).Error; err != nil {
		t.Fatalf("insert workflow_definition: %v", err)
	}

	t.Cleanup(func() {
		_ = db.WithContext(ctx).Exec("DELETE FROM workflow_triggers WHERE project_id = ?", projectID).Error
		_ = db.WithContext(ctx).Exec("DELETE FROM workflow_definitions WHERE id = ?", workflowID).Error
		_ = db.WithContext(ctx).Exec("DELETE FROM projects WHERE id = ?", projectID).Error
	})

	repo := repository.NewWorkflowTriggerRepository(db)

	// Legacy-shaped insert (TargetKind left empty) should default to "dag".
	legacy := &model.WorkflowTrigger{
		WorkflowID: &workflowID,
		ProjectID:  projectID,
		Source:     model.TriggerSourceIM,
		Config:     []byte(`{"command":"/review"}`),
		Enabled:    true,
	}
	if err := repo.Upsert(ctx, legacy); err != nil {
		t.Fatalf("Upsert legacy: %v", err)
	}

	// Plugin-targeted row uses plugin_id (workflow_id must be NULL per
	// workflow_triggers_target_identifier_check). Same config as the DAG row
	// exercises the uniqueness index's target_kind discriminator.
	pluginTrig := &model.WorkflowTrigger{
		PluginID:   "workflow-plugin-review-" + projectID.String(),
		ProjectID:  projectID,
		Source:     model.TriggerSourceIM,
		TargetKind: model.TriggerTargetPlugin,
		Config:     []byte(`{"command":"/review"}`),
		Enabled:    true,
	}
	if err := repo.Upsert(ctx, pluginTrig); err != nil {
		t.Fatalf("Upsert plugin: %v", err)
	}

	if legacy.ID == pluginTrig.ID {
		t.Fatalf("expected distinct IDs for DAG vs plugin targets")
	}

	// ListByWorkflow is scoped to workflow_id, so it must return only the DAG
	// row (plugin rows are keyed by plugin_id and have NULL workflow_id).
	rows, err := repo.ListByWorkflow(ctx, workflowID)
	if err != nil {
		t.Fatalf("ListByWorkflow error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 DAG row for workflow_id, got %d", len(rows))
	}
	if rows[0].ID != legacy.ID || rows[0].TargetKind != model.TriggerTargetDAG {
		t.Errorf("ListByWorkflow should return only the DAG row, got %+v", rows[0])
	}

	// ListEnabledBySourceAndKind must scope to the requested kind.
	pluginOnly, err := repo.ListEnabledBySourceAndKind(ctx, model.TriggerSourceIM, model.TriggerTargetPlugin)
	if err != nil {
		t.Fatalf("ListEnabledBySourceAndKind error: %v", err)
	}
	found := false
	for _, row := range pluginOnly {
		if row.ID == pluginTrig.ID {
			found = true
			if row.TargetKind != model.TriggerTargetPlugin {
				t.Errorf("plugin row returned with TargetKind=%q", row.TargetKind)
			}
		}
		if row.ID == legacy.ID {
			t.Errorf("dag row leaked into plugin scoped list")
		}
	}
	if !found {
		t.Errorf("plugin row missing from ListEnabledBySourceAndKind result")
	}
}

func TestWorkflowTriggerRepository_Integration_UpsertCanonicalizesJSONOrder(t *testing.T) {
	url := os.Getenv("TEST_POSTGRES_URL")
	if url == "" {
		t.Skip("TEST_POSTGRES_URL not set — skipping integration test")
	}

	db, err := database.NewPostgres(url)
	if err != nil {
		t.Fatalf("NewPostgres() error: %v", err)
	}
	// Registered first so it runs LAST in LIFO: DELETE cleanups must hit
	// the live connection, not a closed one (otherwise workflow_triggers
	// rows leak into cross-package integration tests like TriggerFlow).
	t.Cleanup(func() { _ = database.ClosePostgres(db) })

	ctx := context.Background()
	projectID := uuid.New()
	workflowID := uuid.New()

	if err := db.WithContext(ctx).Exec(
		"INSERT INTO projects (id, name, slug) VALUES (?, ?, ?)",
		projectID, "trig-proj5-"+projectID.String(), "trig-slug5-"+projectID.String(),
	).Error; err != nil {
		t.Fatalf("insert project: %v", err)
	}
	if err := db.WithContext(ctx).Exec(
		"INSERT INTO workflow_definitions (id, project_id) VALUES (?, ?)",
		workflowID, projectID,
	).Error; err != nil {
		t.Fatalf("insert workflow_definition: %v", err)
	}

	t.Cleanup(func() {
		_ = db.WithContext(ctx).Exec("DELETE FROM workflow_triggers WHERE workflow_id = ?", workflowID).Error
		_ = db.WithContext(ctx).Exec("DELETE FROM workflow_definitions WHERE id = ?", workflowID).Error
		_ = db.WithContext(ctx).Exec("DELETE FROM projects WHERE id = ?", projectID).Error
	})

	repo := repository.NewWorkflowTriggerRepository(db)

	t1 := &model.WorkflowTrigger{
		WorkflowID:   &workflowID,
		ProjectID:    projectID,
		Source:       model.TriggerSourceIM,
		Config:       []byte(`{"b":1,"a":2}`),
		InputMapping: []byte(`{}`),
		Enabled:      true,
	}
	if err := repo.Upsert(ctx, t1); err != nil {
		t.Fatalf("Upsert 1: %v", err)
	}

	t2 := &model.WorkflowTrigger{
		WorkflowID:   &workflowID,
		ProjectID:    projectID,
		Source:       model.TriggerSourceIM,
		Config:       []byte(`{"a":2,"b":1}`), // same logical JSON, different byte order
		InputMapping: []byte(`{}`),
		Enabled:      true,
	}
	if err := repo.Upsert(ctx, t2); err != nil {
		t.Fatalf("Upsert 2: %v", err)
	}

	if t1.ID != t2.ID {
		t.Fatalf("expected same canonical row for equivalent JSON; got %s vs %s", t1.ID, t2.ID)
	}

	all, err := repo.ListByWorkflow(ctx, workflowID)
	if err != nil {
		t.Fatalf("ListByWorkflow() error: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("expected 1 canonical row, got %d", len(all))
	}
}
