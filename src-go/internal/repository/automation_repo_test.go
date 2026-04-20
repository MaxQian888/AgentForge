package repository

import (
	"context"
	"testing"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
)

func TestNewAutomationRuleRepository(t *testing.T) {
	repo := NewAutomationRuleRepository(nil)
	if repo == nil {
		t.Fatal("expected non-nil AutomationRuleRepository")
	}
}

func TestAutomationRuleRepositoryRoundTripRules(t *testing.T) {
	ctx := context.Background()
	repo := NewAutomationRuleRepository(openFoundationRepoTestDB(t, &automationRuleRecord{}))

	projectID := uuid.New()
	creatorID := uuid.New()
	now := time.Date(2026, 3, 26, 15, 0, 0, 0, time.UTC)

	rule := &model.AutomationRule{
		ID:         uuid.New(),
		ProjectID:  projectID,
		Name:       "Notify when done",
		Enabled:    true,
		EventType:  model.AutomationEventTaskStatusChanged,
		Conditions: `[{"field":"status","op":"eq","value":"done"}]`,
		Actions:    `[{"type":"send_notification","config":{"title":"Task done"}}]`,
		CreatedBy:  creatorID,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := repo.Create(ctx, rule); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	rules, err := repo.ListByProjectAndEvent(ctx, projectID, model.AutomationEventTaskStatusChanged)
	if err != nil {
		t.Fatalf("ListByProjectAndEvent() error = %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("len(rules) = %d, want 1", len(rules))
	}

	rule.Enabled = false
	if err := repo.Update(ctx, rule); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	stored, err := repo.GetByID(ctx, rule.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if stored.Enabled {
		t.Fatal("expected updated rule to be disabled")
	}
}

func TestAutomationLogRepositoryListsProjectLogs(t *testing.T) {
	ctx := context.Background()
	db := openFoundationRepoTestDB(t, &automationRuleRecord{}, &automationLogRecord{})
	ruleRepo := NewAutomationRuleRepository(db)
	logRepo := NewAutomationLogRepository(db)

	projectID := uuid.New()
	rule := &model.AutomationRule{
		ID:         uuid.New(),
		ProjectID:  projectID,
		Name:       "Escalate blockers",
		Enabled:    true,
		EventType:  model.AutomationEventTaskFieldChanged,
		Conditions: `[]`,
		Actions:    `[]`,
		CreatedBy:  uuid.New(),
		CreatedAt:  time.Date(2026, 3, 26, 16, 0, 0, 0, time.UTC),
		UpdatedAt:  time.Date(2026, 3, 26, 16, 0, 0, 0, time.UTC),
	}
	if err := ruleRepo.Create(ctx, rule); err != nil {
		t.Fatalf("Create() rule error = %v", err)
	}

	taskID := uuid.New()
	now := time.Date(2026, 3, 26, 16, 5, 0, 0, time.UTC)
	for _, status := range []string{model.AutomationLogStatusSuccess, model.AutomationLogStatusFailed} {
		if err := logRepo.Create(ctx, &model.AutomationLog{
			ID:          uuid.New(),
			RuleID:      rule.ID,
			TaskID:      &taskID,
			EventType:   model.AutomationEventTaskFieldChanged,
			TriggeredAt: now,
			Status:      status,
			Detail:      `{"status":"` + status + `"}`,
		}); err != nil {
			t.Fatalf("Create() log error = %v", err)
		}
		now = now.Add(time.Minute)
	}

	logs, total, err := logRepo.ListByProject(ctx, projectID, model.AutomationLogListQuery{
		Status: model.AutomationLogStatusFailed,
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("ListByProject() error = %v", err)
	}
	if total != 1 || len(logs) != 1 {
		t.Fatalf("ListByProject() total=%d len=%d, want 1/1", total, len(logs))
	}
	if logs[0].Status != model.AutomationLogStatusFailed {
		t.Fatalf("logs[0].Status = %q, want %q", logs[0].Status, model.AutomationLogStatusFailed)
	}
}
