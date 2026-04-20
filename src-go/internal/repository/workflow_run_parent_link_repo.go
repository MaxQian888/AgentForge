package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type workflowRunParentLinkRecord struct {
	ID                uuid.UUID  `gorm:"column:id;primaryKey"`
	ParentExecutionID uuid.UUID  `gorm:"column:parent_execution_id"`
	ParentKind        string     `gorm:"column:parent_kind"`
	ParentNodeID      string     `gorm:"column:parent_node_id"`
	ChildEngineKind   string     `gorm:"column:child_engine_kind"`
	ChildRunID        uuid.UUID  `gorm:"column:child_run_id"`
	Status            string     `gorm:"column:status"`
	StartedAt         time.Time  `gorm:"column:started_at"`
	TerminatedAt      *time.Time `gorm:"column:terminated_at"`
}

func (workflowRunParentLinkRecord) TableName() string { return "workflow_run_parent_link" }

func (r *workflowRunParentLinkRecord) toModel() *model.WorkflowRunParentLink {
	if r == nil {
		return nil
	}
	parentKind := r.ParentKind
	if parentKind == "" {
		parentKind = model.SubWorkflowParentKindDAGExecution
	}
	return &model.WorkflowRunParentLink{
		ID:                r.ID,
		ParentExecutionID: r.ParentExecutionID,
		ParentKind:        parentKind,
		ParentNodeID:      r.ParentNodeID,
		ChildEngineKind:   r.ChildEngineKind,
		ChildRunID:        r.ChildRunID,
		Status:            r.Status,
		StartedAt:         r.StartedAt,
		TerminatedAt:      r.TerminatedAt,
	}
}

// WorkflowRunParentLinkRepository persists parent↔child sub-workflow linkage
// rows. The (parent_execution_id, parent_node_id) unique index enforces the
// "one child per sub_workflow node" invariant; callers treat ErrNotFound as
// "no link yet" rather than as an error state.
type WorkflowRunParentLinkRepository struct {
	db *gorm.DB
}

func NewWorkflowRunParentLinkRepository(db *gorm.DB) *WorkflowRunParentLinkRepository {
	return &WorkflowRunParentLinkRepository{db: db}
}

// Create inserts a new parent↔child link. The caller must supply an allocated
// ID and a fresh StartedAt; downstream idempotent retries should prefer
// GetByParent + UpdateStatus over re-inserting.
func (r *WorkflowRunParentLinkRepository) Create(ctx context.Context, link *model.WorkflowRunParentLink) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if link.StartedAt.IsZero() {
		link.StartedAt = time.Now().UTC()
	}
	if link.Status == "" {
		link.Status = model.SubWorkflowLinkStatusRunning
	}
	if link.ParentKind == "" {
		link.ParentKind = model.SubWorkflowParentKindDAGExecution
	}
	record := &workflowRunParentLinkRecord{
		ID:                link.ID,
		ParentExecutionID: link.ParentExecutionID,
		ParentKind:        link.ParentKind,
		ParentNodeID:      link.ParentNodeID,
		ChildEngineKind:   link.ChildEngineKind,
		ChildRunID:        link.ChildRunID,
		Status:            link.Status,
		StartedAt:         link.StartedAt,
		TerminatedAt:      link.TerminatedAt,
	}
	if err := r.db.WithContext(ctx).Create(record).Error; err != nil {
		return fmt.Errorf("create workflow run parent link: %w", err)
	}
	return nil
}

// GetByParent returns the single link row matching (parentExecutionID, parentNodeID).
// The combination is unique at the schema level; returns ErrNotFound if no row exists.
func (r *WorkflowRunParentLinkRepository) GetByParent(ctx context.Context, parentExecutionID uuid.UUID, parentNodeID string) (*model.WorkflowRunParentLink, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var record workflowRunParentLinkRecord
	if err := r.db.WithContext(ctx).
		Where("parent_execution_id = ? AND parent_node_id = ?", parentExecutionID, parentNodeID).
		Take(&record).Error; err != nil {
		return nil, fmt.Errorf("get workflow run parent link by parent: %w", normalizeRepositoryError(err))
	}
	return record.toModel(), nil
}

