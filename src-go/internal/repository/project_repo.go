package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"gorm.io/gorm"
)

type ProjectRepository struct {
	db *gorm.DB
}

func NewProjectRepository(db *gorm.DB) *ProjectRepository {
	return &ProjectRepository{db: db}
}

func (r *ProjectRepository) Create(ctx context.Context, project *model.Project) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).Create(newProjectRecord(project)).Error; err != nil {
		return fmt.Errorf("create project: %w", err)
	}
	return nil
}

func (r *ProjectRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Project, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}

	var record projectRecord
	if err := r.db.WithContext(ctx).Where("id = ?", id).Take(&record).Error; err != nil {
		return nil, fmt.Errorf("get project by id: %w", normalizeRepositoryError(err))
	}
	return record.toModel(), nil
}

func (r *ProjectRepository) GetBySlug(ctx context.Context, slug string) (*model.Project, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}

	var record projectRecord
	if err := r.db.WithContext(ctx).Where("slug = ?", slug).Take(&record).Error; err != nil {
		return nil, fmt.Errorf("get project by slug: %w", normalizeRepositoryError(err))
	}
	return record.toModel(), nil
}

func (r *ProjectRepository) List(ctx context.Context) ([]*model.Project, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}

	var records []projectRecord
	if err := r.db.WithContext(ctx).Order("created_at DESC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}

	projects := make([]*model.Project, 0, len(records))
	for i := range records {
		projects = append(projects, records[i].toModel())
	}
	return projects, nil
}

func (r *ProjectRepository) Update(ctx context.Context, id uuid.UUID, req *model.UpdateProjectRequest) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}

	project, err := r.GetByID(ctx, id)
	if err != nil {
		return err
	}
	settingsJSON, err := model.MergeProjectSettings(project.Settings, req.Settings)
	if err != nil {
		return fmt.Errorf("merge project settings: %w", err)
	}

	updates := map[string]any{
		"settings": newJSONText(settingsJSON, "{}"),
	}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.RepoURL != nil {
		updates["repo_url"] = *req.RepoURL
	}
	if req.DefaultBranch != nil {
		updates["default_branch"] = *req.DefaultBranch
	}

	if err := r.db.WithContext(ctx).
		Model(&projectRecord{}).
		Where("id = ?", id).
		Updates(updates).
		Error; err != nil {
		return fmt.Errorf("update project: %w", err)
	}
	return nil
}

func (r *ProjectRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}

	result := r.db.WithContext(ctx).Delete(&projectRecord{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("delete project: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("delete project: %w", ErrNotFound)
	}
	return nil
}
