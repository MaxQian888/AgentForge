package repository

import (
	"context"
	"fmt"
	"strings"

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
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	if limit <= 0 {
		limit = 20
	}
	pattern := "%" + query + "%"

	var records []agentMemoryRecord
	if err := r.db.WithContext(ctx).
		Where("project_id = ? AND (key ILIKE ? OR content ILIKE ?)", projectID, pattern, pattern).
		Order("relevance_score DESC, created_at DESC").
		Limit(limit).
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("search agent memories: %w", err)
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
			"access_count":    gorm.Expr("access_count + 1"),
			"last_accessed_at": gorm.Expr("NOW()"),
			"updated_at":      gorm.Expr("NOW()"),
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
