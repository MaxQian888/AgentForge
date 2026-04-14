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

func TestNewAgentMemoryRepository(t *testing.T) {
	repo := NewAgentMemoryRepository(nil)
	if repo == nil {
		t.Fatal("expected non-nil AgentMemoryRepository")
	}
}

func TestAgentMemoryRepositoryCreateNilDB(t *testing.T) {
	repo := NewAgentMemoryRepository(nil)
	err := repo.Create(context.Background(), &model.AgentMemory{ID: uuid.New(), ProjectID: uuid.New()})
	if err != ErrDatabaseUnavailable {
		t.Fatalf("Create() error = %v, want %v", err, ErrDatabaseUnavailable)
	}
}

func TestAgentMemoryRepositoryGetByIDNilDB(t *testing.T) {
	repo := NewAgentMemoryRepository(nil)
	_, err := repo.GetByID(context.Background(), uuid.New())
	if err != ErrDatabaseUnavailable {
		t.Fatalf("GetByID() error = %v, want %v", err, ErrDatabaseUnavailable)
	}
}

func TestAgentMemoryRecordPreservesMetadataAndAccessTime(t *testing.T) {
	lastAccessedAt := time.Now().UTC().Add(-30 * time.Minute)

	mem := &model.AgentMemory{
		ID:             uuid.New(),
		ProjectID:      uuid.New(),
		Scope:          model.MemoryScopeProject,
		RoleID:         "frontend-developer",
		Category:       model.MemoryCategorySemantic,
		Key:            "nextjs-routing",
		Content:        "Prefer server components for route shells.",
		Metadata:       `{"source":"review"}`,
		RelevanceScore: 0.88,
		AccessCount:    3,
		LastAccessedAt: &lastAccessedAt,
	}

	record := newAgentMemoryRecord(mem)
	result := record.toModel()

	if result.Metadata != `{"source":"review"}` {
		t.Fatalf("Metadata = %q", result.Metadata)
	}
	if result.LastAccessedAt == nil {
		t.Fatal("LastAccessedAt = nil, want non-nil")
	}
}

func TestAgentMemoryRepository_ListByProjectAndTimeRangeAndDeleteOlderThan(t *testing.T) {
	db := openAgentMemoryRepoTestDB(t)
	repo := NewAgentMemoryRepository(db)
	projectID := uuid.New()
	roleID := "planner"

	base := time.Date(2026, 3, 31, 8, 0, 0, 0, time.UTC)
	records := []*model.AgentMemory{
		{
			ID:        uuid.New(),
			ProjectID: projectID,
			Scope:     model.MemoryScopeProject,
			Category:  model.MemoryCategoryEpisodic,
			Key:       "old",
			Content:   "old turn",
			CreatedAt: base.Add(-48 * time.Hour),
			UpdatedAt: base.Add(-48 * time.Hour),
		},
		{
			ID:        uuid.New(),
			ProjectID: projectID,
			Scope:     model.MemoryScopeRole,
			RoleID:    roleID,
			Category:  model.MemoryCategoryEpisodic,
			Key:       "recent-role",
			Content:   "recent role turn",
			CreatedAt: base.Add(-2 * time.Hour),
			UpdatedAt: base.Add(-2 * time.Hour),
		},
		{
			ID:        uuid.New(),
			ProjectID: projectID,
			Scope:     model.MemoryScopeProject,
			Category:  model.MemoryCategorySemantic,
			Key:       "semantic",
			Content:   "semantic note",
			CreatedAt: base.Add(-time.Hour),
			UpdatedAt: base.Add(-time.Hour),
		},
	}
	for _, record := range records {
		if err := db.Create(newAgentMemoryRecord(record)).Error; err != nil {
			t.Fatalf("seed memory: %v", err)
		}
	}

	start := base.Add(-3 * time.Hour)
	end := base
	got, err := repo.ListByProjectAndTimeRange(context.Background(), projectID, model.MemoryCategoryEpisodic, model.MemoryScopeRole, roleID, &start, &end, 10)
	if err != nil {
		t.Fatalf("ListByProjectAndTimeRange() error = %v", err)
	}
	if len(got) != 1 || got[0].Key != "recent-role" {
		t.Fatalf("ListByProjectAndTimeRange() = %#v, want recent-role only", got)
	}

	deleted, err := repo.DeleteOlderThan(context.Background(), projectID, model.MemoryCategoryEpisodic, base.Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("DeleteOlderThan() error = %v", err)
	}
	if deleted != 1 {
		t.Fatalf("DeleteOlderThan() deleted = %d, want 1", deleted)
	}

	remaining, err := repo.ListByProject(context.Background(), projectID, "", "")
	if err != nil {
		t.Fatalf("ListByProject() error = %v", err)
	}
	if len(remaining) != 2 {
		t.Fatalf("len(remaining) = %d, want 2", len(remaining))
	}
}

