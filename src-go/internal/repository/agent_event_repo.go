package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"gorm.io/gorm"
)

type AgentEventRepository struct {
	db *gorm.DB
}

func NewAgentEventRepository(db *gorm.DB) *AgentEventRepository {
	return &AgentEventRepository{db: db}
}

type agentEventRecord struct {
	ID         uuid.UUID `gorm:"column:id;type:uuid;primaryKey"`
	RunID      uuid.UUID `gorm:"column:run_id;type:uuid;not null"`
	TaskID     uuid.UUID `gorm:"column:task_id;type:uuid;not null"`
	ProjectID  uuid.UUID `gorm:"column:project_id;type:uuid;not null"`
	EventType  string    `gorm:"column:event_type;type:varchar(50);not null"`
	Payload    string    `gorm:"column:payload;type:jsonb;default:'{}'"`
	OccurredAt time.Time `gorm:"column:occurred_at;not null"`
	CreatedAt  time.Time `gorm:"column:created_at;not null"`
}

func (agentEventRecord) TableName() string { return "agent_events" }

func newAgentEventRecord(e *model.AgentEvent) *agentEventRecord {
	return &agentEventRecord{
		ID:         e.ID,
		RunID:      e.RunID,
		TaskID:     e.TaskID,
		ProjectID:  e.ProjectID,
		EventType:  e.EventType,
		Payload:    e.Payload,
		OccurredAt: e.OccurredAt,
		CreatedAt:  e.CreatedAt,
	}
}

func (r *agentEventRecord) toModel() *model.AgentEvent {
	return &model.AgentEvent{
		ID:         r.ID,
		RunID:      r.RunID,
		TaskID:     r.TaskID,
		ProjectID:  r.ProjectID,
		EventType:  r.EventType,
		Payload:    r.Payload,
		OccurredAt: r.OccurredAt,
		CreatedAt:  r.CreatedAt,
	}
}

func (r *AgentEventRepository) Create(ctx context.Context, event *model.AgentEvent) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).Create(newAgentEventRecord(event)).Error; err != nil {
		return fmt.Errorf("create agent event: %w", err)
	}
	return nil
}

func (r *AgentEventRepository) ListByRunID(ctx context.Context, runID uuid.UUID, limit int) ([]*model.AgentEvent, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	if limit <= 0 {
		limit = 100
	}
	var records []agentEventRecord
	if err := r.db.WithContext(ctx).Where("run_id = ?", runID).Order("occurred_at ASC").Limit(limit).Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list agent events by run: %w", err)
	}
	events := make([]*model.AgentEvent, len(records))
	for i := range records {
		events[i] = records[i].toModel()
	}
	return events, nil
}

func (r *AgentEventRepository) ListByTaskID(ctx context.Context, taskID uuid.UUID, limit int) ([]*model.AgentEvent, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	if limit <= 0 {
		limit = 100
	}
	var records []agentEventRecord
	if err := r.db.WithContext(ctx).Where("task_id = ?", taskID).Order("occurred_at ASC").Limit(limit).Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list agent events by task: %w", err)
	}
	events := make([]*model.AgentEvent, len(records))
	for i := range records {
		events[i] = records[i].toModel()
	}
	return events, nil
}

func (r *AgentEventRepository) ListByProjectID(ctx context.Context, projectID uuid.UUID, limit int) ([]*model.AgentEvent, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	if limit <= 0 {
		limit = 100
	}
	var records []agentEventRecord
	if err := r.db.WithContext(ctx).Where("project_id = ?", projectID).Order("occurred_at DESC").Limit(limit).Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list agent events by project: %w", err)
	}
	events := make([]*model.AgentEvent, len(records))
	for i := range records {
		events[i] = records[i].toModel()
	}
	return events, nil
}
