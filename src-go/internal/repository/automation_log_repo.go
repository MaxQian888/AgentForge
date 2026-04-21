package repository

import (
	"context"
	"fmt"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type AutomationLogRepository struct {
	db *gorm.DB
}

func NewAutomationLogRepository(db *gorm.DB) *AutomationLogRepository {
	return &AutomationLogRepository{db: db}
}

func (r *AutomationLogRepository) Create(ctx context.Context, entry *model.AutomationLog) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).Create(newAutomationLogRecord(entry)).Error; err != nil {
		return fmt.Errorf("create automation log: %w", err)
	}
	return nil
}

// ListByTraceID returns all automation logs whose detail JSONB has "trace_id" equal to traceID.
// Ordered by triggered_at ASC. Returns at most limit rows (defaults to 10000 when <= 0).
func (r *AutomationLogRepository) ListByTraceID(ctx context.Context, traceID string, limit int) ([]*model.AutomationLog, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	if limit <= 0 {
		limit = 10000
	}
	var records []automationLogRecord
	if err := r.db.WithContext(ctx).
		Where("detail->>'trace_id' = ?", traceID).
		Order("triggered_at ASC").
		Limit(limit).
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list automation logs by trace: %w", err)
	}
	out := make([]*model.AutomationLog, len(records))
	for i := range records {
		out[i] = records[i].toModel()
	}
	return out, nil
}

func (r *AutomationLogRepository) ListByProject(ctx context.Context, projectID uuid.UUID, query model.AutomationLogListQuery) ([]*model.AutomationLog, int, error) {
	if r.db == nil {
		return nil, 0, ErrDatabaseUnavailable
	}
	base := r.db.WithContext(ctx).
		Table("automation_logs AS logs").
		Joins("JOIN automation_rules AS rules ON rules.id = logs.rule_id").
		Where("rules.project_id = ? AND rules.deleted_at IS NULL", projectID)
	if query.EventType != "" {
		base = base.Where("logs.event_type = ?", query.EventType)
	}
	if query.Status != "" {
		base = base.Where("logs.status = ?", query.Status)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count automation logs: %w", err)
	}

	limit := query.Limit
	if limit <= 0 {
		limit = 20
	}
	page := query.Page
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit

	var records []automationLogRecord
	if err := applyPagination(base.Order("logs.triggered_at DESC").Select("logs.*"), limit, offset).Scan(&records).Error; err != nil {
		return nil, 0, fmt.Errorf("list automation logs: %w", err)
	}

	result := make([]*model.AutomationLog, 0, len(records))
	for i := range records {
		result = append(result, records[i].toModel())
	}
	return result, int(total), nil
}
