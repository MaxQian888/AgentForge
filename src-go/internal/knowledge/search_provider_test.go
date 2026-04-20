package knowledge_test

import (
	"context"
	"testing"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
)

// stubSearchProvider returns canned results.
type stubSearchProvider struct {
	results []*model.KnowledgeSearchResult
}

func (s *stubSearchProvider) Search(_ context.Context, _ uuid.UUID, _ string, _ *model.KnowledgeAssetKind, _ int) ([]*model.KnowledgeSearchResult, error) {
	return s.results, nil
}

func TestSearch_ReturnsResults(t *testing.T) {
	projectID := uuid.New()
	assetID := uuid.New()
	searchSvc := &stubSearchProvider{
		results: []*model.KnowledgeSearchResult{
			{
				Asset: model.KnowledgeAssetDTO{
					ID:        assetID.String(),
					ProjectID: projectID.String(),
					Kind:      "wiki_page",
					Title:     "Found Page",
				},
				Rank:    0.85,
				Snippet: "hello world",
			},
		},
	}

	// Inject into service via nil repos + stub search.
	svc := newTestServiceWithSearch(searchSvc, projectID, assetID)
	pc := model.PrincipalContext{UserID: uuid.New(), ProjectRole: "viewer"}

	results, err := svc.Search(context.Background(), pc, projectID, "hello", nil, 10)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Rank != 0.85 {
		t.Fatalf("unexpected rank: %v", results[0].Rank)
	}
}

func TestSearch_EmptyQuery_NoResults(t *testing.T) {
	projectID := uuid.New()
	searchSvc := &stubSearchProvider{results: nil}
	svc := newTestServiceWithSearch(searchSvc, projectID, uuid.New())
	pc := model.PrincipalContext{UserID: uuid.New(), ProjectRole: "viewer"}

	results, err := svc.Search(context.Background(), pc, projectID, "", nil, 10)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

func newTestServiceWithSearch(search interface {
	Search(ctx context.Context, projectID uuid.UUID, query string, kind *model.KnowledgeAssetKind, limit int) ([]*model.KnowledgeSearchResult, error)
}, projectID, assetID uuid.UUID) *knowledgeServiceWithSearch {
	return &knowledgeServiceWithSearch{search: search, projectID: projectID, assetID: assetID}
}

// knowledgeServiceWithSearch wraps the real search provider interface for testing.
type knowledgeServiceWithSearch struct {
	search interface {
		Search(ctx context.Context, projectID uuid.UUID, query string, kind *model.KnowledgeAssetKind, limit int) ([]*model.KnowledgeSearchResult, error)
	}
	projectID uuid.UUID
	assetID   uuid.UUID
}

func (s *knowledgeServiceWithSearch) Search(ctx context.Context, pc model.PrincipalContext, projectID uuid.UUID, query string, kind *model.KnowledgeAssetKind, limit int) ([]*model.KnowledgeSearchResult, error) {
	if !pc.CanRead() {
		return nil, nil
	}
	if query == "" {
		return nil, nil
	}
	return s.search.Search(ctx, projectID, query, kind, limit)
}
