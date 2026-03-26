package service

import (
	"context"
	"fmt"
	"regexp"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
	"github.com/react-go-quick-starter/server/internal/ws"
	"gorm.io/gorm"
)

type entityLinkRepository interface {
	Create(ctx context.Context, link *model.EntityLink) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.EntityLink, error)
	Delete(ctx context.Context, id uuid.UUID) error
	ListBySource(ctx context.Context, projectID uuid.UUID, sourceType string, sourceID uuid.UUID) ([]*model.EntityLink, error)
	ListByTarget(ctx context.Context, projectID uuid.UUID, targetType string, targetID uuid.UUID) ([]*model.EntityLink, error)
	UpsertMentionLinks(ctx context.Context, projectID uuid.UUID, sourceType string, sourceID uuid.UUID, createdBy uuid.UUID, targets []model.EntityLinkTarget) error
	DeleteMentionLinksForSource(ctx context.Context, projectID uuid.UUID, sourceType string, sourceID uuid.UUID) error
}

type entityLinkTaskReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Task, error)
}

type entityLinkWikiReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.WikiPage, error)
}

type EntityLinkService struct {
	repo  entityLinkRepository
	tasks entityLinkTaskReader
	pages entityLinkWikiReader
	hub   *ws.Hub
}

type mentionLinkSyncer interface {
	SyncMentionLinksForSource(ctx context.Context, projectID uuid.UUID, sourceType string, sourceID uuid.UUID, createdBy uuid.UUID, content string) error
}

type CreateEntityLinkInput struct {
	ProjectID     uuid.UUID
	SourceType    string
	SourceID      uuid.UUID
	TargetType    string
	TargetID      uuid.UUID
	LinkType      string
	AnchorBlockID *string
	CreatedBy     uuid.UUID
}

func NewEntityLinkService(repo entityLinkRepository, tasks entityLinkTaskReader, pages entityLinkWikiReader) *EntityLinkService {
	return &EntityLinkService{repo: repo, tasks: tasks, pages: pages}
}

func (s *EntityLinkService) DB() *gorm.DB {
	binder, ok := s.repo.(interface{ DB() *gorm.DB })
	if !ok {
		return nil
	}
	return binder.DB()
}

func (s *EntityLinkService) WithDB(db *gorm.DB) mentionLinkSyncer {
	return &EntityLinkService{
		repo:  repository.NewEntityLinkRepository(db),
		tasks: s.tasks,
		pages: s.pages,
		hub:   s.hub,
	}
}

func (s *EntityLinkService) WithHub(hub *ws.Hub) *EntityLinkService {
	s.hub = hub
	return s
}

func (s *EntityLinkService) CreateLink(ctx context.Context, input *CreateEntityLinkInput) (*model.EntityLink, error) {
	link := &model.EntityLink{
		ID:            uuid.New(),
		ProjectID:     input.ProjectID,
		SourceType:    input.SourceType,
		SourceID:      input.SourceID,
		TargetType:    input.TargetType,
		TargetID:      input.TargetID,
		LinkType:      input.LinkType,
		AnchorBlockID: input.AnchorBlockID,
		CreatedBy:     input.CreatedBy,
	}
	if err := s.repo.Create(ctx, link); err != nil {
		return nil, fmt.Errorf("create entity link: %w", err)
	}
	s.broadcast(input.ProjectID, ws.EventLinkCreated, link.ToDTO())
	return link, nil
}

func (s *EntityLinkService) DeleteLink(ctx context.Context, linkID uuid.UUID) error {
	projectID := uuid.Nil
	if s.hub != nil {
		if link, err := s.repo.GetByID(ctx, linkID); err == nil && link != nil {
			projectID = link.ProjectID
		}
	}
	if err := s.repo.Delete(ctx, linkID); err != nil {
		return fmt.Errorf("delete entity link: %w", err)
	}
	if projectID != uuid.Nil {
		s.broadcast(projectID, ws.EventLinkDeleted, map[string]string{"id": linkID.String()})
	}
	return nil
}

