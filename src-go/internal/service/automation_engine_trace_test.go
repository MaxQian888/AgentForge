package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	applog "github.com/agentforge/server/internal/log"
	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
)

func TestAutomationLog_MergesTraceID(t *testing.T) {
	projectID := uuid.New()
	taskID := uuid.New()
	rule := &model.AutomationRule{
		ID:         uuid.New(),
		ProjectID:  projectID,
		Name:       "trace test rule",
		Enabled:    true,
		EventType:  model.AutomationEventTaskStatusChanged,
		Conditions: `[{"field":"status","op":"eq","value":"done"}]`,
		Actions:    `[{"type":"update_field","config":{"field":"priority","value":"high"}}]`,
	}
	logs := &stubAutomationLogRepo{}
	tasks := &stubAutomationTaskRepo{task: &model.Task{
		ID:        taskID,
		ProjectID: projectID,
		Title:     "Trace Task",
		Status:    model.TaskStatusDone,
		Priority:  "low",
	}}
	fields := &stubAutomationFieldRepo{}

	engine := NewAutomationEngineService(
		&stubAutomationRuleRepo{rules: []*model.AutomationRule{rule}},
		logs, tasks, fields, nil, nil, nil,
	)
	engine.now = func() time.Time { return time.Date(2026, 4, 21, 0, 0, 0, 0, time.UTC) }

	const wantTraceID = "tr_auto000000000000000000"
	ctx := applog.WithTrace(context.Background(), wantTraceID)

	if err := engine.EvaluateRules(ctx, AutomationEvent{
		EventType: model.AutomationEventTaskStatusChanged,
		ProjectID: projectID,
		TaskID:    &taskID,
		Task:      tasks.task,
		Data:      map[string]any{"status": "done"},
	}); err != nil {
		t.Fatalf("EvaluateRules() error = %v", err)
	}

	if len(logs.entries) == 0 {
		t.Fatal("expected at least one automation log entry")
	}

	last := logs.entries[len(logs.entries)-1]
	var got map[string]any
	if err := json.Unmarshal([]byte(last.Detail), &got); err != nil {
		t.Fatalf("failed to unmarshal Detail JSON: %v", err)
	}
	if got["trace_id"] != wantTraceID {
		t.Fatalf("want trace_id=%q in detail; got %+v", wantTraceID, got)
	}
}
