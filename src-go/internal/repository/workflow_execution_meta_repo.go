package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

type WorkflowExecutionMetaRepo struct{ db *sql.DB }

func NewWorkflowExecutionMetaRepo(db *sql.DB) *WorkflowExecutionMetaRepo {
	return &WorkflowExecutionMetaRepo{db: db}
}

// MergeSystemMetadata performs an idempotent jsonb-merge so concurrent
// writers (e.g. trigger_handler stamping reply_target while im_send
// stamps im_dispatched) do not clobber each other.
func (r *WorkflowExecutionMetaRepo) MergeSystemMetadata(ctx context.Context, executionID uuid.UUID, patch map[string]any) error {
	b, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("marshal patch: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `
		UPDATE workflow_executions
		   SET system_metadata = COALESCE(system_metadata, '{}'::jsonb) || $1::jsonb,
		       updated_at = now()
		 WHERE id = $2`, b, executionID)
	return err
}
