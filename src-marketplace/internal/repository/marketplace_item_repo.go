package repository

import (
	"context"

	"github.com/agentforge/marketplace/internal/model"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

// MarketplaceItemRepository handles persistence for marketplace items and versions.
type MarketplaceItemRepository struct {
	db *gorm.DB
}

func NewMarketplaceItemRepository(db *gorm.DB) *MarketplaceItemRepository {
	return &MarketplaceItemRepository{db: db}
}

// --------------------------------------------------------------------------
// Item methods
// --------------------------------------------------------------------------

func (r *MarketplaceItemRepository) Create(ctx context.Context, item *model.MarketplaceItem) error {
	return normalizeRepositoryError(r.db.WithContext(ctx).Create(item).Error)
}

func (r *MarketplaceItemRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.MarketplaceItem, error) {
	var item model.MarketplaceItem
	err := r.db.WithContext(ctx).
		Where("id = ? AND is_deleted = FALSE", id).
		First(&item).Error
	if err != nil {
		return nil, normalizeRepositoryError(err)
	}
	return &item, nil
}

func (r *MarketplaceItemRepository) GetBySlugAndType(ctx context.Context, slug, itemType string) (*model.MarketplaceItem, error) {
	var item model.MarketplaceItem
	err := r.db.WithContext(ctx).
		Where("slug = ? AND type = ? AND is_deleted = FALSE", slug, itemType).
		First(&item).Error
	if err != nil {
		return nil, normalizeRepositoryError(err)
	}
	return &item, nil
}

func (r *MarketplaceItemRepository) List(ctx context.Context, q model.ListItemsQuery) ([]*model.MarketplaceItem, int64, error) {
	// Resolve pagination values.
	page := q.Page
	if page < 1 {
		page = 1
	}
	pageSize := q.PageSize
	if pageSize < 1 {
		pageSize = model.DefaultPageSize
	}
	if pageSize > model.MaxPageSize {
		pageSize = model.MaxPageSize
	}
	offset := (page - 1) * pageSize

	tx := r.db.WithContext(ctx).Model(&model.MarketplaceItem{}).Where("is_deleted = FALSE")

	if q.Type != "" {
		tx = tx.Where("type = ?", q.Type)
	}
	if q.Category != "" {
		tx = tx.Where("category = ?", q.Category)
	}
	if len(q.Tags) > 0 {
		tx = tx.Where("tags @> ?", pq.Array(q.Tags))
	}

	// Count before applying ordering/limit.
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, normalizeRepositoryError(err)
	}

	switch q.Sort {
	case "downloads":
		tx = tx.Order("download_count DESC")
	case "rating":
		tx = tx.Order("avg_rating DESC")
	default:
		tx = tx.Order("created_at DESC")
	}

	var items []*model.MarketplaceItem
	if err := tx.Limit(pageSize).Offset(offset).Find(&items).Error; err != nil {
		return nil, 0, normalizeRepositoryError(err)
	}
	return items, total, nil
}

func (r *MarketplaceItemRepository) ListFeatured(ctx context.Context) ([]*model.MarketplaceItem, error) {
	var items []*model.MarketplaceItem
	err := r.db.WithContext(ctx).
		Where("is_featured = TRUE AND is_deleted = FALSE").
		Order("created_at DESC").
		Find(&items).Error
	return items, normalizeRepositoryError(err)
}

func (r *MarketplaceItemRepository) Search(ctx context.Context, q string) ([]*model.MarketplaceItem, error) {
	pattern := "%" + q + "%"
	var items []*model.MarketplaceItem
	err := r.db.WithContext(ctx).
		Where("(name ILIKE ? OR description ILIKE ?) AND is_deleted = FALSE", pattern, pattern).
		Limit(50).
		Find(&items).Error
	return items, normalizeRepositoryError(err)
}

