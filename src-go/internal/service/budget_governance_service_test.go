package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/service"
)

type mockBudgetSprintReader struct {
	sprintsByID      map[uuid.UUID]*model.Sprint
	sprintsByProject map[uuid.UUID][]*model.Sprint
	getErr           error
	listErr          error
}

func (m *mockBudgetSprintReader) GetByID(_ context.Context, id uuid.UUID) (*model.Sprint, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	sprint, ok := m.sprintsByID[id]
	if !ok {
		return nil, errors.New("sprint not found")
	}
	cloned := *sprint
	return &cloned, nil
}

func (m *mockBudgetSprintReader) ListByProject(_ context.Context, projectID uuid.UUID) ([]*model.Sprint, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	sprints := m.sprintsByProject[projectID]
	out := make([]*model.Sprint, 0, len(sprints))
	for _, sprint := range sprints {
		cloned := *sprint
		out = append(out, &cloned)
	}
	return out, nil
}

func TestBudgetGovernanceService_CheckSprintBudget(t *testing.T) {
	sprintID := uuid.New()
	reader := &mockBudgetSprintReader{
		sprintsByID: map[uuid.UUID]*model.Sprint{
			sprintID: {
				ID:             sprintID,
				TotalBudgetUsd: 10,
				SpentUsd:       5,
			},
		},
	}
	svc := service.NewBudgetGovernanceService(reader)

	result, err := svc.CheckSprintBudget(context.Background(), sprintID, 3)
	if err != nil {
		t.Fatalf("CheckSprintBudget() error = %v", err)
	}
	if !result.Allowed || !result.Warning {
		t.Fatalf("expected warning result, got %+v", result)
	}
	if !strings.Contains(result.WarningMessage, "80% utilized") {
		t.Fatalf("WarningMessage = %q, want 80%% utilization", result.WarningMessage)
	}

	result, err = svc.CheckSprintBudget(context.Background(), sprintID, 6)
	if err != nil {
		t.Fatalf("CheckSprintBudget() error = %v", err)
	}
	if result.Allowed {
		t.Fatalf("expected blocked result, got %+v", result)
	}
	if !strings.Contains(result.Reason, "sprint budget exceeded") {
		t.Fatalf("Reason = %q", result.Reason)
	}
}

func TestBudgetGovernanceService_CheckSprintBudgetAllowsWithoutConfiguredLimit(t *testing.T) {
	sprintID := uuid.New()
	svc := service.NewBudgetGovernanceService(&mockBudgetSprintReader{
		sprintsByID: map[uuid.UUID]*model.Sprint{
			sprintID: {ID: sprintID, TotalBudgetUsd: 0, SpentUsd: 99},
		},
	})

	result, err := svc.CheckSprintBudget(context.Background(), sprintID, 100)
	if err != nil {
		t.Fatalf("CheckSprintBudget() error = %v", err)
	}
	if !result.Allowed || result.Warning || result.Reason != "" {
		t.Fatalf("unexpected unconstrained result: %+v", result)
	}
}

func TestBudgetGovernanceService_CheckSprintBudgetReturnsReaderError(t *testing.T) {
	svc := service.NewBudgetGovernanceService(&mockBudgetSprintReader{getErr: errors.New("boom")})

	_, err := svc.CheckSprintBudget(context.Background(), uuid.New(), 1)
	if err == nil || !strings.Contains(err.Error(), "budget check: fetch sprint") {
		t.Fatalf("CheckSprintBudget() error = %v, want wrapped fetch error", err)
	}
}

