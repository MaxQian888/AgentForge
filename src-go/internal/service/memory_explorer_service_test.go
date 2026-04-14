package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
)

type memoryExplorerRepoStub struct {
	byID          map[uuid.UUID]*model.AgentMemory
	filtered      []*model.AgentMemory
	lastProjectID uuid.UUID
	lastFilter    model.AgentMemoryFilter
	incremented   []uuid.UUID
	deletedMany   []uuid.UUID
}

func (s *memoryExplorerRepoStub) GetByID(_ context.Context, id uuid.UUID) (*model.AgentMemory, error) {
	entry, ok := s.byID[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return entry, nil
}

func (s *memoryExplorerRepoStub) ListFiltered(_ context.Context, projectID uuid.UUID, filter model.AgentMemoryFilter) ([]*model.AgentMemory, error) {
	s.lastProjectID = projectID
	s.lastFilter = filter
	return s.filtered, nil
}

func (s *memoryExplorerRepoStub) IncrementAccess(_ context.Context, id uuid.UUID) error {
	s.incremented = append(s.incremented, id)
	return nil
}

func (s *memoryExplorerRepoStub) Delete(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (s *memoryExplorerRepoStub) DeleteMany(_ context.Context, ids []uuid.UUID) (int64, error) {
	s.deletedMany = append([]uuid.UUID(nil), ids...)
	return int64(len(ids)), nil
}

type episodicExplorerStub struct {
	history      []*model.AgentMemory
	historyQuery EpisodicMemoryQuery
	historyErr   error
	applyLimit   bool
	detail       *model.AgentMemory
	detailAccess MemoryAccessRequest
	detailErr    error
	exportResult *EpisodicMemoryExport
	exportReq    EpisodicMemoryExportRequest
	exportErr    error
}

func (s *episodicExplorerStub) Get(_ context.Context, _ uuid.UUID, access MemoryAccessRequest) (*model.AgentMemory, error) {
	s.detailAccess = access
	if s.detailErr != nil {
		return nil, s.detailErr
	}
	return s.detail, nil
}

func (s *episodicExplorerStub) ListHistory(_ context.Context, query EpisodicMemoryQuery) ([]*model.AgentMemory, error) {
	s.historyQuery = query
	if s.historyErr != nil {
		return nil, s.historyErr
	}
	if s.applyLimit && query.Limit > 0 && len(s.history) > query.Limit {
		return s.history[:query.Limit], nil
	}
	return s.history, nil
}

func (s *episodicExplorerStub) Export(_ context.Context, req EpisodicMemoryExportRequest) (*EpisodicMemoryExport, error) {
	s.exportReq = req
	if s.exportErr != nil {
		return nil, s.exportErr
	}
	return s.exportResult, nil
}

func TestMemoryExplorerService_SearchHonorsFiltersAndAccess(t *testing.T) {
	projectID := uuid.New()
	start := time.Date(2026, 4, 1, 8, 0, 0, 0, time.UTC)
	end := start.Add(2 * time.Hour)
	repo := &memoryExplorerRepoStub{
		filtered: []*model.AgentMemory{
			{
				ID:        uuid.New(),
				ProjectID: projectID,
				Scope:     model.MemoryScopeProject,
				Category:  model.MemoryCategorySemantic,
				Key:       "release-plan",
				Content:   "Use staggered rollout",
				Metadata:  `{"source":"ops"}`,
				CreatedAt: start,
				UpdatedAt: end,
			},
			{
				ID:        uuid.New(),
				ProjectID: projectID,
				Scope:     model.MemoryScopeRole,
				RoleID:    "reviewer",
				Category:  model.MemoryCategorySemantic,
				Key:       "private",
				Content:   "Hidden note",
				CreatedAt: start,
				UpdatedAt: end,
			},
		},
	}
	svc := NewMemoryExplorerService(repo)

	results, err := svc.Search(context.Background(), MemoryExplorerQuery{
		ProjectID: projectID,
		Query:     "release",
		Scope:     model.MemoryScopeProject,
		Category:  model.MemoryCategorySemantic,
		StartAt:   &start,
		EndAt:     &end,
		Limit:     5,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if repo.lastProjectID != projectID {
		t.Fatalf("projectID = %s, want %s", repo.lastProjectID, projectID)
	}
	if repo.lastFilter.Query != "release" || repo.lastFilter.Scope != model.MemoryScopeProject || repo.lastFilter.Category != model.MemoryCategorySemantic || repo.lastFilter.Limit != 5 {
		t.Fatalf("lastFilter = %#v", repo.lastFilter)
	}
	if len(results) != 1 || results[0].Key != "release-plan" {
		t.Fatalf("results = %#v", results)
	}
	if len(repo.incremented) != 1 || repo.incremented[0] != repo.filtered[0].ID {
		t.Fatalf("incremented = %#v", repo.incremented)
	}
}

func TestMemoryExplorerService_GetReturnsDetailMetadata(t *testing.T) {
	projectID := uuid.New()
	memoryID := uuid.New()
	repo := &memoryExplorerRepoStub{
		byID: map[uuid.UUID]*model.AgentMemory{
			memoryID: {
				ID:             memoryID,
				ProjectID:      projectID,
				Scope:          model.MemoryScopeProject,
				Category:       model.MemoryCategorySemantic,
				Key:            "release-plan",
				Content:        "Use staged rollout",
				Metadata:       `{"taskId":"task-1","sessionId":"session-9"}`,
				AccessCount:    2,
				CreatedAt:      time.Date(2026, 4, 1, 8, 0, 0, 0, time.UTC),
				UpdatedAt:      time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC),
				LastAccessedAt: ptrMemoryExplorerTime(time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)),
			},
		},
	}
	svc := NewMemoryExplorerService(repo)

	detail, err := svc.Get(context.Background(), projectID, memoryID, "")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if detail.UpdatedAt == "" || detail.LastAccessedAt == "" {
		t.Fatalf("detail timestamps = %#v", detail)
	}
	if len(detail.RelatedContext) != 2 {
		t.Fatalf("relatedContext = %#v", detail.RelatedContext)
	}
}

func TestMemoryExplorerService_StatsBulkDeleteCleanupAndExport(t *testing.T) {
	projectID := uuid.New()
	keepID := uuid.New()
	roleID := uuid.New()
	now := time.Date(2026, 4, 9, 12, 0, 0, 0, time.UTC)
	repo := &memoryExplorerRepoStub{
		filtered: []*model.AgentMemory{
			{ID: keepID, ProjectID: projectID, Scope: model.MemoryScopeProject, Category: model.MemoryCategoryEpisodic, Key: "k1", Content: "abc", Metadata: `{"a":1}`, CreatedAt: now.Add(-2 * time.Hour), UpdatedAt: now.Add(-2 * time.Hour)},
			{ID: roleID, ProjectID: projectID, Scope: model.MemoryScopeRole, RoleID: "planner", Category: model.MemoryCategorySemantic, Key: "k2", Content: "def", Metadata: `{"b":2}`, CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour), LastAccessedAt: ptrMemoryExplorerTime(now.Add(-30 * time.Minute))},
		},
		byID: map[uuid.UUID]*model.AgentMemory{
			keepID: {ID: keepID, ProjectID: projectID, Scope: model.MemoryScopeProject, Category: model.MemoryCategoryEpisodic, Key: "k1", CreatedAt: now, UpdatedAt: now},
			roleID: {ID: roleID, ProjectID: projectID, Scope: model.MemoryScopeRole, RoleID: "planner", Category: model.MemoryCategorySemantic, Key: "k2", CreatedAt: now, UpdatedAt: now},
		},
	}
	episodic := &episodicExplorerStub{
		history:      []*model.AgentMemory{{ID: keepID, ProjectID: projectID, Scope: model.MemoryScopeProject, Category: model.MemoryCategoryEpisodic, Key: "k1", CreatedAt: now.Add(-24 * time.Hour), UpdatedAt: now.Add(-24 * time.Hour)}},
		exportResult: &EpisodicMemoryExport{ProjectID: projectID.String(), Entries: []EpisodicMemoryExportEntry{{ID: keepID.String()}}},
	}
	svc := NewMemoryExplorerService(repo).WithEpisodic(episodic)
	svc.now = func() time.Time { return now }

	stats, err := svc.Stats(context.Background(), MemoryExplorerQuery{ProjectID: projectID, RoleID: "planner"})
	if err != nil {
		t.Fatalf("Stats() error = %v", err)
	}
	if stats.TotalCount != 2 || stats.ByCategory[model.MemoryCategoryEpisodic] != 1 || stats.ByScope[model.MemoryScopeRole] != 1 {
		t.Fatalf("stats = %#v", stats)
	}

	deleted, err := svc.BulkDelete(context.Background(), projectID, []uuid.UUID{keepID, roleID}, "")
	if err != nil {
		t.Fatalf("BulkDelete() error = %v", err)
	}
	if deleted != 1 || len(repo.deletedMany) != 1 || repo.deletedMany[0] != keepID {
		t.Fatalf("deletedMany = %#v, deleted = %d", repo.deletedMany, deleted)
	}

	exported, err := svc.ExportEpisodic(context.Background(), MemoryExplorerQuery{
		ProjectID: projectID,
		Query:     "k1",
		Scope:     model.MemoryScopeRole,
		Category:  model.MemoryCategoryEpisodic,
		RoleID:    "planner",
	})
	if err != nil {
		t.Fatalf("ExportEpisodic() error = %v", err)
	}
	if exported.ProjectID != projectID.String() ||
		episodic.exportReq.Scope != model.MemoryScopeRole ||
		episodic.exportReq.RoleID != "planner" ||
		episodic.exportReq.Query != "k1" ||
		episodic.exportReq.Category != model.MemoryCategoryEpisodic {
		t.Fatalf("exported = %#v, exportReq = %#v", exported, episodic.exportReq)
	}

	cleanupDeleted, err := svc.CleanupEpisodic(context.Background(), MemoryCleanupInput{ProjectID: projectID, Scope: model.MemoryScopeProject, RetentionDays: 7})
	if err != nil {
		t.Fatalf("CleanupEpisodic() error = %v", err)
	}
	if cleanupDeleted != 1 || episodic.historyQuery.Scope != model.MemoryScopeProject || episodic.historyQuery.EndAt == nil {
		t.Fatalf("cleanup query = %#v deleted=%d", episodic.historyQuery, cleanupDeleted)
	}
}

func TestMemoryExplorerService_GetRejectsInaccessibleRoleScopedEntry(t *testing.T) {
	projectID := uuid.New()
	memoryID := uuid.New()
	repo := &memoryExplorerRepoStub{
		byID: map[uuid.UUID]*model.AgentMemory{
			memoryID: {ID: memoryID, ProjectID: projectID, Scope: model.MemoryScopeRole, RoleID: "planner", Category: model.MemoryCategorySemantic, Key: "private", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
		},
	}
	svc := NewMemoryExplorerService(repo)

	_, err := svc.Get(context.Background(), projectID, memoryID, "reviewer")
	if !errors.Is(err, ErrMemoryAccessDenied) {
		t.Fatalf("Get() error = %v, want ErrMemoryAccessDenied", err)
	}
}

func TestMemoryExplorerService_SearchAppliesEpisodicSearchBeforeLimit(t *testing.T) {
	projectID := uuid.New()
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	repo := &memoryExplorerRepoStub{}
	episodic := &episodicExplorerStub{
		applyLimit: true,
		history: []*model.AgentMemory{
			{
				ID:        uuid.New(),
				ProjectID: projectID,
				Scope:     model.MemoryScopeProject,
				Category:  model.MemoryCategoryEpisodic,
				Key:       "turn-1",
				Content:   "opening context",
				CreatedAt: now.Add(-2 * time.Hour),
				UpdatedAt: now.Add(-2 * time.Hour),
			},
			{
				ID:        uuid.New(),
				ProjectID: projectID,
				Scope:     model.MemoryScopeProject,
				Category:  model.MemoryCategoryEpisodic,
				Key:       "turn-2",
				Content:   "matched detail beyond the first page",
				CreatedAt: now.Add(-time.Hour),
				UpdatedAt: now.Add(-time.Hour),
			},
			{
				ID:        uuid.New(),
				ProjectID: projectID,
				Scope:     model.MemoryScopeProject,
				Category:  model.MemoryCategoryEpisodic,
				Key:       "turn-3",
				Content:   "matched follow up beyond the first page",
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
	}
	svc := NewMemoryExplorerService(repo).WithEpisodic(episodic)

	results, err := svc.Search(context.Background(), MemoryExplorerQuery{
		ProjectID: projectID,
		Query:     "matched",
		Category:  model.MemoryCategoryEpisodic,
		Limit:     1,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if episodic.historyQuery.Limit != 0 {
		t.Fatalf("history limit = %d, want 0 while searching episodic history", episodic.historyQuery.Limit)
	}
	if len(results) != 1 || results[0].Key != "turn-2" {
		t.Fatalf("results = %#v", results)
	}
	if len(repo.incremented) != 1 || repo.incremented[0] != episodic.history[1].ID {
		t.Fatalf("incremented = %#v", repo.incremented)
	}
}

func ptrMemoryExplorerTime(value time.Time) *time.Time {
	return &value
}
