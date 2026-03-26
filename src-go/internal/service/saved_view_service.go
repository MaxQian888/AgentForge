package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

type savedViewRepository interface {
	Create(ctx context.Context, view *model.SavedView) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.SavedView, error)
	ListByProject(ctx context.Context, projectID uuid.UUID, userID uuid.UUID, roles []string) ([]*model.SavedView, error)
	Update(ctx context.Context, view *model.SavedView) error
	Delete(ctx context.Context, id uuid.UUID) error
	SetDefault(ctx context.Context, projectID uuid.UUID, viewID uuid.UUID) error
}

type SavedViewService struct {
	repo savedViewRepository
	now  func() time.Time
}

func NewSavedViewService(repo savedViewRepository) *SavedViewService {
	return &SavedViewService{
		repo: repo,
		now:  func() time.Time { return time.Now().UTC() },
	}
}

func (s *SavedViewService) CreateView(ctx context.Context, view *model.SavedView) error {
	now := s.now()
	if view.ID == uuid.Nil {
		view.ID = uuid.New()
	}
	if view.CreatedAt.IsZero() {
		view.CreatedAt = now
	}
	view.UpdatedAt = now
	return s.repo.Create(ctx, view)
}

func (s *SavedViewService) GetView(ctx context.Context, id uuid.UUID) (*model.SavedView, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *SavedViewService) UpdateView(ctx context.Context, view *model.SavedView) error {
	view.UpdatedAt = s.now()
	return s.repo.Update(ctx, view)
}

func (s *SavedViewService) DeleteView(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *SavedViewService) ListAccessibleViews(ctx context.Context, projectID uuid.UUID, userID uuid.UUID, roles []string) ([]*model.SavedView, error) {
	return s.repo.ListByProject(ctx, projectID, userID, roles)
}

func (s *SavedViewService) SetDefaultView(ctx context.Context, projectID uuid.UUID, viewID uuid.UUID) error {
	return s.repo.SetDefault(ctx, projectID, viewID)
}
