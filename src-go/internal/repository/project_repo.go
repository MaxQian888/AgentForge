package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

type ProjectRepository struct {
	db DBTX
}

func NewProjectRepository(db DBTX) *ProjectRepository {
	return &ProjectRepository{db: db}
}

func (r *ProjectRepository) Create(ctx context.Context, project *model.Project) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	query := `
		INSERT INTO projects (id, name, slug, description, repo_url, default_branch, settings, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
	`
	_, err := r.db.Exec(ctx, query, project.ID, project.Name, project.Slug, project.Description,
		project.RepoURL, project.DefaultBranch, project.Settings)
	if err != nil {
		return fmt.Errorf("create project: %w", err)
	}
	return nil
}

func (r *ProjectRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Project, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	query := `SELECT id, name, slug, description, repo_url, default_branch, settings, created_at, updated_at
		FROM projects WHERE id = $1`
	p := &model.Project{}
	err := r.db.QueryRow(ctx, query, id).Scan(
		&p.ID, &p.Name, &p.Slug, &p.Description, &p.RepoURL, &p.DefaultBranch,
		&p.Settings, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get project by id: %w", err)
	}
	return p, nil
}

func (r *ProjectRepository) GetBySlug(ctx context.Context, slug string) (*model.Project, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	query := `SELECT id, name, slug, description, repo_url, default_branch, settings, created_at, updated_at
		FROM projects WHERE slug = $1`
	p := &model.Project{}
	err := r.db.QueryRow(ctx, query, slug).Scan(
		&p.ID, &p.Name, &p.Slug, &p.Description, &p.RepoURL, &p.DefaultBranch,
		&p.Settings, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get project by slug: %w", err)
	}
	return p, nil
}

func (r *ProjectRepository) List(ctx context.Context) ([]*model.Project, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	query := `SELECT id, name, slug, description, repo_url, default_branch, settings, created_at, updated_at
		FROM projects ORDER BY created_at DESC`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	var projects []*model.Project
	for rows.Next() {
		p := &model.Project{}
		if err := rows.Scan(&p.ID, &p.Name, &p.Slug, &p.Description, &p.RepoURL,
			&p.DefaultBranch, &p.Settings, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

func (r *ProjectRepository) Update(ctx context.Context, id uuid.UUID, req *model.UpdateProjectRequest) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	query := `UPDATE projects SET
		name = COALESCE($1, name),
		description = COALESCE($2, description),
		repo_url = COALESCE($3, repo_url),
		updated_at = NOW()
		WHERE id = $4`
	_, err := r.db.Exec(ctx, query, req.Name, req.Description, req.RepoURL, id)
	if err != nil {
		return fmt.Errorf("update project: %w", err)
	}
	return nil
}
