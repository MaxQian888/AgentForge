package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"gorm.io/gorm"
)

// --- Persistence records ---

type workflowDefinitionRecord struct {
	ID           uuid.UUID  `gorm:"column:id;primaryKey"`
	ProjectID    uuid.UUID  `gorm:"column:project_id"`
	Name         string     `gorm:"column:name"`
	Description  string     `gorm:"column:description"`
	Status       string     `gorm:"column:status"`
	Category     string     `gorm:"column:category"`
	Nodes        rawJSON    `gorm:"column:nodes;type:jsonb"`
	Edges        rawJSON    `gorm:"column:edges;type:jsonb"`
	TemplateVars rawJSON    `gorm:"column:template_vars;type:jsonb"`
	Version      int        `gorm:"column:version"`
	SourceID     *uuid.UUID `gorm:"column:source_id"`
	CreatedAt    time.Time  `gorm:"column:created_at"`
	UpdatedAt    time.Time  `gorm:"column:updated_at"`
}

func (workflowDefinitionRecord) TableName() string { return "workflow_definitions" }

func (r *workflowDefinitionRecord) toModel() *model.WorkflowDefinition {
	if r == nil {
		return nil
	}
	return &model.WorkflowDefinition{
		ID:           r.ID,
		ProjectID:    r.ProjectID,
		Name:         r.Name,
		Description:  r.Description,
		Status:       r.Status,
		Category:     r.Category,
		Nodes:        r.Nodes.Bytes("[]"),
		Edges:        r.Edges.Bytes("[]"),
		TemplateVars: r.TemplateVars.Bytes("{}"),
		Version:      r.Version,
		SourceID:     r.SourceID,
		CreatedAt:    r.CreatedAt,
		UpdatedAt:    r.UpdatedAt,
	}
}

type workflowExecutionRecord struct {
	ID               uuid.UUID  `gorm:"column:id;primaryKey"`
	WorkflowID       uuid.UUID  `gorm:"column:workflow_id"`
	ProjectID        uuid.UUID  `gorm:"column:project_id"`
	TaskID           *uuid.UUID `gorm:"column:task_id"`
	Status           string     `gorm:"column:status"`
	CurrentNodes     rawJSON    `gorm:"column:current_nodes;type:jsonb"`
	Context          rawJSON    `gorm:"column:context;type:jsonb"`
	DataStore        rawJSON    `gorm:"column:data_store;type:jsonb"`
	SystemMetadata   rawJSON    `gorm:"column:system_metadata;type:jsonb"`
	ErrorMessage     string     `gorm:"column:error_message"`
	TriggeredBy      *uuid.UUID `gorm:"column:triggered_by"`
	ActingEmployeeID *uuid.UUID `gorm:"column:acting_employee_id"`
	StartedAt        *time.Time `gorm:"column:started_at"`
	CompletedAt      *time.Time `gorm:"column:completed_at"`
	CreatedAt        time.Time  `gorm:"column:created_at"`
	UpdatedAt        time.Time  `gorm:"column:updated_at"`
}

func (workflowExecutionRecord) TableName() string { return "workflow_executions" }

func (r *workflowExecutionRecord) toModel() *model.WorkflowExecution {
	if r == nil {
		return nil
	}
	return &model.WorkflowExecution{
		ID:               r.ID,
		WorkflowID:       r.WorkflowID,
		ProjectID:        r.ProjectID,
		TaskID:           r.TaskID,
		Status:           r.Status,
		CurrentNodes:     r.CurrentNodes.Bytes("[]"),
		Context:          r.Context.Bytes("{}"),
		DataStore:        r.DataStore.Bytes("{}"),
		SystemMetadata:   r.SystemMetadata.Bytes("{}"),
		ErrorMessage:     r.ErrorMessage,
		TriggeredBy:      r.TriggeredBy,
		ActingEmployeeID: r.ActingEmployeeID,
		StartedAt:        r.StartedAt,
		CompletedAt:      r.CompletedAt,
		CreatedAt:        r.CreatedAt,
		UpdatedAt:        r.UpdatedAt,
	}
}

type workflowNodeExecutionRecord struct {
	ID             uuid.UUID  `gorm:"column:id;primaryKey"`
	ExecutionID    uuid.UUID  `gorm:"column:execution_id"`
	NodeID         string     `gorm:"column:node_id"`
	Status         string     `gorm:"column:status"`
	Result         rawJSON    `gorm:"column:result;type:jsonb"`
	ErrorMessage   string     `gorm:"column:error_message"`
	IterationIndex int        `gorm:"column:iteration_index"`
	StartedAt      *time.Time `gorm:"column:started_at"`
	CompletedAt    *time.Time `gorm:"column:completed_at"`
	CreatedAt      time.Time  `gorm:"column:created_at"`
}

func (workflowNodeExecutionRecord) TableName() string { return "workflow_node_executions" }

