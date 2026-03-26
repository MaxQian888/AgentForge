package model_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

func TestAgentEvent_ToDTO(t *testing.T) {
	occurredAt := time.Date(2026, 3, 26, 9, 15, 0, 0, time.UTC)
	event := &model.AgentEvent{
		ID:         uuid.New(),
		RunID:      uuid.New(),
		TaskID:     uuid.New(),
		ProjectID:  uuid.New(),
		EventType:  model.AgentEventBudgetWarning,
		Payload:    `{"costUsd":4.8}`,
		OccurredAt: occurredAt,
	}

	dto := event.ToDTO()

	if dto.ID != event.ID.String() {
		t.Fatalf("dto.ID = %q, want %q", dto.ID, event.ID.String())
	}
	if dto.RunID != event.RunID.String() {
		t.Fatalf("dto.RunID = %q, want %q", dto.RunID, event.RunID.String())
	}
	if dto.TaskID != event.TaskID.String() {
		t.Fatalf("dto.TaskID = %q, want %q", dto.TaskID, event.TaskID.String())
	}
	if dto.ProjectID != event.ProjectID.String() {
		t.Fatalf("dto.ProjectID = %q, want %q", dto.ProjectID, event.ProjectID.String())
	}
	if dto.EventType != model.AgentEventBudgetWarning {
		t.Fatalf("dto.EventType = %q, want %q", dto.EventType, model.AgentEventBudgetWarning)
	}
	if dto.Payload != `{"costUsd":4.8}` {
		t.Fatalf("dto.Payload = %q", dto.Payload)
	}
	if dto.OccurredAt != occurredAt.Format(time.RFC3339) {
		t.Fatalf("dto.OccurredAt = %q, want %q", dto.OccurredAt, occurredAt.Format(time.RFC3339))
	}
}
