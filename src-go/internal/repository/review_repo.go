package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"gorm.io/gorm"
)

type ReviewRepository struct {
	db *gorm.DB
}

func NewReviewRepository(db *gorm.DB) *ReviewRepository {
	return &ReviewRepository{db: db}
}

const reviewColumns = `id, task_id, pr_url, pr_number, layer, status, risk_level, findings, execution_metadata, summary, recommendation, cost_usd, created_at, updated_at`

func marshalFindings(findings []model.ReviewFinding) ([]byte, error) {
	if len(findings) == 0 {
		return []byte("[]"), nil
	}
	data, err := json.Marshal(findings)
	if err != nil {
		return nil, fmt.Errorf("marshal findings: %w", err)
	}
	return data, nil
}

func marshalReviewExecutionMetadata(metadata *model.ReviewExecutionMetadata) ([]byte, error) {
	if metadata == nil {
		return []byte("{}"), nil
	}
	data, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal execution metadata: %w", err)
	}
	return data, nil
}

func scanReview(row interface{ Scan(dest ...any) error }) (*model.Review, error) {
	review := &model.Review{}
	var findingsRaw []byte
	var executionMetadataRaw []byte
	err := row.Scan(
		&review.ID,
		&review.TaskID,
		&review.PRURL,
		&review.PRNumber,
		&review.Layer,
		&review.Status,
		&review.RiskLevel,
		&findingsRaw,
		&executionMetadataRaw,
		&review.Summary,
		&review.Recommendation,
		&review.CostUSD,
		&review.CreatedAt,
		&review.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if len(findingsRaw) > 0 {
		if err := json.Unmarshal(findingsRaw, &review.Findings); err != nil {
			return nil, fmt.Errorf("unmarshal findings: %w", err)
		}
	}
	if len(executionMetadataRaw) > 0 && string(executionMetadataRaw) != "null" && string(executionMetadataRaw) != "{}" {
		var metadata model.ReviewExecutionMetadata
		if err := json.Unmarshal(executionMetadataRaw, &metadata); err != nil {
			return nil, fmt.Errorf("unmarshal execution metadata: %w", err)
		}
		review.ExecutionMetadata = &metadata
	}
	return review, nil
}

func (r *ReviewRepository) Create(ctx context.Context, review *model.Review) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}

	findingsJSON, err := marshalFindings(review.Findings)
	if err != nil {
		return err
	}
	executionMetadataJSON, err := marshalReviewExecutionMetadata(review.ExecutionMetadata)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO reviews (id, task_id, pr_url, pr_number, layer, status, risk_level, findings, execution_metadata, summary, recommendation, cost_usd, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW(), NOW())
	`
	if err := r.db.WithContext(ctx).Exec(
		query,
		review.ID,
		review.TaskID,
		review.PRURL,
		review.PRNumber,
		review.Layer,
		review.Status,
		review.RiskLevel,
		findingsJSON,
		executionMetadataJSON,
		review.Summary,
		review.Recommendation,
		review.CostUSD,
	).Error; err != nil {
		return fmt.Errorf("create review: %w", err)
	}
	return nil
}

func (r *ReviewRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Review, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	query := `SELECT ` + reviewColumns + ` FROM reviews WHERE id = $1`
	review, err := scanReview(r.db.WithContext(ctx).Raw(query, id).Row())
	if err != nil {
		return nil, fmt.Errorf("get review by id: %w", normalizeRepositoryError(err))
	}
	return review, nil
}

func (r *ReviewRepository) GetByTask(ctx context.Context, taskID uuid.UUID) ([]*model.Review, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	query := `SELECT ` + reviewColumns + ` FROM reviews WHERE task_id = $1 ORDER BY created_at DESC`
	rows, err := r.db.WithContext(ctx).Raw(query, taskID).Rows()
	if err != nil {
		return nil, fmt.Errorf("get reviews by task: %w", err)
	}
	defer rows.Close()

	var reviews []*model.Review
	for rows.Next() {
		review, err := scanReview(rows)
		if err != nil {
			return nil, fmt.Errorf("scan review: %w", err)
		}
		reviews = append(reviews, review)
	}
	return reviews, rows.Err()
}

func (r *ReviewRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).Exec(`UPDATE reviews SET status = $1, updated_at = NOW() WHERE id = $2`, status, id).Error; err != nil {
		return fmt.Errorf("update review status: %w", err)
	}
	return nil
}

func (r *ReviewRepository) UpdateResult(ctx context.Context, review *model.Review) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}

	findingsJSON, err := marshalFindings(review.Findings)
	if err != nil {
		return err
	}
	executionMetadataJSON, err := marshalReviewExecutionMetadata(review.ExecutionMetadata)
	if err != nil {
		return err
	}

	query := `
		UPDATE reviews
		SET status = $1,
		    risk_level = $2,
		    findings = $3,
		    execution_metadata = $4,
		    summary = $5,
		    recommendation = $6,
		    cost_usd = $7,
		    updated_at = NOW()
		WHERE id = $8
	`
	if err := r.db.WithContext(ctx).Exec(
		query,
		review.Status,
		review.RiskLevel,
		findingsJSON,
		executionMetadataJSON,
		review.Summary,
		review.Recommendation,
		review.CostUSD,
		review.ID,
	).Error; err != nil {
		return fmt.Errorf("update review result: %w", err)
	}
	return nil
}