func (r *workflowNodeExecutionRecord) toModel() *model.WorkflowNodeExecution {
	if r == nil {
		return nil
	}
	return &model.WorkflowNodeExecution{
		ID:             r.ID,
		ExecutionID:    r.ExecutionID,
		NodeID:         r.NodeID,
		Status:         r.Status,
		Result:         r.Result.Bytes("null"),
		ErrorMessage:   r.ErrorMessage,
		IterationIndex: r.IterationIndex,
		StartedAt:      r.StartedAt,
		CompletedAt:    r.CompletedAt,
		CreatedAt:      r.CreatedAt,
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
		ID:           def.ID,
		ProjectID:    def.ProjectID,
		Name:         def.Name,
		Description:  def.Description,
		Status:       def.Status,
		Category:     def.Category,
		Nodes:        newRawJSON(def.Nodes, "[]"),
		Edges:        newRawJSON(def.Edges, "[]"),
		TemplateVars: newRawJSON(def.TemplateVars, "{}"),
		Version:      def.Version,
		SourceID:     def.SourceID,
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

func (r *WorkflowDefinitionRepository) ListTemplates(ctx context.Context, category string) ([]*model.WorkflowDefinition, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	q := r.db.WithContext(ctx).Where("status = ?", model.WorkflowDefStatusTemplate)
	if category != "" {
		q = q.Where("category = ?", category)
	}
	var records []workflowDefinitionRecord
	if err := q.Order("name ASC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list workflow templates: %w", err)
	}
	result := make([]*model.WorkflowDefinition, len(records))
	for i := range records {
		result[i] = records[i].toModel()
	}
	return result, nil
}

func (r *WorkflowDefinitionRepository) ListTemplatesForProject(ctx context.Context, projectID uuid.UUID, query string, category string, source string) ([]*model.WorkflowDefinition, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	q := r.db.WithContext(ctx).
		Where("status = ?", model.WorkflowDefStatusTemplate).
		Where("(category IN ? OR (category = ? AND project_id = ?))",
			[]string{model.WorkflowCategorySystem, model.WorkflowCategoryMarketplace},
			model.WorkflowCategoryUser,
			projectID,
		)

	filterCategory := source
	if filterCategory == "" {
		filterCategory = category
	}
	if filterCategory != "" {
		q = q.Where("category = ?", filterCategory)
	}
	if query != "" {
		like := "%" + strings.ToLower(query) + "%"
		q = q.Where("(LOWER(name) LIKE ? OR LOWER(description) LIKE ?)", like, like)
	}

	var records []workflowDefinitionRecord
	if err := q.Order("category ASC, name ASC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list project workflow templates: %w", err)
	}
	result := make([]*model.WorkflowDefinition, len(records))
	for i := range records {
		result[i] = records[i].toModel()
	}
	return result, nil
}

