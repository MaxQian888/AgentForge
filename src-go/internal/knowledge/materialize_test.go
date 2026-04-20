package knowledge_test

import (
	"context"
	"errors"
	"testing"

	"github.com/agentforge/server/internal/knowledge"
	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
)

type captureChunkRepoMat struct {
	chunks []*model.AssetIngestChunk
}

func (r *captureChunkRepoMat) BulkCreate(_ context.Context, chunks []*model.AssetIngestChunk) error {
	r.chunks = append(r.chunks, chunks...)
	return nil
}
func (r *captureChunkRepoMat) ListByAssetID(_ context.Context, id uuid.UUID) ([]*model.AssetIngestChunk, error) {
	var out []*model.AssetIngestChunk
	for _, c := range r.chunks {
		if c.AssetID == id {
			out = append(out, c)
		}
	}
	return out, nil
}
func (r *captureChunkRepoMat) DeleteByAssetID(_ context.Context, id uuid.UUID) error { return nil }

func TestMaterializeAsWiki_Success(t *testing.T) {
	projectID := uuid.New()
	assetID := uuid.New()
	status := model.IngestStatusReady
	ingestFile := &model.KnowledgeAsset{
		ID:           assetID,
		ProjectID:    projectID,
		Kind:         model.KindIngestedFile,
		FileRef:      "project/file.pdf",
		Title:        "Report",
		IngestStatus: &status,
		Version:      1,
	}

	assetRepo := newStubAssetRepo(ingestFile)
	chunkRepo := &captureChunkRepoMat{}

	// Pre-populate chunks.
	chunkRepo.chunks = []*model.AssetIngestChunk{
		{ID: uuid.New(), AssetID: assetID, ChunkIndex: 0, Content: "Chunk one."},
		{ID: uuid.New(), AssetID: assetID, ChunkIndex: 1, Content: "Chunk two."},
	}

	svc := knowledge.NewKnowledgeAssetService(assetRepo, stubVersionRepo{}, stubCommentRepo{}, chunkRepo, nil, nil, nil)
	pc := model.PrincipalContext{UserID: uuid.New(), ProjectRole: "editor"}

	spaceID := uuid.New()
	a, err := svc.MaterializeAsWiki(context.Background(), pc, assetID, model.MaterializeAsWikiRequest{
		WikiSpaceID: spaceID.String(),
		Title:       "Materialized Wiki",
	})
	if err != nil {
		t.Fatalf("materialize failed: %v", err)
	}
	if a.Kind != model.KindWikiPage {
		t.Fatalf("expected wiki_page kind, got: %v", a.Kind)
	}
	if a.Title != "Materialized Wiki" {
		t.Fatalf("expected custom title, got: %v", a.Title)
	}
	if a.WikiSpaceID == nil || *a.WikiSpaceID != spaceID {
		t.Fatal("expected spaceID to be set")
	}
}

func TestMaterializeAsWiki_NotIngestedFile(t *testing.T) {
	projectID := uuid.New()
	spaceID := uuid.New()
	wikiAsset := &model.KnowledgeAsset{
		ID:          uuid.New(),
		ProjectID:   projectID,
		Kind:        model.KindWikiPage,
		WikiSpaceID: &spaceID,
		ContentJSON: `[]`,
		Title:       "Wiki Page",
		Version:     1,
	}

	assetRepo := newStubAssetRepo(wikiAsset)
	svc := knowledge.NewKnowledgeAssetService(assetRepo, stubVersionRepo{}, stubCommentRepo{}, stubChunkRepo{}, nil, nil, nil)
	pc := model.PrincipalContext{UserID: uuid.New(), ProjectRole: "editor"}

	_, err := svc.MaterializeAsWiki(context.Background(), pc, wikiAsset.ID, model.MaterializeAsWikiRequest{
		WikiSpaceID: spaceID.String(),
	})
	if !errors.Is(err, knowledge.ErrUnsupportedKind) {
		t.Fatalf("expected ErrUnsupportedKind, got: %v", err)
	}
}

func TestMaterializeAsWiki_NotReady(t *testing.T) {
	projectID := uuid.New()
	assetID := uuid.New()
	status := model.IngestStatusProcessing
	ingestFile := &model.KnowledgeAsset{
		ID:           assetID,
		ProjectID:    projectID,
		Kind:         model.KindIngestedFile,
		FileRef:      "project/file.pdf",
		Title:        "Report",
		IngestStatus: &status,
		Version:      1,
	}

	assetRepo := newStubAssetRepo(ingestFile)
	svc := knowledge.NewKnowledgeAssetService(assetRepo, stubVersionRepo{}, stubCommentRepo{}, stubChunkRepo{}, nil, nil, nil)
	pc := model.PrincipalContext{UserID: uuid.New(), ProjectRole: "editor"}

	_, err := svc.MaterializeAsWiki(context.Background(), pc, assetID, model.MaterializeAsWikiRequest{
		WikiSpaceID: uuid.New().String(),
	})
	if !errors.Is(err, knowledge.ErrIngestNotReady) {
		t.Fatalf("expected ErrIngestNotReady, got: %v", err)
	}
}
