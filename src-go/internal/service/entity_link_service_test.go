package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

type stubEntityLinkRepo struct {
	links []*model.EntityLink
}

func (r *stubEntityLinkRepo) Create(_ context.Context, link *model.EntityLink) error {
	cloned := *link
	r.links = append(r.links, &cloned)
	return nil
}

func (r *stubEntityLinkRepo) GetByID(_ context.Context, id uuid.UUID) (*model.EntityLink, error) {
	for _, link := range r.links {
		if link.ID == id {
			cloned := *link
			return &cloned, nil
		}
	}
	return nil, errors.New("link not found")
}

func (r *stubEntityLinkRepo) Delete(_ context.Context, id uuid.UUID) error {
	for _, link := range r.links {
		if link.ID == id {
			now := time.Now().UTC()
			link.DeletedAt = &now
		}
	}
	return nil
}

func (r *stubEntityLinkRepo) ListBySource(_ context.Context, projectID uuid.UUID, sourceType string, sourceID uuid.UUID) ([]*model.EntityLink, error) {
	result := make([]*model.EntityLink, 0)
	for _, link := range r.links {
		if link.ProjectID == projectID && link.SourceType == sourceType && link.SourceID == sourceID && link.DeletedAt == nil {
			cloned := *link
			result = append(result, &cloned)
		}
	}
	return result, nil
}

func (r *stubEntityLinkRepo) ListByTarget(_ context.Context, projectID uuid.UUID, targetType string, targetID uuid.UUID) ([]*model.EntityLink, error) {
	result := make([]*model.EntityLink, 0)
	for _, link := range r.links {
		if link.ProjectID == projectID && link.TargetType == targetType && link.TargetID == targetID && link.DeletedAt == nil {
			cloned := *link
			result = append(result, &cloned)
		}
	}
	return result, nil
}

func (r *stubEntityLinkRepo) UpsertMentionLinks(_ context.Context, projectID uuid.UUID, sourceType string, sourceID uuid.UUID, createdBy uuid.UUID, targets []model.EntityLinkTarget) error {
	for _, target := range targets {
		found := false
		for _, existing := range r.links {
			if existing.ProjectID == projectID &&
				existing.SourceType == sourceType &&
				existing.SourceID == sourceID &&
				existing.TargetType == target.EntityType &&
				existing.TargetID == target.EntityID &&
				existing.LinkType == model.EntityLinkTypeMention {
				existing.DeletedAt = nil
				found = true
				break
			}
		}
		if found {
			continue
		}
		r.links = append(r.links, &model.EntityLink{
			ID:         uuid.New(),
			ProjectID:  projectID,
			SourceType: sourceType,
			SourceID:   sourceID,
			TargetType: target.EntityType,
			TargetID:   target.EntityID,
			LinkType:   model.EntityLinkTypeMention,
			CreatedBy:  createdBy,
			CreatedAt:  time.Now().UTC(),
		})
	}
	return nil
}

func (r *stubEntityLinkRepo) DeleteMentionLinksForSource(_ context.Context, projectID uuid.UUID, sourceType string, sourceID uuid.UUID) error {
	for _, link := range r.links {
		if link.ProjectID == projectID && link.SourceType == sourceType && link.SourceID == sourceID && link.LinkType == model.EntityLinkTypeMention && link.DeletedAt == nil {
			now := time.Now().UTC()
			link.DeletedAt = &now
		}
	}
	return nil
}

type stubEntityLinkTaskRepo struct {
	tasks map[uuid.UUID]*model.Task
}

func (r *stubEntityLinkTaskRepo) GetByID(_ context.Context, id uuid.UUID) (*model.Task, error) {
	task, ok := r.tasks[id]
	if !ok {
		return nil, ErrTaskNotFound
	}
	cloned := *task
	return &cloned, nil
}

type stubEntityLinkWikiRepo struct {
	pages map[uuid.UUID]*model.WikiPage
}

func (r *stubEntityLinkWikiRepo) GetByID(_ context.Context, id uuid.UUID) (*model.WikiPage, error) {
	page, ok := r.pages[id]
	if !ok {
		return nil, ErrWikiPageNotFound
	}
	cloned := *page
	return &cloned, nil
}

