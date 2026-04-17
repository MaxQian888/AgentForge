package knowledge_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/knowledge"
	"github.com/react-go-quick-starter/server/internal/model"
)

// treeStubAssetRepo extends stubAssetRepo with richer Descendants support.
type treeStubAssetRepo struct {
	stubAssetRepo
}

func newTreeStubRepo(assets ...*model.KnowledgeAsset) *treeStubAssetRepo {
	r := &treeStubAssetRepo{stubAssetRepo{store: make(map[uuid.UUID]*model.KnowledgeAsset)}}
	for _, a := range assets {
		r.store[a.ID] = a
	}
	return r
}

func (s *treeStubAssetRepo) Descendants(_ context.Context, id uuid.UUID) ([]uuid.UUID, error) {
	var result []uuid.UUID
	queue := []uuid.UUID{id}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, a := range s.store {
			if a.ParentID != nil && *a.ParentID == current {
				result = append(result, a.ID)
				queue = append(queue, a.ID)
			}
		}
	}
	return result, nil
}

func TestTree_CircularMoveDetected(t *testing.T) {
	projectID := uuid.New()
	spaceID := uuid.New()

	parent := &model.KnowledgeAsset{
		ID: uuid.New(), ProjectID: projectID, Kind: model.KindWikiPage,
		WikiSpaceID: &spaceID, ContentJSON: `[]`, Title: "Parent", Version: 1,
	}
	child := &model.KnowledgeAsset{
		ID: uuid.New(), ProjectID: projectID, Kind: model.KindWikiPage,
		WikiSpaceID: &spaceID, ContentJSON: `[]`, Title: "Child", Version: 1,
		ParentID: &parent.ID,
	}

	repo := newTreeStubRepo(parent, child)
	svc := knowledge.NewKnowledgeAssetService(repo, stubVersionRepo{}, stubCommentRepo{}, stubChunkRepo{}, nil, nil, nil)
	pc := model.PrincipalContext{UserID: uuid.New(), ProjectRole: "admin"}

	// Try to move parent under child (circular).
	childIDStr := child.ID.String()
	_, err := svc.Move(context.Background(), pc, parent.ID, model.MoveKnowledgeAssetRequest{
		ParentID:  &childIDStr,
		SortOrder: 0,
	})
	if !errors.Is(err, knowledge.ErrCircularMove) {
		t.Fatalf("expected ErrCircularMove, got: %v", err)
	}
}

func TestTree_MoveToSelf_Rejected(t *testing.T) {
	projectID := uuid.New()
	spaceID := uuid.New()

	page := &model.KnowledgeAsset{
		ID: uuid.New(), ProjectID: projectID, Kind: model.KindWikiPage,
		WikiSpaceID: &spaceID, ContentJSON: `[]`, Title: "Page", Version: 1,
	}

	repo := newTreeStubRepo(page)
	svc := knowledge.NewKnowledgeAssetService(repo, stubVersionRepo{}, stubCommentRepo{}, stubChunkRepo{}, nil, nil, nil)
	pc := model.PrincipalContext{UserID: uuid.New(), ProjectRole: "admin"}

	selfIDStr := page.ID.String()
	_, err := svc.Move(context.Background(), pc, page.ID, model.MoveKnowledgeAssetRequest{
		ParentID:  &selfIDStr,
		SortOrder: 0,
	})
	if !errors.Is(err, knowledge.ErrCircularMove) {
		t.Fatalf("expected ErrCircularMove when moving to self, got: %v", err)
	}
}

func TestTree_DeleteCascade_SoftDeletesDescendants(t *testing.T) {
	projectID := uuid.New()
	spaceID := uuid.New()

	root := &model.KnowledgeAsset{
		ID: uuid.New(), ProjectID: projectID, Kind: model.KindWikiPage,
		WikiSpaceID: &spaceID, ContentJSON: `[]`, Title: "Root", Version: 1,
	}
	child := &model.KnowledgeAsset{
		ID: uuid.New(), ProjectID: projectID, Kind: model.KindWikiPage,
		WikiSpaceID: &spaceID, ContentJSON: `[]`, Title: "Child", Version: 1,
		ParentID: &root.ID,
	}
	grandchild := &model.KnowledgeAsset{
		ID: uuid.New(), ProjectID: projectID, Kind: model.KindWikiPage,
		WikiSpaceID: &spaceID, ContentJSON: `[]`, Title: "Grandchild", Version: 1,
		ParentID: &child.ID,
	}

	repo := newTreeStubRepo(root, child, grandchild)
	svc := knowledge.NewKnowledgeAssetService(repo, stubVersionRepo{}, stubCommentRepo{}, stubChunkRepo{}, nil, nil, nil)
	pc := model.PrincipalContext{UserID: uuid.New(), ProjectRole: "editor"}

	if err := svc.Delete(context.Background(), pc, root.ID); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	// All three should be gone from the stub store.
	if _, ok := repo.store[root.ID]; ok {
		t.Error("root should be deleted")
	}
	if _, ok := repo.store[child.ID]; ok {
		t.Error("child should be cascade-deleted")
	}
	if _, ok := repo.store[grandchild.ID]; ok {
		t.Error("grandchild should be cascade-deleted")
	}
}