func TestAgentMemoryRepository_ListFilteredAndDeleteMany(t *testing.T) {
	db := openAgentMemoryRepoTestDB(t)
	repo := NewAgentMemoryRepository(db)
	projectID := uuid.New()
	start := time.Date(2026, 4, 1, 8, 0, 0, 0, time.UTC)

	records := []*model.AgentMemory{
		{
			ID:        uuid.New(),
			ProjectID: projectID,
			Scope:     model.MemoryScopeProject,
			Category:  model.MemoryCategorySemantic,
			Key:       "release-plan",
			Content:   "Use staged rollout",
			Metadata:  `{"source":"ops"}`,
			CreatedAt: start,
			UpdatedAt: start,
		},
		{
			ID:        uuid.New(),
			ProjectID: projectID,
			Scope:     model.MemoryScopeRole,
			RoleID:    "reviewer",
			Category:  model.MemoryCategorySemantic,
			Key:       "review-note",
			Content:   "Focus on regressions",
			Metadata:  `{"taskId":"task-1"}`,
			CreatedAt: start.Add(time.Hour),
			UpdatedAt: start.Add(time.Hour),
		},
		{
			ID:        uuid.New(),
			ProjectID: projectID,
			Scope:     model.MemoryScopeProject,
			Category:  model.MemoryCategoryProcedural,
			Key:       "deploy-checklist",
			Content:   "Verify migrations first",
			Metadata:  `{"source":"runbook"}`,
			CreatedAt: start.Add(2 * time.Hour),
			UpdatedAt: start.Add(2 * time.Hour),
		},
	}
	for _, record := range records {
		if err := db.Create(newAgentMemoryRecord(record)).Error; err != nil {
			t.Fatalf("seed memory: %v", err)
		}
	}

	filtered, err := repo.ListFiltered(context.Background(), projectID, model.AgentMemoryFilter{
		Query:    "task-1",
		Scope:    model.MemoryScopeRole,
		Category: model.MemoryCategorySemantic,
		RoleID:   "reviewer",
		StartAt:  ptrAgentMemoryTime(start),
		EndAt:    ptrAgentMemoryTime(start.Add(90 * time.Minute)),
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("ListFiltered() error = %v", err)
	}
	if len(filtered) != 1 || filtered[0].Key != "review-note" {
		t.Fatalf("ListFiltered() = %#v, want only review-note", filtered)
	}

	deleted, err := repo.DeleteMany(context.Background(), []uuid.UUID{records[0].ID, records[2].ID})
	if err != nil {
		t.Fatalf("DeleteMany() error = %v", err)
	}
	if deleted != 2 {
		t.Fatalf("DeleteMany() deleted = %d, want 2", deleted)
	}

	remaining, err := repo.ListByProject(context.Background(), projectID, "", "")
	if err != nil {
		t.Fatalf("ListByProject() error = %v", err)
	}
	if len(remaining) != 1 || remaining[0].Key != "review-note" {
		t.Fatalf("remaining = %#v, want only review-note", remaining)
	}
}

func openAgentMemoryRepoTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	if err := db.AutoMigrate(&agentMemoryRecord{}); err != nil {
		t.Fatalf("migrate agent memory table: %v", err)
	}
	return db
}

func ptrAgentMemoryTime(value time.Time) *time.Time {
	return &value
}