func TestEntityLinkServiceCreateListAndDelete(t *testing.T) {
	projectID := uuid.New()
	taskID := uuid.New()
	pageID := uuid.New()
	repo := &stubEntityLinkRepo{}
	svc := NewEntityLinkService(repo, &stubEntityLinkTaskRepo{
		tasks: map[uuid.UUID]*model.Task{
			taskID: {ID: taskID, ProjectID: projectID, Title: "Write PRD links"},
		},
	}, &stubEntityLinkWikiRepo{
		pages: map[uuid.UUID]*model.WikiPage{
			pageID: {ID: pageID, SpaceID: uuid.New(), Title: "PRD"},
		},
	})

	link, err := svc.CreateLink(context.Background(), &CreateEntityLinkInput{
		ProjectID:  projectID,
		SourceType: model.EntityTypeTask,
		SourceID:   taskID,
		TargetType: model.EntityTypeWikiPage,
		TargetID:   pageID,
		LinkType:   model.EntityLinkTypeRequirement,
		CreatedBy:  uuid.New(),
	})
	if err != nil {
		t.Fatalf("CreateLink() error = %v", err)
	}

	links, err := svc.ListLinksForEntity(context.Background(), projectID, model.EntityTypeTask, taskID)
	if err != nil {
		t.Fatalf("ListLinksForEntity() error = %v", err)
	}
	if len(links) != 1 || links[0].ID != link.ID {
		t.Fatalf("ListLinksForEntity() = %+v, want [%s]", links, link.ID)
	}

	docs, err := svc.GetRelatedDocs(context.Background(), projectID, model.EntityTypeTask, taskID)
	if err != nil {
		t.Fatalf("GetRelatedDocs() error = %v", err)
	}
	if len(docs) != 1 || docs[0].ID != pageID {
		t.Fatalf("GetRelatedDocs() = %+v, want [%s]", docs, pageID)
	}

	tasks, err := svc.GetRelatedTasks(context.Background(), projectID, model.EntityTypeWikiPage, pageID)
	if err != nil {
		t.Fatalf("GetRelatedTasks() error = %v", err)
	}
	if len(tasks) != 1 || tasks[0].ID != taskID {
		t.Fatalf("GetRelatedTasks() = %+v, want [%s]", tasks, taskID)
	}

	if err := svc.DeleteLink(context.Background(), link.ID); err != nil {
		t.Fatalf("DeleteLink() error = %v", err)
	}

	links, err = svc.ListLinksForEntity(context.Background(), projectID, model.EntityTypeTask, taskID)
	if err != nil {
		t.Fatalf("ListLinksForEntity() after delete error = %v", err)
	}
	if len(links) != 0 {
		t.Fatalf("len(ListLinksForEntity() after delete) = %d, want 0", len(links))
	}
}

func TestExtractBacklinkTargetsParsesTaskAndPageReferences(t *testing.T) {
	taskID := uuid.New()
	pageID := uuid.New()

	targets := ExtractBacklinkTargets("Refs [[task-" + taskID.String() + "]] and [[page-" + pageID.String() + "]] plus duplicate [[task-" + taskID.String() + "]]")

	if len(targets) != 2 {
		t.Fatalf("len(ExtractBacklinkTargets()) = %d, want 2", len(targets))
	}
	if targets[0].EntityType != model.EntityTypeTask || targets[0].EntityID != taskID {
		t.Fatalf("targets[0] = %+v, want task %s", targets[0], taskID)
	}
	if targets[1].EntityType != model.EntityTypeWikiPage || targets[1].EntityID != pageID {
		t.Fatalf("targets[1] = %+v, want page %s", targets[1], pageID)
	}
}

func TestEntityLinkServiceSyncMentionLinksForSourceReplacesBacklinks(t *testing.T) {
	projectID := uuid.New()
	sourceID := uuid.New()
	createdBy := uuid.New()
	oldTaskID := uuid.New()
	newPageID := uuid.New()
	repo := &stubEntityLinkRepo{
		links: []*model.EntityLink{
			{
				ID:         uuid.New(),
				ProjectID:  projectID,
				SourceType: model.EntityTypeTask,
				SourceID:   sourceID,
				TargetType: model.EntityTypeTask,
				TargetID:   oldTaskID,
				LinkType:   model.EntityLinkTypeMention,
				CreatedBy:  createdBy,
				CreatedAt:  time.Now().UTC(),
			},
		},
	}
	svc := NewEntityLinkService(repo, &stubEntityLinkTaskRepo{}, &stubEntityLinkWikiRepo{})

	if err := svc.SyncMentionLinksForSource(context.Background(), projectID, model.EntityTypeTask, sourceID, createdBy, "See [[page-"+newPageID.String()+"]]"); err != nil {
		t.Fatalf("SyncMentionLinksForSource() error = %v", err)
	}

	links, err := svc.ListLinksForEntity(context.Background(), projectID, model.EntityTypeTask, sourceID)
	if err != nil {
		t.Fatalf("ListLinksForEntity() error = %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("len(ListLinksForEntity()) = %d, want 1", len(links))
	}
	if links[0].TargetType != model.EntityTypeWikiPage || links[0].TargetID != newPageID || links[0].LinkType != model.EntityLinkTypeMention {
		t.Fatalf("links[0] = %+v, want mention page %s", links[0], newPageID)
	}
}
