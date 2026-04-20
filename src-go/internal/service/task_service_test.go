package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/repository"
	"github.com/agentforge/server/internal/ws"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type stubTaskServiceRepo struct {
	tasks         map[uuid.UUID]*model.Task
	lastUpdateReq *model.UpdateTaskRequest
}

func (r *stubTaskServiceRepo) Create(_ context.Context, task *model.Task) error {
	cloned := *task
	if r.tasks == nil {
		r.tasks = make(map[uuid.UUID]*model.Task)
	}
	r.tasks[task.ID] = &cloned
	return nil
}

func (r *stubTaskServiceRepo) GetByID(_ context.Context, id uuid.UUID) (*model.Task, error) {
	task, ok := r.tasks[id]
	if !ok {
		return nil, ErrTaskNotFound
	}
	cloned := *task
	return &cloned, nil
}

func (r *stubTaskServiceRepo) List(_ context.Context, _ uuid.UUID, _ model.TaskListQuery) ([]*model.Task, int, error) {
	return nil, 0, nil
}

func (r *stubTaskServiceRepo) Update(_ context.Context, id uuid.UUID, req *model.UpdateTaskRequest) error {
	r.lastUpdateReq = req
	task := r.tasks[id]
	if req.Title != nil {
		task.Title = *req.Title
	}
	if req.Description != nil {
		task.Description = *req.Description
	}
	task.UpdatedAt = time.Now().UTC()
	return nil
}

func (r *stubTaskServiceRepo) Delete(_ context.Context, id uuid.UUID) error {
	delete(r.tasks, id)
	return nil
}

func (r *stubTaskServiceRepo) TransitionStatus(_ context.Context, id uuid.UUID, newStatus string) error {
	r.tasks[id].Status = newStatus
	return nil
}

func (r *stubTaskServiceRepo) UpdateAssignee(_ context.Context, id uuid.UUID, assigneeID uuid.UUID, assigneeType string) error {
	r.tasks[id].AssigneeID = &assigneeID
	r.tasks[id].AssigneeType = assigneeType
	return nil
}

func TestTaskServiceUpdateSyncsMentionLinksWhenDescriptionChanges(t *testing.T) {
	projectID := uuid.New()
	taskID := uuid.New()
	reporterID := uuid.New()
	repo := &stubTaskServiceRepo{
		tasks: map[uuid.UUID]*model.Task{
			taskID: {
				ID:          taskID,
				ProjectID:   projectID,
				Title:       "Wire docs",
				Description: "Initial",
				Status:      model.TaskStatusTriaged,
				Priority:    "high",
				ReporterID:  &reporterID,
				CreatedAt:   time.Date(2026, 3, 26, 20, 0, 0, 0, time.UTC),
				UpdatedAt:   time.Date(2026, 3, 26, 20, 0, 0, 0, time.UTC),
			},
		},
	}
	linkSyncer := &stubMentionLinkSyncer{}
	description := "See [[page-" + uuid.New().String() + "]]"
	svc := NewTaskService(repo, ws.NewHub(), nil).WithEntityLinkSyncer(linkSyncer)

	updated, err := svc.Update(context.Background(), taskID, &model.UpdateTaskRequest{
		Description: &description,
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Description != description {
		t.Fatalf("updated description = %q, want %q", updated.Description, description)
	}
	if linkSyncer.calls != 1 {
		t.Fatalf("link sync calls = %d, want 1", linkSyncer.calls)
	}
	if linkSyncer.projectID != projectID || linkSyncer.sourceType != model.EntityTypeTask || linkSyncer.sourceID != taskID || linkSyncer.createdBy != reporterID {
		t.Fatalf("link sync args = %+v", linkSyncer)
	}
	if linkSyncer.content != description {
		t.Fatalf("link sync content = %q, want %q", linkSyncer.content, description)
	}
}

type failingMentionLinkSyncer struct {
	base *EntityLinkService
	fail bool
}

func (s *failingMentionLinkSyncer) SyncMentionLinksForSource(ctx context.Context, projectID uuid.UUID, sourceType string, sourceID uuid.UUID, createdBy uuid.UUID, content string) error {
	if s.fail {
		return errors.New("forced mention link failure")
	}
	return s.base.SyncMentionLinksForSource(ctx, projectID, sourceType, sourceID, createdBy, content)
}

func (s *failingMentionLinkSyncer) DB() *gorm.DB {
	return s.base.DB()
}

func (s *failingMentionLinkSyncer) WithDB(db *gorm.DB) mentionLinkSyncer {
	rebound, _ := s.base.WithDB(db).(*EntityLinkService)
	return &failingMentionLinkSyncer{base: rebound, fail: s.fail}
}

func TestTaskServiceUpdateRollsBackWhenMentionSyncFails(t *testing.T) {
	db := openServiceTxTestDB(t)
	projectID := uuid.New()
	taskID := uuid.New()
	reporterID := uuid.New()
	if err := db.Exec(`CREATE TABLE tasks (
		id TEXT PRIMARY KEY,
		project_id TEXT NOT NULL,
		title TEXT,
		description TEXT,
		status TEXT,
		priority TEXT,
		reporter_id TEXT,
		created_at DATETIME,
		updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create tasks table: %v", err)
	}
	if err := db.Exec(`CREATE TABLE entity_links (
		id TEXT PRIMARY KEY,
		project_id TEXT NOT NULL,
		source_type TEXT NOT NULL,
		source_id TEXT NOT NULL,
		target_type TEXT NOT NULL,
		target_id TEXT NOT NULL,
		link_type TEXT NOT NULL,
		anchor_block_id TEXT,
		created_by TEXT NOT NULL,
		created_at DATETIME,
		deleted_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create entity_links table: %v", err)
	}
	if err := db.Exec(
		`INSERT INTO tasks (id, project_id, title, description, status, priority, reporter_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		taskID.String(),
		projectID.String(),
		"Backlink task",
		"before",
		model.TaskStatusTriaged,
		"high",
		reporterID.String(),
		time.Date(2026, 3, 26, 21, 0, 0, 0, time.UTC),
		time.Date(2026, 3, 26, 21, 0, 0, 0, time.UTC),
	).Error; err != nil {
		t.Fatalf("insert task: %v", err)
	}

	taskRepo := repository.NewTaskRepository(db)
	syncer := &failingMentionLinkSyncer{
		base: NewEntityLinkService(repository.NewEntityLinkRepository(db), nil, nil),
		fail: true,
	}
	svc := NewTaskService(taskRepo, ws.NewHub(), nil).WithEntityLinkSyncer(syncer)
	description := "after [[page-" + uuid.New().String() + "]]"

	if _, err := svc.Update(context.Background(), taskID, &model.UpdateTaskRequest{Description: &description}); err == nil {
		t.Fatal("expected Update() to fail when mention sync fails")
	}

	task, err := taskRepo.GetByID(context.Background(), taskID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if task.Description != "before" {
		t.Fatalf("task description = %q, want rollback to keep %q", task.Description, "before")
	}
}

func openServiceTxTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	return db
}
