package knowledge

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"gorm.io/gorm"
)

type pgAssetCommentRepository struct {
	db *gorm.DB
}

func NewPgAssetCommentRepository(db *gorm.DB) AssetCommentRepository {
	return &pgAssetCommentRepository{db: db}
}

func (r *pgAssetCommentRepository) Create(ctx context.Context, c *model.AssetComment) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).Create(newAssetCommentRecord(c)).Error; err != nil {
		return fmt.Errorf("asset_comment create: %w", err)
	}
	return nil
}

func (r *pgAssetCommentRepository) ListByAssetID(ctx context.Context, assetID uuid.UUID) ([]*model.AssetComment, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var records []assetCommentRecord
	if err := r.db.WithContext(ctx).
		Where("asset_id = ? AND deleted_at IS NULL", assetID).
		Order("created_at ASC").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("asset_comment list: %w", err)
	}
	out := make([]*model.AssetComment, 0, len(records))
	for i := range records {
		out = append(out, records[i].toModel())
	}
	return out, nil
}

func (r *pgAssetCommentRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.AssetComment, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var rec assetCommentRecord
	if err := r.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", id).
		Take(&rec).Error; err != nil {
		return nil, normalizeCommentErr(err)
	}
	return rec.toModel(), nil
}

func (r *pgAssetCommentRepository) Update(ctx context.Context, c *model.AssetComment) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	updates := map[string]any{
		"body":        c.Body,
		"resolved_at": c.ResolvedAt,
		"updated_at":  c.UpdatedAt,
	}
	result := r.db.WithContext(ctx).
		Model(&assetCommentRecord{}).
		Where("id = ? AND deleted_at IS NULL", c.ID).
		Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("asset_comment update: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrCommentNotFound
	}
	return nil
}

func (r *pgAssetCommentRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	now := time.Now().UTC()
	result := r.db.WithContext(ctx).
		Model(&assetCommentRecord{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]any{"deleted_at": now, "updated_at": now})
	if result.Error != nil {
		return fmt.Errorf("asset_comment soft delete: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrCommentNotFound
	}
	return nil
}

func normalizeCommentErr(err error) error {
	if err == nil {
		return nil
	}
	if err == gorm.ErrRecordNotFound {
		return ErrCommentNotFound
	}
	return err
}
