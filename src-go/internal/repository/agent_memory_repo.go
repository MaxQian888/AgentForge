package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"gorm.io/gorm"
)

type AgentMemoryRepository struct {
	db *gorm.DB
}

func NewAgentMemoryRepository(db *gorm.DB) *AgentMemoryRepository {
	return &AgentMemoryRepository{db: db}
}

func (r *AgentMemoryRepository) Create(ctx context.Context, mem *model.AgentMemory) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).Create(newAgentMemoryRecord(mem)).Error; err != nil {
		return fmt.Errorf("create agent memory: %w", err)
	}
	return nil
}

func (r *AgentMemoryRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.AgentMemory, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var record agentMemoryRecord
	if err := r.db.WithContext(ctx).Where("id = ?", id).Take(&record).Error; err != nil {
		return nil, fmt.Errorf("get agent memory by id: %w", normalizeRepositoryError(err))
	}
	return record.toModel(), nil
}

func (r *AgentMemoryRepository) ListByProject(ctx context.Context, projectID uuid.UUID, scope, category string) ([]*model.AgentMemory, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}

	q := r.db.WithContext(ctx).Where("project_id = ?", projectID)

	if strings.TrimSpace(scope) != "" {
		q = q.Where("scope = ?", scope)
	}
	if strings.TrimSpace(category) != "" {
		q = q.Where("category = ?", category)
	}

	var records []agentMemoryRecord
	if err := q.Order("relevance_score DESC, created_at DESC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list agent memories by project: %w", err)
	}

	memories := make([]*model.AgentMemory, len(records))
	for i := range records {
		memories[i] = records[i].toModel()
	}
	return memories, nil
}

func (r *AgentMemoryRepository) Search(ctx context.Context, projectID uuid.UUID, query string, limit int) ([]*model.AgentMemory, error) {
	return r.ListFiltered(ctx, projectID, model.AgentMemoryFilter{Query: query, Limit: limit})
}

func (r *AgentMemoryRepository) ListFiltered(ctx context.Context, projectID uuid.UUID, filter model.AgentMemoryFilter) ([]*model.AgentMemory, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}

	query := r.db.WithContext(ctx).Where("project_id = ?", projectID)
	if strings.TrimSpace(filter.Scope) != "" {
		query = query.Where("scope = ?", filter.Scope)
	}
	if strings.TrimSpace(filter.Category) != "" {
		query = query.Where("category = ?", filter.Category)
	}
	if strings.TrimSpace(filter.RoleID) != "" {
		query = query.Where("role_id = ?", filter.RoleID)
	}
	if filter.StartAt != nil {
		query = query.Where("created_at >= ?", *filter.StartAt)
	}
	if filter.EndAt != nil {
		query = query.Where("created_at <= ?", *filter.EndAt)
	}
	if search := strings.TrimSpace(filter.Query); search != "" {
		pattern := "%" + search + "%"
		query = query.Where("LOWER(key) LIKE LOWER(?) OR LOWER(content) LIKE LOWER(?) OR LOWER(metadata) LIKE LOWER(?)", pattern, pattern, pattern)
	}

	query = query.Order("relevance_score DESC, created_at DESC")
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}

	var records []agentMemoryRecord
	if err := query.Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list filtered agent memories: %w", err)
	}

	memories := make([]*model.AgentMemory, len(records))
	for i := range records {
		memories[i] = records[i].toModel()
	}
	return memories, nil
}

func (r *AgentMemoryRepository) IncrementAccess(ctx context.Context, id uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).
		Model(&agentMemoryRecord{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"access_count":     gorm.Expr("access_count + 1"),
			"last_accessed_at": gorm.Expr("NOW()"),
			"updated_at":       gorm.Expr("NOW()"),
		}).Error; err != nil {
		return fmt.Errorf("increment agent memory access: %w", err)
	}
	return nil
}

func (r *AgentMemoryRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).Delete(&agentMemoryRecord{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("delete agent memory: %w", err)
	}
	return nil
}

func (r *AgentMemoryRepository) DeleteMany(ctx context.Context, ids []uuid.UUID) (int64, error) {
	if r.db == nil {
		return 0, ErrDatabaseUnavailable
	}
	if len(ids) == 0 {
		return 0, nil
	}
	result := r.db.WithContext(ctx).Delete(&agentMemoryRecord{}, "id IN ?", ids)
	if result.Error != nil {
		return 0, fmt.Errorf("delete many agent memories: %w", result.Error)
	}
	return result.RowsAffected, nil
}

func (r *AgentMemoryRepository) ListByProjectAndTimeRange(ctx context.Context, projectID uuid.UUID, category, scope, roleID string, start, end *time.Time, limit int) ([]*model.AgentMemory, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}

	query := r.db.WithContext(ctx).Where("project_id = ?", projectID)
	if strings.TrimSpace(category) != "" {
		query = query.Where("category = ?", category)
	}
	if strings.TrimSpace(scope) != "" {
		query = query.Where("scope = ?", scope)
	}
	if strings.TrimSpace(roleID) != "" {
		query = query.Where("role_id = ?", roleID)
	}
	if start != nil {
		query = query.Where("created_at >= ?", *start)
	}
	if end != nil {
		query = query.Where("created_at <= ?", *end)
	}

	query = query.Order("created_at ASC")
	if limit > 0 {
		query = query.Limit(limit)
	}

	var records []agentMemoryRecord
	if err := query.Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list agent memories by project and time range: %w", err)
	}

	memories := make([]*model.AgentMemory, len(records))
	for index := range records {
		memories[index] = records[index].toModel()
	}
	return memories, nil
}

func (r *AgentMemoryRepository) DeleteOlderThan(ctx context.Context, projectID uuid.UUID, category string, before time.Time) (int64, error) {
	if r.db == nil {
		return 0, ErrDatabaseUnavailable
	}

	query := r.db.WithContext(ctx).Where("project_id = ? AND created_at < ?", projectID, before)
	if strings.TrimSpace(category) != "" {
		query = query.Where("category = ?", category)
	}

	result := query.Delete(&agentMemoryRecord{})
	if result.Error != nil {
		return 0, fmt.Errorf("delete agent memories older than cutoff: %w", result.Error)
	}
	return result.RowsAffected, nil
}
