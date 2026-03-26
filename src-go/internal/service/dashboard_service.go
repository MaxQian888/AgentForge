package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

type dashboardRepository interface {
	CreateConfig(ctx context.Context, config *model.DashboardConfig) error
	GetConfig(ctx context.Context, id uuid.UUID) (*model.DashboardConfig, error)
	ListByProject(ctx context.Context, projectID uuid.UUID) ([]*model.DashboardConfig, error)
	UpdateConfig(ctx context.Context, config *model.DashboardConfig) error
	DeleteConfig(ctx context.Context, id uuid.UUID) error
	SaveWidget(ctx context.Context, widget *model.DashboardWidget) error
	DeleteWidget(ctx context.Context, id uuid.UUID) error
	ListWidgetsByDashboard(ctx context.Context, dashboardID uuid.UUID) ([]*model.DashboardWidget, error)
}

type DashboardService struct {
	repo dashboardRepository
	now  func() time.Time
}

func NewDashboardService(repo dashboardRepository) *DashboardService {
	return &DashboardService{
		repo: repo,
		now:  func() time.Time { return time.Now().UTC() },
	}
}

func (s *DashboardService) CreateDashboard(ctx context.Context, config *model.DashboardConfig) error {
	now := s.now()
	if config.ID == uuid.Nil {
		config.ID = uuid.New()
	}
	if config.CreatedAt.IsZero() {
		config.CreatedAt = now
	}
	config.UpdatedAt = now
	return s.repo.CreateConfig(ctx, config)
}

func (s *DashboardService) GetDashboard(ctx context.Context, id uuid.UUID) (*model.DashboardConfig, error) {
	return s.repo.GetConfig(ctx, id)
}

func (s *DashboardService) UpdateDashboard(ctx context.Context, config *model.DashboardConfig) error {
	config.UpdatedAt = s.now()
	return s.repo.UpdateConfig(ctx, config)
}

func (s *DashboardService) DeleteDashboard(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeleteConfig(ctx, id)
}

func (s *DashboardService) ListDashboards(ctx context.Context, projectID uuid.UUID) ([]*model.DashboardConfig, error) {
	return s.repo.ListByProject(ctx, projectID)
}

func (s *DashboardService) SaveWidget(ctx context.Context, widget *model.DashboardWidget) error {
	now := s.now()
	if widget.ID == uuid.Nil {
		widget.ID = uuid.New()
	}
	if widget.CreatedAt.IsZero() {
		widget.CreatedAt = now
	}
	widget.UpdatedAt = now
	return s.repo.SaveWidget(ctx, widget)
}

func (s *DashboardService) DeleteWidget(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeleteWidget(ctx, id)
}

func (s *DashboardService) ListWidgets(ctx context.Context, dashboardID uuid.UUID) ([]*model.DashboardWidget, error) {
	return s.repo.ListWidgetsByDashboard(ctx, dashboardID)
}
