package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"gorm.io/gorm"
)

type AgentRunRepository struct {
	db *gorm.DB
}

func NewAgentRunRepository(db *gorm.DB) *AgentRunRepository {
	return &AgentRunRepository{db: db}
}

const agentRunColumns = `id, task_id, member_id, role_id, status, runtime, provider, model,
	input_tokens, output_tokens, cache_read_tokens, cost_usd, turn_count,
	error_message, started_at, completed_at, created_at, updated_at, team_id, team_role`

func scanAgentRun(row interface{ Scan(dest ...any) error }) (*model.AgentRun, error) {
	run := &model.AgentRun{}
	err := row.Scan(
		&run.ID, &run.TaskID, &run.MemberID, &run.RoleID, &run.Status, &run.Runtime, &run.Provider, &run.Model,
		&run.InputTokens, &run.OutputTokens, &run.CacheReadTokens, &run.CostUsd, &run.TurnCount,
		&run.ErrorMessage, &run.StartedAt, &run.CompletedAt, &run.CreatedAt, &run.UpdatedAt,
		&run.TeamID, &run.TeamRole,
	)
	if err != nil {
		return nil, err
	}
	return run, nil
}

func (r *AgentRunRepository) Create(ctx context.Context, run *model.AgentRun) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	query := `
		INSERT INTO agent_runs (id, task_id, member_id, role_id, status, runtime, provider, model,
			input_tokens, output_tokens, cache_read_tokens, cost_usd, turn_count,
			error_message, started_at, completed_at, created_at, updated_at, team_id, team_role)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,NOW(),NOW(),$17,$18)
	`
	if err := r.db.WithContext(ctx).Exec(
		query,
		run.ID, run.TaskID, run.MemberID, run.RoleID, run.Status, run.Runtime, run.Provider, run.Model,
		run.InputTokens, run.OutputTokens, run.CacheReadTokens, run.CostUsd, run.TurnCount,
		run.ErrorMessage, run.StartedAt, run.CompletedAt, run.TeamID, run.TeamRole,
	).Error; err != nil {
		return fmt.Errorf("create agent run: %w", err)
	}
	return nil
}

func (r *AgentRunRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.AgentRun, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	query := `SELECT ` + agentRunColumns + ` FROM agent_runs WHERE id = $1`
	run, err := scanAgentRun(r.db.WithContext(ctx).Raw(query, id).Row())
	if err != nil {
		return nil, fmt.Errorf("get agent run by id: %w", normalizeRepositoryError(err))
	}
	return run, nil
}

