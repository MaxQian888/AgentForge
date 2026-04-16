package service

import (
	"testing"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

func TestDispatchBudgetAndGuardrailHelpers(t *testing.T) {
	task := &model.Task{
		ID:        uuid.New(),
		ProjectID: uuid.New(),
		Title:     "Budgeted Task",
		BudgetUsd: 10,
		SpentUsd:  8,
	}

	warning, blocked := checkTaskBudget(task, 0)
	if warning == nil || blocked != nil || warning.Scope != "task" {
		t.Fatalf("checkTaskBudget(warn) = %#v / %#v", warning, blocked)
	}

	warning, blocked = checkTaskBudget(task, 3)
	if warning != nil || blocked == nil || blocked.GuardrailScope != "task" {
		t.Fatalf("checkTaskBudget(block) = %#v / %#v", warning, blocked)
	}

	warning, blocked = checkTaskBudget(&model.Task{BudgetUsd: 0}, 5)
	if warning != nil || blocked != nil {
		t.Fatalf("checkTaskBudget(no budget) = %#v / %#v", warning, blocked)
	}

	guardrailType, guardrailScope := inferDispatchGuardrail("project budget exceeded")
	if guardrailType != model.DispatchGuardrailTypeBudget || guardrailScope != "project" {
		t.Fatalf("inferDispatchGuardrail(budget) = %q / %q", guardrailType, guardrailScope)
	}
	guardrailType, guardrailScope = inferDispatchGuardrail("agent pool is at capacity")
	if guardrailType != model.DispatchGuardrailTypePool || guardrailScope != "project" {
		t.Fatalf("inferDispatchGuardrail(pool) = %q / %q", guardrailType, guardrailScope)
	}
	guardrailType, guardrailScope = inferDispatchGuardrail("dispatch target is unavailable")
	if guardrailType != model.DispatchGuardrailTypeTarget || guardrailScope != "member" {
		t.Fatalf("inferDispatchGuardrail(target) = %q / %q", guardrailType, guardrailScope)
	}
	guardrailType, guardrailScope = inferDispatchGuardrail("task already has an active agent run")
	if guardrailType != model.DispatchGuardrailTypeTask || guardrailScope != "task" {
		t.Fatalf("inferDispatchGuardrail(task) = %q / %q", guardrailType, guardrailScope)
	}
	guardrailType, guardrailScope = inferDispatchGuardrail("unknown reason")
	if guardrailType != "" || guardrailScope != "" {
		t.Fatalf("inferDispatchGuardrail(default) = %q / %q", guardrailType, guardrailScope)
	}

	if got := inferBudgetScope("sprint budget warning"); got != "sprint" {
		t.Fatalf("inferBudgetScope(sprint) = %q, want sprint", got)
	}
	if got := inferBudgetScope("task budget warning"); got != "task" {
		t.Fatalf("inferBudgetScope(task) = %q, want task", got)
	}
	if got := inferBudgetScope("project budget warning"); got != "project" {
		t.Fatalf("inferBudgetScope(project) = %q, want project", got)
	}
	if got := inferBudgetScope("budget warning"); got != "" {
		t.Fatalf("inferBudgetScope(default) = %q, want empty", got)
	}
}
