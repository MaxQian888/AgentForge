package knowledge_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/knowledge"
	"github.com/react-go-quick-starter/server/internal/model"
)

// --- stub implementations ---

type stubAssetRepo struct {
	store map[uuid.UUID]*model.KnowledgeAsset
}

func newStubAssetRepo(assets ...*model.KnowledgeAsset) *stubAssetRepo {
	r := &stubAssetRepo{store: make(map[uuid.UUID]*model.KnowledgeAsset)}
	for _, a := range assets {
		r.store[a.ID] = a
	}
	return r
}

func (s *stubAssetRepo) Create(_ context.Context, a *model.KnowledgeAsset) error {
	s.store[a.ID] = a
	return nil
}
func (s *stubAssetRepo) GetByID(_ context.Context, id uuid.UUID) (*model.KnowledgeAsset, error) {
	a, ok := s.store[id]
	if !ok {
		return nil, knowledge.ErrAssetNotFound
	}
	return a, nil
}
func (s *stubAssetRepo) Update(_ context.Context, a *model.KnowledgeAsset) error {
	if _, ok := s.store[a.ID]; !ok {
		return knowledge.ErrAssetNotFound
	}
	s.store[a.ID] = a
	return nil
}
func (s *stubAssetRepo) SoftDelete(_ context.Context, id uuid.UUID) error {
	if _, ok := s.store[id]; !ok {
		return knowledge.ErrAssetNotFound
	}
	delete(s.store, id)
	return nil
}
func (s *stubAssetRepo) Restore(_ context.Context, id uuid.UUID) error {
	return nil
}
func (s *stubAssetRepo) ListByProject(_ context.Context, projectID uuid.UUID, kind *model.KnowledgeAssetKind) ([]*model.KnowledgeAsset, error) {
	var out []*model.KnowledgeAsset
	for _, a := range s.store {
		if a.ProjectID != projectID {
			continue
		}
		if kind != nil && a.Kind != *kind {
			continue
		}
		out = append(out, a)
	}
	return out, nil
}
func (s *stubAssetRepo) ListTree(_ context.Context, spaceID uuid.UUID) ([]*model.KnowledgeAsset, error) {
	return nil, nil
}
func (s *stubAssetRepo) ListByParent(_ context.Context, spaceID uuid.UUID, parentID *uuid.UUID) ([]*model.KnowledgeAsset, error) {
	return nil, nil
}
func (s *stubAssetRepo) Move(_ context.Context, id uuid.UUID, parentID *uuid.UUID, path string, sortOrder int) error {
	return nil
}
func (s *stubAssetRepo) UpdateIngestStatus(_ context.Context, id uuid.UUID, status model.IngestStatus, chunkCount int) error {
	return nil
}
func (s *stubAssetRepo) Descendants(_ context.Context, id uuid.UUID) ([]uuid.UUID, error) {
	return nil, nil
}

type stubVersionRepo struct{}

func (s stubVersionRepo) Create(_ context.Context, v *model.AssetVersion) error  { return nil }
func (s stubVersionRepo) ListByAssetID(_ context.Context, id uuid.UUID) ([]*model.AssetVersion, error) {
	return nil, nil
}
func (s stubVersionRepo) GetByID(_ context.Context, id uuid.UUID) (*model.AssetVersion, error) {
	return nil, knowledge.ErrVersionNotFound
}
func (s stubVersionRepo) MaxVersionNumber(_ context.Context, id uuid.UUID) (int, error) { return 0, nil }

type stubCommentRepo struct{}

func (s stubCommentRepo) Create(_ context.Context, c *model.AssetComment) error { return nil }
func (s stubCommentRepo) ListByAssetID(_ context.Context, id uuid.UUID) ([]*model.AssetComment, error) {
	return nil, nil
}
func (s stubCommentRepo) GetByID(_ context.Context, id uuid.UUID) (*model.AssetComment, error) {
	return nil, knowledge.ErrCommentNotFound
}
func (s stubCommentRepo) Update(_ context.Context, c *model.AssetComment) error { return nil }
func (s stubCommentRepo) SoftDelete(_ context.Context, id uuid.UUID) error      { return nil }

type stubChunkRepo struct{}

func (s stubChunkRepo) BulkCreate(_ context.Context, chunks []*model.AssetIngestChunk) error {
	return nil
}
func (s stubChunkRepo) ListByAssetID(_ context.Context, id uuid.UUID) ([]*model.AssetIngestChunk, error) {
	return nil, nil
}
func (s stubChunkRepo) DeleteByAssetID(_ context.Context, id uuid.UUID) error { return nil }

