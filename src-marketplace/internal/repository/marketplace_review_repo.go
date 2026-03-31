package repository

import (
	"context"

	"github.com/agentforge/marketplace/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// MarketplaceReviewRepository handles persistence for marketplace reviews.
type MarketplaceReviewRepository struct {
	db *gorm.DB
}

func NewMarketplaceReviewRepository(db *gorm.DB) *MarketplaceReviewRepository {
	return &MarketplaceReviewRepository{db: db}
}

// UpsertReview inserts a new review or updates the existing one for (item_id, user_id).
func (r *MarketplaceReviewRepository) UpsertReview(ctx context.Context, rev *model.MarketplaceReview) error {
	sql := `
		INSERT INTO marketplace_reviews (id, item_id, user_id, user_name, rating, comment, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, NOW(), NOW())
		ON CONFLICT (item_id, user_id) DO UPDATE
		SET rating = EXCLUDED.rating,
		    comment = EXCLUDED.comment,
		    updated_at = NOW()
		RETURNING id, created_at, updated_at
	`
	return normalizeRepositoryError(
		r.db.WithContext(ctx).Raw(sql,
			rev.ID, rev.ItemID, rev.UserID, rev.UserName, rev.Rating, rev.Comment,
		).Scan(rev).Error,
	)
}

// GetByItemAndUser retrieves the review for a specific (item, user) pair.
func (r *MarketplaceReviewRepository) GetByItemAndUser(ctx context.Context, itemID, userID uuid.UUID) (*model.MarketplaceReview, error) {
	var rev model.MarketplaceReview
	err := r.db.WithContext(ctx).
		Where("item_id = ? AND user_id = ?", itemID, userID).
		First(&rev).Error
	if err != nil {
		return nil, normalizeRepositoryError(err)
	}
	return &rev, nil
}

// ListByItem returns paginated reviews for a given item, newest first.
func (r *MarketplaceReviewRepository) ListByItem(ctx context.Context, itemID uuid.UUID, limit, offset int) ([]*model.MarketplaceReview, error) {
	var reviews []*model.MarketplaceReview
	err := r.db.WithContext(ctx).
		Where("item_id = ?", itemID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&reviews).Error
	return reviews, normalizeRepositoryError(err)
}

// DeleteByItemAndUser hard-deletes a review.
func (r *MarketplaceReviewRepository) DeleteByItemAndUser(ctx context.Context, itemID, userID uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Where("item_id = ? AND user_id = ?", itemID, userID).
		Delete(&model.MarketplaceReview{})
	if res.Error != nil {
		return normalizeRepositoryError(res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// ComputeRatingStats returns the average rating and total count for an item.
func (r *MarketplaceReviewRepository) ComputeRatingStats(ctx context.Context, itemID uuid.UUID) (avg float64, count int, err error) {
	type result struct {
		Avg   float64
		Count int
	}
	var res result
	dbErr := r.db.WithContext(ctx).
		Raw("SELECT COALESCE(AVG(rating), 0) AS avg, COUNT(*) AS count FROM marketplace_reviews WHERE item_id = ?", itemID).
		Scan(&res).Error
	if dbErr != nil {
		return 0, 0, normalizeRepositoryError(dbErr)
	}
	return res.Avg, res.Count, nil
}
