//go:build integration

package repository_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
	"github.com/react-go-quick-starter/server/pkg/database"
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
	defer func() { _ = database.ClosePostgres(db) }()

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
		WorkflowID: workflowID,
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
		WorkflowID: workflowID,
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
	defer func() { _ = database.ClosePostgres(db) }()

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
		WorkflowID: workflowID,
		ProjectID:  projectID,
		Source:     model.TriggerSourceIM,
		Config:     []byte(`{"channel":"#alpha"}`),
		Enabled:    true,
	}
	t2 := &model.WorkflowTrigger{
		WorkflowID: workflowID,
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
	defer func() { _ = database.ClosePostgres(db) }()

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
		WorkflowID: workflowID,
		ProjectID:  projectID,
		Source:     model.TriggerSourceSchedule,
		Config:     []byte(`{"cron":"0 * * * *"}`),
		Enabled:    true,
	}
	disabledTrig := &model.WorkflowTrigger{
		WorkflowID: workflowID,
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
	defer func() { _ = database.ClosePostgres(db) }()

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
		WorkflowID: workflowID,
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
