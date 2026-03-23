package service_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/service"
	"github.com/react-go-quick-starter/server/internal/ws"
)

type mockAgentRunRepo struct {
	runs       map[uuid.UUID]*model.AgentRun
	runsByTask map[uuid.UUID][]*model.AgentRun
}

func newMockAgentRunRepo() *mockAgentRunRepo {
	return &mockAgentRunRepo{
		runs:       make(map[uuid.UUID]*model.AgentRun),
		runsByTask: make(map[uuid.UUID][]*model.AgentRun),
	}
}

func (m *mockAgentRunRepo) Create(_ context.Context, run *model.AgentRun) error {
	cloned := *run
	m.runs[run.ID] = &cloned
	m.runsByTask[run.TaskID] = append(m.runsByTask[run.TaskID], &cloned)
	return nil
}

func (m *mockAgentRunRepo) GetByID(_ context.Context, id uuid.UUID) (*model.AgentRun, error) {
	run, ok := m.runs[id]
	if !ok {
		return nil, service.ErrAgentNotFound
	}
	cloned := *run
	return &cloned, nil
}

func (m *mockAgentRunRepo) GetByTask(_ context.Context, taskID uuid.UUID) ([]*model.AgentRun, error) {
	runs := m.runsByTask[taskID]
	out := make([]*model.AgentRun, 0, len(runs))
	for _, run := range runs {
		cloned := *run
		out = append(out, &cloned)
	}
	return out, nil
}

func (m *mockAgentRunRepo) ListActive(_ context.Context) ([]*model.AgentRun, error) {
	return nil, nil
}

func (m *mockAgentRunRepo) UpdateStatus(_ context.Context, id uuid.UUID, status string) error {
	run, ok := m.runs[id]
	if !ok {
		return service.ErrAgentNotFound
	}
	run.Status = status
	return nil
}

func (m *mockAgentRunRepo) UpdateCost(_ context.Context, _ uuid.UUID, _, _, _ int64, _ float64, _ int) error {
	return nil
}

type mockAgentBridge struct{}

func (m *mockAgentBridge) Execute(_ context.Context, req service.BridgeExecuteRequest) (*service.BridgeExecuteResponse, error) {
	return &service.BridgeExecuteResponse{SessionID: req.TaskID + "-session"}, nil
}

func (m *mockAgentBridge) GetStatus(_ context.Context, _ string) (*service.BridgeStatusResponse, error) {
	return nil, nil
}

func (m *mockAgentBridge) Cancel(_ context.Context, _, _ string) error {
	return nil
}

type mockAgentTaskRepo struct {
	task *model.Task
}

func (m *mockAgentTaskRepo) GetByID(_ context.Context, id uuid.UUID) (*model.Task, error) {
	if m.task == nil || m.task.ID != id {
		return nil, service.ErrAgentTaskNotFound
	}
	cloned := *m.task
	return &cloned, nil
}

func (m *mockAgentTaskRepo) UpdateRuntime(_ context.Context, _ uuid.UUID, _, _, _ string) error {
	return nil
}

func (m *mockAgentTaskRepo) ClearRuntime(_ context.Context, _ uuid.UUID) error {
	return nil
}

type mockAgentProjectRepo struct {
	project *model.Project
}

func (m *mockAgentProjectRepo) GetByID(_ context.Context, id uuid.UUID) (*model.Project, error) {
	if m.project == nil || m.project.ID != id {
		return nil, service.ErrAgentProjectNotFound
	}
	cloned := *m.project
	return &cloned, nil
}

type mockWorktreeManager struct{}

func (m *mockWorktreeManager) Create(_ context.Context, _, _, _ string) (string, error) {
	return "/tmp/worktree", nil
}

func (m *mockWorktreeManager) Remove(_ context.Context, _, _ string) error {
	return nil
}

func TestAgentService_SpawnCreatesStartingRun(t *testing.T) {
	taskID := uuid.New()
	memberID := uuid.New()
	projectID := uuid.New()
	repo := newMockAgentRunRepo()
	taskRepo := &mockAgentTaskRepo{task: &model.Task{
		ID:          taskID,
		ProjectID:   projectID,
		Title:       "Spawn agent",
		Description: "Wire the spawn flow",
		BudgetUsd:   5,
	}}
	projectRepo := &mockAgentProjectRepo{project: &model.Project{ID: projectID, Slug: "agentforge"}}

	svc := service.NewAgentService(repo, taskRepo, projectRepo, ws.NewHub(), &mockAgentBridge{}, &mockWorktreeManager{})

	run, err := svc.Spawn(context.Background(), taskID, memberID, "anthropic", "claude-sonnet", 5)
	if err != nil {
		t.Fatalf("Spawn() error = %v", err)
	}

	if run.Status != model.AgentRunStatusRunning {
		t.Fatalf("status = %s, want %s", run.Status, model.AgentRunStatusRunning)
	}
	if run.TaskID != taskID {
		t.Fatalf("task id = %s, want %s", run.TaskID, taskID)
	}
	if len(repo.runsByTask[taskID]) != 1 {
		t.Fatalf("expected one run stored for task %s", taskID)
	}
}

func TestAgentService_SpawnRejectsExistingActiveRun(t *testing.T) {
	taskID := uuid.New()
	memberID := uuid.New()
	projectID := uuid.New()
	repo := newMockAgentRunRepo()
	repo.runsByTask[taskID] = []*model.AgentRun{
		{ID: uuid.New(), TaskID: taskID, Status: model.AgentRunStatusRunning},
	}
	taskRepo := &mockAgentTaskRepo{task: &model.Task{
		ID:          taskID,
		ProjectID:   projectID,
		Title:       "Spawn agent",
		Description: "Wire the spawn flow",
		BudgetUsd:   5,
	}}
	projectRepo := &mockAgentProjectRepo{project: &model.Project{ID: projectID, Slug: "agentforge"}}

	svc := service.NewAgentService(repo, taskRepo, projectRepo, ws.NewHub(), &mockAgentBridge{}, &mockWorktreeManager{})

	_, err := svc.Spawn(context.Background(), taskID, memberID, "anthropic", "claude-sonnet", 5)
	if err != service.ErrAgentAlreadyRunning {
		t.Fatalf("expected ErrAgentAlreadyRunning, got %v", err)
	}
}
