package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/service"
)

type episodicMemoryRepoStub struct {
	created         []*model.AgentMemory
	listResult      []*model.AgentMemory
	rangeResult     []*model.AgentMemory
	getResult       *model.AgentMemory
	createErr       error
	listErr         error
	rangeErr        error
	getErr          error
	deleteOlderErr  error
	deleteOlderRows int64

	lastListProjectID      uuid.UUID
	lastListScope          string
	lastListCategory       string
	lastRangeProjectID     uuid.UUID
	lastRangeCategory      string
	lastRangeScope         string
	lastRangeRoleID        string
	lastRangeStart         *time.Time
	lastRangeEnd           *time.Time
	lastRangeLimit         int
	lastDeleteOlderProject uuid.UUID
	lastDeleteOlderCat     string
	lastDeleteOlderBefore  time.Time
}

func (s *episodicMemoryRepoStub) Create(_ context.Context, mem *model.AgentMemory) error {
	if s.createErr != nil {
		return s.createErr
	}
	cloned := *mem
	s.created = append(s.created, &cloned)
	return nil
}

func (s *episodicMemoryRepoStub) GetByID(_ context.Context, id uuid.UUID) (*model.AgentMemory, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	if s.getResult != nil && s.getResult.ID == id {
		cloned := *s.getResult
		return &cloned, nil
	}
	return nil, nil
}

func (s *episodicMemoryRepoStub) ListByProject(_ context.Context, projectID uuid.UUID, scope, category string) ([]*model.AgentMemory, error) {
	s.lastListProjectID = projectID
	s.lastListScope = scope
	s.lastListCategory = category
	if s.listErr != nil {
		return nil, s.listErr
	}
	return cloneMemorySlice(s.listResult), nil
}

func (s *episodicMemoryRepoStub) Search(_ context.Context, _ uuid.UUID, _ string, _ int) ([]*model.AgentMemory, error) {
	return nil, nil
}

