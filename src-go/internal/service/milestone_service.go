package service

import (
	"context"
	"fmt"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
)

type milestoneRepository interface {
	Create(ctx context.Context, milestone *model.Milestone) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Milestone, error)
	ListByProject(ctx context.Context, projectID uuid.UUID) ([]*model.Milestone, error)
	Update(ctx context.Context, milestone *model.Milestone) error
	Delete(ctx context.Context, id uuid.UUID) error
	GetWithMetrics(ctx context.Context, id uuid.UUID) (*model.Milestone, model.MilestoneMetrics, error)
}

type milestoneTaskRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Task, error)
	Update(ctx context.Context, id uuid.UUID, req *model.UpdateTaskRequest) error
}

type milestoneSprintRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Sprint, error)
	Update(ctx context.Context, sprint *model.Sprint) error
}

type MilestoneService struct {
	repo    milestoneRepository
	tasks   milestoneTaskRepository
	sprints milestoneSprintRepository
	now     func() time.Time
}

func NewMilestoneService(repo milestoneRepository, tasks milestoneTaskRepository, sprints milestoneSprintRepository) *MilestoneService {
	return &MilestoneService{
		repo:    repo,
		tasks:   tasks,
		sprints: sprints,
		now:     func() time.Time { return time.Now().UTC() },
	}
}

func (s *MilestoneService) CreateMilestone(ctx context.Context, milestone *model.Milestone) error {
	now := s.now()
	if milestone.ID == uuid.Nil {
		milestone.ID = uuid.New()
	}
	if milestone.CreatedAt.IsZero() {
		milestone.CreatedAt = now
	}
	milestone.UpdatedAt = now
	return s.repo.Create(ctx, milestone)
}

func (s *MilestoneService) GetMilestone(ctx context.Context, id uuid.UUID) (*model.Milestone, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *MilestoneService) ListMilestones(ctx context.Context, projectID uuid.UUID) ([]*model.Milestone, error) {
	return s.repo.ListByProject(ctx, projectID)
}

func (s *MilestoneService) UpdateMilestone(ctx context.Context, milestone *model.Milestone) error {
	milestone.UpdatedAt = s.now()
	return s.repo.Update(ctx, milestone)
}

func (s *MilestoneService) DeleteMilestone(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *MilestoneService) AssignTaskToMilestone(ctx context.Context, taskID uuid.UUID, milestoneID uuid.UUID) error {
	task, err := s.tasks.GetByID(ctx, taskID)
	if err != nil {
		return fmt.Errorf("get task: %w", err)
	}
	if task == nil {
		return fmt.Errorf("task %s not found", taskID)
	}
	milestoneIDString := milestoneID.String()
	return s.tasks.Update(ctx, taskID, &model.UpdateTaskRequest{MilestoneID: &milestoneIDString})
}

func (s *MilestoneService) AssignSprintToMilestone(ctx context.Context, sprintID uuid.UUID, milestoneID uuid.UUID) error {
	sprint, err := s.sprints.GetByID(ctx, sprintID)
	if err != nil {
		return fmt.Errorf("get sprint: %w", err)
	}
	if sprint == nil {
		return fmt.Errorf("sprint %s not found", sprintID)
	}
	sprint.MilestoneID = &milestoneID
	sprint.UpdatedAt = s.now()
	return s.sprints.Update(ctx, sprint)
}

func (s *MilestoneService) GetCompletionMetrics(ctx context.Context, milestoneID uuid.UUID) (model.MilestoneMetrics, error) {
	_, metrics, err := s.repo.GetWithMetrics(ctx, milestoneID)
	return metrics, err
}
