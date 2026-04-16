package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	eventbus "github.com/react-go-quick-starter/server/internal/eventbus"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
	"github.com/react-go-quick-starter/server/internal/ws"
	"github.com/react-go-quick-starter/server/pkg/database"
	"gorm.io/gorm"
)

var (
	ErrWikiSpaceNotFound   = errors.New("wiki space not found")
	ErrWikiPageNotFound    = errors.New("wiki page not found")
	ErrWikiPageConflict    = errors.New("wiki page conflict")
	ErrWikiCircularMove    = errors.New("wiki circular move")
	ErrWikiTemplateNotFound = errors.New("wiki template not found")
	ErrWikiTemplateImmutable = errors.New("wiki template is immutable")
	ErrPageVersionNotFound = errors.New("page version not found")
	ErrPageCommentNotFound = errors.New("page comment not found")
)

type wikiSpaceRepository interface {
	Create(ctx context.Context, space *model.WikiSpace) error
	GetByProjectID(ctx context.Context, projectID uuid.UUID) (*model.WikiSpace, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.WikiSpace, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type wikiPageRepository interface {
	Create(ctx context.Context, page *model.WikiPage) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.WikiPage, error)
	Update(ctx context.Context, page *model.WikiPage) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
	ListTree(ctx context.Context, spaceID uuid.UUID) ([]*model.WikiPage, error)
	ListByParent(ctx context.Context, spaceID uuid.UUID, parentID *uuid.UUID) ([]*model.WikiPage, error)
	MovePage(ctx context.Context, id uuid.UUID, parentID *uuid.UUID, path string, sortOrder int) error
	UpdateSortOrder(ctx context.Context, id uuid.UUID, sortOrder int) error
}

type wikiPageRepositoryTxBinder interface {
	wikiPageRepository
	DB() *gorm.DB
}

type pageVersionRepository interface {
	Create(ctx context.Context, version *model.PageVersion) error
	ListByPageID(ctx context.Context, pageID uuid.UUID) ([]*model.PageVersion, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.PageVersion, error)
}

type pageCommentRepository interface {
	Create(ctx context.Context, comment *model.PageComment) error
	ListByPageID(ctx context.Context, pageID uuid.UUID) ([]*model.PageComment, error)
	Update(ctx context.Context, comment *model.PageComment) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
}

type pageFavoriteRepository interface {
	Add(ctx context.Context, pageID, userID uuid.UUID) error
	Remove(ctx context.Context, pageID, userID uuid.UUID) error
	ListByUser(ctx context.Context, userID uuid.UUID) ([]*model.PageFavorite, error)
	ListByPage(ctx context.Context, pageID uuid.UUID) ([]*model.PageFavorite, error)
}

type pageRecentAccessRepository interface {
	Touch(ctx context.Context, pageID, userID uuid.UUID, accessedAt time.Time) error
	ListByUser(ctx context.Context, userID uuid.UUID, limit int) ([]*model.PageRecentAccess, error)
}


type wikiNotificationCreator interface {
	Create(ctx context.Context, targetID uuid.UUID, ntype, title, body, data string) (*model.Notification, error)
}

type wikiIMNotifier interface {
	Notify(ctx context.Context, req *model.IMNotifyRequest) error
}

type WikiService struct {
	spaces      wikiSpaceRepository
	pages       wikiPageRepository
	versions    pageVersionRepository
	comments    pageCommentRepository
	favorites   pageFavoriteRepository
	recent      pageRecentAccessRepository
	bus         eventbus.Publisher
	notifier    wikiNotificationCreator
	imNotifier  wikiIMNotifier
	imChannels  IMEventChannelResolver
	imPlatform  string
	imChannelID string
	linkSyncer  mentionLinkSyncer
	now         func() time.Time
}

func NewWikiService(
	spaces wikiSpaceRepository,
	pages wikiPageRepository,
	versions pageVersionRepository,
	comments pageCommentRepository,
	favorites pageFavoriteRepository,
	recent pageRecentAccessRepository,
	bus eventbus.Publisher,
) *WikiService {
	return &WikiService{
		spaces:    spaces,
		pages:     pages,
		versions:  versions,
		comments:  comments,
		favorites: favorites,
		recent:    recent,
		bus:       bus,
		now:       func() time.Time { return time.Now().UTC() },
	}
}

func (s *WikiService) WithNotificationCreator(notifier wikiNotificationCreator) *WikiService {
	s.notifier = notifier
	return s
}

func (s *WikiService) WithIMForwarder(notifier wikiIMNotifier, platform, channelID string) *WikiService {
	s.imNotifier = notifier
	s.imPlatform = strings.TrimSpace(platform)
	s.imChannelID = strings.TrimSpace(channelID)
	return s
}

func (s *WikiService) WithIMChannelResolver(resolver IMEventChannelResolver) *WikiService {
	s.imChannels = resolver
	return s
}

func (s *WikiService) WithEntityLinkSyncer(linkSyncer mentionLinkSyncer) *WikiService {
	s.linkSyncer = linkSyncer
	return s
}

func (s *WikiService) CreateSpace(ctx context.Context, projectID uuid.UUID) (*model.WikiSpace, error) {
	if s.spaces == nil {
		return nil, ErrWikiSpaceNotFound
	}
	space := &model.WikiSpace{
		ID:        uuid.New(),
		ProjectID: projectID,
		CreatedAt: s.now(),
	}
	if err := s.spaces.Create(ctx, space); err != nil {
		return nil, fmt.Errorf("create wiki space: %w", err)
	}
	return space, nil
}

func (s *WikiService) GetSpaceByProjectID(ctx context.Context, projectID uuid.UUID) (*model.WikiSpace, error) {
	if s.spaces == nil {
		return nil, ErrWikiSpaceNotFound
	}
	space, err := s.spaces.GetByProjectID(ctx, projectID)
	if err != nil {
		return nil, ErrWikiSpaceNotFound
	}
	return space, nil
}

func (s *WikiService) GetSpaceByID(ctx context.Context, spaceID uuid.UUID) (*model.WikiSpace, error) {
	if s.spaces == nil {
		return nil, ErrWikiSpaceNotFound
	}
	space, err := s.spaces.GetByID(ctx, spaceID)
	if err != nil {
		return nil, ErrWikiSpaceNotFound
	}
	return space, nil
}

func (s *WikiService) DeleteProjectSpace(ctx context.Context, projectID uuid.UUID) error {
	space, err := s.GetSpaceByProjectID(ctx, projectID)
	if err != nil {
		return err
	}
	if err := s.spaces.Delete(ctx, space.ID); err != nil {
		return fmt.Errorf("delete wiki project space: %w", err)
	}
	return nil
}

func (s *WikiService) CreatePage(
	ctx context.Context,
	projectID uuid.UUID,
	spaceID uuid.UUID,
	title string,
	parentID *uuid.UUID,
	content string,
	createdBy *uuid.UUID,
) (*model.WikiPage, error) {
	now := s.now()
	trimmedTitle := strings.TrimSpace(title)
	if trimmedTitle == "" {
		trimmedTitle = "Untitled"
	}

	sortOrder := 0
	path := ""
	if parentID != nil {
		parent, err := s.getPage(ctx, *parentID)
		if err != nil {
			return nil, err
		}
		children, err := s.pages.ListByParent(ctx, spaceID, parentID)
		if err != nil {
			return nil, fmt.Errorf("list sibling pages: %w", err)
		}
		sortOrder = len(children)
		path = parent.Path + "/" + uuid.NewString()
		page := &model.WikiPage{
			ID:               uuid.MustParse(path[strings.LastIndex(path, "/")+1:]),
			SpaceID:          spaceID,
			ParentID:         cloneUUIDPointer(parentID),
			Title:            trimmedTitle,
			Content:          normalizeWikiContent(content),
			ContentText:      extractPlainText(normalizeWikiContent(content)),
			Path:             path,
			SortOrder:        sortOrder,
			IsTemplate:       false,
			TemplateCategory: "",
			IsSystem:         false,
			IsPinned:         false,
			CreatedBy:        cloneUUIDPointer(createdBy),
			UpdatedBy:        cloneUUIDPointer(createdBy),
			CreatedAt:        now,
			UpdatedAt:        now,
		}
		if err := s.pages.Create(ctx, page); err != nil {
			return nil, fmt.Errorf("create wiki page: %w", err)
		}
		s.broadcast(projectID, ws.EventWikiPageCreated, map[string]any{
			"id":       page.ID.String(),
			"title":    page.Title,
			"parentId": nullableUUIDString(page.ParentID),
			"spaceId":  page.SpaceID.String(),
		})
		return page, nil
	}

	children, err := s.pages.ListByParent(ctx, spaceID, nil)
	if err != nil {
		return nil, fmt.Errorf("list root pages: %w", err)
	}
	sortOrder = len(children)
	page := &model.WikiPage{
		ID:               uuid.New(),
		SpaceID:          spaceID,
		Title:            trimmedTitle,
		Content:          normalizeWikiContent(content),
		ContentText:      extractPlainText(normalizeWikiContent(content)),
		Path:             "",
		SortOrder:        sortOrder,
		IsTemplate:       false,
		TemplateCategory: "",
		IsSystem:         false,
		IsPinned:         false,
		CreatedBy:        cloneUUIDPointer(createdBy),
		UpdatedBy:        cloneUUIDPointer(createdBy),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	page.Path = "/" + page.ID.String()
	if err := s.pages.Create(ctx, page); err != nil {
		return nil, fmt.Errorf("create wiki page: %w", err)
	}
	s.broadcast(projectID, ws.EventWikiPageCreated, map[string]any{
		"id":       page.ID.String(),
		"title":    page.Title,
		"parentId": nullableUUIDString(page.ParentID),
		"spaceId":  page.SpaceID.String(),
	})
	s.forwardIMEvent(ctx, model.NotificationTypeWikiPageUpdated, page.Title, "New wiki page created", map[string]string{
		"href": "/docs/" + page.ID.String(),
	})
	return page, nil
}

func (s *WikiService) UpdatePage(
	ctx context.Context,
	projectID uuid.UUID,
	pageID uuid.UUID,
	title string,
	content string,
	contentText string,
	updatedBy *uuid.UUID,
	expectedUpdatedAt *time.Time,
	templateCategory *string,
) (*model.WikiPage, error) {
	if s.linkSyncer != nil && updatedBy != nil {
		if binder, ok := s.pages.(wikiPageRepositoryTxBinder); ok && binder.DB() != nil {
			if linkBinder, ok := s.linkSyncer.(mentionLinkSyncerTxBinder); ok {
				var page *model.WikiPage
				err := database.WithTx(ctx, binder.DB(), func(tx *gorm.DB) error {
					txPages := repository.NewWikiPageRepository(tx)
					txSyncer := linkBinder.WithDB(tx)
					current, err := txPages.GetByID(ctx, pageID)
					if err != nil {
						return ErrWikiPageNotFound
					}
					if expectedUpdatedAt != nil && current.UpdatedAt.After(*expectedUpdatedAt) {
						return ErrWikiPageConflict
					}
					if current.IsTemplate && current.IsSystem {
						return ErrWikiTemplateImmutable
					}
					current.Title = strings.TrimSpace(title)
					if current.Title == "" {
						current.Title = "Untitled"
					}
					current.Content = normalizeWikiContent(content)
					if strings.TrimSpace(contentText) == "" {
						current.ContentText = extractPlainText(current.Content)
					} else {
						current.ContentText = contentText
					}
					if current.IsTemplate && templateCategory != nil {
						if trimmed := strings.TrimSpace(*templateCategory); trimmed != "" {
							current.TemplateCategory = trimmed
						}
					}
					current.UpdatedBy = cloneUUIDPointer(updatedBy)
					current.UpdatedAt = s.now()
					if err := txPages.Update(ctx, current); err != nil {
						return fmt.Errorf("update wiki page: %w", err)
					}
					if err := txSyncer.SyncMentionLinksForSource(ctx, projectID, model.EntityTypeWikiPage, current.ID, *updatedBy, current.Content); err != nil {
						return fmt.Errorf("sync wiki page mention links: %w", err)
					}
					page = current
					return nil
				})
				if err != nil {
					return nil, err
				}
				s.broadcast(projectID, ws.EventWikiPageUpdated, map[string]any{
					"id":       page.ID.String(),
					"title":    page.Title,
					"parentId": nullableUUIDString(page.ParentID),
					"spaceId":  page.SpaceID.String(),
				})
				s.notifyPageSubscribers(ctx, page.ID, model.NotificationTypeWikiPageUpdated, "Wiki page updated", page.Title+" was updated.", map[string]string{
					"href": "/docs/" + page.ID.String(),
				})
				return page, nil
			}
		}
	}

	page, err := s.getPage(ctx, pageID)
	if err != nil {
		return nil, err
	}
	if expectedUpdatedAt != nil && page.UpdatedAt.After(*expectedUpdatedAt) {
		return nil, ErrWikiPageConflict
	}
	if page.IsTemplate && page.IsSystem {
		return nil, ErrWikiTemplateImmutable
	}

	page.Title = strings.TrimSpace(title)
	if page.Title == "" {
		page.Title = "Untitled"
	}
	page.Content = normalizeWikiContent(content)
	if strings.TrimSpace(contentText) == "" {
		page.ContentText = extractPlainText(page.Content)
	} else {
		page.ContentText = contentText
	}
	if page.IsTemplate && templateCategory != nil {
		if trimmed := strings.TrimSpace(*templateCategory); trimmed != "" {
			page.TemplateCategory = trimmed
		}
	}
	page.UpdatedBy = cloneUUIDPointer(updatedBy)
	page.UpdatedAt = s.now()

	if err := s.pages.Update(ctx, page); err != nil {
		return nil, fmt.Errorf("update wiki page: %w", err)
	}
	if s.linkSyncer != nil && updatedBy != nil {
		if err := s.linkSyncer.SyncMentionLinksForSource(ctx, projectID, model.EntityTypeWikiPage, page.ID, *updatedBy, page.Content); err != nil {
			return nil, fmt.Errorf("sync wiki page mention links: %w", err)
		}
	}
	s.broadcast(projectID, ws.EventWikiPageUpdated, map[string]any{
		"id":       page.ID.String(),
		"title":    page.Title,
		"parentId": nullableUUIDString(page.ParentID),
		"spaceId":  page.SpaceID.String(),
	})
	s.notifyPageSubscribers(ctx, page.ID, model.NotificationTypeWikiPageUpdated, "Wiki page updated", page.Title+" was updated.", map[string]string{
		"href": "/docs/" + page.ID.String(),
	})
	return page, nil
}

func (s *WikiService) DeletePage(ctx context.Context, projectID uuid.UUID, pageID uuid.UUID) error {
	page, err := s.getPage(ctx, pageID)
	if err != nil {
		return err
	}
	if page.IsTemplate && page.IsSystem {
		return ErrWikiTemplateImmutable
	}
	tree, err := s.pages.ListTree(ctx, page.SpaceID)
	if err != nil {
		return fmt.Errorf("list wiki tree for delete: %w", err)
	}
	for _, candidate := range tree {
		if candidate.ID == page.ID || strings.HasPrefix(candidate.Path, page.Path+"/") {
			if err := s.pages.SoftDelete(ctx, candidate.ID); err != nil {
				return fmt.Errorf("soft delete wiki page subtree: %w", err)
			}
		}
	}
	s.broadcast(projectID, ws.EventWikiPageDeleted, map[string]any{
		"id": page.ID.String(),
	})
	return nil
}

func (s *WikiService) MovePage(
	ctx context.Context,
	projectID uuid.UUID,
	pageID uuid.UUID,
	newParentID *uuid.UUID,
	sortOrder int,
) (*model.WikiPage, error) {
	page, err := s.getPage(ctx, pageID)
	if err != nil {
		return nil, err
	}

	oldPath := page.Path
	newPath := "/" + page.ID.String()
	var newParent *model.WikiPage
	if newParentID != nil {
		newParent, err = s.getPage(ctx, *newParentID)
		if err != nil {
			return nil, err
		}
		if newParent.SpaceID != page.SpaceID {
			return nil, ErrWikiPageNotFound
		}
		if newParent.ID == page.ID || strings.HasPrefix(newParent.Path, page.Path+"/") {
			return nil, ErrWikiCircularMove
		}
		newPath = newParent.Path + "/" + page.ID.String()
	}

	if err := s.pages.MovePage(ctx, page.ID, newParentID, newPath, sortOrder); err != nil {
		return nil, fmt.Errorf("move wiki page: %w", err)
	}

	tree, err := s.pages.ListTree(ctx, page.SpaceID)
	if err != nil {
		return nil, fmt.Errorf("list wiki tree for move: %w", err)
	}
	for _, candidate := range tree {
		if candidate.ID == page.ID {
			continue
		}
		if strings.HasPrefix(candidate.Path, oldPath+"/") {
			updatedPath := strings.Replace(candidate.Path, oldPath, newPath, 1)
			if err := s.pages.MovePage(ctx, candidate.ID, candidate.ParentID, updatedPath, candidate.SortOrder); err != nil {
				return nil, fmt.Errorf("move descendant wiki page: %w", err)
			}
		}
	}

	moved, err := s.getPage(ctx, page.ID)
	if err != nil {
		return nil, err
	}
	s.broadcast(projectID, ws.EventWikiPageMoved, map[string]any{
		"id":          moved.ID.String(),
		"oldParentId": nullableUUIDString(page.ParentID),
		"newParentId": nullableUUIDString(newParentID),
		"sortOrder":   moved.SortOrder,
	})
	return moved, nil
}

func (s *WikiService) GetPageTree(ctx context.Context, spaceID uuid.UUID) ([]*model.WikiPage, error) {
	if s.pages == nil {
		return nil, ErrWikiPageNotFound
	}
	return s.pages.ListTree(ctx, spaceID)
}

func (s *WikiService) GetPage(ctx context.Context, pageID uuid.UUID) (*model.WikiPage, error) {
	return s.getPage(ctx, pageID)
}

func (s *WikiService) GetPageContext(ctx context.Context, pageID uuid.UUID) (*model.WikiSpace, *model.WikiPage, error) {
	page, err := s.getPage(ctx, pageID)
	if err != nil {
		return nil, nil, err
	}
	if s.spaces == nil {
		return nil, nil, ErrWikiSpaceNotFound
	}
	space, err := s.spaces.GetByID(ctx, page.SpaceID)
	if err != nil {
		return nil, nil, ErrWikiSpaceNotFound
	}
	return space, page, nil
}

func (s *WikiService) ListVersions(ctx context.Context, pageID uuid.UUID) ([]*model.PageVersion, error) {
	if s.versions == nil {
		return nil, ErrPageVersionNotFound
	}
	return s.versions.ListByPageID(ctx, pageID)
}

func (s *WikiService) GetVersion(ctx context.Context, versionID uuid.UUID) (*model.PageVersion, error) {
	if s.versions == nil {
		return nil, ErrPageVersionNotFound
	}
	version, err := s.versions.GetByID(ctx, versionID)
	if err != nil {
		return nil, ErrPageVersionNotFound
	}
	return version, nil
}

func (s *WikiService) CreateVersion(
	ctx context.Context,
	projectID uuid.UUID,
	pageID uuid.UUID,
	name string,
	createdBy *uuid.UUID,
) (*model.PageVersion, error) {
	page, err := s.getPage(ctx, pageID)
	if err != nil {
		return nil, err
	}
	versions, err := s.versions.ListByPageID(ctx, pageID)
	if err != nil {
		return nil, fmt.Errorf("list page versions: %w", err)
	}
	next := 1
	if len(versions) > 0 {
		next = versions[0].VersionNumber + 1
	}
	version := &model.PageVersion{
		ID:            uuid.New(),
		PageID:        pageID,
		VersionNumber: next,
		Name:          strings.TrimSpace(name),
		Content:       page.Content,
		CreatedBy:     cloneUUIDPointer(createdBy),
		CreatedAt:     s.now(),
	}
	if version.Name == "" {
		version.Name = fmt.Sprintf("v%d", version.VersionNumber)
	}
	if err := s.versions.Create(ctx, version); err != nil {
		return nil, fmt.Errorf("create page version: %w", err)
	}
	s.broadcast(projectID, ws.EventWikiVersionPublished, map[string]any{
		"id":            version.ID.String(),
		"pageId":        pageID.String(),
		"versionNumber": version.VersionNumber,
		"name":          version.Name,
	})
	s.notifyPageSubscribers(ctx, pageID, model.NotificationTypeWikiVersionPublished, "Wiki version published", version.Name, map[string]string{
		"href": "/docs/" + pageID.String() + "?version=" + version.ID.String(),
	})
	return version, nil
}

func (s *WikiService) RestoreVersion(
	ctx context.Context,
	projectID uuid.UUID,
	pageID uuid.UUID,
	versionID uuid.UUID,
	updatedBy *uuid.UUID,
) (*model.WikiPage, *model.PageVersion, error) {
	page, err := s.getPage(ctx, pageID)
	if err != nil {
		return nil, nil, err
	}
	version, err := s.versions.GetByID(ctx, versionID)
	if err != nil {
		return nil, nil, ErrPageVersionNotFound
	}

	page.Content = version.Content
	page.ContentText = extractPlainText(version.Content)
	page.UpdatedBy = cloneUUIDPointer(updatedBy)
	page.UpdatedAt = s.now()
	if err := s.pages.Update(ctx, page); err != nil {
		return nil, nil, fmt.Errorf("restore wiki page: %w", err)
	}

	restoreVersion, err := s.CreateVersion(ctx, projectID, pageID, fmt.Sprintf("Restored from v%d", version.VersionNumber), updatedBy)
	if err != nil {
		return nil, nil, err
	}
	return page, restoreVersion, nil
}

func (s *WikiService) CreateComment(
	ctx context.Context,
	projectID uuid.UUID,
	pageID uuid.UUID,
	body string,
	anchorBlockID *string,
	parentCommentID *uuid.UUID,
	createdBy *uuid.UUID,
	mentions string,
) (*model.PageComment, error) {
	if _, err := s.getPage(ctx, pageID); err != nil {
		return nil, err
	}
	comment := &model.PageComment{
		ID:              uuid.New(),
		PageID:          pageID,
		AnchorBlockID:   cloneStringPointer(anchorBlockID),
		ParentCommentID: cloneUUIDPointer(parentCommentID),
		Body:            strings.TrimSpace(body),
		Mentions:        normalizeWikiMentions(mentions),
		CreatedBy:       cloneUUIDPointer(createdBy),
		CreatedAt:       s.now(),
		UpdatedAt:       s.now(),
	}
	if err := s.comments.Create(ctx, comment); err != nil {
		return nil, fmt.Errorf("create page comment: %w", err)
	}
	s.broadcast(projectID, ws.EventWikiCommentCreated, map[string]any{
		"id":            comment.ID.String(),
		"pageId":        pageID.String(),
		"anchorBlockId": nullableString(comment.AnchorBlockID),
		"parentId":      nullableUUIDString(comment.ParentCommentID),
	})
	s.notifyMentionedUsers(ctx, pageID, comment.ID, comment.Body, mentions)
	return comment, nil
}

func (s *WikiService) ResolveComment(
	ctx context.Context,
	projectID uuid.UUID,
	pageID uuid.UUID,
	commentID uuid.UUID,
) (*model.PageComment, error) {
	comment, err := s.getComment(ctx, pageID, commentID)
	if err != nil {
		return nil, err
	}
	now := s.now()
	comment.ResolvedAt = &now
	comment.UpdatedAt = now
	if err := s.comments.Update(ctx, comment); err != nil {
		return nil, fmt.Errorf("resolve page comment: %w", err)
	}
	s.broadcast(projectID, ws.EventWikiCommentResolved, map[string]any{
		"id":       comment.ID.String(),
		"pageId":   pageID.String(),
		"resolved": true,
	})
	return comment, nil
}

func (s *WikiService) ReopenComment(
	ctx context.Context,
	projectID uuid.UUID,
	pageID uuid.UUID,
	commentID uuid.UUID,
) (*model.PageComment, error) {
	comment, err := s.getComment(ctx, pageID, commentID)
	if err != nil {
		return nil, err
	}
	comment.ResolvedAt = nil
	comment.UpdatedAt = s.now()
	if err := s.comments.Update(ctx, comment); err != nil {
		return nil, fmt.Errorf("reopen page comment: %w", err)
	}
	s.broadcast(projectID, ws.EventWikiCommentResolved, map[string]any{
		"id":       comment.ID.String(),
		"pageId":   pageID.String(),
		"resolved": false,
	})
	return comment, nil
}

func (s *WikiService) DeleteComment(ctx context.Context, projectID uuid.UUID, pageID uuid.UUID, commentID uuid.UUID) error {
	_ = projectID
	if _, err := s.getComment(ctx, pageID, commentID); err != nil {
		return err
	}
	if err := s.comments.SoftDelete(ctx, commentID); err != nil {
		return fmt.Errorf("delete page comment: %w", err)
	}
	return nil
}

func (s *WikiService) ListComments(ctx context.Context, pageID uuid.UUID) ([]*model.PageComment, error) {
	if s.comments == nil {
		return nil, ErrPageCommentNotFound
	}
	return s.comments.ListByPageID(ctx, pageID)
}

func (s *WikiService) SeedBuiltInTemplates(ctx context.Context, projectID uuid.UUID, spaceID uuid.UUID) ([]*model.WikiPage, error) {
	definitions := []struct {
		title    string
		category string
		content  string
	}{
		{title: "PRD", category: "prd", content: `[{"type":"heading","content":"Summary"},{"type":"paragraph","content":"Goals"}]`},
		{title: "RFC", category: "rfc", content: `[{"type":"heading","content":"Proposal"},{"type":"paragraph","content":"Context"}]`},
		{title: "ADR", category: "adr", content: `[{"type":"heading","content":"Status"},{"type":"paragraph","content":"Decision"}]`},
		{title: "Postmortem", category: "postmortem", content: `[{"type":"heading","content":"Impact"},{"type":"paragraph","content":"Root cause"}]`},
		{title: "Onboarding", category: "onboarding", content: `[{"type":"heading","content":"Getting Started"},{"type":"paragraph","content":"Checklist"}]`},
		{title: "Runbook", category: "runbook", content: `[{"type":"heading","content":"Runbook"},{"type":"paragraph","content":"Steps"}]`},
		{title: "Agent Task Brief", category: "agent-task-brief", content: `[{"type":"heading","content":"Context"},{"type":"paragraph","content":"Execution plan"}]`},
	}

	existing, err := s.pages.ListTree(ctx, spaceID)
	if err != nil {
		return nil, fmt.Errorf("list templates for seed: %w", err)
	}
	existingCategories := make(map[string]struct{}, len(existing))
	for _, page := range existing {
		if page.IsTemplate && page.IsSystem {
			existingCategories[page.TemplateCategory] = struct{}{}
		}
	}

	seeded := make([]*model.WikiPage, 0, len(definitions))
	for _, definition := range definitions {
		if _, ok := existingCategories[definition.category]; ok {
			continue
		}
		page := &model.WikiPage{
			ID:               uuid.New(),
			SpaceID:          spaceID,
			Title:            definition.title,
			Content:          definition.content,
			ContentText:      extractPlainText(definition.content),
			Path:             "/templates/" + definition.category + "/" + uuid.NewString(),
			SortOrder:        len(seeded),
			IsTemplate:       true,
			TemplateCategory: definition.category,
			IsSystem:         true,
			IsPinned:         false,
			CreatedAt:        s.now(),
			UpdatedAt:        s.now(),
		}
		if err := s.pages.Create(ctx, page); err != nil {
			return nil, fmt.Errorf("seed wiki template %s: %w", definition.category, err)
		}
		seeded = append(seeded, page)
	}
	return seeded, nil
}

func (s *WikiService) CreateTemplate(
	ctx context.Context,
	projectID uuid.UUID,
	spaceID uuid.UUID,
	title string,
	category string,
	content string,
	createdBy *uuid.UUID,
) (*model.WikiPage, error) {
	_ = projectID
	now := s.now()
	trimmedTitle := strings.TrimSpace(title)
	if trimmedTitle == "" {
		trimmedTitle = "Untitled Template"
	}
	trimmedCategory := strings.TrimSpace(category)
	if trimmedCategory == "" {
		trimmedCategory = "custom"
	}
	normalizedContent := normalizeWikiContent(content)
	template := &model.WikiPage{
		ID:               uuid.New(),
		SpaceID:          spaceID,
		Title:            trimmedTitle,
		Content:          normalizedContent,
		ContentText:      extractPlainText(normalizedContent),
		Path:             "/templates/custom/" + uuid.NewString(),
		SortOrder:        0,
		IsTemplate:       true,
		TemplateCategory: trimmedCategory,
		IsSystem:         false,
		IsPinned:         false,
		CreatedBy:        cloneUUIDPointer(createdBy),
		UpdatedBy:        cloneUUIDPointer(createdBy),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := s.pages.Create(ctx, template); err != nil {
		return nil, fmt.Errorf("create wiki template: %w", err)
	}
	return template, nil
}

func (s *WikiService) CreateTemplateFromPage(
	ctx context.Context,
	projectID uuid.UUID,
	pageID uuid.UUID,
	name string,
	category string,
	createdBy *uuid.UUID,
) (*model.WikiPage, error) {
	page, err := s.getPage(ctx, pageID)
	if err != nil {
		return nil, err
	}
	space, err := s.GetSpaceByProjectID(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if page.SpaceID != space.ID {
		return nil, ErrWikiPageNotFound
	}
	template := &model.WikiPage{
		ID:               uuid.New(),
		SpaceID:          page.SpaceID,
		Title:            strings.TrimSpace(name),
		Content:          page.Content,
		ContentText:      page.ContentText,
		Path:             "/templates/custom/" + uuid.NewString(),
		SortOrder:        0,
		IsTemplate:       true,
		TemplateCategory: strings.TrimSpace(category),
		IsSystem:         false,
		IsPinned:         false,
		CreatedBy:        cloneUUIDPointer(createdBy),
		UpdatedBy:        cloneUUIDPointer(createdBy),
		CreatedAt:        s.now(),
		UpdatedAt:        s.now(),
	}
	if template.Title == "" {
		template.Title = page.Title
	}
	if err := s.pages.Create(ctx, template); err != nil {
		return nil, fmt.Errorf("create wiki template from page: %w", err)
	}
	return template, nil
}

func (s *WikiService) CreatePageFromTemplate(
	ctx context.Context,
	projectID uuid.UUID,
	spaceID uuid.UUID,
	templateID uuid.UUID,
	parentID *uuid.UUID,
	title string,
	createdBy *uuid.UUID,
) (*model.WikiPage, error) {
	template, err := s.getPage(ctx, templateID)
	if err != nil {
		return nil, ErrWikiTemplateNotFound
	}
	if !template.IsTemplate || template.SpaceID != spaceID {
		return nil, ErrWikiTemplateNotFound
	}
	return s.CreatePage(ctx, projectID, spaceID, title, parentID, template.Content, createdBy)
}

func (s *WikiService) ListTemplates(ctx context.Context, spaceID uuid.UUID, query string, category string, source string) ([]*model.WikiPage, error) {
	tree, err := s.pages.ListTree(ctx, spaceID)
	if err != nil {
		return nil, fmt.Errorf("list wiki templates: %w", err)
	}
	normalizedQuery := strings.ToLower(strings.TrimSpace(query))
	normalizedCategory := strings.ToLower(strings.TrimSpace(category))
	normalizedSource := strings.ToLower(strings.TrimSpace(source))
	templates := make([]*model.WikiPage, 0)
	for _, page := range tree {
		if page.IsTemplate {
			if normalizedCategory != "" && strings.ToLower(page.TemplateCategory) != normalizedCategory {
				continue
			}
			templateSource := "custom"
			if page.IsSystem {
				templateSource = "system"
			}
			if normalizedSource != "" && normalizedSource != templateSource {
				continue
			}
			if normalizedQuery != "" {
				haystack := strings.ToLower(strings.Join([]string{
					page.Title,
					page.TemplateCategory,
					page.ContentText,
				}, " "))
				if !strings.Contains(haystack, normalizedQuery) {
					continue
				}
			}
			templates = append(templates, page)
		}
	}
	return templates, nil
}

func (s *WikiService) AddFavorite(ctx context.Context, pageID, userID uuid.UUID) error {
	if s.favorites == nil {
		return nil
	}
	return s.favorites.Add(ctx, pageID, userID)
}

func (s *WikiService) RemoveFavorite(ctx context.Context, pageID, userID uuid.UUID) error {
	if s.favorites == nil {
		return nil
	}
	return s.favorites.Remove(ctx, pageID, userID)
}

func (s *WikiService) ListFavorites(ctx context.Context, userID uuid.UUID) ([]*model.PageFavorite, error) {
	if s.favorites == nil {
		return nil, nil
	}
	return s.favorites.ListByUser(ctx, userID)
}

func (s *WikiService) SetPinned(ctx context.Context, projectID uuid.UUID, pageID uuid.UUID, pinned bool, updatedBy *uuid.UUID) error {
	page, err := s.getPage(ctx, pageID)
	if err != nil {
		return err
	}
	page.IsPinned = pinned
	page.UpdatedBy = cloneUUIDPointer(updatedBy)
	page.UpdatedAt = s.now()
	if err := s.pages.Update(ctx, page); err != nil {
		return fmt.Errorf("set wiki page pinned: %w", err)
	}
	return nil
}

func (s *WikiService) TouchRecentAccess(ctx context.Context, pageID, userID uuid.UUID) error {
	if s.recent == nil {
		return nil
	}
	return s.recent.Touch(ctx, pageID, userID, s.now())
}

func (s *WikiService) ListRecentAccess(ctx context.Context, userID uuid.UUID, limit int) ([]*model.PageRecentAccess, error) {
	if s.recent == nil {
		return nil, nil
	}
	return s.recent.ListByUser(ctx, userID, limit)
}

func (s *WikiService) getPage(ctx context.Context, pageID uuid.UUID) (*model.WikiPage, error) {
	if s.pages == nil {
		return nil, ErrWikiPageNotFound
	}
	page, err := s.pages.GetByID(ctx, pageID)
	if err != nil {
		return nil, ErrWikiPageNotFound
	}
	return page, nil
}

func (s *WikiService) getComment(ctx context.Context, pageID uuid.UUID, commentID uuid.UUID) (*model.PageComment, error) {
	if s.comments == nil {
		return nil, ErrPageCommentNotFound
	}
	comments, err := s.comments.ListByPageID(ctx, pageID)
	if err != nil {
		return nil, fmt.Errorf("list page comments: %w", err)
	}
	for _, comment := range comments {
		if comment.ID == commentID {
			return comment, nil
		}
	}
	return nil, ErrPageCommentNotFound
}

func (s *WikiService) broadcast(projectID uuid.UUID, eventType string, payload any) {
	_ = eventbus.PublishLegacy(context.Background(), s.bus, eventType, projectID.String(), payload)
}

func normalizeWikiContent(content string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return "[]"
	}
	return trimmed
}

func normalizeWikiMentions(mentions string) string {
	trimmed := strings.TrimSpace(mentions)
	if trimmed == "" {
		return "[]"
	}
	return trimmed
}

func extractPlainText(content string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" || trimmed == "[]" {
		return ""
	}
	return trimmed
}

func nullableUUIDString(id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	return id.String()
}

func nullableString(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func cloneUUIDPointer(value *uuid.UUID) *uuid.UUID {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneStringPointer(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func (s *WikiService) notifyPageSubscribers(ctx context.Context, pageID uuid.UUID, notificationType, title, body string, data map[string]string) {
	if s.favorites == nil || s.notifier == nil {
		s.forwardIMEvent(ctx, notificationType, title, body, data)
		return
	}
	favorites, err := s.favorites.ListByPage(ctx, pageID)
	if err != nil {
		return
	}
	payload, _ := json.Marshal(data)
	for _, favorite := range favorites {
		if favorite == nil {
			continue
		}
		_, _ = s.notifier.Create(ctx, favorite.UserID, notificationType, title, body, string(payload))
	}
	s.forwardIMEvent(ctx, notificationType, title, body, data)
}

func (s *WikiService) notifyMentionedUsers(ctx context.Context, pageID uuid.UUID, commentID uuid.UUID, body string, mentions string) {
	if s.notifier == nil {
		s.forwardIMEvent(ctx, model.NotificationTypeWikiCommentMention, "Wiki comment mention", body, map[string]string{
			"href": "/docs/" + pageID.String() + "#comment-" + commentID.String(),
		})
		return
	}
	mentionedUserIDs := parseMentionedUserIDs(mentions)
	payload, _ := json.Marshal(map[string]string{
		"href": "/docs/" + pageID.String() + "#comment-" + commentID.String(),
	})
	for _, userID := range mentionedUserIDs {
		_, _ = s.notifier.Create(ctx, userID, model.NotificationTypeWikiCommentMention, "Wiki comment mention", body, string(payload))
	}
	s.forwardIMEvent(ctx, model.NotificationTypeWikiCommentMention, "Wiki comment mention", body, map[string]string{
		"href": "/docs/" + pageID.String() + "#comment-" + commentID.String(),
	})
}

func (s *WikiService) forwardIMEvent(ctx context.Context, eventType, title, body string, data map[string]string) {
	if s.imNotifier == nil {
		return
	}
	if s.imChannels != nil {
		channels, err := s.imChannels.ResolveChannelsForEvent(ctx, eventType, "", "")
		if err == nil && len(channels) > 0 {
			for _, channel := range channels {
				if channel == nil {
					continue
				}
				_ = s.imNotifier.Notify(ctx, &model.IMNotifyRequest{
					Platform:  strings.TrimSpace(channel.Platform),
					ChannelID: strings.TrimSpace(channel.ChannelID),
					Event:     eventType,
					Title:     title,
					Body:      body,
					Data:      data,
				})
			}
			return
		}
	}
	if s.imPlatform == "" || s.imChannelID == "" {
		return
	}
	_ = s.imNotifier.Notify(ctx, &model.IMNotifyRequest{
		Platform:  s.imPlatform,
		ChannelID: s.imChannelID,
		Event:     eventType,
		Title:     title,
		Body:      body,
		Data:      data,
	})
}

func parseMentionedUserIDs(raw string) []uuid.UUID {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var direct []string
	if err := json.Unmarshal([]byte(raw), &direct); err == nil {
		return parseUUIDStrings(direct)
	}
	var objects []map[string]any
	if err := json.Unmarshal([]byte(raw), &objects); err == nil {
		values := make([]string, 0, len(objects))
		for _, item := range objects {
			if id, ok := item["id"].(string); ok {
				values = append(values, id)
			}
		}
		return parseUUIDStrings(values)
	}
	return nil
}

func parseUUIDStrings(values []string) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(values))
	for _, value := range values {
		id, err := uuid.Parse(strings.TrimSpace(value))
		if err == nil {
			ids = append(ids, id)
		}
	}
	return ids
}
