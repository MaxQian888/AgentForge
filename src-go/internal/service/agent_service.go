package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	bridgeclient "github.com/react-go-quick-starter/server/internal/bridge"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/ws"
)

// AgentRunRepository defines persistence for agent runs.
type AgentRunRepository interface {
	Create(ctx context.Context, run *model.AgentRun) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.AgentRun, error)
	GetByTask(ctx context.Context, taskID uuid.UUID) ([]*model.AgentRun, error)
	ListActive(ctx context.Context) ([]*model.AgentRun, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status string) error
	UpdateCost(ctx context.Context, id uuid.UUID, inputTokens, outputTokens, cacheReadTokens int64, costUsd float64, turnCount int) error
}

type AgentTaskRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Task, error)
	UpdateRuntime(ctx context.Context, id uuid.UUID, branch, worktreePath, sessionID string) error
	ClearRuntime(ctx context.Context, id uuid.UUID) error
}

type AgentProjectRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Project, error)
}

type WorktreeManager interface {
	Create(ctx context.Context, projectSlug, taskID, branchName string) (string, error)
	Remove(ctx context.Context, projectSlug, taskID string) error
}

// BridgeClient defines the interface for calling the TypeScript bridge.
type BridgeClient interface {
	Execute(ctx context.Context, req BridgeExecuteRequest) (*BridgeExecuteResponse, error)
	GetStatus(ctx context.Context, taskID string) (*BridgeStatusResponse, error)
	Cancel(ctx context.Context, taskID, reason string) error
}

type BridgeExecuteRequest = bridgeclient.ExecuteRequest
type BridgeExecuteResponse = bridgeclient.ExecuteResponse
type BridgeStatusResponse = bridgeclient.StatusResponse

var (
	ErrAgentAlreadyRunning  = errors.New("agent already running for this task")
	ErrAgentNotFound        = errors.New("agent run not found")
	ErrAgentNotRunning      = errors.New("agent is not running")
	ErrAgentTaskNotFound    = errors.New("agent task not found")
	ErrAgentProjectNotFound = errors.New("agent project not found")
)

type AgentService struct {
	runRepo   AgentRunRepository
	taskRepo  AgentTaskRepository
	projects  AgentProjectRepository
	hub       *ws.Hub
	bridge    BridgeClient
	worktrees WorktreeManager
}

func NewAgentService(
	runRepo AgentRunRepository,
	taskRepo AgentTaskRepository,
	projects AgentProjectRepository,
	hub *ws.Hub,
	bridge BridgeClient,
	worktrees WorktreeManager,
) *AgentService {
	return &AgentService{
		runRepo:   runRepo,
		taskRepo:  taskRepo,
		projects:  projects,
		hub:       hub,
		bridge:    bridge,
		worktrees: worktrees,
	}
}

// Spawn creates a run, provisions a worktree, starts bridge execution, and publishes lifecycle updates.
func (s *AgentService) Spawn(ctx context.Context, taskID, memberID uuid.UUID, provider, modelName string, budgetUsd float64) (*model.AgentRun, error) {
	runs, err := s.runRepo.GetByTask(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("check existing runs: %w", err)
	}
	for _, r := range runs {
		if r.Status == model.AgentRunStatusRunning || r.Status == model.AgentRunStatusStarting {
			return nil, ErrAgentAlreadyRunning
		}
	}

	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return nil, ErrAgentTaskNotFound
	}
	project, err := s.projects.GetByID(ctx, task.ProjectID)
	if err != nil {
		return nil, ErrAgentProjectNotFound
	}

	run := &model.AgentRun{
		ID:        uuid.New(),
		TaskID:    taskID,
		MemberID:  memberID,
		Status:    model.AgentRunStatusStarting,
		Provider:  provider,
		Model:     modelName,
		StartedAt: time.Now().UTC(),
	}
	if err := s.runRepo.Create(ctx, run); err != nil {
		return nil, fmt.Errorf("create agent run: %w", err)
	}

	branchName := fmt.Sprintf("agent/%s", taskID.String())
	worktreePath, err := s.worktrees.Create(ctx, project.Slug, taskID.String(), branchName)
	if err != nil {
		_ = s.failSpawn(ctx, run, task, project.Slug, false)
		return nil, fmt.Errorf("create worktree: %w", err)
	}

	sessionID := uuid.New().String()
	resp, err := s.bridge.Execute(ctx, BridgeExecuteRequest{
		TaskID:         taskID.String(),
		SessionID:      sessionID,
		MemberID:       memberID.String(),
		Provider:       provider,
		Model:          modelName,
		Prompt:         buildSpawnPrompt(task),
		WorktreePath:   worktreePath,
		BranchName:     branchName,
		MaxTurns:       50,
		BudgetUSD:      resolveSpawnBudget(task.BudgetUsd, budgetUsd),
		PermissionMode: "default",
	})
	if err != nil {
		_ = s.failSpawn(ctx, run, task, project.Slug, true)
		return nil, fmt.Errorf("start bridge execution: %w", err)
	}
	if resp != nil && resp.SessionID != "" {
		sessionID = resp.SessionID
	}

	if err := s.taskRepo.UpdateRuntime(ctx, task.ID, branchName, worktreePath, sessionID); err != nil {
		_ = s.failSpawn(ctx, run, task, project.Slug, true)
		return nil, fmt.Errorf("persist task runtime: %w", err)
	}
	if err := s.runRepo.UpdateStatus(ctx, run.ID, model.AgentRunStatusRunning); err != nil {
		_ = s.failSpawn(ctx, run, task, project.Slug, true)
		return nil, fmt.Errorf("mark run running: %w", err)
	}

	run.Status = model.AgentRunStatusRunning
	s.broadcastEvent(ws.EventAgentStarted, task.ProjectID.String(), run.ToDTO())
	return run, nil
}

