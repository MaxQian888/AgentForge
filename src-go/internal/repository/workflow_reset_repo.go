package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type WorkflowResetRepository struct {
	db *gorm.DB
}

func NewWorkflowResetRepository(db *gorm.DB) *WorkflowResetRepository {
	return &WorkflowResetRepository{db: db}
}

func (r *WorkflowResetRepository) ResetNodesAndUpdateCounter(ctx context.Context, executionID uuid.UUID, nodeIDs []string, counterKey string, counterValue float64) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if len(nodeIDs) > 0 {
			if err := tx.Where("execution_id = ? AND node_id IN ?", executionID, nodeIDs).Delete(&workflowNodeExecutionRecord{}).Error; err != nil {
				return fmt.Errorf("delete workflow node executions: %w", err)
			}
		}
		if counterKey == "" {
			return nil
		}

		var current workflowExecutionRecord
		if err := tx.Where("id = ?", executionID).Take(&current).Error; err != nil {
			return fmt.Errorf("get execution for counter update: %w", normalizeRepositoryError(err))
		}

		ds := make(map[string]any)
		if len(current.DataStore) > 0 {
			if err := json.Unmarshal(current.DataStore, &ds); err != nil {
				return fmt.Errorf("unmarshal datastore: %w", err)
			}
		}
		ds[counterKey] = counterValue

		updated, err := json.Marshal(ds)
		if err != nil {
			return fmt.Errorf("marshal datastore: %w", err)
		}
		if err := tx.Model(&workflowExecutionRecord{}).Where("id = ?", executionID).Updates(map[string]any{
			"data_store": newRawJSON(updated, "{}"),
			"updated_at": gorm.Expr("NOW()"),
		}).Error; err != nil {
			return fmt.Errorf("update datastore: %w", err)
		}
		return nil
	})
}
