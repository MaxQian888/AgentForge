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

func TestNewAgentEventRepository(t *testing.T) {
	repo := NewAgentEventRepository(nil)
	if repo == nil {
		t.Fatal("expected non-nil AgentEventRepository")
	}
}

func TestAgentEventRepositoryCreateNilDB(t *testing.T) {
	repo := NewAgentEventRepository(nil)
	err := repo.Create(context.Background(), &model.AgentEvent{ID: uuid.New(), RunID: uuid.New(), TaskID: uuid.New(), ProjectID: uuid.New()})
	if err != ErrDatabaseUnavailable {
		t.Fatalf("Create() error = %v, want %v", err, ErrDatabaseUnavailable)
	}
}

func TestAgentEventRecordRoundTrip(t *testing.T) {
	occurredAt := time.Date(2026, 3, 26, 8, 0, 0, 0, time.UTC)
	createdAt := occurredAt.Add(30 * time.Second)
	event := &model.AgentEvent{
		ID:         uuid.New(),
		RunID:      uuid.New(),
		TaskID:     uuid.New(),
		ProjectID:  uuid.New(),
		EventType:  model.AgentEventCompleted,
		Payload:    `{"summary":"done"}`,
		OccurredAt: occurredAt,
		CreatedAt:  createdAt,
	}

	record := newAgentEventRecord(event)
	result := record.toModel()

	if result.ID != event.ID || result.RunID != event.RunID || result.TaskID != event.TaskID || result.ProjectID != event.ProjectID {
		t.Fatalf("round trip IDs mismatch: got %+v want %+v", result, event)
	}
	if result.EventType != event.EventType || result.Payload != event.Payload {
		t.Fatalf("round trip payload mismatch: got %+v want %+v", result, event)
	}
	if !result.OccurredAt.Equal(occurredAt) || !result.CreatedAt.Equal(createdAt) {
		t.Fatalf("round trip timestamps mismatch: got occurred=%v created=%v", result.OccurredAt, result.CreatedAt)
	}
}

func TestAgentEventRepositoryCreateAndList(t *testing.T) {
	ctx := context.Background()
	repo := NewAgentEventRepository(openAgentEventRepoTestDB(t))

	projectID := uuid.New()
	runID := uuid.New()
	taskID := uuid.New()
	otherTaskID := uuid.New()
	base := time.Date(2026, 3, 26, 10, 0, 0, 0, time.UTC)

	events := []*model.AgentEvent{
		{
			ID:         uuid.New(),
			RunID:      runID,
			TaskID:     taskID,
			ProjectID:  projectID,
			EventType:  model.AgentEventSpawn,
			Payload:    `{"step":1}`,
			OccurredAt: base.Add(2 * time.Minute),
			CreatedAt:  base.Add(2 * time.Minute),
		},
		{
			ID:         uuid.New(),
			RunID:      runID,
			TaskID:     taskID,
			ProjectID:  projectID,
			EventType:  model.AgentEventRunning,
			Payload:    `{"step":2}`,
			OccurredAt: base,
			CreatedAt:  base,
		},
		{
			ID:         uuid.New(),
			RunID:      runID,
			TaskID:     otherTaskID,
			ProjectID:  projectID,
			EventType:  model.AgentEventCompleted,
			Payload:    `{"step":3}`,
			OccurredAt: base.Add(4 * time.Minute),
			CreatedAt:  base.Add(4 * time.Minute),
		},
	}

	for _, event := range events {
		if err := repo.Create(ctx, event); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}

	runEvents, err := repo.ListByRunID(ctx, runID, 2)
	if err != nil {
		t.Fatalf("ListByRunID() error = %v", err)
	}
	if len(runEvents) != 2 {
		t.Fatalf("len(runEvents) = %d, want 2", len(runEvents))
	}
	if runEvents[0].OccurredAt.After(runEvents[1].OccurredAt) {
		t.Fatalf("run events should be ascending by occurred_at, got %v then %v", runEvents[0].OccurredAt, runEvents[1].OccurredAt)
	}
	if runEvents[0].EventType != model.AgentEventRunning || runEvents[1].EventType != model.AgentEventSpawn {
		t.Fatalf("unexpected run event order: %+v", runEvents)
	}

	taskEvents, err := repo.ListByTaskID(ctx, taskID, 0)
	if err != nil {
		t.Fatalf("ListByTaskID() error = %v", err)
	}
	if len(taskEvents) != 2 {
		t.Fatalf("len(taskEvents) = %d, want 2", len(taskEvents))
	}
	if taskEvents[0].EventType != model.AgentEventRunning || taskEvents[1].EventType != model.AgentEventSpawn {
		t.Fatalf("unexpected task event order: %+v", taskEvents)
	}

	projectEvents, err := repo.ListByProjectID(ctx, projectID, 0)
	if err != nil {
		t.Fatalf("ListByProjectID() error = %v", err)
	}
	if len(projectEvents) != 3 {
		t.Fatalf("len(projectEvents) = %d, want 3", len(projectEvents))
	}
	if !projectEvents[0].OccurredAt.After(projectEvents[1].OccurredAt) {
		t.Fatalf("project events should be descending by occurred_at, got %v then %v", projectEvents[0].OccurredAt, projectEvents[1].OccurredAt)
	}
	if projectEvents[0].EventType != model.AgentEventCompleted {
		t.Fatalf("newest project event = %q, want %q", projectEvents[0].EventType, model.AgentEventCompleted)
	}
}

func openAgentEventRepoTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	if err := db.AutoMigrate(&agentEventRecord{}); err != nil {
		t.Fatalf("migrate agent events table: %v", err)
	}
	return db
}
