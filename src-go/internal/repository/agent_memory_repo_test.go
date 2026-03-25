package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

type stubAgentMemoryRow struct{}

func (stubAgentMemoryRow) Scan(dest ...any) error {
	now := time.Now().UTC()
	lastAccessedAt := now.Add(-30 * time.Minute)

	*(dest[0].(*uuid.UUID)) = uuid.New()
	*(dest[1].(*uuid.UUID)) = uuid.New()
	*(dest[2].(*string)) = model.MemoryScopeProject
	*(dest[3].(*string)) = "frontend-developer"
	*(dest[4].(*string)) = model.MemoryCategorySemantic
	*(dest[5].(*string)) = "nextjs-routing"
	*(dest[6].(*string)) = "Prefer server components for route shells."
	*(dest[7].(*string)) = `{"source":"review"}`
	*(dest[8].(*float64)) = 0.88
	*(dest[9].(*int)) = 3
	*(dest[10].(**time.Time)) = &lastAccessedAt
	*(dest[11].(*time.Time)) = now
	*(dest[12].(*time.Time)) = now

	return nil
}

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

func TestScanAgentMemoryPreservesMetadataAndAccessTime(t *testing.T) {
	mem, err := scanAgentMemory(stubAgentMemoryRow{})
	if err != nil {
		t.Fatalf("scanAgentMemory() error = %v", err)
	}
	if mem.Metadata != `{"source":"review"}` {
		t.Fatalf("Metadata = %q", mem.Metadata)
	}
	if mem.LastAccessedAt == nil {
		t.Fatal("LastAccessedAt = nil, want non-nil")
	}
}
