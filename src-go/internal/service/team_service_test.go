package service_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/service"
	"github.com/react-go-quick-starter/server/internal/ws"
)

type mockTeamRunRepo struct {
	team *model.AgentTeam
}

func (m *mockTeamRunRepo) Create(_ context.Context, team *model.AgentTeam) error {
	cloned := *team
	m.team = &cloned
	return nil
}

func (m *mockTeamRunRepo) GetByID(_ context.Context, id uuid.UUID) (*model.AgentTeam, error) {
	if m.team == nil || m.team.ID != id {
		return nil, service.ErrTeamNotFound
	}
	cloned := *m.team
	return &cloned, nil
}

func (m *mockTeamRunRepo) GetByTask(_ context.Context, taskID uuid.UUID) (*model.AgentTeam, error) {
	if m.team == nil || m.team.TaskID != taskID {
		return nil, service.ErrTeamNotFound
	}
	cloned := *m.team
	return &cloned, nil
}

func (m *mockTeamRunRepo) GetTeamSummary(_ context.Context, id uuid.UUID) (*model.AgentTeamSummaryDTO, error) {
	if m.team == nil || m.team.ID != id {
		return nil, service.ErrTeamNotFound
	}
	return nil, service.ErrTeamNotFound
}

func (m *mockTeamRunRepo) ListByProject(_ context.Context, projectID uuid.UUID, _ string) ([]*model.AgentTeam, error) {
	if m.team == nil || m.team.ProjectID != projectID {
		return nil, nil
	}
	cloned := *m.team
	return []*model.AgentTeam{&cloned}, nil
}

func (m *mockTeamRunRepo) ListTeamSummaries(_ context.Context, projectID uuid.UUID, _ string) ([]*model.AgentTeamSummaryDTO, error) {
	if m.team == nil || m.team.ProjectID != projectID {
		return nil, nil
	}
	return nil, service.ErrTeamNotFound
}

func (m *mockTeamRunRepo) ListActive(_ context.Context) ([]*model.AgentTeam, error) {
	if m.team == nil {
		return nil, nil
	}
	cloned := *m.team
	return []*model.AgentTeam{&cloned}, nil
}

func (m *mockTeamRunRepo) UpdateStatus(_ context.Context, id uuid.UUID, status string) error {
	if m.team == nil || m.team.ID != id {
		return service.ErrTeamNotFound
	}
	m.team.Status = status
	return nil
}

func (m *mockTeamRunRepo) UpdateStatusWithError(_ context.Context, id uuid.UUID, status, errorMessage string) error {
	if m.team == nil || m.team.ID != id {
		return service.ErrTeamNotFound
	}
	m.team.Status = status
	m.team.ErrorMessage = errorMessage
	return nil
}

func (m *mockTeamRunRepo) UpdateSpent(_ context.Context, id uuid.UUID, spent float64) error {
	if m.team == nil || m.team.ID != id {
		return service.ErrTeamNotFound
	}
	m.team.TotalSpentUsd = spent
	return nil
}

func (m *mockTeamRunRepo) SetPlannerRun(_ context.Context, id uuid.UUID, plannerRunID uuid.UUID) error {
	if m.team == nil || m.team.ID != id {
		return service.ErrTeamNotFound
	}
	m.team.PlannerRunID = &plannerRunID
	return nil
}

func (m *mockTeamRunRepo) SetReviewerRun(_ context.Context, id uuid.UUID, reviewerRunID uuid.UUID) error {
	if m.team == nil || m.team.ID != id {
		return service.ErrTeamNotFound
	}
	m.team.ReviewerRunID = &reviewerRunID
	return nil
}

func (m *mockTeamRunRepo) Delete(_ context.Context, id uuid.UUID) error {
	if m.team == nil || m.team.ID != id {
		return service.ErrTeamNotFound
	}
	m.team = nil
	return nil
}

func (m *mockTeamRunRepo) Update(_ context.Context, id uuid.UUID, req *model.UpdateTeamRequest) error {
	if m.team == nil || m.team.ID != id {
		return service.ErrTeamNotFound
	}
	if req.Name != nil {
		m.team.Name = *req.Name
	}
	if req.TotalBudgetUsd != nil {
		m.team.TotalBudgetUsd = *req.TotalBudgetUsd
	}
	return nil
}

type mockTeamAgentRunRepo struct {
	runsByTeam     map[uuid.UUID][]*model.AgentRun
	teamFieldsByID map[uuid.UUID]struct {
		teamID   uuid.UUID
		teamRole string
	}
}

func newMockTeamAgentRunRepo() *mockTeamAgentRunRepo {
	return &mockTeamAgentRunRepo{
		runsByTeam: make(map[uuid.UUID][]*model.AgentRun),
		teamFieldsByID: make(map[uuid.UUID]struct {
			teamID   uuid.UUID
			teamRole string
		}),
	}
}

