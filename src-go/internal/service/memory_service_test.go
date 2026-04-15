package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

type memoryServiceRepoStub struct {
	created     *model.AgentMemory
	getByID     *model.AgentMemory
	search      []*model.AgentMemory
	projectList []*model.AgentMemory
	deletedID   uuid.UUID
}

func (s *memoryServiceRepoStub) Create(_ context.Context, mem *model.AgentMemory) error {
	s.created = mem
	return nil
}

func (s *memoryServiceRepoStub) GetByID(_ context.Context, _ uuid.UUID) (*model.AgentMemory, error) {
	return s.getByID, nil
}

func (s *memoryServiceRepoStub) ListByProject(_ context.Context, _ uuid.UUID, _, _ string) ([]*model.AgentMemory, error) {
	return s.projectList, nil
}

func (s *memoryServiceRepoStub) Search(_ context.Context, _ uuid.UUID, _ string, _ int) ([]*model.AgentMemory, error) {
	return s.search, nil
}

func (s *memoryServiceRepoStub) IncrementAccess(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (s *memoryServiceRepoStub) Update(_ context.Context, mem *model.AgentMemory) error {
	s.created = mem
	return nil
}

func (s *memoryServiceRepoStub) Delete(_ context.Context, id uuid.UUID) error {
	s.deletedID = id
	return nil
}

func TestMemoryServiceStoreNormalizesOperatorNoteAlias(t *testing.T) {
	repo := &memoryServiceRepoStub{}
	svc := NewMemoryService(repo)
	svc.now = func() time.Time {
		return time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	}

	mem, err := svc.Store(context.Background(), StoreMemoryInput{
		ProjectID:      uuid.New(),
		Category:       "operator_note",
		Key:            "release-note",
		Content:        "Remember to tag the rollout.",
		Metadata:       `{"tags":["ops","ops"]}`,
		RelevanceScore: 0.5,
	})
	if err != nil {
		t.Fatalf("Store() error = %v", err)
	}
	if repo.created == nil {
		t.Fatal("expected created memory to be captured")
	}
	if mem.Category != model.MemoryCategoryEpisodic || repo.created.Category != model.MemoryCategoryEpisodic {
		t.Fatalf("stored category = %q repo=%q, want %q", mem.Category, repo.created.Category, model.MemoryCategoryEpisodic)
	}
	if mem.ToDTO().Kind != "operator_note" || !mem.ToDTO().Editable {
		t.Fatalf("stored dto = %#v", mem.ToDTO())
	}
	if len(mem.ToDTO().Tags) != 1 || mem.ToDTO().Tags[0] != "ops" {
		t.Fatalf("stored tags = %#v", mem.ToDTO().Tags)
	}
}

func TestMemoryServiceUpdateAllowsOperatorNoteButRejectsReadOnlyContentEdits(t *testing.T) {
	projectID := uuid.New()
	memoryID := uuid.New()
	repo := &memoryServiceRepoStub{
		getByID: &model.AgentMemory{
			ID:        memoryID,
			ProjectID: projectID,
			Scope:     model.MemoryScopeProject,
			Category:  model.MemoryCategoryEpisodic,
			Key:       "release-note",
			Content:   "Remember the checklist.",
			Metadata:  `{"kind":"operator_note","editable":true,"tags":["ops"]}`,
			CreatedAt: time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC),
		},
	}
	svc := NewMemoryService(repo)
	svc.now = func() time.Time {
		return time.Date(2026, 4, 15, 13, 0, 0, 0, time.UTC)
	}
	key := "release-note"
	content := "Remember the checklist and rollout tag."
	tags := []string{"ops", "release"}

	updated, err := svc.Update(context.Background(), UpdateMemoryInput{
		ProjectID: projectID,
		ID:        memoryID,
		Key:       &key,
		Content:   &content,
		Tags:      &tags,
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Content != content || len(updated.ToDTO().Tags) != 2 || !updated.ToDTO().Editable {
		t.Fatalf("updated memory = %#v", updated.ToDTO())
	}

	readOnlyID := uuid.New()
	repo.getByID = &model.AgentMemory{
		ID:        readOnlyID,
		ProjectID: projectID,
		Scope:     model.MemoryScopeProject,
		Category:  model.MemoryCategorySemantic,
		Key:       "fact",
		Content:   "Stable fact",
		Metadata:  `{"tags":["kb"]}`,
		CreatedAt: time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC),
	}
	changedContent := "Mutated fact"
	if _, err := svc.Update(context.Background(), UpdateMemoryInput{
		ProjectID: projectID,
		ID:        readOnlyID,
		Content:   &changedContent,
	}); !errors.Is(err, ErrMemoryNotEditable) {
		t.Fatalf("Update(read-only) error = %v, want ErrMemoryNotEditable", err)
	}

	newTags := []string{"kb", "ops"}
	updatedTags, err := svc.Update(context.Background(), UpdateMemoryInput{
		ProjectID: projectID,
		ID:        readOnlyID,
		Tags:      &newTags,
	})
	if err != nil {
		t.Fatalf("Update(tags only) error = %v", err)
	}
	if len(updatedTags.ToDTO().Tags) != 2 || updatedTags.Content != "Stable fact" {
		t.Fatalf("updated tags only = %#v", updatedTags.ToDTO())
	}
}
