package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PageFavoriteRepository struct {
	db *gorm.DB
}

func NewPageFavoriteRepository(db *gorm.DB) *PageFavoriteRepository {
	return &PageFavoriteRepository{db: db}
}

func (r *PageFavoriteRepository) Add(ctx context.Context, pageID, userID uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}

	record := newPageFavoriteRecord(&model.PageFavorite{
		PageID:    pageID,
		UserID:    userID,
		CreatedAt: time.Now().UTC(),
	})
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(record).Error; err != nil {
		return fmt.Errorf("add page favorite: %w", err)
	}
	return nil
}

func (r *PageFavoriteRepository) Remove(ctx context.Context, pageID, userID uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).
		Delete(&pageFavoriteRecord{}, "page_id = ? AND user_id = ?", pageID, userID).
		Error; err != nil {
		return fmt.Errorf("remove page favorite: %w", err)
	}
	return nil
}

func (r *PageFavoriteRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]*model.PageFavorite, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}

	var records []pageFavoriteRecord
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list page favorites: %w", err)
	}

	favorites := make([]*model.PageFavorite, 0, len(records))
	for i := range records {
		favorites = append(favorites, records[i].toModel())
	}
	return favorites, nil
}

func (r *PageFavoriteRepository) ListByPage(ctx context.Context, pageID uuid.UUID) ([]*model.PageFavorite, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}

	var records []pageFavoriteRecord
	if err := r.db.WithContext(ctx).
		Where("page_id = ?", pageID).
		Order("created_at DESC").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list page favorites by page: %w", err)
	}

	favorites := make([]*model.PageFavorite, 0, len(records))
	for i := range records {
		favorites = append(favorites, records[i].toModel())
	}
	return favorites, nil
}

type PageRecentAccessRepository struct {
	db *gorm.DB
}

func NewPageRecentAccessRepository(db *gorm.DB) *PageRecentAccessRepository {
	return &PageRecentAccessRepository{db: db}
}

func (r *PageRecentAccessRepository) Touch(ctx context.Context, pageID, userID uuid.UUID, accessedAt time.Time) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}

	record := newPageRecentAccessRecord(&model.PageRecentAccess{
		PageID:     pageID,
		UserID:     userID,
		AccessedAt: accessedAt,
	})
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "page_id"}, {Name: "user_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"accessed_at"}),
		}).
		Create(record).Error; err != nil {
		return fmt.Errorf("touch page recent access: %w", err)
	}
	return nil
}

func (r *PageRecentAccessRepository) ListByUser(ctx context.Context, userID uuid.UUID, limit int) ([]*model.PageRecentAccess, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	if limit <= 0 {
		limit = 20
	}

	var records []pageRecentAccessRecord
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("accessed_at DESC").
		Limit(limit).
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list page recent access: %w", err)
	}

	accesses := make([]*model.PageRecentAccess, 0, len(records))
	for i := range records {
		accesses = append(accesses, records[i].toModel())
	}
	return accesses, nil
}