func newTestService(assetRepo knowledge.KnowledgeAssetRepository) *knowledge.KnowledgeAssetService {
	return knowledge.NewKnowledgeAssetService(
		assetRepo,
		stubVersionRepo{},
		stubCommentRepo{},
		stubChunkRepo{},
		nil,
		nil,
		nil,
	)
}

// --- Tests ---

func makeWikiAsset(projectID uuid.UUID) *model.KnowledgeAsset {
	spaceID := uuid.New()
	return &model.KnowledgeAsset{
		ID:          uuid.New(),
		ProjectID:   projectID,
		Kind:        model.KindWikiPage,
		WikiSpaceID: &spaceID,
		ContentJSON: `[{"type":"paragraph"}]`,
		Title:       "Test Page",
		Version:     1,
	}
}

func TestRBAC_Viewer_CanRead(t *testing.T) {
	projectID := uuid.New()
	asset := makeWikiAsset(projectID)
	repo := newStubAssetRepo(asset)
	svc := newTestService(repo)

	pc := model.PrincipalContext{UserID: uuid.New(), ProjectRole: "viewer"}
	a, err := svc.Get(context.Background(), pc, asset.ID)
	if err != nil {
		t.Fatalf("viewer should be able to read: %v", err)
	}
	if a.ID != asset.ID {
		t.Fatal("unexpected asset returned")
	}
}

func TestRBAC_Viewer_CannotCreate(t *testing.T) {
	projectID := uuid.New()
	repo := newStubAssetRepo()
	svc := newTestService(repo)

	pc := model.PrincipalContext{UserID: uuid.New(), ProjectRole: "viewer"}
	spaceID := uuid.New()
	_, err := svc.Create(context.Background(), pc, &model.KnowledgeAsset{
		ProjectID:   projectID,
		Kind:        model.KindWikiPage,
		WikiSpaceID: &spaceID,
		ContentJSON: `[]`,
		Title:       "New Page",
	})
	if !errors.Is(err, knowledge.ErrAssetForbidden) {
		t.Fatalf("expected ErrAssetForbidden for viewer create, got: %v", err)
	}
}

func TestRBAC_Editor_CanCreate(t *testing.T) {
	projectID := uuid.New()
	repo := newStubAssetRepo()
	svc := newTestService(repo)

	pc := model.PrincipalContext{UserID: uuid.New(), ProjectRole: "editor"}
	spaceID := uuid.New()
	a, err := svc.Create(context.Background(), pc, &model.KnowledgeAsset{
		ProjectID:   projectID,
		Kind:        model.KindWikiPage,
		WikiSpaceID: &spaceID,
		ContentJSON: `[{"type":"paragraph"}]`,
		Title:       "New Page",
	})
	if err != nil {
		t.Fatalf("editor should be able to create: %v", err)
	}
	if a.ID == uuid.Nil {
		t.Fatal("expected non-nil ID")
	}
}

func TestRBAC_Editor_CannotRestore(t *testing.T) {
	projectID := uuid.New()
	asset := makeWikiAsset(projectID)
	repo := newStubAssetRepo(asset)
	svc := newTestService(repo)

	pc := model.PrincipalContext{UserID: uuid.New(), ProjectRole: "editor"}
	_, err := svc.Restore(context.Background(), pc, asset.ID)
	if !errors.Is(err, knowledge.ErrAssetForbidden) {
		t.Fatalf("expected ErrAssetForbidden for editor restore, got: %v", err)
	}
}

func TestRBAC_Admin_CanRestore(t *testing.T) {
	projectID := uuid.New()
	asset := makeWikiAsset(projectID)
	repo := newStubAssetRepo(asset)
	svc := newTestService(repo)

	pc := model.PrincipalContext{UserID: uuid.New(), ProjectRole: "admin"}
	// Restore calls repo.Restore which is a no-op stub, then GetByID.
	_, err := svc.Restore(context.Background(), pc, asset.ID)
	if err != nil {
		t.Fatalf("admin should be able to restore: %v", err)
	}
}

func TestRBAC_NoRole_CannotRead(t *testing.T) {
	projectID := uuid.New()
	asset := makeWikiAsset(projectID)
	repo := newStubAssetRepo(asset)
	svc := newTestService(repo)

	pc := model.PrincipalContext{UserID: uuid.New(), ProjectRole: ""}
	_, err := svc.Get(context.Background(), pc, asset.ID)
	if !errors.Is(err, knowledge.ErrAssetForbidden) {
		t.Fatalf("expected ErrAssetForbidden for no-role read, got: %v", err)
	}
}
