package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

func TestNewSavedViewRepository(t *testing.T) {
	repo := NewSavedViewRepository(nil)
	if repo == nil {
		t.Fatal("expected non-nil SavedViewRepository")
	}
}

func TestSavedViewRepositoryListByProjectNilDB(t *testing.T) {
	repo := NewSavedViewRepository(nil)
	_, err := repo.ListByProject(context.Background(), uuid.New(), uuid.New(), []string{"reviewer"})
	if err != ErrDatabaseUnavailable {
		t.Fatalf("ListByProject() error = %v, want %v", err, ErrDatabaseUnavailable)
	}
}

func TestSavedViewRepositoryFiltersAccessibleViewsAndSetsDefault(t *testing.T) {
	ctx := context.Background()
	repo := NewSavedViewRepository(openFoundationRepoTestDB(t, &savedViewRecord{}))

	projectID := uuid.New()
	userID := uuid.New()
	otherUserID := uuid.New()
	now := time.Date(2026, 3, 26, 13, 0, 0, 0, time.UTC)

	personal := &model.SavedView{
		ID:         uuid.New(),
		ProjectID:  projectID,
		Name:       "Mine",
		OwnerID:    &userID,
		IsDefault:  false,
		SharedWith: `{}`,
		Config:     `{"layout":"table","columns":["title"]}`,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	roleShared := &model.SavedView{
		ID:         uuid.New(),
		ProjectID:  projectID,
		Name:       "Review Queue",
		OwnerID:    nil,
		IsDefault:  false,
		SharedWith: `{"roleIds":["reviewer"]}`,
		Config:     `{"layout":"list","filters":[{"field":"status","op":"eq","value":"in_review"}]}`,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	memberShared := &model.SavedView{
		ID:         uuid.New(),
		ProjectID:  projectID,
		Name:       "Leadership",
		OwnerID:    &otherUserID,
		IsDefault:  false,
		SharedWith: `{"memberIds":["` + userID.String() + `"]}`,
		Config:     `{"layout":"board"}`,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	for _, view := range []*model.SavedView{personal, roleShared, memberShared} {
		if err := repo.Create(ctx, view); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}

	views, err := repo.ListByProject(ctx, projectID, userID, []string{"reviewer"})
	if err != nil {
		t.Fatalf("ListByProject() error = %v", err)
	}
	if len(views) != 3 {
		t.Fatalf("len(views) = %d, want 3", len(views))
	}

	if err := repo.SetDefault(ctx, projectID, roleShared.ID); err != nil {
		t.Fatalf("SetDefault() error = %v", err)
	}

	views, err = repo.ListByProject(ctx, projectID, userID, []string{"reviewer"})
	if err != nil {
		t.Fatalf("ListByProject() after SetDefault error = %v", err)
	}
	defaultCount := 0
	for _, view := range views {
		if view.IsDefault {
			defaultCount++
			if view.ID != roleShared.ID {
				t.Fatalf("unexpected default view %s", view.ID)
			}
		}
	}
	if defaultCount != 1 {
		t.Fatalf("defaultCount = %d, want 1", defaultCount)
	}
}
