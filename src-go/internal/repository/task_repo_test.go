package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

func TestNewTaskRepository(t *testing.T) {
	repo := NewTaskRepository(nil)
	if repo == nil {
		t.Fatal("expected non-nil TaskRepository")
	}
}

func TestTaskRepositoryCreateNilDB(t *testing.T) {
	repo := NewTaskRepository(nil)
	err := repo.Create(context.Background(), &model.Task{ID: uuid.New(), ProjectID: uuid.New()})
	if err != ErrDatabaseUnavailable {
		t.Fatalf("Create() error = %v, want %v", err, ErrDatabaseUnavailable)
	}
}

func TestTaskRepositoryGetByIDNilDB(t *testing.T) {
	repo := NewTaskRepository(nil)
	_, err := repo.GetByID(context.Background(), uuid.New())
	if err != ErrDatabaseUnavailable {
		t.Fatalf("GetByID() error = %v, want %v", err, ErrDatabaseUnavailable)
	}
}

func TestTaskRepositoryListNilDB(t *testing.T) {
	repo := NewTaskRepository(nil)
	_, _, err := repo.List(context.Background(), uuid.New(), model.TaskListQuery{})
	if err != ErrDatabaseUnavailable {
		t.Fatalf("List() error = %v, want %v", err, ErrDatabaseUnavailable)
	}
}

func TestTaskRepositoryUpdateNilDB(t *testing.T) {
	repo := NewTaskRepository(nil)
	err := repo.Update(context.Background(), uuid.New(), &model.UpdateTaskRequest{})
	if err != ErrDatabaseUnavailable {
		t.Fatalf("Update() error = %v, want %v", err, ErrDatabaseUnavailable)
	}
}

func TestTaskRepositoryCreateChildrenNilDB(t *testing.T) {
	repo := NewTaskRepository(nil)
	_, err := repo.CreateChildren(context.Background(), []model.TaskChildInput{{ProjectID: uuid.New()}})
	if err != ErrDatabaseUnavailable {
		t.Fatalf("CreateChildren() error = %v, want %v", err, ErrDatabaseUnavailable)
	}
}

func TestNormalizeTaskBlockedByReturnsUUIDs(t *testing.T) {
	blockerID := uuid.New()

	ids, err := normalizeTaskBlockedBy([]string{blockerID.String()})
	if err != nil {
		t.Fatalf("normalizeTaskBlockedBy() error = %v", err)
	}
	if len(ids) != 1 || ids[0] != blockerID {
		t.Fatalf("normalizeTaskBlockedBy() = %v, want [%s]", ids, blockerID)
	}
}

func TestNormalizeTaskBlockedByRejectsBlankValues(t *testing.T) {
	_, err := normalizeTaskBlockedBy([]string{" "})
	if err == nil {
		t.Fatal("expected error for blank blockedBy value")
	}
}

func TestNormalizeTaskBlockedByRejectsInvalidUUID(t *testing.T) {
	_, err := normalizeTaskBlockedBy([]string{"not-a-uuid"})
	if err == nil {
		t.Fatal("expected error for invalid blockedBy value")
	}
}

func TestNormalizeOptionalTaskBlockedByNilPointer(t *testing.T) {
	value, err := normalizeOptionalTaskBlockedBy(nil)
	if err != nil {
		t.Fatalf("normalizeOptionalTaskBlockedBy(nil) error = %v", err)
	}
	if value != nil {
		t.Fatalf("normalizeOptionalTaskBlockedBy(nil) = %v, want nil", value)
	}
}

func TestNormalizeOptionalTaskBlockedByConvertsProvidedIDs(t *testing.T) {
	blockerID := uuid.New()
	blockedBy := []string{blockerID.String()}

	value, err := normalizeOptionalTaskBlockedBy(&blockedBy)
	if err != nil {
		t.Fatalf("normalizeOptionalTaskBlockedBy() error = %v", err)
	}

	ids, ok := value.([]uuid.UUID)
	if !ok {
		t.Fatalf("normalizeOptionalTaskBlockedBy() type = %T, want []uuid.UUID", value)
	}
	if len(ids) != 1 || ids[0] != blockerID {
		t.Fatalf("normalizeOptionalTaskBlockedBy() = %v, want [%s]", ids, blockerID)
	}
}

func TestTaskRecordConvertsBlockedByUUIDsToStrings(t *testing.T) {
	blockerID := uuid.New()
	record := &taskRecord{
		ID:        uuid.New(),
		ProjectID: uuid.New(),
		BlockedBy: newUUIDList([]uuid.UUID{blockerID}),
	}

	task := record.toModel()
	if len(task.BlockedBy) != 1 || task.BlockedBy[0] != blockerID.String() {
		t.Fatalf("blockedBy = %v, want [%s]", task.BlockedBy, blockerID.String())
	}
}

func TestTaskRecordHandlesNullAssigneeType(t *testing.T) {
	record := &taskRecord{
		ID:           uuid.New(),
		ProjectID:    uuid.New(),
		AssigneeType: nil,
	}

	task := record.toModel()
	if task.AssigneeType != "" {
		t.Fatalf("assigneeType = %q, want empty string", task.AssigneeType)
	}
}