func TestBudgetGovernanceService_CheckProjectBudget(t *testing.T) {
	projectID := uuid.New()
	reader := &mockBudgetSprintReader{
		sprintsByProject: map[uuid.UUID][]*model.Sprint{
			projectID: {
				{ID: uuid.New(), ProjectID: projectID, SpentUsd: 4.5},
				{ID: uuid.New(), ProjectID: projectID, SpentUsd: 3.0},
			},
		},
	}
	svc := service.NewBudgetGovernanceService(reader)
	svc.SetProjectBudgetLimit(10)

	result, err := svc.CheckProjectBudget(context.Background(), projectID, 0.5)
	if err != nil {
		t.Fatalf("CheckProjectBudget() error = %v", err)
	}
	if !result.Allowed || !result.Warning {
		t.Fatalf("expected project warning result, got %+v", result)
	}
	if !strings.Contains(result.WarningMessage, "80% utilized") {
		t.Fatalf("WarningMessage = %q, want 80%% utilization", result.WarningMessage)
	}

	result, err = svc.CheckProjectBudget(context.Background(), projectID, 3)
	if err != nil {
		t.Fatalf("CheckProjectBudget() error = %v", err)
	}
	if result.Allowed {
		t.Fatalf("expected blocked project result, got %+v", result)
	}
	if !strings.Contains(result.Reason, "project budget exceeded") {
		t.Fatalf("Reason = %q", result.Reason)
	}
}

func TestBudgetGovernanceService_CheckProjectBudgetAllowsWithoutLimitAndWrapsErrors(t *testing.T) {
	projectID := uuid.New()
	svc := service.NewBudgetGovernanceService(&mockBudgetSprintReader{})

	result, err := svc.CheckProjectBudget(context.Background(), projectID, 20)
	if err != nil {
		t.Fatalf("CheckProjectBudget() error = %v", err)
	}
	if !result.Allowed || result.Warning {
		t.Fatalf("unexpected unconstrained project result: %+v", result)
	}

	svc.SetProjectBudgetLimit(10)
	svc = service.NewBudgetGovernanceService(&mockBudgetSprintReader{listErr: errors.New("project read failed")})
	svc.SetProjectBudgetLimit(10)
	_, err = svc.CheckProjectBudget(context.Background(), projectID, 1)
	if err == nil || !strings.Contains(err.Error(), "budget check: list project sprints") {
		t.Fatalf("CheckProjectBudget() error = %v, want wrapped list error", err)
	}
}

func TestBudgetGovernanceService_CheckBudgetDelegatesSprintThenProject(t *testing.T) {
	projectID := uuid.New()
	sprintID := uuid.New()
	reader := &mockBudgetSprintReader{
		sprintsByID: map[uuid.UUID]*model.Sprint{
			sprintID: {
				ID:             sprintID,
				ProjectID:      projectID,
				TotalBudgetUsd: 10,
				SpentUsd:       7.5,
			},
		},
		sprintsByProject: map[uuid.UUID][]*model.Sprint{
			projectID: {
				{ID: sprintID, ProjectID: projectID, SpentUsd: 7.5},
			},
		},
	}
	svc := service.NewBudgetGovernanceService(reader)
	svc.SetProjectBudgetLimit(20)

	result, err := svc.CheckBudget(context.Background(), projectID, &sprintID, 0.5)
	if err != nil {
		t.Fatalf("CheckBudget() error = %v", err)
	}
	if !result.Allowed || !result.Warning {
		t.Fatalf("CheckBudget() = %+v, want sprint warning", result)
	}

	result, err = svc.CheckBudget(context.Background(), projectID, &sprintID, 3)
	if err != nil {
		t.Fatalf("CheckBudget() error = %v", err)
	}
	if result.Allowed || !strings.Contains(result.Reason, "sprint budget exceeded") {
		t.Fatalf("CheckBudget() = %+v, want sprint block", result)
	}

	result, err = svc.CheckBudget(context.Background(), projectID, nil, 13)
	if err != nil {
		t.Fatalf("CheckBudget() project-only error = %v", err)
	}
	if result.Allowed || !strings.Contains(result.Reason, "project budget exceeded") {
		t.Fatalf("CheckBudget() project-only = %+v, want project block", result)
	}
}
