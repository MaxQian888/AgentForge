package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"gorm.io/gorm"
)

type workflowRunMappingRecord struct {
	ID          uuid.UUID `gorm:"column:id;primaryKey"`
	ExecutionID uuid.UUID `gorm:"column:execution_id"`
	NodeID      string    `gorm:"column:node_id"`
	AgentRunID  uuid.UUID `gorm:"column:agent_run_id"`
	CreatedAt   time.Time `gorm:"column:created_at"`
}

func (workflowRunMappingRecord) TableName() string { return "workflow_run_mappings" }

func (r *workflowRunMappingRecord) toModel() *model.WorkflowRunMapping {
	if r == nil {
		return nil
	}
	return &model.WorkflowRunMapping{
		ID:          r.ID,
		ExecutionID: r.ExecutionID,
		NodeID:      r.NodeID,
		AgentRunID:  r.AgentRunID,
		CreatedAt:   r.CreatedAt,
	}
}

// WorkflowRunMappingRepository manages workflow-to-agent-run mappings.
type WorkflowRunMappingRepository struct {
	db *gorm.DB
}

func NewWorkflowRunMappingRepository(db *gorm.DB) *WorkflowRunMappingRepository {
	return &WorkflowRunMappingRepository{db: db}
}

func (r *WorkflowRunMappingRepository) Create(ctx context.Context, mapping *model.WorkflowRunMapping) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	record := &workflowRunMappingRecord{
		ID:          mapping.ID,
		ExecutionID: mapping.ExecutionID,
		NodeID:      mapping.NodeID,
		AgentRunID:  mapping.AgentRunID,
	}
	if err := r.db.WithContext(ctx).Create(record).Error; err != nil {
		return fmt.Errorf("create workflow run mapping: %w", err)
	}
	return nil
}

func (r *WorkflowRunMappingRepository) GetByAgentRunID(ctx context.Context, agentRunID uuid.UUID) (*model.WorkflowRunMapping, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var record workflowRunMappingRecord
	if err := r.db.WithContext(ctx).Where("agent_run_id = ?", agentRunID).Take(&record).Error; err != nil {
		return nil, fmt.Errorf("get workflow run mapping: %w", normalizeRepositoryError(err))
	}
	return record.toModel(), nil
}

func (r *WorkflowRunMappingRepository) ListByExecution(ctx context.Context, executionID uuid.UUID) ([]*model.WorkflowRunMapping, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var records []workflowRunMappingRecord
	if err := r.db.WithContext(ctx).Where("execution_id = ?", executionID).Order("created_at ASC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list workflow run mappings: %w", err)
	}
	result := make([]*model.WorkflowRunMapping, len(records))
	for i := range records {
		result[i] = records[i].toModel()
	}
	return result, nil
}
