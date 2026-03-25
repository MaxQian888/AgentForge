package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"gorm.io/gorm"
)

type AgentTeamRepository struct {
	db *gorm.DB
}

func NewAgentTeamRepository(db *gorm.DB) *AgentTeamRepository {
	return &AgentTeamRepository{db: db}
}

const agentTeamColumns = `id, project_id, task_id, name, status, strategy,
	planner_run_id, reviewer_run_id, total_budget_usd, total_spent_usd,
	config, error_message, created_at, updated_at`

func scanAgentTeam(row interface{ Scan(dest ...any) error }) (*model.AgentTeam, error) {
	team := &model.AgentTeam{}
	err := row.Scan(
		&team.ID, &team.ProjectID, &team.TaskID, &team.Name, &team.Status, &team.Strategy,
		&team.PlannerRunID, &team.ReviewerRunID, &team.TotalBudgetUsd, &team.TotalSpentUsd,
		&team.Config, &team.ErrorMessage, &team.CreatedAt, &team.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return team, nil
}

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
	if err := r.db.WithContext(ctx).Exec(
		query,
		team.ID, team.ProjectID, team.TaskID, team.Name, team.Status, team.Strategy,
		team.PlannerRunID, team.ReviewerRunID, team.TotalBudgetUsd, team.TotalSpentUsd,
		team.Config, team.ErrorMessage,
	).Error; err != nil {
		return fmt.Errorf("create agent team: %w", err)
	}
	return nil
}

func (r *AgentTeamRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.AgentTeam, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	query := `SELECT ` + agentTeamColumns + ` FROM agent_teams WHERE id = $1`
	team, err := scanAgentTeam(r.db.WithContext(ctx).Raw(query, id).Row())
	if err != nil {
		return nil, fmt.Errorf("get agent team by id: %w", normalizeRepositoryError(err))
	}
	return team, nil
}

func (r *AgentTeamRepository) GetByTask(ctx context.Context, taskID uuid.UUID) (*model.AgentTeam, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	query := `SELECT ` + agentTeamColumns + ` FROM agent_teams WHERE task_id = $1 ORDER BY created_at DESC LIMIT 1`
	team, err := scanAgentTeam(r.db.WithContext(ctx).Raw(query, taskID).Row())
	if err != nil {
		return nil, fmt.Errorf("get agent team by task: %w", normalizeRepositoryError(err))
	}
	return team, nil
}

func (r *AgentTeamRepository) ListByProject(ctx context.Context, projectID uuid.UUID) ([]*model.AgentTeam, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	query := `SELECT ` + agentTeamColumns + ` FROM agent_teams WHERE project_id = $1 ORDER BY created_at DESC`
	rows, err := r.db.WithContext(ctx).Raw(query, projectID).Rows()
	if err != nil {
		return nil, fmt.Errorf("list agent teams by project: %w", err)
	}
	defer rows.Close()

	var teams []*model.AgentTeam
	for rows.Next() {
		team, err := scanAgentTeam(rows)
		if err != nil {
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
	rows, err := r.db.WithContext(ctx).Raw(query).Rows()
	if err != nil {
		return nil, fmt.Errorf("list active agent teams: %w", err)
	}
	defer rows.Close()

	var teams []*model.AgentTeam
	for rows.Next() {
		team, err := scanAgentTeam(rows)
		if err != nil {
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
	if err := r.db.WithContext(ctx).Exec(query, status, id).Error; err != nil {
		return fmt.Errorf("update agent team status: %w", err)
	}
	return nil
}

func (r *AgentTeamRepository) UpdateStatusWithError(ctx context.Context, id uuid.UUID, status, errorMessage string) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	query := `UPDATE agent_teams SET status = $1, error_message = $2, updated_at = NOW() WHERE id = $3`
	if err := r.db.WithContext(ctx).Exec(query, status, errorMessage, id).Error; err != nil {
		return fmt.Errorf("update agent team status with error: %w", err)
	}
	return nil
}

func (r *AgentTeamRepository) UpdateSpent(ctx context.Context, id uuid.UUID, spent float64) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	query := `UPDATE agent_teams SET total_spent_usd = $1, updated_at = NOW() WHERE id = $2`
	if err := r.db.WithContext(ctx).Exec(query, spent, id).Error; err != nil {
		return fmt.Errorf("update agent team spent: %w", err)
	}
	return nil
}

func (r *AgentTeamRepository) SetPlannerRun(ctx context.Context, id uuid.UUID, plannerRunID uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	query := `UPDATE agent_teams SET planner_run_id = $1, updated_at = NOW() WHERE id = $2`
	if err := r.db.WithContext(ctx).Exec(query, plannerRunID, id).Error; err != nil {
		return fmt.Errorf("set agent team planner run: %w", err)
	}
	return nil
}

func (r *AgentTeamRepository) SetReviewerRun(ctx context.Context, id uuid.UUID, reviewerRunID uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	query := `UPDATE agent_teams SET reviewer_run_id = $1, updated_at = NOW() WHERE id = $2`
	if err := r.db.WithContext(ctx).Exec(query, reviewerRunID, id).Error; err != nil {
		return fmt.Errorf("set agent team reviewer run: %w", err)
	}
	return nil
}
