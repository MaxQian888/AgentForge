package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

type SprintRepository struct {
	db DBTX
}

func NewSprintRepository(db DBTX) *SprintRepository {
	return &SprintRepository{db: db}
}

func (r *SprintRepository) Create(ctx context.Context, sprint *model.Sprint) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	query := `
		INSERT INTO sprints (id, project_id, name, start_date, end_date, status, total_budget_usd, spent_usd, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
	`
	_, err := r.db.Exec(ctx, query, sprint.ID, sprint.ProjectID, sprint.Name,
		sprint.StartDate, sprint.EndDate, sprint.Status, sprint.TotalBudgetUsd, sprint.SpentUsd)
	if err != nil {
		return fmt.Errorf("create sprint: %w", err)
	}
	return nil
}

func (r *SprintRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Sprint, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	query := `SELECT id, project_id, name, start_date, end_date, status, total_budget_usd, spent_usd, created_at, updated_at
		FROM sprints WHERE id = $1`
	s := &model.Sprint{}
	err := r.db.QueryRow(ctx, query, id).Scan(
		&s.ID, &s.ProjectID, &s.Name, &s.StartDate, &s.EndDate,
		&s.Status, &s.TotalBudgetUsd, &s.SpentUsd, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get sprint by id: %w", err)
	}
	return s, nil
}

func (r *SprintRepository) ListByProject(ctx context.Context, projectID uuid.UUID) ([]*model.Sprint, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	query := `SELECT id, project_id, name, start_date, end_date, status, total_budget_usd, spent_usd, created_at, updated_at
		FROM sprints WHERE project_id = $1 ORDER BY start_date DESC`
	rows, err := r.db.Query(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("list sprints: %w", err)
	}
	defer rows.Close()

	var sprints []*model.Sprint
	for rows.Next() {
		s := &model.Sprint{}
		if err := rows.Scan(&s.ID, &s.ProjectID, &s.Name, &s.StartDate, &s.EndDate,
			&s.Status, &s.TotalBudgetUsd, &s.SpentUsd, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan sprint: %w", err)
		}
		sprints = append(sprints, s)
	}
	return sprints, rows.Err()
}

func (r *SprintRepository) GetActive(ctx context.Context, projectID uuid.UUID) (*model.Sprint, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	query := `SELECT id, project_id, name, start_date, end_date, status, total_budget_usd, spent_usd, created_at, updated_at
		FROM sprints WHERE project_id = $1 AND status = 'active' LIMIT 1`
	s := &model.Sprint{}
	err := r.db.QueryRow(ctx, query, projectID).Scan(
		&s.ID, &s.ProjectID, &s.Name, &s.StartDate, &s.EndDate,
		&s.Status, &s.TotalBudgetUsd, &s.SpentUsd, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get active sprint: %w", err)
	}
	return s, nil
}
