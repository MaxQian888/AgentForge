package knowledge

import (
	"context"
	"fmt"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type pgAssetIngestChunkRepository struct {
	db *gorm.DB
}

func NewPgAssetIngestChunkRepository(db *gorm.DB) AssetIngestChunkRepository {
	return &pgAssetIngestChunkRepository{db: db}
}

func (r *pgAssetIngestChunkRepository) BulkCreate(ctx context.Context, chunks []*model.AssetIngestChunk) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if len(chunks) == 0 {
		return nil
	}
	records := make([]assetIngestChunkRecord, 0, len(chunks))
	for _, c := range chunks {
		records = append(records, *newAssetIngestChunkRecord(c))
	}
	if err := r.db.WithContext(ctx).Create(&records).Error; err != nil {
		return fmt.Errorf("asset_ingest_chunk bulk create: %w", err)
	}
	return nil
}

func (r *pgAssetIngestChunkRepository) ListByAssetID(ctx context.Context, assetID uuid.UUID) ([]*model.AssetIngestChunk, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var records []assetIngestChunkRecord
	if err := r.db.WithContext(ctx).
		Where("asset_id = ?", assetID).
		Order("chunk_index ASC").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("asset_ingest_chunk list: %w", err)
	}
	out := make([]*model.AssetIngestChunk, 0, len(records))
	for i := range records {
		out = append(out, records[i].toModel())
	}
	return out, nil
}

func (r *pgAssetIngestChunkRepository) DeleteByAssetID(ctx context.Context, assetID uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).
		Where("asset_id = ?", assetID).
		Delete(&assetIngestChunkRecord{}).Error; err != nil {
		return fmt.Errorf("asset_ingest_chunk delete: %w", err)
	}
	return nil
}
