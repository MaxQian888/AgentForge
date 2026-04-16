package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	eventbus "github.com/react-go-quick-starter/server/internal/eventbus"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
	"github.com/react-go-quick-starter/server/internal/ws"
	"github.com/react-go-quick-starter/server/pkg/database"
	"gorm.io/gorm"
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
	repo       TaskRepository
	hub        *ws.Hub
	bus        eventbus.Publisher
	linkSyncer mentionLinkSyncer
}

type taskRepositoryTxBinder interface {
	TaskRepository
	DB() *gorm.DB
}

type mentionLinkSyncerTxBinder interface {
	mentionLinkSyncer
	DB() *gorm.DB
	WithDB(db *gorm.DB) mentionLinkSyncer
}

func NewTaskService(repo TaskRepository, hub *ws.Hub, bus eventbus.Publisher) *TaskService {
	return &TaskService{repo: repo, hub: hub, bus: bus}
}

func (s *TaskService) WithEntityLinkSyncer(linkSyncer mentionLinkSyncer) *TaskService {
	s.linkSyncer = linkSyncer
	return s
}

func (s *TaskService) Create(ctx context.Context, projectID uuid.UUID, req *model.CreateTaskRequest, reporterID *uuid.UUID) (*model.Task, error) {
	task := &model.Task{
		ID:          uuid.New(),
		ProjectID:   projectID,
		Title:       req.Title,
		Description: req.Description,
		Status:      model.TaskStatusInbox,
		Priority:    req.Priority,
		ReporterID:  reporterID,
		Labels:      req.Labels,
		BudgetUsd:   req.BudgetUsd,
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

	_ = eventbus.PublishLegacy(ctx, s.bus, ws.EventTaskCreated, projectID.String(), task.ToDTO())

	return task, nil
}

func (s *TaskService) GetByID(ctx context.Context, id uuid.UUID) (*model.Task, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *TaskService) List(ctx context.Context, projectID uuid.UUID, q model.TaskListQuery) ([]*model.Task, int, error) {
	return s.repo.List(ctx, projectID, q)
}

func (s *TaskService) Update(ctx context.Context, id uuid.UUID, req *model.UpdateTaskRequest) (*model.Task, error) {
	if req.Description != nil && s.linkSyncer != nil {
		if binder, ok := s.repo.(taskRepositoryTxBinder); ok && binder.DB() != nil {
			if linkBinder, ok := s.linkSyncer.(mentionLinkSyncerTxBinder); ok {
				var task *model.Task
				err := database.WithTx(ctx, binder.DB(), func(tx *gorm.DB) error {
					txRepo := repository.NewTaskRepository(tx)
					txSyncer := linkBinder.WithDB(tx)
					if err := txRepo.Update(ctx, id, req); err != nil {
						return err
					}
					updated, err := txRepo.GetByID(ctx, id)
					if err != nil {
						return err
					}
					if updated.ReporterID != nil {
						if err := txSyncer.SyncMentionLinksForSource(ctx, updated.ProjectID, model.EntityTypeTask, updated.ID, *updated.ReporterID, updated.Description); err != nil {
							return err
						}
					}
					task = updated
					return nil
				})
				if err != nil {
					return nil, fmt.Errorf("update task: %w", err)
				}
				_ = eventbus.PublishLegacy(ctx, s.bus, ws.EventTaskUpdated, task.ProjectID.String(), task.ToDTO())
				return task, nil
			}
		}
	}

	if err := s.repo.Update(ctx, id, req); err != nil {
		return nil, fmt.Errorf("update task: %w", err)
	}
	task, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if s.linkSyncer != nil && req.Description != nil && task.ReporterID != nil {
		if err := s.linkSyncer.SyncMentionLinksForSource(ctx, task.ProjectID, model.EntityTypeTask, task.ID, *task.ReporterID, task.Description); err != nil {
			return nil, fmt.Errorf("sync task mention links: %w", err)
		}
	}

	_ = eventbus.PublishLegacy(ctx, s.bus, ws.EventTaskUpdated, task.ProjectID.String(), task.ToDTO())

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

	_ = eventbus.PublishLegacy(ctx, s.bus, ws.EventTaskDeleted, task.ProjectID.String(), map[string]string{"id": id.String()})
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

	_ = eventbus.PublishLegacy(ctx, s.bus, ws.EventTaskTransitioned, task.ProjectID.String(), map[string]any{
		"task":   task.ToDTO(),
		"reason": req.Reason,
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

	_ = eventbus.PublishLegacy(ctx, s.bus, ws.EventTaskAssigned, task.ProjectID.String(), task.ToDTO())
	return task, nil
}
