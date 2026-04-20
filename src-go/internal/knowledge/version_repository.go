package knowledge

import (
	"context"
	"fmt"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type pgAssetVersionRepository struct {
	db *gorm.DB
}

func NewPgAssetVersionRepository(db *gorm.DB) AssetVersionRepository {
	return &pgAssetVersionRepository{db: db}
}

func (r *pgAssetVersionRepository) Create(ctx context.Context, v *model.AssetVersion) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).Create(newAssetVersionRecord(v)).Error; err != nil {
		return fmt.Errorf("asset_version create: %w", err)
	}
	return nil
}

func (r *pgAssetVersionRepository) ListByAssetID(ctx context.Context, assetID uuid.UUID) ([]*model.AssetVersion, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var records []assetVersionRecord
	if err := r.db.WithContext(ctx).
		Where("asset_id = ?", assetID).
		Order("version_number DESC").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("asset_version list: %w", err)
	}
	out := make([]*model.AssetVersion, 0, len(records))
	for i := range records {
		out = append(out, records[i].toModel())
	}
	return out, nil
}

func (r *pgAssetVersionRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.AssetVersion, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var rec assetVersionRecord
	if err := r.db.WithContext(ctx).Where("id = ?", id).Take(&rec).Error; err != nil {
		return nil, normalizeErr(err)
	}
	return rec.toModel(), nil
}

func (r *pgAssetVersionRepository) MaxVersionNumber(ctx context.Context, assetID uuid.UUID) (int, error) {
	if r.db == nil {
		return 0, ErrDatabaseUnavailable
	}
	var max int
	row := r.db.WithContext(ctx).
		Model(&assetVersionRecord{}).
		Where("asset_id = ?", assetID).
		Select("COALESCE(MAX(version_number),0)").
		Row()
	if err := row.Scan(&max); err != nil {
		return 0, fmt.Errorf("asset_version max: %w", err)
	}
	return max, nil
}
