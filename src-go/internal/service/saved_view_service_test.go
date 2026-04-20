package service

import (
	"context"
	"testing"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
)

type fakeSavedViewRepo struct {
	views        map[uuid.UUID]*model.SavedView
	projectViews map[uuid.UUID][]*model.SavedView
	created      *model.SavedView
	updated      *model.SavedView
	deletedID    uuid.UUID
	setDefaultID uuid.UUID
}

func (f *fakeSavedViewRepo) Create(_ context.Context, view *model.SavedView) error {
	f.created = view
	if f.views == nil {
		f.views = map[uuid.UUID]*model.SavedView{}
	}
	f.views[view.ID] = view
	f.projectViews[view.ProjectID] = append(f.projectViews[view.ProjectID], view)
	return nil
}

func (f *fakeSavedViewRepo) GetByID(_ context.Context, id uuid.UUID) (*model.SavedView, error) {
	return f.views[id], nil
}

func (f *fakeSavedViewRepo) ListByProject(_ context.Context, projectID uuid.UUID, userID uuid.UUID, roles []string) ([]*model.SavedView, error) {
	result := make([]*model.SavedView, 0)
	for _, view := range f.projectViews[projectID] {
		if view.IsAccessibleTo(userID, roles) {
			result = append(result, view)
		}
	}
	return result, nil
}

func (f *fakeSavedViewRepo) Update(_ context.Context, view *model.SavedView) error {
	f.updated = view
	f.views[view.ID] = view
	return nil
}

func (f *fakeSavedViewRepo) Delete(_ context.Context, id uuid.UUID) error {
	f.deletedID = id
	return nil
}

func (f *fakeSavedViewRepo) SetDefault(_ context.Context, projectID uuid.UUID, viewID uuid.UUID) error {
	f.setDefaultID = viewID
	return nil
}

func TestSavedViewServiceCreateAndListViews(t *testing.T) {
	projectID := uuid.New()
	userID := uuid.New()
	repo := &fakeSavedViewRepo{
		views:        map[uuid.UUID]*model.SavedView{},
		projectViews: map[uuid.UUID][]*model.SavedView{},
	}
	service := NewSavedViewService(repo)

	view := &model.SavedView{
		ProjectID:  projectID,
		Name:       "Triaged",
		OwnerID:    &userID,
		Config:     `{"layout":"table"}`,
		SharedWith: `{}`,
	}
	if err := service.CreateView(context.Background(), view); err != nil {
		t.Fatalf("CreateView() error = %v", err)
	}
	if view.ID == uuid.Nil {
		t.Fatal("expected service to assign an ID")
	}

	views, err := service.ListAccessibleViews(context.Background(), projectID, userID, nil)
	if err != nil {
		t.Fatalf("ListAccessibleViews() error = %v", err)
	}
	if len(views) != 1 {
		t.Fatalf("len(views) = %d, want 1", len(views))
	}
}

func TestSavedViewServiceUpdateDeleteAndSetDefault(t *testing.T) {
	projectID := uuid.New()
	viewID := uuid.New()
	view := &model.SavedView{ID: viewID, ProjectID: projectID, Name: "Mine"}
	repo := &fakeSavedViewRepo{
		views:        map[uuid.UUID]*model.SavedView{viewID: view},
		projectViews: map[uuid.UUID][]*model.SavedView{projectID: {view}},
	}
	service := NewSavedViewService(repo)

	view.Name = "Updated"
	if err := service.UpdateView(context.Background(), view); err != nil {
		t.Fatalf("UpdateView() error = %v", err)
	}
	if repo.updated == nil || repo.updated.Name != "Updated" {
		t.Fatalf("unexpected updated view: %+v", repo.updated)
	}
	if err := service.SetDefaultView(context.Background(), projectID, viewID); err != nil {
		t.Fatalf("SetDefaultView() error = %v", err)
	}
	if repo.setDefaultID != viewID {
		t.Fatalf("repo.setDefaultID = %s, want %s", repo.setDefaultID, viewID)
	}
	if err := service.DeleteView(context.Background(), viewID); err != nil {
		t.Fatalf("DeleteView() error = %v", err)
	}
	if repo.deletedID != viewID {
		t.Fatalf("repo.deletedID = %s, want %s", repo.deletedID, viewID)
	}
}
