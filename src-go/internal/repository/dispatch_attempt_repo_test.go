package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"gorm.io/gorm"
)

func TestDispatchAttemptRepository_PersistsRichAttemptMetadata(t *testing.T) {
	ctx := context.Background()
	repo := NewDispatchAttemptRepository(openDispatchAttemptRepoTestDB(t))
	projectID := uuid.New()
	taskID := uuid.New()
	memberID := uuid.New()
	priority := model.PriorityHigh
	createdAt := time.Now().UTC().Truncate(time.Second)

	attempt := &model.DispatchAttempt{
		ID:             uuid.New(),
		ProjectID:      projectID,
		TaskID:         taskID,
		MemberID:       &memberID,
		Outcome:        model.DispatchStatusQueued,
		TriggerSource:  "manual",
		Reason:         "agent pool is at capacity",
		Runtime:        "codex",
		Provider:       "openai",
		Model:          "gpt-5-codex",
		RoleID:         "reviewer",
		QueueEntryID:   "entry-123",
		QueuePriority:  &priority,
		GuardrailType:  model.DispatchGuardrailTypePool,
		GuardrailScope: "project",
		RecoveryDisposition: model.QueueRecoveryDispositionRecoverable,
		CreatedAt:      createdAt,
	}

	if err := repo.Create(ctx, attempt); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	list, err := repo.ListByTaskID(ctx, taskID, 10)
	if err != nil {
		t.Fatalf("ListByTaskID() error = %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len(ListByTaskID()) = %d, want 1", len(list))
	}
	got := list[0]
	if got.Runtime != "codex" || got.Provider != "openai" || got.Model != "gpt-5-codex" {
		t.Fatalf("got runtime tuple = %+v", got)
	}
	if got.RoleID != "reviewer" || got.QueueEntryID != "entry-123" {
		t.Fatalf("got queue linkage = %+v", got)
	}
	if got.QueuePriority == nil || *got.QueuePriority != model.PriorityHigh {
		t.Fatalf("got queue priority = %#v", got.QueuePriority)
	}
	if got.RecoveryDisposition != model.QueueRecoveryDispositionRecoverable {
		t.Fatalf("got recoveryDisposition = %q, want recoverable", got.RecoveryDisposition)
	}
}

func TestDispatchAttemptRepository_BackwardCompatibleReadWithEmptyRichFields(t *testing.T) {
	ctx := context.Background()
	db := openDispatchAttemptRepoTestDB(t)
	repo := NewDispatchAttemptRepository(db)
	projectID := uuid.New()
	taskID := uuid.New()

	if err := db.Exec(`INSERT INTO dispatch_attempts (id, project_id, task_id, outcome, trigger_source, reason, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, uuid.New().String(), projectID.String(), taskID.String(), model.DispatchStatusBlocked, "assignment", "dispatch target is unavailable", time.Now().UTC()).Error; err != nil {
		t.Fatalf("insert dispatch attempt: %v", err)
	}

	list, err := repo.ListByProjectID(ctx, projectID, 10)
	if err != nil {
		t.Fatalf("ListByProjectID() error = %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len(ListByProjectID()) = %d, want 1", len(list))
	}
	got := list[0]
	if got.Runtime != "" || got.Provider != "" || got.Model != "" || got.QueueEntryID != "" || got.QueuePriority != nil {
		t.Fatalf("expected empty rich fields on backward-compatible read, got %+v", got)
	}
}

func openDispatchAttemptRepoTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}

	schema := []string{
		`CREATE TABLE dispatch_attempts (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL,
			task_id TEXT NOT NULL,
			member_id TEXT,
			outcome TEXT NOT NULL,
			trigger_source TEXT NOT NULL,
			reason TEXT,
			runtime TEXT,
			provider TEXT,
			model TEXT,
			role_id TEXT,
			queue_entry_id TEXT,
			queue_priority INTEGER,
			guardrail_type TEXT,
			guardrail_scope TEXT,
			recovery_disposition TEXT,
			created_at DATETIME NOT NULL
		)`,
	}

	for _, stmt := range schema {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create dispatch attempt schema: %v", err)
		}
	}

	return db
}
