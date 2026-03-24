package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

type TaskProgressRepository struct {
	db DBTX
}

func NewTaskProgressRepository(db DBTX) *TaskProgressRepository {
	return &TaskProgressRepository{db: db}
}

const taskProgressColumns = `task_id, last_activity_at, last_activity_source, last_transition_at,
	health_status, risk_reason, risk_since_at, last_alert_state, last_alert_at,
	last_recovered_at, created_at, updated_at`

func scanTaskProgress(row interface{ Scan(dest ...any) error }) (*model.TaskProgressSnapshot, error) {
	snapshot := &model.TaskProgressSnapshot{}
	err := row.Scan(
		&snapshot.TaskID,
		&snapshot.LastActivityAt,
		&snapshot.LastActivitySource,
		&snapshot.LastTransitionAt,
		&snapshot.HealthStatus,
		&snapshot.RiskReason,
		&snapshot.RiskSinceAt,
		&snapshot.LastAlertState,
		&snapshot.LastAlertAt,
		&snapshot.LastRecoveredAt,
		&snapshot.CreatedAt,
		&snapshot.UpdatedAt,
	)
	return snapshot, err
}

func (r *TaskProgressRepository) GetByTaskID(ctx context.Context, taskID uuid.UUID) (*model.TaskProgressSnapshot, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}

	query := `SELECT ` + taskProgressColumns + ` FROM task_progress_snapshots WHERE task_id = $1`
	snapshot, err := scanTaskProgress(r.db.QueryRow(ctx, query, taskID))
	if err != nil {
		return nil, fmt.Errorf("get task progress snapshot: %w", err)
	}
	return snapshot, nil
}

func (r *TaskProgressRepository) Upsert(ctx context.Context, snapshot *model.TaskProgressSnapshot) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}

	query := `
		INSERT INTO task_progress_snapshots (
			task_id,
			last_activity_at,
			last_activity_source,
			last_transition_at,
			health_status,
			risk_reason,
			risk_since_at,
			last_alert_state,
			last_alert_at,
			last_recovered_at,
			created_at,
			updated_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,NOW(),NOW())
		ON CONFLICT (task_id) DO UPDATE SET
			last_activity_at = EXCLUDED.last_activity_at,
			last_activity_source = EXCLUDED.last_activity_source,
			last_transition_at = EXCLUDED.last_transition_at,
			health_status = EXCLUDED.health_status,
			risk_reason = EXCLUDED.risk_reason,
			risk_since_at = EXCLUDED.risk_since_at,
			last_alert_state = EXCLUDED.last_alert_state,
			last_alert_at = EXCLUDED.last_alert_at,
			last_recovered_at = EXCLUDED.last_recovered_at,
			updated_at = NOW()
	`

	_, err := r.db.Exec(
		ctx,
		query,
		snapshot.TaskID,
		snapshot.LastActivityAt,
		snapshot.LastActivitySource,
		snapshot.LastTransitionAt,
		snapshot.HealthStatus,
		snapshot.RiskReason,
		snapshot.RiskSinceAt,
		snapshot.LastAlertState,
		snapshot.LastAlertAt,
		snapshot.LastRecoveredAt,
	)
	if err != nil {
		return fmt.Errorf("upsert task progress snapshot: %w", err)
	}
	return nil
}

func (r *TaskProgressRepository) ListByTaskIDs(ctx context.Context, taskIDs []uuid.UUID) (map[uuid.UUID]*model.TaskProgressSnapshot, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	if len(taskIDs) == 0 {
		return map[uuid.UUID]*model.TaskProgressSnapshot{}, nil
	}

	placeholders := make([]string, 0, len(taskIDs))
	args := make([]any, 0, len(taskIDs))
	for idx, id := range taskIDs {
		placeholders = append(placeholders, fmt.Sprintf("$%d", idx+1))
		args = append(args, id)
	}

	query := `SELECT ` + taskProgressColumns + ` FROM task_progress_snapshots WHERE task_id IN (` + strings.Join(placeholders, ", ") + `)`
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list task progress snapshots: %w", err)
	}
	defer rows.Close()

	result := make(map[uuid.UUID]*model.TaskProgressSnapshot, len(taskIDs))
	for rows.Next() {
		snapshot, err := scanTaskProgress(rows)
		if err != nil {
			return nil, fmt.Errorf("scan task progress snapshot: %w", err)
		}
		result[snapshot.TaskID] = snapshot
	}
	return result, rows.Err()
}
