package service

import (
	"context"
	"testing"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
)

type fakeMilestoneRepo struct {
	milestones map[uuid.UUID]*model.Milestone
	metrics    map[uuid.UUID]model.MilestoneMetrics
	created    *model.Milestone
	updated    *model.Milestone
	deletedID  uuid.UUID
}

func (f *fakeMilestoneRepo) Create(_ context.Context, milestone *model.Milestone) error {
	f.created = milestone
	if f.milestones == nil {
		f.milestones = map[uuid.UUID]*model.Milestone{}
	}
	f.milestones[milestone.ID] = milestone
	return nil
}

func (f *fakeMilestoneRepo) GetByID(_ context.Context, id uuid.UUID) (*model.Milestone, error) {
	return f.milestones[id], nil
}

func (f *fakeMilestoneRepo) ListByProject(_ context.Context, projectID uuid.UUID) ([]*model.Milestone, error) {
	result := make([]*model.Milestone, 0)
	for _, milestone := range f.milestones {
		if milestone.ProjectID == projectID {
			result = append(result, milestone)
		}
	}
	return result, nil
}

func (f *fakeMilestoneRepo) Update(_ context.Context, milestone *model.Milestone) error {
	f.updated = milestone
	f.milestones[milestone.ID] = milestone
	return nil
}

func (f *fakeMilestoneRepo) Delete(_ context.Context, id uuid.UUID) error {
	f.deletedID = id
	return nil
}

func (f *fakeMilestoneRepo) GetWithMetrics(_ context.Context, id uuid.UUID) (*model.Milestone, model.MilestoneMetrics, error) {
	return f.milestones[id], f.metrics[id], nil
}

type fakeMilestoneTaskRepo struct {
	task      *model.Task
	updateID  uuid.UUID
	updateReq *model.UpdateTaskRequest
}

func (f *fakeMilestoneTaskRepo) GetByID(_ context.Context, id uuid.UUID) (*model.Task, error) {
	return f.task, nil
}

func (f *fakeMilestoneTaskRepo) Update(_ context.Context, id uuid.UUID, req *model.UpdateTaskRequest) error {
	f.updateID = id
	f.updateReq = req
	return nil
}

type fakeMilestoneSprintRepo struct {
	sprint  *model.Sprint
	updated *model.Sprint
}

func (f *fakeMilestoneSprintRepo) GetByID(_ context.Context, id uuid.UUID) (*model.Sprint, error) {
	return f.sprint, nil
}

func (f *fakeMilestoneSprintRepo) Update(_ context.Context, sprint *model.Sprint) error {
	f.updated = sprint
	return nil
}

func TestMilestoneServiceCreateAssignAndMetrics(t *testing.T) {
	projectID := uuid.New()
	milestoneID := uuid.New()
	repo := &fakeMilestoneRepo{
		milestones: map[uuid.UUID]*model.Milestone{
			milestoneID: {ID: milestoneID, ProjectID: projectID, Name: "v2.0"},
		},
		metrics: map[uuid.UUID]model.MilestoneMetrics{
			milestoneID: {TotalTasks: 3, CompletedTasks: 2, CompletionRate: 66.67},
		},
	}
	taskRepo := &fakeMilestoneTaskRepo{task: &model.Task{ID: uuid.New(), ProjectID: projectID}}
	sprintRepo := &fakeMilestoneSprintRepo{sprint: &model.Sprint{ID: uuid.New(), ProjectID: projectID, Name: "Sprint 7"}}
	service := NewMilestoneService(repo, taskRepo, sprintRepo)

	milestone := &model.Milestone{ProjectID: projectID, Name: "v3.0", Status: model.MilestoneStatusPlanned}
	if err := service.CreateMilestone(context.Background(), milestone); err != nil {
		t.Fatalf("CreateMilestone() error = %v", err)
	}
	if milestone.ID == uuid.Nil {
		t.Fatal("expected CreateMilestone to assign an ID")
	}

	if err := service.AssignTaskToMilestone(context.Background(), taskRepo.task.ID, milestoneID); err != nil {
		t.Fatalf("AssignTaskToMilestone() error = %v", err)
	}
	if taskRepo.updateReq == nil || taskRepo.updateReq.MilestoneID == nil || *taskRepo.updateReq.MilestoneID != milestoneID.String() {
		t.Fatalf("unexpected task update request: %+v", taskRepo.updateReq)
	}

	if err := service.AssignSprintToMilestone(context.Background(), sprintRepo.sprint.ID, milestoneID); err != nil {
		t.Fatalf("AssignSprintToMilestone() error = %v", err)
	}
	if sprintRepo.updated == nil || sprintRepo.updated.MilestoneID == nil || *sprintRepo.updated.MilestoneID != milestoneID {
		t.Fatalf("unexpected sprint update: %+v", sprintRepo.updated)
	}

	metrics, err := service.GetCompletionMetrics(context.Background(), milestoneID)
	if err != nil {
		t.Fatalf("GetCompletionMetrics() error = %v", err)
	}
	if metrics.TotalTasks != 3 || metrics.CompletedTasks != 2 {
		t.Fatalf("unexpected metrics: %+v", metrics)
	}
}

func TestMilestoneServiceUpdateAndDelete(t *testing.T) {
	projectID := uuid.New()
	milestoneID := uuid.New()
	repo := &fakeMilestoneRepo{
		milestones: map[uuid.UUID]*model.Milestone{
			milestoneID: {
				ID:        milestoneID,
				ProjectID: projectID,
				Name:      "v2.0",
			},
		},
		metrics: map[uuid.UUID]model.MilestoneMetrics{},
	}
	service := NewMilestoneService(repo, &fakeMilestoneTaskRepo{}, &fakeMilestoneSprintRepo{})

	targetDate := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	update := &model.Milestone{
		ID:         milestoneID,
		ProjectID:  projectID,
		Name:       "v2.1",
		TargetDate: &targetDate,
		Status:     model.MilestoneStatusInProgress,
	}
	if err := service.UpdateMilestone(context.Background(), update); err != nil {
		t.Fatalf("UpdateMilestone() error = %v", err)
	}
	if repo.updated == nil || repo.updated.Name != "v2.1" {
		t.Fatalf("unexpected updated milestone: %+v", repo.updated)
	}
	if err := service.DeleteMilestone(context.Background(), milestoneID); err != nil {
		t.Fatalf("DeleteMilestone() error = %v", err)
	}
	if repo.deletedID != milestoneID {
		t.Fatalf("repo.deletedID = %s, want %s", repo.deletedID, milestoneID)
	}
}
