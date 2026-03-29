package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"gorm.io/gorm"
)

type DispatchAttemptRepository struct {
	db *gorm.DB
}

func NewDispatchAttemptRepository(db *gorm.DB) *DispatchAttemptRepository {
	return &DispatchAttemptRepository{db: db}
}

type dispatchAttemptRecord struct {
	ID             uuid.UUID  `gorm:"column:id;type:uuid;primaryKey"`
	ProjectID      uuid.UUID  `gorm:"column:project_id;type:uuid;not null"`
	TaskID         uuid.UUID  `gorm:"column:task_id;type:uuid;not null"`
	MemberID       *uuid.UUID `gorm:"column:member_id;type:uuid"`
	Outcome        string     `gorm:"column:outcome;type:varchar(20);not null"`
	TriggerSource  string     `gorm:"column:trigger_source;type:varchar(40);not null"`
	Reason         string     `gorm:"column:reason;type:text"`
	GuardrailType  string     `gorm:"column:guardrail_type;type:varchar(40)"`
	GuardrailScope string     `gorm:"column:guardrail_scope;type:varchar(40)"`
	CreatedAt      time.Time  `gorm:"column:created_at;not null"`
}

func (dispatchAttemptRecord) TableName() string { return "dispatch_attempts" }

func newDispatchAttemptRecord(attempt *model.DispatchAttempt) *dispatchAttemptRecord {
	if attempt == nil {
		return nil
	}
	return &dispatchAttemptRecord{
		ID:             attempt.ID,
		ProjectID:      attempt.ProjectID,
		TaskID:         attempt.TaskID,
		MemberID:       attempt.MemberID,
		Outcome:        attempt.Outcome,
		TriggerSource:  attempt.TriggerSource,
		Reason:         attempt.Reason,
		GuardrailType:  attempt.GuardrailType,
		GuardrailScope: attempt.GuardrailScope,
		CreatedAt:      attempt.CreatedAt,
	}
}

func (r *dispatchAttemptRecord) toModel() *model.DispatchAttempt {
	if r == nil {
		return nil
	}
	return &model.DispatchAttempt{
		ID:             r.ID,
		ProjectID:      r.ProjectID,
		TaskID:         r.TaskID,
		MemberID:       r.MemberID,
		Outcome:        r.Outcome,
		TriggerSource:  r.TriggerSource,
		Reason:         r.Reason,
		GuardrailType:  r.GuardrailType,
		GuardrailScope: r.GuardrailScope,
		CreatedAt:      r.CreatedAt,
	}
}

func (r *DispatchAttemptRepository) Create(ctx context.Context, attempt *model.DispatchAttempt) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).Create(newDispatchAttemptRecord(attempt)).Error; err != nil {
		return fmt.Errorf("create dispatch attempt: %w", err)
	}
	return nil
}

func (r *DispatchAttemptRepository) ListByTaskID(ctx context.Context, taskID uuid.UUID, limit int) ([]*model.DispatchAttempt, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	if limit <= 0 {
		limit = 100
	}
	var records []dispatchAttemptRecord
	if err := r.db.WithContext(ctx).Where("task_id = ?", taskID).Order("created_at DESC").Limit(limit).Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list dispatch attempts by task: %w", err)
	}
	return toDispatchAttempts(records), nil
}

func (r *DispatchAttemptRepository) ListByProjectID(ctx context.Context, projectID uuid.UUID, limit int) ([]*model.DispatchAttempt, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	if limit <= 0 {
		limit = 500
	}
	var records []dispatchAttemptRecord
	if err := r.db.WithContext(ctx).Where("project_id = ?", projectID).Order("created_at DESC").Limit(limit).Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list dispatch attempts by project: %w", err)
	}
	return toDispatchAttempts(records), nil
}

func toDispatchAttempts(records []dispatchAttemptRecord) []*model.DispatchAttempt {
	attempts := make([]*model.DispatchAttempt, 0, len(records))
	for i := range records {
		attempts = append(attempts, records[i].toModel())
	}
	return attempts
}
