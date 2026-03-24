package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

type AgentRunRepository struct {
	db DBTX
}

func NewAgentRunRepository(db DBTX) *AgentRunRepository {
	return &AgentRunRepository{db: db}
}

const agentRunColumns = `id, task_id, member_id, role_id, status, provider, model,
	input_tokens, output_tokens, cache_read_tokens, cost_usd, turn_count,
	error_message, started_at, completed_at, created_at, updated_at`

func (r *AgentRunRepository) Create(ctx context.Context, run *model.AgentRun) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	query := `
		INSERT INTO agent_runs (id, task_id, member_id, role_id, status, provider, model,
			input_tokens, output_tokens, cache_read_tokens, cost_usd, turn_count,
			error_message, started_at, completed_at, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,NOW(),NOW())
	`
	_, err := r.db.Exec(ctx, query,
		run.ID, run.TaskID, run.MemberID, run.RoleID, run.Status, run.Provider, run.Model,
		run.InputTokens, run.OutputTokens, run.CacheReadTokens, run.CostUsd, run.TurnCount,
		run.ErrorMessage, run.StartedAt, run.CompletedAt,
	)
	if err != nil {
		return fmt.Errorf("create agent run: %w", err)
	}
	return nil
}

func (r *AgentRunRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.AgentRun, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	query := `SELECT ` + agentRunColumns + ` FROM agent_runs WHERE id = $1`
	run := &model.AgentRun{}
	err := r.db.QueryRow(ctx, query, id).Scan(
		&run.ID, &run.TaskID, &run.MemberID, &run.RoleID, &run.Status, &run.Provider, &run.Model,
		&run.InputTokens, &run.OutputTokens, &run.CacheReadTokens, &run.CostUsd, &run.TurnCount,
		&run.ErrorMessage, &run.StartedAt, &run.CompletedAt, &run.CreatedAt, &run.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get agent run by id: %w", err)
	}
	return run, nil
}

func (r *AgentRunRepository) GetByTask(ctx context.Context, taskID uuid.UUID) ([]*model.AgentRun, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	query := `SELECT ` + agentRunColumns + ` FROM agent_runs WHERE task_id = $1 ORDER BY created_at DESC`
	rows, err := r.db.Query(ctx, query, taskID)
	if err != nil {
		return nil, fmt.Errorf("get agent runs by task: %w", err)
	}
	defer rows.Close()

	var runs []*model.AgentRun
	for rows.Next() {
		run := &model.AgentRun{}
		if err := rows.Scan(
			&run.ID, &run.TaskID, &run.MemberID, &run.RoleID, &run.Status, &run.Provider, &run.Model,
			&run.InputTokens, &run.OutputTokens, &run.CacheReadTokens, &run.CostUsd, &run.TurnCount,
			&run.ErrorMessage, &run.StartedAt, &run.CompletedAt, &run.CreatedAt, &run.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan agent run: %w", err)
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

func (r *AgentRunRepository) ListActive(ctx context.Context) ([]*model.AgentRun, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	query := `SELECT ` + agentRunColumns + ` FROM agent_runs WHERE status IN ('starting', 'running') ORDER BY created_at`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list active agent runs: %w", err)
	}
	defer rows.Close()

	var runs []*model.AgentRun
	for rows.Next() {
		run := &model.AgentRun{}
		if err := rows.Scan(
			&run.ID, &run.TaskID, &run.MemberID, &run.RoleID, &run.Status, &run.Provider, &run.Model,
			&run.InputTokens, &run.OutputTokens, &run.CacheReadTokens, &run.CostUsd, &run.TurnCount,
			&run.ErrorMessage, &run.StartedAt, &run.CompletedAt, &run.CreatedAt, &run.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan agent run: %w", err)
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

func (r *AgentRunRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	query := `UPDATE agent_runs SET status = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.Exec(ctx, query, status, id)
	if err != nil {
		return fmt.Errorf("update agent run status: %w", err)
	}
	return nil
}

func (r *AgentRunRepository) UpdateCost(ctx context.Context, id uuid.UUID, inputTokens, outputTokens, cacheReadTokens int64, costUsd float64, turnCount int) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	query := `UPDATE agent_runs SET
		input_tokens = $1, output_tokens = $2, cache_read_tokens = $3,
		cost_usd = $4, turn_count = $5, updated_at = NOW()
		WHERE id = $6`
	_, err := r.db.Exec(ctx, query, inputTokens, outputTokens, cacheReadTokens, costUsd, turnCount, id)
	if err != nil {
		return fmt.Errorf("update agent run cost: %w", err)
	}
	return nil
}