// UpdateStatus changes the status of an agent run.
func (s *AgentService) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	if err := s.runRepo.UpdateStatus(ctx, id, status); err != nil {
		return fmt.Errorf("update agent status: %w", err)
	}

	run, err := s.runRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	eventType := ws.EventAgentProgress
	switch status {
	case model.AgentRunStatusCompleted:
		eventType = ws.EventAgentCompleted
	case model.AgentRunStatusFailed, model.AgentRunStatusCancelled, model.AgentRunStatusBudgetExceeded:
		eventType = ws.EventAgentFailed
	}

	s.broadcastEvent(eventType, s.lookupProjectID(ctx, run.TaskID), run.ToDTO())
	return nil
}

// UpdateCost records cost data for an agent run.
func (s *AgentService) UpdateCost(ctx context.Context, id uuid.UUID, inputTokens, outputTokens, cacheReadTokens int64, costUsd float64, turnCount int) error {
	if err := s.runRepo.UpdateCost(ctx, id, inputTokens, outputTokens, cacheReadTokens, costUsd, turnCount); err != nil {
		return fmt.Errorf("update agent cost: %w", err)
	}

	run, err := s.runRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	s.broadcastEvent(ws.EventAgentCostUpdate, s.lookupProjectID(ctx, run.TaskID), run.ToDTO())
	return nil
}

// GetByID returns an agent run by ID.
func (s *AgentService) GetByID(ctx context.Context, id uuid.UUID) (*model.AgentRun, error) {
	return s.runRepo.GetByID(ctx, id)
}

// GetByTask returns all agent runs for a task.
func (s *AgentService) GetByTask(ctx context.Context, taskID uuid.UUID) ([]*model.AgentRun, error) {
	return s.runRepo.GetByTask(ctx, taskID)
}

// ListActive returns all currently active agent runs.
func (s *AgentService) ListActive(ctx context.Context) ([]*model.AgentRun, error) {
	return s.runRepo.ListActive(ctx)
}

// Cancel stops a running agent.
func (s *AgentService) Cancel(ctx context.Context, id uuid.UUID, reason string) error {
	run, err := s.runRepo.GetByID(ctx, id)
	if err != nil {
		return ErrAgentNotFound
	}
	if run.Status != model.AgentRunStatusRunning && run.Status != model.AgentRunStatusStarting {
		return ErrAgentNotRunning
	}

	if s.bridge != nil {
		_ = s.bridge.Cancel(ctx, run.TaskID.String(), reason)
	}

	return s.UpdateStatus(ctx, id, model.AgentRunStatusCancelled)
}

func (s *AgentService) failSpawn(ctx context.Context, run *model.AgentRun, task *model.Task, projectSlug string, removeWorktree bool) error {
	if err := s.runRepo.UpdateStatus(ctx, run.ID, model.AgentRunStatusFailed); err != nil {
		return err
	}
	run.Status = model.AgentRunStatusFailed
	if s.taskRepo != nil {
		_ = s.taskRepo.ClearRuntime(ctx, task.ID)
	}
	if removeWorktree && s.worktrees != nil {
		_ = s.worktrees.Remove(ctx, projectSlug, task.ID.String())
	}
	s.broadcastEvent(ws.EventAgentFailed, task.ProjectID.String(), run.ToDTO())
	return nil
}

func (s *AgentService) lookupProjectID(ctx context.Context, taskID uuid.UUID) string {
	if s.taskRepo == nil {
		return ""
	}
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return ""
	}
	return task.ProjectID.String()
}

func (s *AgentService) broadcastEvent(eventType, projectID string, payload any) {
	if s.hub == nil {
		return
	}
	s.hub.BroadcastEvent(&ws.Event{
		Type:      eventType,
		ProjectID: projectID,
		Payload:   payload,
	})
}

func buildSpawnPrompt(task *model.Task) string {
	var prompt strings.Builder
	prompt.WriteString(strings.TrimSpace(task.Title))
	if desc := strings.TrimSpace(task.Description); desc != "" {
		prompt.WriteString("\n\n")
		prompt.WriteString(desc)
	}
	return prompt.String()
}

func resolveSpawnBudget(taskBudget, requestBudget float64) float64 {
	switch {
	case requestBudget > 0:
		return requestBudget
	case taskBudget > 0:
		return taskBudget
	default:
		return 1
	}
}
