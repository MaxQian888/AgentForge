package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

type stubTaskRow struct {
	blockedBy                   []uuid.UUID
	requireNullableAssigneeType bool
}

func (r stubTaskRow) Scan(dest ...any) error {
	now := time.Now().UTC()
	projectID := uuid.New()

	*(dest[0].(*uuid.UUID)) = uuid.New()
	*(dest[1].(*uuid.UUID)) = projectID
	*(dest[2].(**uuid.UUID)) = nil
	*(dest[3].(**uuid.UUID)) = nil
	*(dest[4].(*string)) = "Create first functional task"
	*(dest[5].(*string)) = ""
	*(dest[6].(*string)) = model.TaskStatusInbox
	*(dest[7].(*string)) = "medium"
	*(dest[8].(**uuid.UUID)) = nil
	if r.requireNullableAssigneeType {
		assigneeTypeDest, ok := dest[9].(**string)
		if !ok {
			return fmt.Errorf("assignee_type target = %T, want **string", dest[9])
		}
		*assigneeTypeDest = nil
	} else {
		assigneeTypeDest, ok := dest[9].(**string)
		if !ok {
			return fmt.Errorf("assignee_type target = %T, want **string", dest[9])
		}
		value := ""
		*assigneeTypeDest = &value
	}
	*(dest[10].(**uuid.UUID)) = nil
	*(dest[11].(*[]string)) = []string{}
	*(dest[12].(*float64)) = 0
	*(dest[13].(*float64)) = 0
	*(dest[14].(*string)) = ""
	*(dest[15].(*string)) = ""
	*(dest[16].(*string)) = ""
	*(dest[17].(*string)) = ""
	*(dest[18].(*int)) = 0

	blockedByDest, ok := dest[19].(*[]uuid.UUID)
	if !ok {
		return fmt.Errorf("blocked_by target = %T, want *[]uuid.UUID", dest[19])
	}
	*blockedByDest = append([]uuid.UUID(nil), r.blockedBy...)

	*(dest[20].(**time.Time)) = nil
	*(dest[21].(**time.Time)) = nil
	*(dest[22].(*time.Time)) = now
	*(dest[23].(*time.Time)) = now
	*(dest[24].(**time.Time)) = nil
	*(dest[25].(**time.Time)) = nil
	*(dest[26].(**string)) = nil
	*(dest[27].(**time.Time)) = nil
	*(dest[28].(**string)) = nil
	*(dest[29].(**string)) = nil
	*(dest[30].(**time.Time)) = nil
	*(dest[31].(**string)) = nil
	*(dest[32].(**time.Time)) = nil
	*(dest[33].(**time.Time)) = nil
	*(dest[34].(**time.Time)) = nil
	*(dest[35].(**time.Time)) = nil

	return nil
}

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

func TestScanTaskConvertsBlockedByUUIDsToStrings(t *testing.T) {
	blockerID := uuid.New()

	task, err := scanTask(stubTaskRow{blockedBy: []uuid.UUID{blockerID}})
	if err != nil {
		t.Fatalf("scanTask() error = %v", err)
	}

	if len(task.BlockedBy) != 1 || task.BlockedBy[0] != blockerID.String() {
		t.Fatalf("blockedBy = %v, want [%s]", task.BlockedBy, blockerID.String())
	}
}

func TestScanTaskHandlesNullAssigneeType(t *testing.T) {
	task, err := scanTask(stubTaskRow{requireNullableAssigneeType: true})
	if err != nil {
		t.Fatalf("scanTask() error = %v", err)
	}

	if task.AssigneeType != "" {
		t.Fatalf("assigneeType = %q, want empty string", task.AssigneeType)
	}
}
