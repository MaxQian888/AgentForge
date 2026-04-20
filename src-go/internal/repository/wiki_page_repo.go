package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type WikiPageRepository struct {
	db *gorm.DB
}

func NewWikiPageRepository(db *gorm.DB) *WikiPageRepository {
	return &WikiPageRepository{db: db}
}

func (r *WikiPageRepository) DB() *gorm.DB {
	if r == nil {
		return nil
	}
	return r.db
}

func (r *WikiPageRepository) WithDB(db *gorm.DB) *WikiPageRepository {
	return &WikiPageRepository{db: db}
}

func (r *WikiPageRepository) Create(ctx context.Context, page *model.WikiPage) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).Create(newWikiPageRecord(page)).Error; err != nil {
		return fmt.Errorf("create wiki page: %w", err)
	}
	return nil
}

func (r *WikiPageRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.WikiPage, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}

	var record wikiPageRecord
	if err := r.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", id).
		Take(&record).
		Error; err != nil {
		return nil, normalizeRepositoryError(err)
	}
	return record.toModel(), nil
}

func (r *WikiPageRepository) Update(ctx context.Context, page *model.WikiPage) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	updates := map[string]any{
		"title":             page.Title,
		"content":           newJSONText(page.Content, "[]"),
		"content_text":      page.ContentText,
		"is_template":       page.IsTemplate,
		"template_category": optionalStringPointer(page.TemplateCategory),
		"is_system":         page.IsSystem,
		"is_pinned":         page.IsPinned,
		"updated_by":        page.UpdatedBy,
		"updated_at":        page.UpdatedAt,
	}
	result := r.db.WithContext(ctx).
		Model(&wikiPageRecord{}).
		Where("id = ? AND deleted_at IS NULL", page.ID).
		Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("update wiki page: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *WikiPageRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}

	now := time.Now().UTC()
	result := r.db.WithContext(ctx).
		Model(&wikiPageRecord{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]any{"deleted_at": now, "updated_at": now})
	if result.Error != nil {
		return fmt.Errorf("soft delete wiki page: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *WikiPageRepository) ListTree(ctx context.Context, spaceID uuid.UUID) ([]*model.WikiPage, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}

	var records []wikiPageRecord
	if err := r.db.WithContext(ctx).
		Where("space_id = ? AND deleted_at IS NULL", spaceID).
		Order("path ASC").
		Order("sort_order ASC").
		Order("created_at ASC").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list wiki page tree: %w", err)
	}
	return wikiPageModels(records), nil
}

func (r *WikiPageRepository) ListByParent(ctx context.Context, spaceID uuid.UUID, parentID *uuid.UUID) ([]*model.WikiPage, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}

	query := r.db.WithContext(ctx).
		Where("space_id = ? AND deleted_at IS NULL", spaceID)
	if parentID == nil {
		query = query.Where("parent_id IS NULL")
	} else {
		query = query.Where("parent_id = ?", *parentID)
	}

	var records []wikiPageRecord
	if err := query.Order("sort_order ASC").Order("created_at ASC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list wiki pages by parent: %w", err)
	}
	return wikiPageModels(records), nil
}

func (r *WikiPageRepository) MovePage(ctx context.Context, id uuid.UUID, parentID *uuid.UUID, path string, sortOrder int) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}

	result := r.db.WithContext(ctx).
		Model(&wikiPageRecord{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]any{
			"parent_id":  parentID,
			"path":       path,
			"sort_order": sortOrder,
			"updated_at": time.Now().UTC(),
		})
	if result.Error != nil {
		return fmt.Errorf("move wiki page: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *WikiPageRepository) UpdateSortOrder(ctx context.Context, id uuid.UUID, sortOrder int) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}

	result := r.db.WithContext(ctx).
		Model(&wikiPageRecord{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]any{
			"sort_order": sortOrder,
			"updated_at": time.Now().UTC(),
		})
	if result.Error != nil {
		return fmt.Errorf("update wiki page sort order: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func wikiPageModels(records []wikiPageRecord) []*model.WikiPage {
	pages := make([]*model.WikiPage, 0, len(records))
	for i := range records {
		pages = append(pages, records[i].toModel())
	}
	return pages
}
