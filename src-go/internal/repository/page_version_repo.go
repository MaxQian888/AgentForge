package repository

import (
	"context"
	"fmt"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PageVersionRepository struct {
	db *gorm.DB
}

func NewPageVersionRepository(db *gorm.DB) *PageVersionRepository {
	return &PageVersionRepository{db: db}
}

func (r *PageVersionRepository) Create(ctx context.Context, version *model.PageVersion) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).Create(newPageVersionRecord(version)).Error; err != nil {
		return fmt.Errorf("create page version: %w", err)
	}
	return nil
}

func (r *PageVersionRepository) ListByPageID(ctx context.Context, pageID uuid.UUID) ([]*model.PageVersion, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}

	var records []pageVersionRecord
	if err := r.db.WithContext(ctx).
		Where("page_id = ?", pageID).
		Order("version_number DESC").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list page versions: %w", err)
	}

	versions := make([]*model.PageVersion, 0, len(records))
	for i := range records {
		versions = append(versions, records[i].toModel())
	}
	return versions, nil
}

func (r *PageVersionRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.PageVersion, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}

	var record pageVersionRecord
	if err := r.db.WithContext(ctx).Where("id = ?", id).Take(&record).Error; err != nil {
		return nil, normalizeRepositoryError(err)
	}
	return record.toModel(), nil
}