func (r *MarketplaceItemRepository) Update(ctx context.Context, id uuid.UUID, req model.UpdateItemRequest) error {
	updates := map[string]interface{}{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Category != nil {
		updates["category"] = *req.Category
	}
	if req.Tags != nil {
		updates["tags"] = model.StringArray(req.Tags)
	}
	if req.IconURL != nil {
		updates["icon_url"] = req.IconURL
	}
	if req.RepositoryURL != nil {
		updates["repository_url"] = req.RepositoryURL
	}
	if req.License != nil {
		updates["license"] = *req.License
	}
	if req.ExtraMetadata != nil {
		updates["extra_metadata"] = req.ExtraMetadata
	}
	if len(updates) == 0 {
		return nil
	}
	res := r.db.WithContext(ctx).
		Model(&model.MarketplaceItem{}).
		Where("id = ? AND is_deleted = FALSE", id).
		Updates(updates)
	if res.Error != nil {
		return normalizeRepositoryError(res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *MarketplaceItemRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Model(&model.MarketplaceItem{}).
		Where("id = ?", id).
		Update("is_deleted", true)
	if res.Error != nil {
		return normalizeRepositoryError(res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *MarketplaceItemRepository) IncrementDownloadCount(ctx context.Context, id uuid.UUID) error {
	return normalizeRepositoryError(
		r.db.WithContext(ctx).
			Model(&model.MarketplaceItem{}).
			Where("id = ?", id).
			UpdateColumn("download_count", gorm.Expr("download_count + 1")).Error,
	)
}

func (r *MarketplaceItemRepository) UpdateRatingStats(ctx context.Context, id uuid.UUID, avg float64, count int) error {
	return normalizeRepositoryError(
		r.db.WithContext(ctx).
			Model(&model.MarketplaceItem{}).
			Where("id = ?", id).
			Updates(map[string]interface{}{
				"avg_rating":   avg,
				"rating_count": count,
			}).Error,
	)
}

func (r *MarketplaceItemRepository) SetVerified(ctx context.Context, id uuid.UUID, v bool) error {
	return normalizeRepositoryError(
		r.db.WithContext(ctx).
			Model(&model.MarketplaceItem{}).
			Where("id = ?", id).
			Update("is_verified", v).Error,
	)
}

func (r *MarketplaceItemRepository) SetFeatured(ctx context.Context, id uuid.UUID, v bool) error {
	return normalizeRepositoryError(
		r.db.WithContext(ctx).
			Model(&model.MarketplaceItem{}).
			Where("id = ?", id).
			Update("is_featured", v).Error,
	)
}

// --------------------------------------------------------------------------
// Version methods
// --------------------------------------------------------------------------

func (r *MarketplaceItemRepository) CreateVersion(ctx context.Context, v *model.MarketplaceItemVersion) error {
	return normalizeRepositoryError(r.db.WithContext(ctx).Create(v).Error)
}

func (r *MarketplaceItemRepository) ListVersions(ctx context.Context, itemID uuid.UUID) ([]*model.MarketplaceItemVersion, error) {
	var versions []*model.MarketplaceItemVersion
	err := r.db.WithContext(ctx).
		Where("item_id = ?", itemID).
		Order("created_at DESC").
		Find(&versions).Error
	return versions, normalizeRepositoryError(err)
}

func (r *MarketplaceItemRepository) GetVersion(ctx context.Context, itemID uuid.UUID, version string) (*model.MarketplaceItemVersion, error) {
	var v model.MarketplaceItemVersion
	err := r.db.WithContext(ctx).
		Where("item_id = ? AND version = ?", itemID, version).
		First(&v).Error
	if err != nil {
		return nil, normalizeRepositoryError(err)
	}
	return &v, nil
}

func (r *MarketplaceItemRepository) YankVersion(ctx context.Context, itemID uuid.UUID, version string) error {
	res := r.db.WithContext(ctx).
		Model(&model.MarketplaceItemVersion{}).
		Where("item_id = ? AND version = ?", itemID, version).
		Update("is_yanked", true)
	if res.Error != nil {
		return normalizeRepositoryError(res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *MarketplaceItemRepository) SetLatestVersion(ctx context.Context, itemID uuid.UUID, version string) error {
	// Unset current latest.
	if err := r.db.WithContext(ctx).
		Model(&model.MarketplaceItemVersion{}).
		Where("item_id = ? AND is_latest = TRUE", itemID).
		Update("is_latest", false).Error; err != nil {
		return normalizeRepositoryError(err)
	}
	// Set the new latest.
	res := r.db.WithContext(ctx).
		Model(&model.MarketplaceItemVersion{}).
		Where("item_id = ? AND version = ?", itemID, version).
		Update("is_latest", true)
	if res.Error != nil {
		return normalizeRepositoryError(res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateLatestVersion sets the latest_version string on the item record.
func (r *MarketplaceItemRepository) UpdateLatestVersion(ctx context.Context, id uuid.UUID, version string) error {
	return normalizeRepositoryError(
		r.db.WithContext(ctx).
			Model(&model.MarketplaceItem{}).
			Where("id = ?", id).
			Update("latest_version", version).Error,
	)
}
