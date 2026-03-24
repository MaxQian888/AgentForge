package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

var (
	ErrTaskNotFound             = errors.New("task not found")
	ErrTaskAlreadyDecomposed    = errors.New("task already has child tasks")
	ErrInvalidTaskDecomposition = errors.New("invalid task decomposition")
)

type TaskDecompositionRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Task, error)
	HasChildren(ctx context.Context, parentID uuid.UUID) (bool, error)
	CreateChildren(ctx context.Context, inputs []model.TaskChildInput) ([]*model.Task, error)
}

type BridgeDecomposeRequest struct {
	TaskID      string `json:"task_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    string `json:"priority"`
	Provider    string `json:"provider,omitempty"`
	Model       string `json:"model,omitempty"`
}

type BridgeDecomposeSubtask struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    string `json:"priority"`
}

type BridgeDecomposeResponse struct {
	Summary  string                   `json:"summary"`
	Subtasks []BridgeDecomposeSubtask `json:"subtasks"`
}

type TaskDecompositionBridge interface {
	DecomposeTask(ctx context.Context, req BridgeDecomposeRequest) (*BridgeDecomposeResponse, error)
}

type TaskDecompositionService struct {
	repo   TaskDecompositionRepository
	bridge TaskDecompositionBridge
}

func NewTaskDecompositionService(repo TaskDecompositionRepository, bridge TaskDecompositionBridge) *TaskDecompositionService {
	return &TaskDecompositionService{repo: repo, bridge: bridge}
}

func (s *TaskDecompositionService) Decompose(ctx context.Context, taskID uuid.UUID) (*model.TaskDecompositionResponse, error) {
	parent, err := s.repo.GetByID(ctx, taskID)
	if err != nil {
		return nil, ErrTaskNotFound
	}

	hasChildren, err := s.repo.HasChildren(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("check child tasks: %w", err)
	}
	if hasChildren {
		return nil, ErrTaskAlreadyDecomposed
	}

	if s.bridge == nil {
		return nil, fmt.Errorf("bridge client unavailable")
	}

	result, err := s.bridge.DecomposeTask(ctx, BridgeDecomposeRequest{
		TaskID:      parent.ID.String(),
		Title:       parent.Title,
		Description: parent.Description,
		Priority:    normalizeTaskPriority(parent.Priority, "medium"),
	})
	if err != nil {
		return nil, fmt.Errorf("bridge decompose task: %w", err)
	}
	if err := validateTaskDecomposition(result); err != nil {
		return nil, err
	}

	inputs := make([]model.TaskChildInput, 0, len(result.Subtasks))
	for _, subtask := range result.Subtasks {
		inputs = append(inputs, model.TaskChildInput{
			ParentID:    parent.ID,
			ProjectID:   parent.ProjectID,
			SprintID:    parent.SprintID,
			ReporterID:  parent.ReporterID,
			Title:       strings.TrimSpace(subtask.Title),
			Description: strings.TrimSpace(subtask.Description),
			Priority:    normalizeTaskPriority(subtask.Priority, normalizeTaskPriority(parent.Priority, "medium")),
			Labels:      append([]string(nil), parent.Labels...),
			BudgetUSD:   0,
		})
	}

	children, err := s.repo.CreateChildren(ctx, inputs)
	if err != nil {
		return nil, fmt.Errorf("create child tasks: %w", err)
	}

	response := &model.TaskDecompositionResponse{
		ParentTask: parent.ToDTO(),
		Summary:    strings.TrimSpace(result.Summary),
		Subtasks:   make([]model.TaskDTO, 0, len(children)),
	}
	for _, child := range children {
		response.Subtasks = append(response.Subtasks, child.ToDTO())
	}
	return response, nil
}

func validateTaskDecomposition(result *BridgeDecomposeResponse) error {
	if result == nil {
		return ErrInvalidTaskDecomposition
	}
	if strings.TrimSpace(result.Summary) == "" || len(result.Subtasks) == 0 {
		return ErrInvalidTaskDecomposition
	}
	for _, subtask := range result.Subtasks {
		if strings.TrimSpace(subtask.Title) == "" || strings.TrimSpace(subtask.Description) == "" {
			return ErrInvalidTaskDecomposition
		}
	}
	return nil
}

func normalizeTaskPriority(priority string, fallback string) string {
	switch strings.ToLower(strings.TrimSpace(priority)) {
	case "critical":
		return "critical"
	case "high":
		return "high"
	case "medium":
		return "medium"
	case "low":
		return "low"
	default:
		switch strings.ToLower(strings.TrimSpace(fallback)) {
		case "critical", "high", "medium", "low":
			return strings.ToLower(strings.TrimSpace(fallback))
		default:
			return "medium"
		}
	}
}
