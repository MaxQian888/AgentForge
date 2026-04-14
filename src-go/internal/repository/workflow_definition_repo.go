package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"gorm.io/gorm"
)

// --- Persistence records ---

type workflowDefinitionRecord struct {
	ID          uuid.UUID `gorm:"column:id;primaryKey"`
	ProjectID   uuid.UUID `gorm:"column:project_id"`
	Name        string    `gorm:"column:name"`
	Description string    `gorm:"column:description"`
	Status      string    `gorm:"column:status"`
	Nodes       rawJSON   `gorm:"column:nodes;type:jsonb"`
	Edges       rawJSON   `gorm:"column:edges;type:jsonb"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

func (workflowDefinitionRecord) TableName() string { return "workflow_definitions" }

func (r *workflowDefinitionRecord) toModel() *model.WorkflowDefinition {
	if r == nil {
		return nil
	}
	return &model.WorkflowDefinition{
		ID:          r.ID,
		ProjectID:   r.ProjectID,
		Name:        r.Name,
		Description: r.Description,
		Status:      r.Status,
		Nodes:       r.Nodes.Bytes("[]"),
		Edges:       r.Edges.Bytes("[]"),
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

type workflowExecutionRecord struct {
	ID           uuid.UUID `gorm:"column:id;primaryKey"`
	WorkflowID   uuid.UUID `gorm:"column:workflow_id"`
	ProjectID    uuid.UUID `gorm:"column:project_id"`
	TaskID       *uuid.UUID `gorm:"column:task_id"`
	Status       string    `gorm:"column:status"`
	CurrentNodes rawJSON   `gorm:"column:current_nodes;type:jsonb"`
	Context      rawJSON   `gorm:"column:context;type:jsonb"`
	ErrorMessage string    `gorm:"column:error_message"`
	StartedAt    *time.Time `gorm:"column:started_at"`
	CompletedAt  *time.Time `gorm:"column:completed_at"`
	CreatedAt    time.Time  `gorm:"column:created_at"`
	UpdatedAt    time.Time  `gorm:"column:updated_at"`
}

func (workflowExecutionRecord) TableName() string { return "workflow_executions" }

func (r *workflowExecutionRecord) toModel() *model.WorkflowExecution {
	if r == nil {
		return nil
	}
	return &model.WorkflowExecution{
		ID:           r.ID,
		WorkflowID:   r.WorkflowID,
		ProjectID:    r.ProjectID,
		TaskID:       r.TaskID,
		Status:       r.Status,
		CurrentNodes: r.CurrentNodes.Bytes("[]"),
		Context:      r.Context.Bytes("{}"),
		ErrorMessage: r.ErrorMessage,
		StartedAt:    r.StartedAt,
		CompletedAt:  r.CompletedAt,
		CreatedAt:    r.CreatedAt,
		UpdatedAt:    r.UpdatedAt,
	}
}

type workflowNodeExecutionRecord struct {
	ID           uuid.UUID       `gorm:"column:id;primaryKey"`
	ExecutionID  uuid.UUID       `gorm:"column:execution_id"`
	NodeID       string          `gorm:"column:node_id"`
	Status       string          `gorm:"column:status"`
	Result       rawJSON         `gorm:"column:result;type:jsonb"`
	ErrorMessage string          `gorm:"column:error_message"`
	StartedAt    *time.Time      `gorm:"column:started_at"`
	CompletedAt  *time.Time      `gorm:"column:completed_at"`
	CreatedAt    time.Time       `gorm:"column:created_at"`
}

func (workflowNodeExecutionRecord) TableName() string { return "workflow_node_executions" }

func (r *workflowNodeExecutionRecord) toModel() *model.WorkflowNodeExecution {
	if r == nil {
		return nil
	}
	return &model.WorkflowNodeExecution{
		ID:           r.ID,
		ExecutionID:  r.ExecutionID,
		NodeID:       r.NodeID,
		Status:       r.Status,
		Result:       r.Result.Bytes("null"),
		ErrorMessage: r.ErrorMessage,
		StartedAt:    r.StartedAt,
		CompletedAt:  r.CompletedAt,
		CreatedAt:    r.CreatedAt,
	}
}

// --- WorkflowDefinitionRepository ---

type WorkflowDefinitionRepository struct {
	db *gorm.DB
}

func NewWorkflowDefinitionRepository(db *gorm.DB) *WorkflowDefinitionRepository {
	return &WorkflowDefinitionRepository{db: db}
}

func (r *WorkflowDefinitionRepository) Create(ctx context.Context, def *model.WorkflowDefinition) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	record := &workflowDefinitionRecord{
		ID:          def.ID,
		ProjectID:   def.ProjectID,
		Name:        def.Name,
		Description: def.Description,
		Status:      def.Status,
		Nodes:       newRawJSON(def.Nodes, "[]"),
		Edges:       newRawJSON(def.Edges, "[]"),
	}
	if err := r.db.WithContext(ctx).Create(record).Error; err != nil {
		return fmt.Errorf("create workflow definition: %w", err)
	}
	return nil
}

func (r *WorkflowDefinitionRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.WorkflowDefinition, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var record workflowDefinitionRecord
	if err := r.db.WithContext(ctx).Where("id = ?", id).Take(&record).Error; err != nil {
		return nil, fmt.Errorf("get workflow definition: %w", normalizeRepositoryError(err))
	}
	return record.toModel(), nil
}

func (r *WorkflowDefinitionRepository) ListByProject(ctx context.Context, projectID uuid.UUID) ([]*model.WorkflowDefinition, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var records []workflowDefinitionRecord
	if err := r.db.WithContext(ctx).Where("project_id = ?", projectID).Order("created_at DESC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list workflow definitions: %w", err)
	}
	result := make([]*model.WorkflowDefinition, len(records))
	for i := range records {
		result[i] = records[i].toModel()
	}
	return result, nil
}

func (r *WorkflowDefinitionRepository) Update(ctx context.Context, id uuid.UUID, def *model.WorkflowDefinition) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	updates := map[string]any{
		"updated_at": gorm.Expr("NOW()"),
	}
	if def.Name != "" {
		updates["name"] = def.Name
	}
	if def.Description != "" {
		updates["description"] = def.Description
	}
	if def.Status != "" {
		updates["status"] = def.Status
	}
	if len(def.Nodes) > 0 {
		updates["nodes"] = newRawJSON(def.Nodes, "[]")
	}
	if len(def.Edges) > 0 {
		updates["edges"] = newRawJSON(def.Edges, "[]")
	}
	result := r.db.WithContext(ctx).Model(&workflowDefinitionRecord{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("update workflow definition: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *WorkflowDefinitionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	result := r.db.WithContext(ctx).Where("id = ?", id).Delete(&workflowDefinitionRecord{})
	if result.Error != nil {
		return fmt.Errorf("delete workflow definition: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// --- WorkflowExecutionRepository ---

type WorkflowExecutionRepository struct {
	db *gorm.DB
}

func NewWorkflowExecutionRepository(db *gorm.DB) *WorkflowExecutionRepository {
	return &WorkflowExecutionRepository{db: db}
}

func (r *WorkflowExecutionRepository) CreateExecution(ctx context.Context, exec *model.WorkflowExecution) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	record := &workflowExecutionRecord{
		ID:           exec.ID,
		WorkflowID:   exec.WorkflowID,
		ProjectID:    exec.ProjectID,
		TaskID:       exec.TaskID,
		Status:       exec.Status,
		CurrentNodes: newRawJSON(exec.CurrentNodes, "[]"),
		Context:      newRawJSON(exec.Context, "{}"),
		ErrorMessage: exec.ErrorMessage,
		StartedAt:    exec.StartedAt,
		CompletedAt:  exec.CompletedAt,
	}
	if err := r.db.WithContext(ctx).Create(record).Error; err != nil {
		return fmt.Errorf("create workflow execution: %w", err)
	}
	return nil
}

func (r *WorkflowExecutionRepository) GetExecution(ctx context.Context, id uuid.UUID) (*model.WorkflowExecution, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var record workflowExecutionRecord
	if err := r.db.WithContext(ctx).Where("id = ?", id).Take(&record).Error; err != nil {
		return nil, fmt.Errorf("get workflow execution: %w", normalizeRepositoryError(err))
	}
	return record.toModel(), nil
}

func (r *WorkflowExecutionRepository) ListExecutions(ctx context.Context, workflowID uuid.UUID) ([]*model.WorkflowExecution, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var records []workflowExecutionRecord
	if err := r.db.WithContext(ctx).Where("workflow_id = ?", workflowID).Order("created_at DESC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list workflow executions: %w", err)
	}
	result := make([]*model.WorkflowExecution, len(records))
	for i := range records {
		result[i] = records[i].toModel()
	}
	return result, nil
}

func (r *WorkflowExecutionRepository) UpdateExecution(ctx context.Context, id uuid.UUID, status string, currentNodes json.RawMessage, errorMessage string) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	updates := map[string]any{
		"status":        status,
		"current_nodes": newRawJSON(currentNodes, "[]"),
		"error_message": errorMessage,
		"updated_at":    gorm.Expr("NOW()"),
	}
	result := r.db.WithContext(ctx).Model(&workflowExecutionRecord{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("update workflow execution: %w", result.Error)
	}
	return nil
}

func (r *WorkflowExecutionRepository) CompleteExecution(ctx context.Context, id uuid.UUID, status string) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	now := time.Now().UTC()
	updates := map[string]any{
		"status":        status,
		"current_nodes": newRawJSON(nil, "[]"),
		"completed_at":  now,
		"updated_at":    gorm.Expr("NOW()"),
	}
	result := r.db.WithContext(ctx).Model(&workflowExecutionRecord{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("complete workflow execution: %w", result.Error)
	}
	return nil
}

// --- WorkflowNodeExecutionRepository ---

type WorkflowNodeExecutionRepository struct {
	db *gorm.DB
}

func NewWorkflowNodeExecutionRepository(db *gorm.DB) *WorkflowNodeExecutionRepository {
	return &WorkflowNodeExecutionRepository{db: db}
}

func (r *WorkflowNodeExecutionRepository) CreateNodeExecution(ctx context.Context, nodeExec *model.WorkflowNodeExecution) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	record := &workflowNodeExecutionRecord{
		ID:           nodeExec.ID,
		ExecutionID:  nodeExec.ExecutionID,
		NodeID:       nodeExec.NodeID,
		Status:       nodeExec.Status,
		Result:       newRawJSON(nodeExec.Result, "null"),
		ErrorMessage: nodeExec.ErrorMessage,
		StartedAt:    nodeExec.StartedAt,
		CompletedAt:  nodeExec.CompletedAt,
	}
	if err := r.db.WithContext(ctx).Create(record).Error; err != nil {
		return fmt.Errorf("create workflow node execution: %w", err)
	}
	return nil
}

func (r *WorkflowNodeExecutionRepository) UpdateNodeExecution(ctx context.Context, id uuid.UUID, status string, result json.RawMessage, errorMessage string) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	updates := map[string]any{
		"status":        status,
		"error_message": errorMessage,
	}
	if len(result) > 0 {
		updates["result"] = newRawJSON(result, "null")
	}
	if status == model.NodeExecCompleted || status == model.NodeExecFailed || status == model.NodeExecSkipped {
		now := time.Now().UTC()
		updates["completed_at"] = now
	}
	if status == model.NodeExecRunning {
		now := time.Now().UTC()
		updates["started_at"] = now
	}
	dbResult := r.db.WithContext(ctx).Model(&workflowNodeExecutionRecord{}).Where("id = ?", id).Updates(updates)
	if dbResult.Error != nil {
		return fmt.Errorf("update workflow node execution: %w", dbResult.Error)
	}
	return nil
}

func (r *WorkflowNodeExecutionRepository) ListNodeExecutions(ctx context.Context, executionID uuid.UUID) ([]*model.WorkflowNodeExecution, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var records []workflowNodeExecutionRecord
	if err := r.db.WithContext(ctx).Where("execution_id = ?", executionID).Order("created_at ASC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list workflow node executions: %w", err)
	}
	result := make([]*model.WorkflowNodeExecution, len(records))
	for i := range records {
		result[i] = records[i].toModel()
	}
	return result, nil
}