func (m *mockTeamAgentRunRepo) ListByTeam(_ context.Context, teamID uuid.UUID) ([]*model.AgentRun, error) {
	runs := m.runsByTeam[teamID]
	out := make([]*model.AgentRun, 0, len(runs))
	for _, run := range runs {
		cloned := *run
		out = append(out, &cloned)
	}
	return out, nil
}

func (m *mockTeamAgentRunRepo) SetTeamFields(_ context.Context, id uuid.UUID, teamID uuid.UUID, teamRole string) error {
	m.teamFieldsByID[id] = struct {
		teamID   uuid.UUID
		teamRole string
	}{teamID: teamID, teamRole: teamRole}
	return nil
}

func (m *mockTeamAgentRunRepo) GetByID(_ context.Context, id uuid.UUID) (*model.AgentRun, error) {
	for _, runs := range m.runsByTeam {
		for _, run := range runs {
			if run.ID == id {
				cloned := *run
				return &cloned, nil
			}
		}
	}
	return nil, service.ErrAgentNotFound
}

type teamSpawnCall struct {
	teamID    uuid.UUID
	teamRole  string
	taskID    uuid.UUID
	memberID  uuid.UUID
	runtime   string
	provider  string
	model     string
	budgetUsd float64
	roleID    string
}

type mockTeamSpawner struct {
	calls []teamSpawnCall
}

func (m *mockTeamSpawner) Spawn(_ context.Context, taskID, memberID uuid.UUID, runtime, provider, modelName string, budgetUsd float64, roleID string) (*model.AgentRun, error) {
	return m.recordSpawn(uuid.Nil, "", taskID, memberID, runtime, provider, modelName, budgetUsd, roleID)
}

func (m *mockTeamSpawner) SpawnForTeam(_ context.Context, teamID uuid.UUID, teamRole string, taskID, memberID uuid.UUID, runtime, provider, modelName string, budgetUsd float64, roleID string) (*model.AgentRun, error) {
	return m.recordSpawn(teamID, teamRole, taskID, memberID, runtime, provider, modelName, budgetUsd, roleID)
}

func (m *mockTeamSpawner) recordSpawn(teamID uuid.UUID, teamRole string, taskID, memberID uuid.UUID, runtime, provider, modelName string, budgetUsd float64, roleID string) (*model.AgentRun, error) {
	call := teamSpawnCall{
		teamID:    teamID,
		teamRole:  teamRole,
		taskID:    taskID,
		memberID:  memberID,
		runtime:   runtime,
		provider:  provider,
		model:     modelName,
		budgetUsd: budgetUsd,
		roleID:    roleID,
	}
	m.calls = append(m.calls, call)
	return &model.AgentRun{
		ID:       uuid.New(),
		TaskID:   taskID,
		MemberID: memberID,
		Runtime:  runtime,
		Provider: provider,
		Model:    modelName,
		Status:   model.AgentRunStatusRunning,
	}, nil
}

func (m *mockTeamSpawner) Cancel(_ context.Context, _ uuid.UUID, _ string) error {
	return nil
}

type mockTeamTaskRepo struct {
	task              *model.Task
	hasChildren       bool
	createdChildTasks []*model.Task
}

func (m *mockTeamTaskRepo) GetByID(_ context.Context, id uuid.UUID) (*model.Task, error) {
	if m.task == nil || m.task.ID != id {
		return nil, service.ErrTeamTaskNotFound
	}
	cloned := *m.task
	return &cloned, nil
}

func (m *mockTeamTaskRepo) HasChildren(_ context.Context, parentID uuid.UUID) (bool, error) {
	if m.task == nil || m.task.ID != parentID {
		return false, service.ErrTeamTaskNotFound
	}
	return m.hasChildren, nil
}

func (m *mockTeamTaskRepo) CreateChildren(_ context.Context, inputs []model.TaskChildInput) ([]*model.Task, error) {
	children := make([]*model.Task, 0, len(inputs))
	for _, input := range inputs {
		child := &model.Task{
			ID:          uuid.New(),
			ProjectID:   input.ProjectID,
			ParentID:    &input.ParentID,
			SprintID:    input.SprintID,
			Title:       input.Title,
			Description: input.Description,
			Priority:    input.Priority,
			BudgetUsd:   input.BudgetUSD,
		}
		children = append(children, child)
	}
	m.createdChildTasks = children
	return children, nil
}

func (m *mockTeamTaskRepo) ListChildren(_ context.Context, parentID uuid.UUID) ([]*model.Task, error) {
	if m.createdChildTasks != nil {
		return m.createdChildTasks, nil
	}
	return nil, nil
}

type mockTeamProjectRepo struct {
	project *model.Project
}