func (r *AgentRunRepository) GetByTask(ctx context.Context, taskID uuid.UUID) ([]*model.AgentRun, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	query := `SELECT ` + agentRunColumns + ` FROM agent_runs WHERE task_id = $1 ORDER BY created_at DESC`
	rows, err := r.db.WithContext(ctx).Raw(query, taskID).Rows()
	if err != nil {
		return nil, fmt.Errorf("get agent runs by task: %w", err)
	}
	defer rows.Close()

	var runs []*model.AgentRun
	for rows.Next() {
		run, err := scanAgentRun(rows)
		if err != nil {
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
	query := `SELECT ` + agentRunColumns + ` FROM agent_runs WHERE status IN ('starting', 'running', 'paused') ORDER BY created_at`
	rows, err := r.db.WithContext(ctx).Raw(query).Rows()
	if err != nil {
		return nil, fmt.Errorf("list active agent runs: %w", err)
	}
	defer rows.Close()

	var runs []*model.AgentRun
	for rows.Next() {
		run, err := scanAgentRun(rows)
		if err != nil {
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
	if err := r.db.WithContext(ctx).Exec(query, status, id).Error; err != nil {
		return fmt.Errorf("update agent run status: %w", err)
	}
	return nil
}

func (r *AgentRunRepository) ListByProject(ctx context.Context, projectID uuid.UUID) ([]*model.AgentRun, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	query := `SELECT ` + agentRunColumns + ` FROM agent_runs ar
		JOIN tasks t ON ar.task_id = t.id
		WHERE t.project_id = $1 ORDER BY ar.created_at DESC`
	rows, err := r.db.WithContext(ctx).Raw(query, projectID).Rows()
	if err != nil {
		return nil, fmt.Errorf("list agent runs by project: %w", err)
	}
	defer rows.Close()

	var runs []*model.AgentRun
	for rows.Next() {
		run, err := scanAgentRun(rows)
		if err != nil {
			return nil, fmt.Errorf("scan agent run: %w", err)
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

func (r *AgentRunRepository) ListBySprint(ctx context.Context, sprintID uuid.UUID) ([]*model.AgentRun, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	query := `SELECT ` + agentRunColumns + ` FROM agent_runs ar
		JOIN tasks t ON ar.task_id = t.id
		WHERE t.sprint_id = $1 ORDER BY ar.created_at DESC`
	rows, err := r.db.WithContext(ctx).Raw(query, sprintID).Rows()
	if err != nil {
		return nil, fmt.Errorf("list agent runs by sprint: %w", err)
	}
	defer rows.Close()

	var runs []*model.AgentRun
	for rows.Next() {
		run, err := scanAgentRun(rows)
		if err != nil {
			return nil, fmt.Errorf("scan agent run: %w", err)
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

func (r *AgentRunRepository) AggregateByProject(ctx context.Context, projectID uuid.UUID) (*model.CostSummaryDTO, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	query := `SELECT
		COALESCE(SUM(ar.cost_usd), 0),
		COALESCE(SUM(ar.input_tokens), 0),
		COALESCE(SUM(ar.output_tokens), 0),
		COALESCE(SUM(ar.cache_read_tokens), 0),
		COALESCE(SUM(ar.turn_count), 0),
		COUNT(ar.id)
		FROM agent_runs ar
		JOIN tasks t ON ar.task_id = t.id
		WHERE t.project_id = $1`
	s := &model.CostSummaryDTO{}
	err := r.db.WithContext(ctx).Raw(query, projectID).Row().Scan(
		&s.TotalCostUsd, &s.TotalInputTokens, &s.TotalOutputTokens,
		&s.TotalCacheReadTokens, &s.TotalTurns, &s.RunCount,
	)
	if err != nil {
		return nil, fmt.Errorf("aggregate cost by project: %w", err)
	}
	return s, nil
}

func (r *AgentRunRepository) ListByTeam(ctx context.Context, teamID uuid.UUID) ([]*model.AgentRun, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	query := `SELECT ` + agentRunColumns + ` FROM agent_runs WHERE team_id = $1 ORDER BY created_at`
	rows, err := r.db.WithContext(ctx).Raw(query, teamID).Rows()
	if err != nil {
		return nil, fmt.Errorf("list agent runs by team: %w", err)
	}
	defer rows.Close()

	var runs []*model.AgentRun
	for rows.Next() {
		run, err := scanAgentRun(rows)
		if err != nil {
			return nil, fmt.Errorf("scan agent run: %w", err)
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

func (r *AgentRunRepository) SetTeamFields(ctx context.Context, id uuid.UUID, teamID uuid.UUID, teamRole string) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	query := `UPDATE agent_runs SET team_id = $1, team_role = $2, updated_at = NOW() WHERE id = $3`
	if err := r.db.WithContext(ctx).Exec(query, teamID, teamRole, id).Error; err != nil {
		return fmt.Errorf("set agent run team fields: %w", err)
	}
	return nil
}

// AgentPerformanceRow holds aggregated performance data for a role.
type AgentPerformanceRow struct {
	RoleID         string
	TotalRuns      int
	CompletedRuns  int
	AvgCostUsd     float64
	AvgDurationSec float64
	TotalCostUsd   float64
}

func (r *AgentRunRepository) AggregatePerformance(ctx context.Context, from, to time.Time, projectID *uuid.UUID) ([]AgentPerformanceRow, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	query := `SELECT
		ar.role_id,
		COUNT(ar.id) AS total_runs,
		COUNT(ar.id) FILTER (WHERE ar.status = 'completed') AS completed_runs,
		COALESCE(AVG(ar.cost_usd), 0) AS avg_cost,
		COALESCE(AVG(EXTRACT(EPOCH FROM (COALESCE(ar.completed_at, NOW()) - ar.started_at))), 0) AS avg_duration,
		COALESCE(SUM(ar.cost_usd), 0) AS total_cost
	FROM agent_runs ar`
	args := []interface{}{from, to}
	if projectID != nil {
		query += ` JOIN tasks t ON ar.task_id = t.id WHERE ar.created_at BETWEEN $1 AND $2 AND t.project_id = $3`
		args = append(args, *projectID)
	} else {
		query += ` WHERE ar.created_at BETWEEN $1 AND $2`
	}
	query += ` GROUP BY ar.role_id ORDER BY total_runs DESC`

	rows, err := r.db.WithContext(ctx).Raw(query, args...).Rows()
	if err != nil {
		return nil, fmt.Errorf("aggregate agent performance: %w", err)
	}
	defer rows.Close()

	var results []AgentPerformanceRow
	for rows.Next() {
		var row AgentPerformanceRow
		if err := rows.Scan(&row.RoleID, &row.TotalRuns, &row.CompletedRuns, &row.AvgCostUsd, &row.AvgDurationSec, &row.TotalCostUsd); err != nil {
			return nil, fmt.Errorf("scan agent performance row: %w", err)
		}
		results = append(results, row)
	}
	return results, rows.Err()
}

func (r *AgentRunRepository) UpdateCost(ctx context.Context, id uuid.UUID, inputTokens, outputTokens, cacheReadTokens int64, costUsd float64, turnCount int) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	query := `UPDATE agent_runs SET
		input_tokens = $1, output_tokens = $2, cache_read_tokens = $3,
		cost_usd = $4, turn_count = $5, updated_at = NOW()
		WHERE id = $6`
	if err := r.db.WithContext(ctx).Exec(query, inputTokens, outputTokens, cacheReadTokens, costUsd, turnCount, id).Error; err != nil {
		return fmt.Errorf("update agent run cost: %w", err)
	}
	return nil
}