func (s *episodicMemoryRepoStub) IncrementAccess(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (s *episodicMemoryRepoStub) Delete(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (s *episodicMemoryRepoStub) ListByProjectAndTimeRange(_ context.Context, projectID uuid.UUID, category, scope, roleID string, start, end *time.Time, limit int) ([]*model.AgentMemory, error) {
	s.lastRangeProjectID = projectID
	s.lastRangeCategory = category
	s.lastRangeScope = scope
	s.lastRangeRoleID = roleID
	s.lastRangeStart = start
	s.lastRangeEnd = end
	s.lastRangeLimit = limit
	if s.rangeErr != nil {
		return nil, s.rangeErr
	}
	return cloneMemorySlice(s.rangeResult), nil
}

func (s *episodicMemoryRepoStub) DeleteOlderThan(_ context.Context, projectID uuid.UUID, category string, before time.Time) (int64, error) {
	s.lastDeleteOlderProject = projectID
	s.lastDeleteOlderCat = category
	s.lastDeleteOlderBefore = before
	if s.deleteOlderErr != nil {
		return 0, s.deleteOlderErr
	}
	return s.deleteOlderRows, nil
}

func TestEpisodicMemoryService_StoreTurnAndQueryRange(t *testing.T) {
	t.Parallel()

	projectID := uuid.New()
	repo := &episodicMemoryRepoStub{
		rangeResult: []*model.AgentMemory{{
			ID:        uuid.New(),
			ProjectID: projectID,
			Scope:     model.MemoryScopeRole,
			RoleID:    "planner",
			Category:  model.MemoryCategoryEpisodic,
			Key:       "session:s1:turn:3",
			Content:   "third turn",
			Metadata:  `{"sessionId":"s1","turnNumber":3,"actor":"assistant"}`,
		}},
	}
	svc := service.NewEpisodicMemoryService(repo)

	stored, err := svc.StoreTurn(context.Background(), service.StoreConversationTurnInput{
		ProjectID:  projectID,
		Scope:      model.MemoryScopeRole,
		RoleID:     "planner",
		SessionID:  "s1",
		TurnNumber: 3,
		Actor:      "assistant",
		Content:    "third turn",
	})
	if err != nil {
		t.Fatalf("StoreTurn() error = %v", err)
	}
	if stored.Category != model.MemoryCategoryEpisodic || stored.Key != "session:s1:turn:3" {
		t.Fatalf("StoreTurn() = %#v", stored)
	}
	if len(repo.created) != 1 || repo.created[0].Category != model.MemoryCategoryEpisodic {
		t.Fatalf("repo.created = %#v", repo.created)
	}

	start := time.Date(2026, 3, 31, 8, 0, 0, 0, time.UTC)
	end := start.Add(2 * time.Hour)
	history, err := svc.ListHistory(context.Background(), service.EpisodicMemoryQuery{
		ProjectID: projectID,
		Scope:     model.MemoryScopeRole,
		RoleID:    "planner",
		StartAt:   &start,
		EndAt:     &end,
		Limit:     20,
	})
	if err != nil {
		t.Fatalf("ListHistory() error = %v", err)
	}
	if len(history) != 1 || history[0].RoleID != "planner" {
		t.Fatalf("ListHistory() = %#v", history)
	}
	if repo.lastRangeCategory != model.MemoryCategoryEpisodic || repo.lastRangeRoleID != "planner" {
		t.Fatalf("range call = category=%q role=%q", repo.lastRangeCategory, repo.lastRangeRoleID)
	}
}

func TestEpisodicMemoryService_AccessControlRetentionExportImportAndMigration(t *testing.T) {
	t.Parallel()

	projectID := uuid.New()
	now := time.Date(2026, 3, 31, 10, 0, 0, 0, time.UTC)
	roleEntry := &model.AgentMemory{
		ID:        uuid.New(),
		ProjectID: projectID,
		Scope:     model.MemoryScopeRole,
		RoleID:    "planner",
		Category:  model.MemoryCategoryEpisodic,
		Key:       "session:s1:turn:1",
		Content:   "private planner note",
		Metadata:  `{"sessionId":"s1","turnNumber":1}`,
		CreatedAt: now,
	}
	repo := &episodicMemoryRepoStub{
		listResult:      []*model.AgentMemory{roleEntry},
		getResult:       roleEntry,
		deleteOlderRows: 2,
	}
	svc := service.NewEpisodicMemoryService(repo)

	if _, err := svc.Get(context.Background(), roleEntry.ID, service.MemoryAccessRequest{
		ProjectID: projectID,
		RoleID:    "reviewer",
	}); !errors.Is(err, service.ErrMemoryAccessDenied) {
		t.Fatalf("Get() error = %v, want ErrMemoryAccessDenied", err)
	}

	deleted, err := svc.ApplyRetention(context.Background(), projectID, 30, now)
	if err != nil {
		t.Fatalf("ApplyRetention() error = %v", err)
	}
	if deleted != 2 {
		t.Fatalf("ApplyRetention() deleted = %d, want 2", deleted)
	}

	exported, err := svc.Export(context.Background(), service.EpisodicMemoryExportRequest{
		ProjectID: projectID,
		RoleID:    "planner",
	})
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	if len(exported.Entries) != 1 || exported.Entries[0].ID != roleEntry.ID.String() {
		t.Fatalf("Export() = %#v", exported)
	}

	importPayload, err := json.Marshal(service.EpisodicMemoryExport{
		ProjectID: projectID.String(),
		Entries: []service.EpisodicMemoryExportEntry{{
			ID:        uuid.New().String(),
			Scope:     model.MemoryScopeProject,
			Category:  model.MemoryCategoryEpisodic,
			Key:       "session:s2:turn:1",
			Content:   "imported note",
			Metadata:  `{"sessionId":"s2","turnNumber":1}`,
			CreatedAt: now.Add(-time.Hour).Format(time.RFC3339),
		}},
	})
	if err != nil {
		t.Fatalf("Marshal(importPayload) error = %v", err)
	}
	imported, err := svc.Import(context.Background(), projectID, importPayload)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if imported != 1 {
		t.Fatalf("Import() imported = %d, want 1", imported)
	}

	dir := t.TempDir()
	snapshotPath := filepath.Join(dir, "task-1.json")
	snapshot := map[string]any{
		"task_id":     "task-1",
		"session_id":  "session-1",
		"status":      "paused",
		"turn_number": 4,
		"spent_usd":   1.25,
		"created_at":  now.Add(-2 * time.Hour).UnixMilli(),
		"updated_at":  now.UnixMilli(),
		"request": map[string]any{
			"prompt":  "Resume the task",
			"runtime": "codex",
		},
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("Marshal(snapshot) error = %v", err)
	}
	if err := os.WriteFile(snapshotPath, raw, 0o644); err != nil {
		t.Fatalf("WriteFile(snapshot) error = %v", err)
	}

	migrated, err := svc.ImportSessionSnapshots(context.Background(), service.SessionSnapshotImportRequest{
		ProjectID: projectID,
		RoleID:    "planner",
		Scope:     model.MemoryScopeRole,
		Dir:       dir,
	})
	if err != nil {
		t.Fatalf("ImportSessionSnapshots() error = %v", err)
	}
	if migrated != 1 {
		t.Fatalf("ImportSessionSnapshots() = %d, want 1", migrated)
	}
}

func cloneMemorySlice(input []*model.AgentMemory) []*model.AgentMemory {
	if input == nil {
		return nil
	}
	cloned := make([]*model.AgentMemory, 0, len(input))
	for _, item := range input {
		if item == nil {
			cloned = append(cloned, nil)
			continue
		}
		copyItem := *item
		cloned = append(cloned, &copyItem)
	}
	return cloned
}
