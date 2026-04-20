package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type WorkflowRepository struct {
	db *gorm.DB
}

func NewWorkflowRepository(db *gorm.DB) *WorkflowRepository {
	return &WorkflowRepository{db: db}
}

func (r *WorkflowRepository) GetByProject(ctx context.Context, projectID uuid.UUID) (*model.WorkflowConfig, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}

	var record workflowConfigRecord
	if err := r.db.WithContext(ctx).Where("project_id = ?", projectID).Take(&record).Error; err != nil {
		return nil, fmt.Errorf("get workflow config: %w", normalizeRepositoryError(err))
	}
	return record.toModel(), nil
}

func (r *WorkflowRepository) Upsert(ctx context.Context, projectID uuid.UUID, transitions json.RawMessage, triggers json.RawMessage) (*model.WorkflowConfig, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}

	record := newWorkflowConfigRecord(projectID, transitions, triggers)
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "project_id"}},
			DoUpdates: clause.Assignments(map[string]any{
				"transitions": record.Transitions,
				"triggers":    record.Triggers,
				"updated_at":  gorm.Expr("NOW()"),
			}),
		}).
		Create(record).Error; err != nil {
		return nil, fmt.Errorf("upsert workflow config: %w", err)
	}
	return r.GetByProject(ctx, projectID)
}
