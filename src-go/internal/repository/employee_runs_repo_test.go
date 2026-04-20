package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestEmployeeRunsRepository_NilDB(t *testing.T) {
	repo := NewEmployeeRunsRepository(nil)
	_, err := repo.ListByEmployee(context.Background(), uuid.New(), EmployeeRunKindAll, 1, 20)
	if err != ErrDatabaseUnavailable {
		t.Fatalf("expected ErrDatabaseUnavailable, got %v", err)
	}
}

func TestEmployeeRunsRepository_PaginationDefaults(t *testing.T) {
	repo := NewEmployeeRunsRepository(nil)
	// page <= 0 must coerce to 1; size <= 0 must coerce to 20; size > 200 capped to 200
	if got := normalizeRunsPage(0); got != 1 {
		t.Errorf("normalizeRunsPage(0) = %d, want 1", got)
	}
	if got := normalizeRunsPage(-5); got != 1 {
		t.Errorf("normalizeRunsPage(-5) = %d, want 1", got)
	}
	if got := normalizeRunsSize(0); got != 20 {
		t.Errorf("normalizeRunsSize(0) = %d, want 20", got)
	}
	if got := normalizeRunsSize(500); got != 200 {
		t.Errorf("normalizeRunsSize(500) = %d, want 200", got)
	}
	// explicit suppress unused-import warning
	_ = repo
}
