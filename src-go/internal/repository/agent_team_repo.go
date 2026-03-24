package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

type AgentTeamRepository struct {
	db DBTX
}

func NewAgentTeamRepository(db DBTX) *AgentTeamRepository {
	return &AgentTeamRepository{db: db}
}

const agentTeamColumns = `id, project_id, task_id, name, status, strategy,
	planner_run_id, reviewer_run_id, total_budget_usd, total_spent_usd,
	config, error_message, created_at, updated_at`

func (r *AgentTeamRepository) Create(ctx context.Context, team *model.AgentTeam) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	query := `
		INSERT INTO agent_teams (id, project_id, task_id, name, status, strategy,
			planner_run_id, reviewer_run_id, total_budget_usd, total_spent_usd,
			config, error_message, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,NOW(),NOW())
	`
	_, err := r.db.Exec(ctx, query,
		team.ID, team.ProjectID, team.TaskID, team.Name, team.Status, team.Strategy,
		team.PlannerRunID, team.ReviewerRunID, team.TotalBudgetUsd, team.TotalSpentUsd,
		team.Config, team.ErrorMessage,
	)
	if err != nil {
		return fmt.Errorf("create agent team: %w", err)
	}
	return nil
}

func (r *AgentTeamRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.AgentTeam, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	query := `SELECT ` + agentTeamColumns + ` FROM agent_teams WHERE id = $1`
	team := &model.AgentTeam{}
	err := r.db.QueryRow(ctx, query, id).Scan(
		&team.ID, &team.ProjectID, &team.TaskID, &team.Name, &team.Status, &team.Strategy,
		&team.PlannerRunID, &team.ReviewerRunID, &team.TotalBudgetUsd, &team.TotalSpentUsd,
		&team.Config, &team.ErrorMessage, &team.CreatedAt, &team.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get agent team by id: %w", err)
	}
	return team, nil
}

func (r *AgentTeamRepository) GetByTask(ctx context.Context, taskID uuid.UUID) (*model.AgentTeam, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	query := `SELECT ` + agentTeamColumns + ` FROM agent_teams WHERE task_id = $1 ORDER BY created_at DESC LIMIT 1`
	team := &model.AgentTeam{}
	err := r.db.QueryRow(ctx, query, taskID).Scan(
		&team.ID, &team.ProjectID, &team.TaskID, &team.Name, &team.Status, &team.Strategy,
		&team.PlannerRunID, &team.ReviewerRunID, &team.TotalBudgetUsd, &team.TotalSpentUsd,
		&team.Config, &team.ErrorMessage, &team.CreatedAt, &team.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get agent team by task: %w", err)
	}
	return team, nil
}

func (r *AgentTeamRepository) ListByProject(ctx context.Context, projectID uuid.UUID) ([]*model.AgentTeam, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	query := `SELECT ` + agentTeamColumns + ` FROM agent_teams WHERE project_id = $1 ORDER BY created_at DESC`
	rows, err := r.db.Query(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("list agent teams by project: %w", err)
	}
	defer rows.Close()

	var teams []*model.AgentTeam
	for rows.Next() {
		team := &model.AgentTeam{}
		if err := rows.Scan(
			&team.ID, &team.ProjectID, &team.TaskID, &team.Name, &team.Status, &team.Strategy,
			&team.PlannerRunID, &team.ReviewerRunID, &team.TotalBudgetUsd, &team.TotalSpentUsd,
			&team.Config, &team.ErrorMessage, &team.CreatedAt, &team.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan agent team: %w", err)
		}
		teams = append(teams, team)
	}
	return teams, rows.Err()
}

func (r *AgentTeamRepository) ListActive(ctx context.Context) ([]*model.AgentTeam, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	query := `SELECT ` + agentTeamColumns + ` FROM agent_teams WHERE status IN ('pending', 'planning', 'executing', 'reviewing') ORDER BY created_at`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list active agent teams: %w", err)
	}
	defer rows.Close()

	var teams []*model.AgentTeam
	for rows.Next() {
		team := &model.AgentTeam{}
		if err := rows.Scan(
			&team.ID, &team.ProjectID, &team.TaskID, &team.Name, &team.Status, &team.Strategy,
			&team.PlannerRunID, &team.ReviewerRunID, &team.TotalBudgetUsd, &team.TotalSpentUsd,
			&team.Config, &team.ErrorMessage, &team.CreatedAt, &team.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan agent team: %w", err)
		}
		teams = append(teams, team)
	}
	return teams, rows.Err()
}

func (r *AgentTeamRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	query := `UPDATE agent_teams SET status = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.Exec(ctx, query, status, id)
	if err != nil {
		return fmt.Errorf("update agent team status: %w", err)
	}
	return nil
}

func (r *AgentTeamRepository) UpdateStatusWithError(ctx context.Context, id uuid.UUID, status, errorMessage string) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	query := `UPDATE agent_teams SET status = $1, error_message = $2, updated_at = NOW() WHERE id = $3`
	_, err := r.db.Exec(ctx, query, status, errorMessage, id)
	if err != nil {
		return fmt.Errorf("update agent team status with error: %w", err)
	}
	return nil
}

func (r *AgentTeamRepository) UpdateSpent(ctx context.Context, id uuid.UUID, spent float64) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	query := `UPDATE agent_teams SET total_spent_usd = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.Exec(ctx, query, spent, id)
	if err != nil {
		return fmt.Errorf("update agent team spent: %w", err)
	}
	return nil
}

func (r *AgentTeamRepository) SetPlannerRun(ctx context.Context, id uuid.UUID, plannerRunID uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	query := `UPDATE agent_teams SET planner_run_id = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.Exec(ctx, query, plannerRunID, id)
	if err != nil {
		return fmt.Errorf("set agent team planner run: %w", err)
	}
	return nil
}

func (r *AgentTeamRepository) SetReviewerRun(ctx context.Context, id uuid.UUID, reviewerRunID uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	query := `UPDATE agent_teams SET reviewer_run_id = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.Exec(ctx, query, reviewerRunID, id)
	if err != nil {
		return fmt.Errorf("set agent team reviewer run: %w", err)
	}
	return nil
}
