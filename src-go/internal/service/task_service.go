package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/ws"
)

// TaskRepository defines persistence for tasks.
type TaskRepository interface {
	Create(ctx context.Context, task *model.Task) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Task, error)
	List(ctx context.Context, projectID uuid.UUID, q model.TaskListQuery) ([]*model.Task, int, error)
	Update(ctx context.Context, id uuid.UUID, req *model.UpdateTaskRequest) error
	Delete(ctx context.Context, id uuid.UUID) error
	TransitionStatus(ctx context.Context, id uuid.UUID, newStatus string) error
	UpdateAssignee(ctx context.Context, id uuid.UUID, assigneeID uuid.UUID, assigneeType string) error
}

type TaskService struct {
	repo TaskRepository
	hub  *ws.Hub
}

func NewTaskService(repo TaskRepository, hub *ws.Hub) *TaskService {
	return &TaskService{repo: repo, hub: hub}
}

func (s *TaskService) Create(ctx context.Context, projectID uuid.UUID, req *model.CreateTaskRequest, reporterID *uuid.UUID) (*model.Task, error) {
	task := &model.Task{
		ID:        uuid.New(),
		ProjectID: projectID,
		Title:     req.Title,
		Description: req.Description,
		Status:    model.TaskStatusInbox,
		Priority:  req.Priority,
		ReporterID: reporterID,
		Labels:    req.Labels,
		BudgetUsd: req.BudgetUsd,
	}
	if req.ParentID != nil {
		id, err := uuid.Parse(*req.ParentID)
		if err != nil {
			return nil, fmt.Errorf("invalid parent id: %w", err)
		}
		task.ParentID = &id
	}
	if req.SprintID != nil {
		id, err := uuid.Parse(*req.SprintID)
		if err != nil {
			return nil, fmt.Errorf("invalid sprint id: %w", err)
		}
		task.SprintID = &id
	}

	if err := s.repo.Create(ctx, task); err != nil {
		return nil, fmt.Errorf("create task: %w", err)
	}

	s.hub.BroadcastEvent(&ws.Event{
		Type:      ws.EventTaskCreated,
		ProjectID: projectID.String(),
		Payload:   task.ToDTO(),
	})

	return task, nil
}

func (s *TaskService) GetByID(ctx context.Context, id uuid.UUID) (*model.Task, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *TaskService) List(ctx context.Context, projectID uuid.UUID, q model.TaskListQuery) ([]*model.Task, int, error) {
	return s.repo.List(ctx, projectID, q)
}

func (s *TaskService) Update(ctx context.Context, id uuid.UUID, req *model.UpdateTaskRequest) (*model.Task, error) {
	if err := s.repo.Update(ctx, id, req); err != nil {
		return nil, fmt.Errorf("update task: %w", err)
	}
	task, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	s.hub.BroadcastEvent(&ws.Event{
		Type:      ws.EventTaskUpdated,
		ProjectID: task.ProjectID.String(),
		Payload:   task.ToDTO(),
	})

	return task, nil
}

func (s *TaskService) Delete(ctx context.Context, id uuid.UUID) error {
	task, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete task: %w", err)
	}

	s.hub.BroadcastEvent(&ws.Event{
		Type:      ws.EventTaskDeleted,
		ProjectID: task.ProjectID.String(),
		Payload:   map[string]string{"id": id.String()},
	})
	return nil
}

func (s *TaskService) Transition(ctx context.Context, id uuid.UUID, req *model.TransitionRequest) (*model.Task, error) {
	if err := s.repo.TransitionStatus(ctx, id, req.Status); err != nil {
		return nil, err
	}
	task, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	s.hub.BroadcastEvent(&ws.Event{
		Type:      ws.EventTaskTransitioned,
		ProjectID: task.ProjectID.String(),
		Payload: map[string]any{
			"task":   task.ToDTO(),
			"reason": req.Reason,
		},
	})
	return task, nil
}

func (s *TaskService) Assign(ctx context.Context, id uuid.UUID, req *model.AssignRequest) (*model.Task, error) {
	assigneeID, err := uuid.Parse(req.AssigneeID)
	if err != nil {
		return nil, fmt.Errorf("invalid assignee id: %w", err)
	}
	if req.AssigneeType != model.MemberTypeHuman && req.AssigneeType != model.MemberTypeAgent {
		return nil, errors.New("assignee type must be 'human' or 'agent'")
	}

	if err := s.repo.UpdateAssignee(ctx, id, assigneeID, req.AssigneeType); err != nil {
		return nil, fmt.Errorf("assign task: %w", err)
	}

	// Auto-transition to assigned status.
	_ = s.repo.TransitionStatus(ctx, id, model.TaskStatusAssigned)

	task, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	s.hub.BroadcastEvent(&ws.Event{
		Type:      ws.EventTaskAssigned,
		ProjectID: task.ProjectID.String(),
		Payload:   task.ToDTO(),
	})
	return task, nil
}
