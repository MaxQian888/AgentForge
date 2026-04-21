package repository

import (
	"context"
	"testing"
	"time"

	"errors"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

// newMinimalTaskRecord creates a taskRecord with only the required fields set.
func newMinimalTaskRecord(id, projectID uuid.UUID, parentID *uuid.UUID) *taskRecord {
	now := time.Now().UTC()
	return &taskRecord{
		ID:        id,
		ProjectID: projectID,
		ParentID:  parentID,
		Title:     "test task",
		Status:    model.TaskStatusInbox,
		Priority:  "medium",
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestGetAncestorRoot_NilDB(t *testing.T) {
	repo := NewTaskRepository(nil)
	_, err := repo.GetAncestorRoot(context.Background(), uuid.New())
	if err != ErrDatabaseUnavailable {
		t.Fatalf("GetAncestorRoot() with nil db = %v, want ErrDatabaseUnavailable", err)
	}
}

func TestGetAncestorRoot_RootReturnsItself(t *testing.T) {
	db := openFoundationRepoTestDB(t, &taskRecord{})
	repo := NewTaskRepository(db)
	projectID := uuid.New()

	root := newMinimalTaskRecord(uuid.New(), projectID, nil)
	if err := db.Create(root).Error; err != nil {
		t.Fatalf("create root: %v", err)
	}

	got, err := repo.GetAncestorRoot(context.Background(), root.ID)
	if err != nil {
		t.Fatalf("GetAncestorRoot() error = %v", err)
	}
	if got.ID != root.ID {
		t.Fatalf("GetAncestorRoot() = %v, want root %v", got.ID, root.ID)
	}
}

func TestGetAncestorRoot_TwoHopChain(t *testing.T) {
	db := openFoundationRepoTestDB(t, &taskRecord{})
	repo := NewTaskRepository(db)
	projectID := uuid.New()

	rootID := uuid.New()
	midID := uuid.New()
	leafID := uuid.New()

	root := newMinimalTaskRecord(rootID, projectID, nil)
	mid := newMinimalTaskRecord(midID, projectID, &rootID)
	leaf := newMinimalTaskRecord(leafID, projectID, &midID)

	for _, rec := range []*taskRecord{root, mid, leaf} {
		if err := db.Create(rec).Error; err != nil {
			t.Fatalf("create task record %v: %v", rec.ID, err)
		}
	}

	got, err := repo.GetAncestorRoot(context.Background(), leafID)
	if err != nil {
		t.Fatalf("GetAncestorRoot() error = %v", err)
	}
	if got.ID != rootID {
		t.Fatalf("GetAncestorRoot() = %v, want root %v", got.ID, rootID)
	}
}

func TestGetAncestorRoot_DeepChain(t *testing.T) {
	db := openFoundationRepoTestDB(t, &taskRecord{})
	repo := NewTaskRepository(db)
	projectID := uuid.New()

	// Build a 6-node chain: root → n1 → n2 → n3 → n4 → leaf
	ids := make([]uuid.UUID, 6)
	for i := range ids {
		ids[i] = uuid.New()
	}

	root := newMinimalTaskRecord(ids[0], projectID, nil)
	if err := db.Create(root).Error; err != nil {
		t.Fatalf("create root: %v", err)
	}
	for i := 1; i < len(ids); i++ {
		parentID := ids[i-1]
		rec := newMinimalTaskRecord(ids[i], projectID, &parentID)
		if err := db.Create(rec).Error; err != nil {
			t.Fatalf("create chain node %d: %v", i, err)
		}
	}

	leaf := ids[len(ids)-1]
	got, err := repo.GetAncestorRoot(context.Background(), leaf)
	if err != nil {
		t.Fatalf("GetAncestorRoot() error = %v", err)
	}
	if got.ID != ids[0] {
		t.Fatalf("GetAncestorRoot() = %v, want root %v", got.ID, ids[0])
	}
}

func TestGetAncestorRoot_NotFound(t *testing.T) {
	db := openFoundationRepoTestDB(t, &taskRecord{})
	repo := NewTaskRepository(db)

	_, err := repo.GetAncestorRoot(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("GetAncestorRoot() on missing task expected error, got nil")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetAncestorRoot() error = %v, want ErrNotFound", err)
	}
}

func TestGetAncestorRoot_CycleDetected(t *testing.T) {
	db := openFoundationRepoTestDB(t, &taskRecord{})
	repo := NewTaskRepository(db)
	projectID := uuid.New()

	// Construct a two-node cycle by bypassing FK constraints:
	// A.ParentID = B, B.ParentID = A
	aID := uuid.New()
	bID := uuid.New()
	a := newMinimalTaskRecord(aID, projectID, &bID)
	b := newMinimalTaskRecord(bID, projectID, &aID)

	// Insert both without FK enforcement (SQLite test DB has no FK constraints).
	if err := db.Create(a).Error; err != nil {
		t.Fatalf("create a: %v", err)
	}
	if err := db.Create(b).Error; err != nil {
		t.Fatalf("create b: %v", err)
	}

	_, err := repo.GetAncestorRoot(context.Background(), aID)
	if err == nil {
		t.Fatal("GetAncestorRoot() on cycle expected error, got nil")
	}
	if !errors.Is(err, ErrTaskAncestorCycle) {
		t.Fatalf("GetAncestorRoot() error = %v, want ErrTaskAncestorCycle", err)
	}
}
