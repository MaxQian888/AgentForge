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

const agentMemoryColumns = `id, project_id, scope, role_id, category, key, content,
	metadata, relevance_score, access_count, last_accessed_at, created_at, updated_at`

func scanAgentMemory(row interface{ Scan(dest ...any) error }) (*model.AgentMemory, error) {
	mem := &model.AgentMemory{}
	err := row.Scan(
		&mem.ID, &mem.ProjectID, &mem.Scope, &mem.RoleID, &mem.Category, &mem.Key, &mem.Content,
		&mem.Metadata, &mem.RelevanceScore, &mem.AccessCount, &mem.LastAccessedAt, &mem.CreatedAt, &mem.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return mem, nil
}

func (r *AgentMemoryRepository) Create(ctx context.Context, mem *model.AgentMemory) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	query := `
		INSERT INTO agent_memory (id, project_id, scope, role_id, category, key, content,
			metadata, relevance_score, access_count, last_accessed_at, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,NOW(),NOW())
	`
	if err := r.db.WithContext(ctx).Exec(
		query,
		mem.ID, mem.ProjectID, mem.Scope, mem.RoleID, mem.Category, mem.Key, mem.Content,
		mem.Metadata, mem.RelevanceScore, mem.AccessCount, mem.LastAccessedAt,
	).Error; err != nil {
		return fmt.Errorf("create agent memory: %w", err)
	}
	return nil
}

func (r *AgentMemoryRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.AgentMemory, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	query := `SELECT ` + agentMemoryColumns + ` FROM agent_memory WHERE id = $1`
	mem, err := scanAgentMemory(r.db.WithContext(ctx).Raw(query, id).Row())
	if err != nil {
		return nil, fmt.Errorf("get agent memory by id: %w", normalizeRepositoryError(err))
	}
	return mem, nil
}

func (r *AgentMemoryRepository) ListByProject(ctx context.Context, projectID uuid.UUID, scope, category string) ([]*model.AgentMemory, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}

	conditions := []string{"project_id = $1"}
	args := []any{projectID}
	argIdx := 2

	if strings.TrimSpace(scope) != "" {
		conditions = append(conditions, fmt.Sprintf("scope = $%d", argIdx))
		args = append(args, scope)
		argIdx++
	}
	if strings.TrimSpace(category) != "" {
		conditions = append(conditions, fmt.Sprintf("category = $%d", argIdx))
		args = append(args, category)
		argIdx++
	}

	query := `SELECT ` + agentMemoryColumns + ` FROM agent_memory WHERE ` + strings.Join(conditions, " AND ") + ` ORDER BY relevance_score DESC, created_at DESC`
	rows, err := r.db.WithContext(ctx).Raw(query, args...).Rows()
	if err != nil {
		return nil, fmt.Errorf("list agent memories by project: %w", err)
	}
	defer rows.Close()

	var memories []*model.AgentMemory
	for rows.Next() {
		mem, err := scanAgentMemory(rows)
		if err != nil {
			return nil, fmt.Errorf("scan agent memory: %w", err)
		}
		memories = append(memories, mem)
	}
	return memories, rows.Err()
}

func (r *AgentMemoryRepository) Search(ctx context.Context, projectID uuid.UUID, query string, limit int) ([]*model.AgentMemory, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	if limit <= 0 {
		limit = 20
	}
	searchQuery := `SELECT ` + agentMemoryColumns + ` FROM agent_memory
		WHERE project_id = $1 AND (key ILIKE $2 OR content ILIKE $2)
		ORDER BY relevance_score DESC, created_at DESC LIMIT $3`
	pattern := "%" + query + "%"
	rows, err := r.db.WithContext(ctx).Raw(searchQuery, projectID, pattern, limit).Rows()
	if err != nil {
		return nil, fmt.Errorf("search agent memories: %w", err)
	}
	defer rows.Close()

	var memories []*model.AgentMemory
	for rows.Next() {
		mem, err := scanAgentMemory(rows)
		if err != nil {
			return nil, fmt.Errorf("scan agent memory: %w", err)
		}
		memories = append(memories, mem)
	}
	return memories, rows.Err()
}

func (r *AgentMemoryRepository) IncrementAccess(ctx context.Context, id uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	query := `UPDATE agent_memory SET access_count = access_count + 1, last_accessed_at = NOW(), updated_at = NOW() WHERE id = $1`
	if err := r.db.WithContext(ctx).Exec(query, id).Error; err != nil {
		return fmt.Errorf("increment agent memory access: %w", err)
	}
	return nil
}

func (r *AgentMemoryRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	query := `DELETE FROM agent_memory WHERE id = $1`
	if err := r.db.WithContext(ctx).Exec(query, id).Error; err != nil {
		return fmt.Errorf("delete agent memory: %w", err)
	}
	return nil
}
