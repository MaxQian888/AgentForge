package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
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

// BridgeClient defines the interface for calling the TypeScript bridge.
type BridgeClient interface {
	Execute(ctx context.Context, req BridgeExecuteRequest) (*BridgeExecuteResponse, error)
	GetStatus(ctx context.Context, taskID string) (*BridgeStatusResponse, error)
	Cancel(ctx context.Context, taskID, reason string) error
}

// BridgeExecuteRequest is sent to the TS bridge to start an agent.
type BridgeExecuteRequest struct {
	TaskID    string `json:"taskId"`
	MemberID  string `json:"memberId"`
	Provider  string `json:"provider"`
	Model     string `json:"model"`
	Prompt    string `json:"prompt"`
	Worktree  string `json:"worktree"`
	BudgetUsd float64 `json:"budgetUsd"`
}

// BridgeExecuteResponse is returned by the bridge after starting an agent.
type BridgeExecuteResponse struct {
	SessionID string `json:"sessionId"`
}

// BridgeStatusResponse is the agent status from the bridge.
type BridgeStatusResponse struct {
	Status      string  `json:"status"`
	CostUsd     float64 `json:"costUsd"`
	TurnCount   int     `json:"turnCount"`
	InputTokens int64   `json:"inputTokens"`
	OutputTokens int64  `json:"outputTokens"`
}

var (
	ErrAgentAlreadyRunning = errors.New("agent already running for this task")
	ErrAgentNotFound       = errors.New("agent run not found")
	ErrAgentNotRunning     = errors.New("agent is not running")
)

type AgentService struct {
	runRepo AgentRunRepository
	hub     *ws.Hub
	bridge  BridgeClient
}

func NewAgentService(runRepo AgentRunRepository, hub *ws.Hub, bridge BridgeClient) *AgentService {
	return &AgentService{runRepo: runRepo, hub: hub, bridge: bridge}
}

// Spawn creates a new agent run record and notifies via WebSocket.
// The actual bridge call is deferred to the orchestrator.
func (s *AgentService) Spawn(ctx context.Context, taskID, memberID uuid.UUID, provider, modelName string, budgetUsd float64) (*model.AgentRun, error) {
	// Check for existing active run on this task.
	runs, err := s.runRepo.GetByTask(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("check existing runs: %w", err)
	}
	for _, r := range runs {
		if r.Status == model.AgentRunStatusRunning || r.Status == model.AgentRunStatusStarting {
			return nil, ErrAgentAlreadyRunning
		}
	}

	run := &model.AgentRun{
		ID:        uuid.New(),
		TaskID:    taskID,
		MemberID:  memberID,
		Status:    model.AgentRunStatusStarting,
		Provider:  provider,
		Model:     modelName,
		StartedAt: time.Now(),
	}

	if err := s.runRepo.Create(ctx, run); err != nil {
		return nil, fmt.Errorf("create agent run: %w", err)
	}

	s.hub.BroadcastEvent(&ws.Event{
		Type:    ws.EventAgentStarted,
		Payload: run.ToDTO(),
	})

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

	s.hub.BroadcastEvent(&ws.Event{
		Type:    eventType,
		Payload: run.ToDTO(),
	})
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

	s.hub.BroadcastEvent(&ws.Event{
		Type:    ws.EventAgentCostUpdate,
		Payload: run.ToDTO(),
	})
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

	// Cancel via bridge if available.
	if s.bridge != nil {
		_ = s.bridge.Cancel(ctx, run.TaskID.String(), reason)
	}

	return s.UpdateStatus(ctx, id, model.AgentRunStatusCancelled)
}