// GetByChild returns the link row that identifies a given child run. Because
// child_run_id is not globally unique across engines (DAG execution ids and
// plugin run ids are distinct UUID generators, but there's no DB-level guard),
// callers MUST supply the engine kind too.
func (r *WorkflowRunParentLinkRepository) GetByChild(ctx context.Context, engineKind string, childRunID uuid.UUID) (*model.WorkflowRunParentLink, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var record workflowRunParentLinkRecord
	if err := r.db.WithContext(ctx).
		Where("child_engine_kind = ? AND child_run_id = ?", engineKind, childRunID).
		Take(&record).Error; err != nil {
		return nil, fmt.Errorf("get workflow run parent link by child: %w", normalizeRepositoryError(err))
	}
	return record.toModel(), nil
}

// ListByParentExecution returns every link row whose parent execution matches.
// Used by the execution read DTO to expose a run's outgoing sub-invocations.
func (r *WorkflowRunParentLinkRepository) ListByParentExecution(ctx context.Context, parentExecutionID uuid.UUID) ([]*model.WorkflowRunParentLink, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var records []workflowRunParentLinkRecord
	if err := r.db.WithContext(ctx).
		Where("parent_execution_id = ?", parentExecutionID).
		Order("started_at ASC").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list workflow run parent links by execution: %w", err)
	}
	result := make([]*model.WorkflowRunParentLink, len(records))
	for i := range records {
		result[i] = records[i].toModel()
	}
	return result, nil
}

// GetByChildForParentKind returns a link row for the given child where the
// parent side has the specified parent_kind. Used by the cross-engine resume
// paths to disambiguate a child that could be referenced by either a DAG
// execution parent or a plugin run parent. Returns ErrNotFound when no row
// matches.
func (r *WorkflowRunParentLinkRepository) GetByChildForParentKind(ctx context.Context, parentKind, engineKind string, childRunID uuid.UUID) (*model.WorkflowRunParentLink, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var record workflowRunParentLinkRecord
	if err := r.db.WithContext(ctx).
		Where("parent_kind = ? AND child_engine_kind = ? AND child_run_id = ?", parentKind, engineKind, childRunID).
		Take(&record).Error; err != nil {
		return nil, fmt.Errorf("get workflow run parent link by child for parent kind: %w", normalizeRepositoryError(err))
	}
	return record.toModel(), nil
}

// ListByParentRun returns every link row whose parent side is a plugin run
// (parent_kind = 'plugin_run') and whose parent_execution_id matches the given
// plugin run id. Used by the plugin runtime's cancellation hook to find DAG
// children that must be cancelled when the parent plugin run is cancelled.
func (r *WorkflowRunParentLinkRepository) ListByParentRun(ctx context.Context, parentRunID uuid.UUID) ([]*model.WorkflowRunParentLink, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var records []workflowRunParentLinkRecord
	if err := r.db.WithContext(ctx).
		Where("parent_kind = ? AND parent_execution_id = ?", model.SubWorkflowParentKindPluginRun, parentRunID).
		Order("started_at ASC").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list workflow run parent links by plugin run: %w", err)
	}
	result := make([]*model.WorkflowRunParentLink, len(records))
	for i := range records {
		result[i] = records[i].toModel()
	}
	return result, nil
}

// UpdateStatus transitions an existing link into a terminal status and stamps
// terminated_at. Safe to call idempotently — already-terminated rows are
// left unchanged.
func (r *WorkflowRunParentLinkRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	now := time.Now().UTC()
	updates := map[string]any{
		"status":        status,
		"terminated_at": now,
	}
	result := r.db.WithContext(ctx).
		Model(&workflowRunParentLinkRecord{}).
		Where("id = ?", id).
		Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("update workflow run parent link status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