func (m *mockTeamProjectRepo) GetByID(_ context.Context, id uuid.UUID) (*model.Project, error) {
	if m.project == nil || m.project.ID != id {
		return nil, service.ErrAgentProjectNotFound
	}
	cloned := *m.project
	return &cloned, nil
}

func TestTeamService_StartTeamUsesProjectDefaultsAndPersistsRuntimeConfig(t *testing.T) {
	taskID := uuid.New()
	memberID := uuid.New()
	projectID := uuid.New()
	teamRepo := &mockTeamRunRepo{}
	runRepo := newMockTeamAgentRunRepo()
	spawner := &mockTeamSpawner{}
	taskRepo := &mockTeamTaskRepo{task: &model.Task{
		ID:          taskID,
		ProjectID:   projectID,
		Title:       "Coordinate provider support",
		Description: "Start the planning team",
		BudgetUsd:   12,
	}}
	projectRepo := &mockTeamProjectRepo{project: &model.Project{
		ID:       projectID,
		Slug:     "agentforge",
		Settings: `{"coding_agent":{"runtime":"codex","provider":"openai","model":"gpt-5-codex"}}`,
	}}

	svc := service.NewTeamService(teamRepo, runRepo, spawner, taskRepo, projectRepo, nil, ws.NewHub())

	team, err := svc.StartTeam(context.Background(), service.StartTeamInput{
		TaskID:         taskID,
		MemberID:       memberID,
		Name:           "Provider-complete team",
		Strategy:       "plan-code-review",
		TotalBudgetUsd: 12,
	})
	if err != nil {
		t.Fatalf("StartTeam() error = %v", err)
	}

	if team == nil {
		t.Fatal("expected team to be created")
	}
	if len(spawner.calls) != 1 {
		t.Fatalf("planner spawn calls = %d, want 1", len(spawner.calls))
	}
	planner := spawner.calls[0]
	if planner.runtime != "codex" || planner.provider != "openai" || planner.model != "gpt-5-codex" {
		t.Fatalf("planner spawn selection = %#v", planner)
	}
	if planner.teamID != team.ID || planner.teamRole != model.TeamRolePlanner {
		t.Fatalf("planner team context = %#v, want %s/%s", planner, team.ID, model.TeamRolePlanner)
	}
	if teamRepo.team == nil || !strings.Contains(teamRepo.team.Config, "\"runtime\":\"codex\"") {
		t.Fatalf("team config = %q, want persisted runtime selection", teamRepo.team.Config)
	}
}

func TestTeamService_ProcessRunCompletionPropagatesRuntimeConfigToCoders(t *testing.T) {
	taskID := uuid.New()
	memberID := uuid.New()
	projectID := uuid.New()
	teamID := uuid.New()
	teamRepo := &mockTeamRunRepo{team: &model.AgentTeam{
		ID:             teamID,
		ProjectID:      projectID,
		TaskID:         taskID,
		Name:           "Provider-complete team",
		Status:         model.TeamStatusPlanning,
		Strategy:       "plan-code-review",
		TotalBudgetUsd: 12,
		Config:         `{"runtime":"codex","provider":"openai","model":"gpt-5-codex"}`,
	}}
	runRepo := newMockTeamAgentRunRepo()
	spawner := &mockTeamSpawner{}
	taskRepo := &mockTeamTaskRepo{task: &model.Task{
		ID:          taskID,
		ProjectID:   projectID,
		Title:       "Coordinate provider support",
		Description: "Create child work",
		BudgetUsd:   12,
		AssigneeID:  &memberID,
	}}
	projectRepo := &mockTeamProjectRepo{project: &model.Project{ID: projectID, Slug: "agentforge"}}

	svc := service.NewTeamService(teamRepo, runRepo, spawner, taskRepo, projectRepo, nil, ws.NewHub())

	svc.ProcessRunCompletion(context.Background(), &model.AgentRun{
		ID:       uuid.New(),
		TaskID:   taskID,
		MemberID: memberID,
		TeamID:   &teamID,
		TeamRole: model.TeamRolePlanner,
		Status:   model.AgentRunStatusCompleted,
		Runtime:  "codex",
		Provider: "openai",
		Model:    "gpt-5-codex",
	})

	if len(spawner.calls) != 1 {
		t.Fatalf("coder spawn calls = %d, want 1", len(spawner.calls))
	}
	coder := spawner.calls[0]
	if coder.roleID != "coding-agent" {
		t.Fatalf("coder role id = %q, want coding-agent", coder.roleID)
	}
	if coder.runtime != "codex" || coder.provider != "openai" || coder.model != "gpt-5-codex" {
		t.Fatalf("coder spawn selection = %#v", coder)
	}
	if coder.teamID != teamID || coder.teamRole != model.TeamRoleCoder {
		t.Fatalf("coder team context = %#v, want %s/%s", coder, teamID, model.TeamRoleCoder)
	}
}
