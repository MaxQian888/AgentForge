package knowledge

import (
	"context"
	"fmt"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
)

// KnowledgeAssetService implements the business logic for the knowledge domain.
type KnowledgeAssetService struct {
	assets   KnowledgeAssetRepository
	versions AssetVersionRepository
	comments AssetCommentRepository
	chunks   AssetIngestChunkRepository
	search   SearchProvider
	index    IndexPipeline
	bus      eventPublisher
}

// eventPublisher is a minimal interface so the service can emit events
// without coupling to the eventbus package directly. Wire this via
// knowledgeEventBusAdapter in routes.go.
type eventPublisher interface {
	PublishKnowledgeEvent(ctx context.Context, eventType string, projectID string, payload map[string]any) error
}

func NewKnowledgeAssetService(
	assets KnowledgeAssetRepository,
	versions AssetVersionRepository,
	comments AssetCommentRepository,
	chunks AssetIngestChunkRepository,
	search SearchProvider,
	index IndexPipeline,
	bus eventPublisher,
) *KnowledgeAssetService {
	if index == nil {
		index = NoopIndexPipeline{}
	}
	return &KnowledgeAssetService{
		assets:   assets,
		versions: versions,
		comments: comments,
		chunks:   chunks,
		search:   search,
		index:    index,
		bus:      bus,
	}
}

// --- CRUD ---

func (s *KnowledgeAssetService) Create(ctx context.Context, pc model.PrincipalContext, a *model.KnowledgeAsset) (*model.KnowledgeAsset, error) {
	if !pc.CanWrite() {
		return nil, ErrAssetForbidden
	}
	if err := ValidateKnowledgeAsset(a); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	a.ID = uuid.New()
	a.CreatedBy = &pc.UserID
	a.UpdatedBy = &pc.UserID
	a.CreatedAt = now
	a.UpdatedAt = now
	a.Version = 1
	if err := s.assets.Create(ctx, a); err != nil {
		return nil, fmt.Errorf("knowledge_asset service create: %w", err)
	}
	s.publishEvent(ctx, "knowledge.asset.created", a.ProjectID.String(), map[string]any{
		"assetId": a.ID.String(),
		"kind":    string(a.Kind),
	})
	return a, nil
}

