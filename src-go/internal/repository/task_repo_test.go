package repository

import (
	"context"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/google/uuid"
	pgxmock "github.com/pashagolub/pgxmock/v4"
	"github.com/react-go-quick-starter/server/internal/model"
)

type stubTaskRow struct {
	blockedBy                  []uuid.UUID
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

func TestTaskRepository_UpdatePersistsSprintID(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	repo := NewTaskRepository(mock)
	taskID := uuid.New()
	sprintID := uuid.New().String()
	req := &model.UpdateTaskRequest{
		SprintID: &sprintID,
	}

	mock.ExpectExec(regexp.QuoteMeta("UPDATE tasks SET")).
		WithArgs(
			req.Title,
			req.Description,
			req.Priority,
			req.BudgetUsd,
			req.SprintID,
			req.PlannedStartAt,
			req.PlannedEndAt,
			nil,
			taskID,
		).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	if err := repo.Update(context.Background(), taskID, req); err != nil {
		t.Fatalf("Update() error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestTaskRepository_CreateNormalizesEmptyOptionalFields(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	repo := NewTaskRepository(mock)
	task := &model.Task{
		ID:          uuid.New(),
		ProjectID:   uuid.New(),
		Title:       "Create first functional task",
		Description: "",
		Status:      model.TaskStatusInbox,
		Priority:    "medium",
		Labels:      []string{},
		BudgetUsd:   0,
	}

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO tasks")).
		WithArgs(
			task.ID,
			task.ProjectID,
			task.ParentID,
			task.SprintID,
			task.Title,
			task.Description,
			task.Status,
			task.Priority,
			task.AssigneeID,
			nil,
			task.ReporterID,
			task.Labels,
			task.BudgetUsd,
			task.SpentUsd,
			task.AgentBranch,
			task.AgentWorktree,
			task.AgentSessionID,
			task.PRUrl,
			task.PRNumber,
			[]uuid.UUID{},
			task.PlannedStartAt,
			task.PlannedEndAt,
			task.CompletedAt,
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	if err := repo.Create(context.Background(), task); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestTaskRepository_UpdateConvertsBlockedByToUUIDArray(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error: %v", err)
	}
	defer mock.Close()

	repo := NewTaskRepository(mock)
	taskID := uuid.New()
	blockerID := uuid.New()
	blockedBy := []string{blockerID.String()}
	req := &model.UpdateTaskRequest{
		BlockedBy: &blockedBy,
	}

	mock.ExpectExec(regexp.QuoteMeta("UPDATE tasks SET")).
		WithArgs(
			req.Title,
			req.Description,
			req.Priority,
			req.BudgetUsd,
			req.SprintID,
			req.PlannedStartAt,
			req.PlannedEndAt,
			[]uuid.UUID{blockerID},
			taskID,
		).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	if err := repo.Update(context.Background(), taskID, req); err != nil {
		t.Fatalf("Update() error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
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
