package service

import (
	"context"
	"testing"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
)

type stubDocTaskRepo struct {
	created []*model.Task
}

func (r *stubDocTaskRepo) Create(_ context.Context, task *model.Task) error {
	cloned := *task
	r.created = append(r.created, &cloned)
	return nil
}

func (r *stubDocTaskRepo) CreateChildren(_ context.Context, inputs []model.TaskChildInput) ([]*model.Task, error) {
	tasks := make([]*model.Task, 0, len(inputs))
	for _, input := range inputs {
		task := &model.Task{
			ID:          uuid.New(),
			ProjectID:   input.ProjectID,
			ParentID:    &input.ParentID,
			Title:       input.Title,
			Description: input.Description,
			Status:      model.TaskStatusInbox,
			Priority:    input.Priority,
			ReporterID:  input.ReporterID,
			Labels:      input.Labels,
		}
		r.created = append(r.created, task)
		tasks = append(tasks, task)
	}
	return tasks, nil
}

type stubDocWikiRepo struct {
	page     *model.WikiPage
	spacesBy map[uuid.UUID]*model.WikiSpace
}

func (r *stubDocWikiRepo) GetByID(_ context.Context, id uuid.UUID) (*model.WikiPage, error) {
	if r.page == nil || r.page.ID != id {
		return nil, ErrWikiPageNotFound
	}
	cloned := *r.page
	return &cloned, nil
}

func (r *stubDocWikiRepo) GetSpaceByID(_ context.Context, id uuid.UUID) (*model.WikiSpace, error) {
	if r.spacesBy == nil {
		return nil, ErrWikiSpaceNotFound
	}
	space, ok := r.spacesBy[id]
	if !ok {
		return nil, ErrWikiSpaceNotFound
	}
	cloned := *space
	return &cloned, nil
}

func TestDocDecompositionServiceCreatesTasksAndRequirementLinks(t *testing.T) {
	projectID := uuid.New()
	pageID := uuid.New()
	spaceID := uuid.New()
	parentTaskID := uuid.New()
	createdBy := uuid.New()
	taskRepo := &stubDocTaskRepo{}
	linkRepo := &stubEntityLinkRepo{}
	svc := NewDocDecompositionService(taskRepo, &stubDocWikiRepo{
		page: &model.WikiPage{
			ID:      pageID,
			SpaceID: spaceID,
			Content: `[{"id":"block-a","type":"paragraph","content":"First task"},{"id":"block-b","type":"paragraph","content":[{"type":"text","text":"Second task"}]}]`,
		},
		spacesBy: map[uuid.UUID]*model.WikiSpace{
			spaceID: {ID: spaceID, ProjectID: projectID},
		},
	}, linkRepo)

	resp, err := svc.DecomposeTasksFromBlocks(context.Background(), projectID, pageID, []string{"block-a", "block-b"}, &parentTaskID, &createdBy)
	if err != nil {
		t.Fatalf("DecomposeTasksFromBlocks() error = %v", err)
	}
	if len(resp.Tasks) != 2 {
		t.Fatalf("len(resp.Tasks) = %d, want 2", len(resp.Tasks))
	}
	if len(linkRepo.links) != 2 {
		t.Fatalf("len(linkRepo.links) = %d, want 2", len(linkRepo.links))
	}
	if linkRepo.links[0].LinkType != model.EntityLinkTypeRequirement || linkRepo.links[0].AnchorBlockID == nil || *linkRepo.links[0].AnchorBlockID != "block-a" {
		t.Fatalf("first link = %+v", linkRepo.links[0])
	}
}

func TestDocDecompositionServiceRejectsPageOutsideProject(t *testing.T) {
	projectID := uuid.New()
	otherProjectID := uuid.New()
	pageID := uuid.New()
	spaceID := uuid.New()
	taskRepo := &stubDocTaskRepo{}
	linkRepo := &stubEntityLinkRepo{}
	svc := NewDocDecompositionService(taskRepo, &stubDocWikiRepo{
		page: &model.WikiPage{
			ID:      pageID,
			SpaceID: spaceID,
			Content: `[{"id":"block-a","type":"paragraph","content":"Foreign task"}]`,
		},
		spacesBy: map[uuid.UUID]*model.WikiSpace{
			spaceID: {ID: spaceID, ProjectID: otherProjectID},
		},
	}, linkRepo)

	if _, err := svc.DecomposeTasksFromBlocks(context.Background(), projectID, pageID, []string{"block-a"}, nil, nil); err != ErrWikiPageNotFound {
		t.Fatalf("DecomposeTasksFromBlocks() error = %v, want %v", err, ErrWikiPageNotFound)
	}
	if len(taskRepo.created) != 0 {
		t.Fatalf("created tasks = %d, want 0", len(taskRepo.created))
	}
	if len(linkRepo.links) != 0 {
		t.Fatalf("created links = %d, want 0", len(linkRepo.links))
	}
}