func (s *KnowledgeAssetService) Get(ctx context.Context, pc model.PrincipalContext, id uuid.UUID) (*model.KnowledgeAsset, error) {
	if !pc.CanRead() {
		return nil, ErrAssetForbidden
	}
	a, err := s.assets.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (s *KnowledgeAssetService) Update(ctx context.Context, pc model.PrincipalContext, id uuid.UUID, req model.UpdateKnowledgeAssetRequest) (*model.KnowledgeAsset, error) {
	if !pc.CanWrite() {
		return nil, ErrAssetForbidden
	}
	a, err := s.assets.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	// Optimistic lock check.
	if req.ExpectedVersion != nil && *req.ExpectedVersion != a.Version {
		return nil, ErrAssetConflict
	}
	a.Title = req.Title
	if req.ContentJSON != "" {
		a.ContentJSON = req.ContentJSON
	}
	if req.ContentText != "" {
		a.ContentText = req.ContentText
	}
	if req.TemplateCategory != nil {
		a.TemplateCategory = *req.TemplateCategory
	}
	a.UpdatedBy = &pc.UserID
	a.UpdatedAt = time.Now().UTC()
	a.Version++

	if err := ValidateKnowledgeAsset(a); err != nil {
		return nil, err
	}
	if err := s.assets.Update(ctx, a); err != nil {
		return nil, fmt.Errorf("knowledge_asset service update: %w", err)
	}
	_ = s.index.EnqueueContentChanged(ctx, a.ID, string(a.Kind), a.ProjectID, a.Version)
	s.publishEvent(ctx, "knowledge.asset.updated", a.ProjectID.String(), map[string]any{
		"assetId": a.ID.String(),
		"kind":    string(a.Kind),
	})
	return a, nil
}

func (s *KnowledgeAssetService) Delete(ctx context.Context, pc model.PrincipalContext, id uuid.UUID) error {
	if !pc.CanWrite() {
		return ErrAssetForbidden
	}
	a, err := s.assets.GetByID(ctx, id)
	if err != nil {
		return err
	}
	// Cascade soft-delete descendants for wiki_page.
	if a.Kind == model.KindWikiPage {
		desc, err := s.assets.Descendants(ctx, id)
		if err != nil {
			return fmt.Errorf("knowledge_asset service descendants: %w", err)
		}
		for _, descID := range desc {
			_ = s.assets.SoftDelete(ctx, descID)
		}
	}
	if err := s.assets.SoftDelete(ctx, id); err != nil {
		return err
	}
	s.publishEvent(ctx, "knowledge.asset.deleted", a.ProjectID.String(), map[string]any{
		"assetId": id.String(),
		"kind":    string(a.Kind),
	})
	return nil
}

func (s *KnowledgeAssetService) Restore(ctx context.Context, pc model.PrincipalContext, id uuid.UUID) (*model.KnowledgeAsset, error) {
	if !pc.CanAdmin() {
		return nil, ErrAssetForbidden
	}
	if err := s.assets.Restore(ctx, id); err != nil {
		return nil, err
	}
	return s.assets.GetByID(ctx, id)
}

func (s *KnowledgeAssetService) List(ctx context.Context, pc model.PrincipalContext, projectID uuid.UUID, kind *model.KnowledgeAssetKind) ([]*model.KnowledgeAsset, error) {
	if !pc.CanRead() {
		return nil, ErrAssetForbidden
	}
	return s.assets.ListByProject(ctx, projectID, kind)
}

func (s *KnowledgeAssetService) ListTree(ctx context.Context, pc model.PrincipalContext, spaceID uuid.UUID) ([]*model.KnowledgeAsset, error) {
	if !pc.CanRead() {
		return nil, ErrAssetForbidden
	}
	return s.assets.ListTree(ctx, spaceID)
}

func (s *KnowledgeAssetService) Move(ctx context.Context, pc model.PrincipalContext, id uuid.UUID, req model.MoveKnowledgeAssetRequest) (*model.KnowledgeAsset, error) {
	if !pc.CanAdmin() {
		return nil, ErrAssetForbidden
	}
	a, err := s.assets.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if a.Kind != model.KindWikiPage {
		return nil, fmt.Errorf("%w: only wiki_page assets can be moved in tree", ErrUnsupportedKind)
	}
	parentID, err := parseOptionalUUIDStr(req.ParentID)
	if err != nil {
		return nil, fmt.Errorf("invalid parentId: %w", err)
	}
	// Circular-move guard.
	if parentID != nil {
		if *parentID == id {
			return nil, ErrCircularMove
		}
		desc, err := s.assets.Descendants(ctx, id)
		if err != nil {
			return nil, err
		}
		for _, d := range desc {
			if d == *parentID {
				return nil, ErrCircularMove
			}
		}
	}
	newPath := buildPath(parentID, a.Title)
	if err := s.assets.Move(ctx, id, parentID, newPath, req.SortOrder); err != nil {
		return nil, err
	}
	a.ParentID = parentID
	a.Path = newPath
	a.SortOrder = req.SortOrder
	s.publishEvent(ctx, "knowledge.asset.moved", a.ProjectID.String(), map[string]any{
		"assetId": id.String(),
	})
	return a, nil
}

// --- Versions ---

func (s *KnowledgeAssetService) ListVersions(ctx context.Context, pc model.PrincipalContext, assetID uuid.UUID) ([]*model.AssetVersion, error) {
	if !pc.CanRead() {
		return nil, ErrAssetForbidden
	}
	return s.versions.ListByAssetID(ctx, assetID)
}

func (s *KnowledgeAssetService) CreateVersion(ctx context.Context, pc model.PrincipalContext, assetID uuid.UUID, name string) (*model.AssetVersion, error) {
	if !pc.CanWrite() {
		return nil, ErrAssetForbidden
	}
	a, err := s.assets.GetByID(ctx, assetID)
	if err != nil {
		return nil, err
	}
	maxVer, err := s.versions.MaxVersionNumber(ctx, assetID)
	if err != nil {
		return nil, err
	}
	v := &model.AssetVersion{
		ID:            uuid.New(),
		AssetID:       assetID,
		VersionNumber: maxVer + 1,
		Name:          name,
		KindSnapshot:  string(a.Kind),
		ContentJSON:   a.ContentJSON,
		FileRef:       a.FileRef,
		CreatedBy:     &pc.UserID,
		CreatedAt:     time.Now().UTC(),
	}
	if err := s.versions.Create(ctx, v); err != nil {
		return nil, fmt.Errorf("create asset version: %w", err)
	}
	return v, nil
}

func (s *KnowledgeAssetService) GetVersion(ctx context.Context, pc model.PrincipalContext, versionID uuid.UUID) (*model.AssetVersion, error) {
	if !pc.CanRead() {
		return nil, ErrAssetForbidden
	}
	return s.versions.GetByID(ctx, versionID)
}

func (s *KnowledgeAssetService) RestoreVersion(ctx context.Context, pc model.PrincipalContext, assetID, versionID uuid.UUID) (*model.KnowledgeAsset, *model.AssetVersion, error) {
	if !pc.CanWrite() {
		return nil, nil, ErrAssetForbidden
	}
	a, err := s.assets.GetByID(ctx, assetID)
	if err != nil {
		return nil, nil, err
	}
	ver, err := s.versions.GetByID(ctx, versionID)
	if err != nil {
		return nil, nil, ErrVersionNotFound
	}
	a.ContentJSON = ver.ContentJSON
	a.UpdatedBy = &pc.UserID
	a.UpdatedAt = time.Now().UTC()
	a.Version++
	if err := s.assets.Update(ctx, a); err != nil {
		return nil, nil, err
	}
	return a, ver, nil
}

// --- Comments ---

func (s *KnowledgeAssetService) ListComments(ctx context.Context, pc model.PrincipalContext, assetID uuid.UUID) ([]*model.AssetComment, error) {
	if !pc.CanRead() {
		return nil, ErrAssetForbidden
	}
	return s.comments.ListByAssetID(ctx, assetID)
}

func (s *KnowledgeAssetService) CreateComment(ctx context.Context, pc model.PrincipalContext, assetID uuid.UUID, req model.CreateAssetCommentRequest) (*model.AssetComment, error) {
	if !pc.CanWrite() {
		return nil, ErrAssetForbidden
	}
	parentID, err := parseOptionalUUIDStr(req.ParentCommentID)
	if err != nil {
		return nil, fmt.Errorf("invalid parentCommentId: %w", err)
	}
	now := time.Now().UTC()
	c := &model.AssetComment{
		ID:              uuid.New(),
		AssetID:         assetID,
		AnchorBlockID:   req.AnchorBlockID,
		ParentCommentID: parentID,
		Body:            req.Body,
		Mentions:        req.Mentions,
		CreatedBy:       &pc.UserID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := s.comments.Create(ctx, c); err != nil {
		return nil, fmt.Errorf("create asset comment: %w", err)
	}
	a, _ := s.assets.GetByID(ctx, assetID)
	if a != nil {
		s.publishEvent(ctx, "knowledge.comment.created", a.ProjectID.String(), map[string]any{
			"assetId":   assetID.String(),
			"commentId": c.ID.String(),
		})
	}
	return c, nil
}

func (s *KnowledgeAssetService) UpdateComment(ctx context.Context, pc model.PrincipalContext, assetID, commentID uuid.UUID, req model.UpdateAssetCommentRequest) (*model.AssetComment, error) {
	if !pc.CanWrite() {
		return nil, ErrAssetForbidden
	}
	c, err := s.comments.GetByID(ctx, commentID)
	if err != nil {
		return nil, ErrCommentNotFound
	}
	if req.Body != nil {
		c.Body = *req.Body
	}
	if req.Resolved != nil {
		if *req.Resolved {
			now := time.Now().UTC()
			c.ResolvedAt = &now
		} else {
			c.ResolvedAt = nil
		}
	}
	c.UpdatedAt = time.Now().UTC()
	if err := s.comments.Update(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *KnowledgeAssetService) DeleteComment(ctx context.Context, pc model.PrincipalContext, assetID, commentID uuid.UUID) error {
	if !pc.CanWrite() {
		return ErrAssetForbidden
	}
	return s.comments.SoftDelete(ctx, commentID)
}

// --- Search ---

func (s *KnowledgeAssetService) Search(ctx context.Context, pc model.PrincipalContext, projectID uuid.UUID, query string, kind *model.KnowledgeAssetKind, limit int) ([]*model.KnowledgeSearchResult, error) {
	if !pc.CanRead() {
		return nil, ErrAssetForbidden
	}
	if s.search == nil {
		return []*model.KnowledgeSearchResult{}, nil
	}
	return s.search.Search(ctx, projectID, query, kind, limit)
}

// --- helpers ---

func (s *KnowledgeAssetService) publishEvent(ctx context.Context, eventType, projectID string, payload map[string]any) {
	if s.bus == nil {
		return
	}
	_ = s.bus.PublishKnowledgeEvent(ctx, eventType, projectID, payload)
}

func buildPath(parentID *uuid.UUID, title string) string {
	if parentID == nil {
		return "/" + title
	}
	return "/" + parentID.String() + "/" + title
}

func parseOptionalUUIDStr(s *string) (*uuid.UUID, error) {
	if s == nil || *s == "" {
		return nil, nil
	}
	id, err := uuid.Parse(*s)
	if err != nil {
		return nil, err
	}
	return &id, nil
}