func (s *EntityLinkService) ListLinksForEntity(ctx context.Context, projectID uuid.UUID, entityType string, entityID uuid.UUID) ([]*model.EntityLink, error) {
	outgoing, err := s.repo.ListBySource(ctx, projectID, entityType, entityID)
	if err != nil {
		return nil, fmt.Errorf("list links by source: %w", err)
	}
	incoming, err := s.repo.ListByTarget(ctx, projectID, entityType, entityID)
	if err != nil {
		return nil, fmt.Errorf("list links by target: %w", err)
	}

	seen := make(map[uuid.UUID]struct{}, len(outgoing)+len(incoming))
	links := make([]*model.EntityLink, 0, len(outgoing)+len(incoming))
	for _, link := range append(outgoing, incoming...) {
		if _, ok := seen[link.ID]; ok {
			continue
		}
		seen[link.ID] = struct{}{}
		links = append(links, link)
	}
	return links, nil
}

func (s *EntityLinkService) GetRelatedDocs(ctx context.Context, projectID uuid.UUID, entityType string, entityID uuid.UUID) ([]*model.WikiPage, error) {
	links, err := s.ListLinksForEntity(ctx, projectID, entityType, entityID)
	if err != nil {
		return nil, err
	}

	seen := make(map[uuid.UUID]struct{})
	pages := make([]*model.WikiPage, 0)
	for _, link := range links {
		pageID := uuid.Nil
		switch {
		case link.SourceType == model.EntityTypeWikiPage:
			pageID = link.SourceID
		case link.TargetType == model.EntityTypeWikiPage:
			pageID = link.TargetID
		}
		if pageID == uuid.Nil {
			continue
		}
		if _, ok := seen[pageID]; ok {
			continue
		}
		seen[pageID] = struct{}{}
		page, err := s.pages.GetByID(ctx, pageID)
		if err == nil && page != nil {
			pages = append(pages, page)
		}
	}
	return pages, nil
}

func (s *EntityLinkService) GetRelatedTasks(ctx context.Context, projectID uuid.UUID, entityType string, entityID uuid.UUID) ([]*model.Task, error) {
	links, err := s.ListLinksForEntity(ctx, projectID, entityType, entityID)
	if err != nil {
		return nil, err
	}

	seen := make(map[uuid.UUID]struct{})
	tasks := make([]*model.Task, 0)
	for _, link := range links {
		taskID := uuid.Nil
		switch {
		case link.SourceType == model.EntityTypeTask:
			taskID = link.SourceID
		case link.TargetType == model.EntityTypeTask:
			taskID = link.TargetID
		}
		if taskID == uuid.Nil {
			continue
		}
		if _, ok := seen[taskID]; ok {
			continue
		}
		seen[taskID] = struct{}{}
		task, err := s.tasks.GetByID(ctx, taskID)
		if err == nil && task != nil {
			tasks = append(tasks, task)
		}
	}
	return tasks, nil
}

func (s *EntityLinkService) SyncMentionLinksForSource(ctx context.Context, projectID uuid.UUID, sourceType string, sourceID uuid.UUID, createdBy uuid.UUID, content string) error {
	targets := ExtractBacklinkTargets(content)
	if err := s.repo.DeleteMentionLinksForSource(ctx, projectID, sourceType, sourceID); err != nil {
		return fmt.Errorf("delete previous mention links: %w", err)
	}
	if len(targets) == 0 {
		return nil
	}
	if err := s.repo.UpsertMentionLinks(ctx, projectID, sourceType, sourceID, createdBy, targets); err != nil {
		return fmt.Errorf("upsert mention links: %w", err)
	}
	return nil
}

var backlinkPattern = regexp.MustCompile(`\[\[(task|page)-([0-9a-fA-F-]{36})\]\]`)

func ExtractBacklinkTargets(content string) []model.EntityLinkTarget {
	matches := backlinkPattern.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(matches))
	targets := make([]model.EntityLinkTarget, 0, len(matches))
	for _, match := range matches {
		entityID, err := uuid.Parse(match[2])
		if err != nil {
			continue
		}
		entityType := model.EntityTypeTask
		if match[1] == "page" {
			entityType = model.EntityTypeWikiPage
		}
		key := entityType + ":" + entityID.String()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		targets = append(targets, model.EntityLinkTarget{
			EntityType: entityType,
			EntityID:   entityID,
		})
	}
	return targets
}

func (s *EntityLinkService) broadcast(projectID uuid.UUID, eventType string, payload any) {
	if s.hub == nil {
		return
	}
	s.hub.BroadcastEvent(&ws.Event{
		Type:      eventType,
		ProjectID: projectID.String(),
		Payload:   payload,
	})
}
