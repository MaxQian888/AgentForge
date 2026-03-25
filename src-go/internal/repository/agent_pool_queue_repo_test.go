package repository

import (
	"context"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"gorm.io/gorm"
)

func TestAgentPoolQueueRepository_InMemoryLifecycle(t *testing.T) {
	ctx := context.Background()
	repo := NewAgentPoolQueueRepository()
	projectID := uuid.New()
	taskID := uuid.New()
	memberID := uuid.New()

	entry, err := repo.QueueAgentAdmission(ctx, QueueAgentAdmissionRecord{
		ProjectID: projectID,
		TaskID:    taskID,
		MemberID:  memberID,
		Runtime:   "codex",
		Provider:  "openai",
		Model:     "gpt-5-codex",
		RoleID:    "planner-agent",
		BudgetUSD: 5,
		Reason:    "agent pool is at capacity",
	})
	if err != nil {
		t.Fatalf("QueueAgentAdmission() error = %v", err)
	}

	if entry.Status != model.AgentPoolQueueStatusQueued {
		t.Fatalf("entry.Status = %q, want queued", entry.Status)
	}
	if count, err := repo.CountQueuedByProject(ctx, projectID); err != nil || count != 1 {
		t.Fatalf("CountQueuedByProject() = %d, %v, want 1, nil", count, err)
	}

	reserved, err := repo.ReserveNextQueuedByProject(ctx, projectID)
	if err != nil {
		t.Fatalf("ReserveNextQueuedByProject() error = %v", err)
	}
	if reserved == nil || reserved.EntryID != entry.EntryID {
		t.Fatalf("reserved = %+v, want entry %s", reserved, entry.EntryID)
	}
	if reserved.Status != model.AgentPoolQueueStatusAdmitted {
		t.Fatalf("reserved.Status = %q, want admitted", reserved.Status)
	}

	runID := uuid.New()
	if err := repo.CompleteQueuedEntry(ctx, entry.EntryID, model.AgentPoolQueueStatusPromoted, "started", &runID); err != nil {
		t.Fatalf("CompleteQueuedEntry() error = %v", err)
	}

	list, err := repo.ListAllQueued(ctx, 10)
	if err != nil {
		t.Fatalf("ListAllQueued() error = %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("len(ListAllQueued()) = %d, want 0 after promotion", len(list))
	}
}

func TestAgentPoolQueueRepository_PersistsThroughDatabase(t *testing.T) {
	ctx := context.Background()
	repo := NewAgentPoolQueueRepository(openAgentPoolQueueRepoTestDB(t))
	projectID := uuid.New()
	taskID := uuid.New()
	memberID := uuid.New()

	entry, err := repo.QueueAgentAdmission(ctx, QueueAgentAdmissionRecord{
		ProjectID: projectID,
		TaskID:    taskID,
		MemberID:  memberID,
		Runtime:   "claude_code",
		Provider:  "anthropic",
		Model:     "claude-sonnet-4-5",
		RoleID:    "coding-agent",
		BudgetUSD: 8,
		Reason:    "agent pool is at capacity",
	})
	if err != nil {
		t.Fatalf("QueueAgentAdmission() error = %v", err)
	}

	allQueued, err := repo.ListAllQueued(ctx, 10)
	if err != nil {
		t.Fatalf("ListAllQueued() error = %v", err)
	}
	if len(allQueued) != 1 || allQueued[0].EntryID != entry.EntryID {
		t.Fatalf("unexpected queued entries: %+v", allQueued)
	}

	projectQueued, err := repo.ListQueuedByProject(ctx, projectID, 10)
	if err != nil {
		t.Fatalf("ListQueuedByProject() error = %v", err)
	}
	if len(projectQueued) != 1 || projectQueued[0].TaskID != taskID.String() {
		t.Fatalf("unexpected project queue: %+v", projectQueued)
	}

	reserved, err := repo.ReserveNextQueuedByProject(ctx, projectID)
	if err != nil {
		t.Fatalf("ReserveNextQueuedByProject() error = %v", err)
	}
	if reserved == nil || reserved.Status != model.AgentPoolQueueStatusAdmitted {
		t.Fatalf("unexpected reserved entry: %+v", reserved)
	}

	if count, err := repo.CountQueuedByProject(ctx, projectID); err != nil || count != 0 {
		t.Fatalf("CountQueuedByProject() after reserve = %d, %v, want 0, nil", count, err)
	}
}

func openAgentPoolQueueRepoTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}

	schema := []string{
		`CREATE TABLE agent_pool_queue_entries (
			entry_id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL,
			task_id TEXT NOT NULL,
			member_id TEXT NOT NULL,
			status TEXT NOT NULL,
			reason TEXT NOT NULL,
			runtime TEXT NOT NULL,
			provider TEXT NOT NULL,
			model TEXT NOT NULL,
			role_id TEXT,
			budget_usd REAL NOT NULL DEFAULT 0,
			agent_run_id TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	for _, stmt := range schema {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create agent pool queue schema: %v", err)
		}
	}

	return db
}
