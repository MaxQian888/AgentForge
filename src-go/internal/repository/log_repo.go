package repository

import (
	"context"
	"fmt"

	"github.com/agentforge/server/internal/model"
	"gorm.io/gorm"
)

// LogRepository handles persistence of log entries.
type LogRepository struct {
	db *gorm.DB
}

// NewLogRepository creates a new LogRepository.
func NewLogRepository(db *gorm.DB) *LogRepository {
	return &LogRepository{db: db}
}

// Create inserts a new log entry.
func (r *LogRepository) Create(ctx context.Context, log *model.Log) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).Create(newLogRecord(log)).Error; err != nil {
		return fmt.Errorf("create log: %w", err)
	}
	return nil
}

// List returns a paginated list of log entries matching the given filters.
func (r *LogRepository) List(ctx context.Context, req model.LogListRequest) ([]model.Log, int64, error) {
	if r.db == nil {
		return nil, 0, ErrDatabaseUnavailable
	}

	query := r.db.WithContext(ctx).Model(&logRecord{}).Where("project_id = ?", req.ProjectID)

	if req.Tab != "" {
		query = query.Where("tab = ?", req.Tab)
	}
	if req.Level != "" {
		query = query.Where("level = ?", req.Level)
	}
	if req.Search != "" {
		query = query.Where("summary ILIKE ?", "%"+req.Search+"%")
	}
	if req.From != nil {
		query = query.Where("created_at >= ?", *req.From)
	}
	if req.To != nil {
		query = query.Where("created_at <= ?", *req.To)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count logs: %w", err)
	}

	var records []logRecord
	offset := (req.Page - 1) * req.PageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(req.PageSize).Find(&records).Error; err != nil {
		return nil, 0, fmt.Errorf("list logs: %w", err)
	}

	logs := make([]model.Log, len(records))
	for i := range records {
		logs[i] = *records[i].toModel()
	}
	return logs, total, nil
}
