package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
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
