package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TaskProgressRepository struct {
	db *gorm.DB
}

func NewTaskProgressRepository(db *gorm.DB) *TaskProgressRepository {
	return &TaskProgressRepository{db: db}
}

func (r *TaskProgressRepository) GetByTaskID(ctx context.Context, taskID uuid.UUID) (*model.TaskProgressSnapshot, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}

	var record taskProgressSnapshotRecord
	if err := r.db.WithContext(ctx).Where("task_id = ?", taskID).Take(&record).Error; err != nil {
		return nil, fmt.Errorf("get task progress snapshot: %w", normalizeRepositoryError(err))
	}
	return record.toModel(), nil
}

func (r *TaskProgressRepository) Upsert(ctx context.Context, snapshot *model.TaskProgressSnapshot) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}

	record := newTaskProgressSnapshotRecord(snapshot)
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "task_id"}},
			DoUpdates: clause.Assignments(map[string]any{
				"last_activity_at":     record.LastActivityAt,
				"last_activity_source": record.LastActivitySource,
				"last_transition_at":   record.LastTransitionAt,
				"health_status":        record.HealthStatus,
				"risk_reason":          record.RiskReason,
				"risk_since_at":        record.RiskSinceAt,
				"last_alert_state":     record.LastAlertState,
				"last_alert_at":        record.LastAlertAt,
				"last_recovered_at":    record.LastRecoveredAt,
				"updated_at":           gorm.Expr("NOW()"),
			}),
		}).
		Create(record).Error; err != nil {
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

	var records []taskProgressSnapshotRecord
	if err := r.db.WithContext(ctx).Where("task_id IN ?", taskIDs).Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list task progress snapshots: %w", err)
	}

	result := make(map[uuid.UUID]*model.TaskProgressSnapshot, len(records))
	for i := range records {
		m := records[i].toModel()
		result[m.TaskID] = m
	}
	return result, nil
}