func (r *WorkflowDefinitionRepository) ListTemplatesByName(ctx context.Context, name string) ([]*model.WorkflowDefinition, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var records []workflowDefinitionRecord
	if err := r.db.WithContext(ctx).Where("status = ? AND name = ?", model.WorkflowDefStatusTemplate, name).Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list workflow templates by name: %w", err)
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
	if def.Category != "" {
		updates["category"] = def.Category
	}
	if len(def.Nodes) > 0 {
		updates["nodes"] = newRawJSON(def.Nodes, "[]")
	}
	if len(def.Edges) > 0 {
		updates["edges"] = newRawJSON(def.Edges, "[]")
	}
	if len(def.TemplateVars) > 0 {
		updates["template_vars"] = newRawJSON(def.TemplateVars, "{}")
	}
	if def.Version > 0 {
		updates["version"] = def.Version
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
		ID:               exec.ID,
		WorkflowID:       exec.WorkflowID,
		ProjectID:        exec.ProjectID,
		TaskID:           exec.TaskID,
		Status:           exec.Status,
		CurrentNodes:     newRawJSON(exec.CurrentNodes, "[]"),
		Context:          newRawJSON(exec.Context, "{}"),
		DataStore:        newRawJSON(exec.DataStore, "{}"),
		SystemMetadata:   newRawJSON(exec.SystemMetadata, "{}"),
		ErrorMessage:     exec.ErrorMessage,
		TriggeredBy:      exec.TriggeredBy,
		ActingEmployeeID: exec.ActingEmployeeID,
		StartedAt:        exec.StartedAt,
		CompletedAt:      exec.CompletedAt,
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

// ListExecutionsByActingEmployee returns every workflow execution whose
// acting_employee_id matches the given employee, ordered by created_at DESC.
// Used by the attribution read surface so "list runs acting as employee X"
// needs no agent_run JOIN.
func (r *WorkflowExecutionRepository) ListExecutionsByActingEmployee(ctx context.Context, employeeID uuid.UUID) ([]*model.WorkflowExecution, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var records []workflowExecutionRecord
	if err := r.db.WithContext(ctx).
		Where("acting_employee_id = ?", employeeID).
		Order("created_at DESC").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list workflow executions by acting employee: %w", err)
	}
	result := make([]*model.WorkflowExecution, len(records))
	for i := range records {
		result[i] = records[i].toModel()
	}
	return result, nil
}

// WorkflowExecutionListFilter narrows a project-scoped DAG workflow execution
// listing. All fields are optional; zero values mean "no filter on this
// dimension". Used by the unified workflow-run view service so DAG execution
// filtering happens at the SQL layer before the cross-engine merge.
type WorkflowExecutionListFilter struct {
	Statuses         []string
	ActingEmployeeID *uuid.UUID
	TriggerID        *uuid.UUID
	TriggeredByKind  string
	StartedAfter     *time.Time
	StartedBefore    *time.Time
}

// ListByProjectFiltered returns DAG workflow executions for the given project,
// newest-first (started_at DESC, id DESC as tiebreaker), narrowed by the
// supplied filter. A zero or negative limit returns every match; callers pass
// their per-engine fetch budget.
func (r *WorkflowExecutionRepository) ListByProjectFiltered(ctx context.Context, projectID uuid.UUID, filter WorkflowExecutionListFilter, limit int) ([]*model.WorkflowExecution, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	q := r.db.WithContext(ctx).Model(&workflowExecutionRecord{}).Where("project_id = ?", projectID)
	if len(filter.Statuses) > 0 {
		q = q.Where("status IN ?", filter.Statuses)
	}
	if filter.ActingEmployeeID != nil {
		q = q.Where("acting_employee_id = ?", *filter.ActingEmployeeID)
	}
	if filter.TriggerID != nil {
		q = q.Where("triggered_by = ?", *filter.TriggerID)
	}
	if filter.TriggeredByKind != "" {
		switch filter.TriggeredByKind {
		case "trigger":
			q = q.Where("triggered_by IS NOT NULL")
		case "manual":
			q = q.Where("triggered_by IS NULL")
		case "sub_workflow":
			// Sub-workflow children are materialized by the parent-link table;
			// the DAG execution itself doesn't carry a "sub_workflow" flag, so
			// we filter on the link table via a subquery.
			q = q.Where("id IN (?)", r.db.WithContext(ctx).
				Table("workflow_run_parent_link").
				Select("child_run_id").
				Where("child_engine_kind = ?", model.SubWorkflowEngineDAG))
		default:
			q = q.Where("1 = 0")
		}
	}
	if filter.StartedAfter != nil {
		q = q.Where("started_at > ?", *filter.StartedAfter)
	}
	if filter.StartedBefore != nil {
		q = q.Where("started_at < ?", *filter.StartedBefore)
	}
	q = q.Order("started_at DESC NULLS LAST").Order("id DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	var records []workflowExecutionRecord
	if err := q.Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list workflow executions by project filtered: %w", err)
	}
	result := make([]*model.WorkflowExecution, len(records))
	for i := range records {
		result[i] = records[i].toModel()
	}
	return result, nil
}

// ListActiveByProject returns every non-terminal workflow execution for the
// given project. Used by the project lifecycle service to cascade-cancel
// executions on archive.
func (r *WorkflowExecutionRepository) ListActiveByProject(ctx context.Context, projectID uuid.UUID) ([]*model.WorkflowExecution, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	activeStatuses := []string{
		model.WorkflowExecStatusPending,
		model.WorkflowExecStatusRunning,
		model.WorkflowExecStatusPaused,
	}
	var records []workflowExecutionRecord
	if err := r.db.WithContext(ctx).
		Where("project_id = ? AND status IN ?", projectID, activeStatuses).
		Order("created_at DESC").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list active workflow executions by project: %w", err)
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

func (r *WorkflowExecutionRepository) UpdateExecutionDataStore(ctx context.Context, id uuid.UUID, dataStore json.RawMessage) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	updates := map[string]any{
		"data_store": newRawJSON(dataStore, "{}"),
		"updated_at": gorm.Expr("NOW()"),
	}
	result := r.db.WithContext(ctx).Model(&workflowExecutionRecord{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("update workflow execution data_store: %w", result.Error)
	}
	return nil
}

// UpdateExecutionSystemMetadata replaces the system_metadata jsonb document
// for the execution. Callers MUST pass the full document; this is a
// last-write-wins replacement intended for backend-internal flags
// (reply_target, im_dispatched, final_output) that are never written by DAG
// node code (see spec §6.3).
func (r *WorkflowExecutionRepository) UpdateExecutionSystemMetadata(ctx context.Context, id uuid.UUID, systemMetadata json.RawMessage) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	updates := map[string]any{
		"system_metadata": newRawJSON(systemMetadata, "{}"),
		"updated_at":      gorm.Expr("NOW()"),
	}
	result := r.db.WithContext(ctx).Model(&workflowExecutionRecord{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("update workflow execution system_metadata: %w", result.Error)
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

// DeleteNodeExecutionsByNodeIDs removes node execution records for given node IDs
// within an execution, used by loop nodes to reset downstream nodes for re-execution.
func (r *WorkflowNodeExecutionRepository) DeleteNodeExecutionsByNodeIDs(ctx context.Context, executionID uuid.UUID, nodeIDs []string) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if len(nodeIDs) == 0 {
		return nil
	}
	result := r.db.WithContext(ctx).Where("execution_id = ? AND node_id IN ?", executionID, nodeIDs).Delete(&workflowNodeExecutionRecord{})
	if result.Error != nil {
		return fmt.Errorf("delete workflow node executions: %w", result.Error)
	}
	return nil
}
