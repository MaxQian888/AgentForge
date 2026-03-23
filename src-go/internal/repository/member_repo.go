package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

type MemberRepository struct {
	db DBTX
}

func NewMemberRepository(db DBTX) *MemberRepository {
	return &MemberRepository{db: db}
}

func (r *MemberRepository) Create(ctx context.Context, member *model.Member) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	query := `
		INSERT INTO members (id, project_id, user_id, name, type, role, email, avatar_url, agent_config, skills, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW(), NOW())
	`
	_, err := r.db.Exec(ctx, query, member.ID, member.ProjectID, member.UserID, member.Name,
		member.Type, member.Role, member.Email, member.AvatarURL, member.AgentConfig, member.Skills, member.IsActive)
	if err != nil {
		return fmt.Errorf("create member: %w", err)
	}
	return nil
}

func (r *MemberRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Member, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	query := `SELECT id, project_id, user_id, name, type, role, email, avatar_url, agent_config, skills, is_active, created_at, updated_at
		FROM members WHERE id = $1`
	m := &model.Member{}
	err := r.db.QueryRow(ctx, query, id).Scan(
		&m.ID, &m.ProjectID, &m.UserID, &m.Name, &m.Type, &m.Role, &m.Email,
		&m.AvatarURL, &m.AgentConfig, &m.Skills, &m.IsActive, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get member by id: %w", err)
	}
	return m, nil
}

func (r *MemberRepository) ListByProject(ctx context.Context, projectID uuid.UUID) ([]*model.Member, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	query := `SELECT id, project_id, user_id, name, type, role, email, avatar_url, agent_config, skills, is_active, created_at, updated_at
		FROM members WHERE project_id = $1 ORDER BY created_at`
	rows, err := r.db.Query(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}
	defer rows.Close()

	var members []*model.Member
	for rows.Next() {
		m := &model.Member{}
		if err := rows.Scan(&m.ID, &m.ProjectID, &m.UserID, &m.Name, &m.Type, &m.Role,
			&m.Email, &m.AvatarURL, &m.AgentConfig, &m.Skills, &m.IsActive, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan member: %w", err)
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

func (r *MemberRepository) Update(ctx context.Context, id uuid.UUID, req *model.UpdateMemberRequest) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	query := `UPDATE members SET
		name = COALESCE($1, name),
		role = COALESCE($2, role),
		email = COALESCE($3, email),
		agent_config = COALESCE($4, agent_config),
		is_active = COALESCE($5, is_active),
		updated_at = NOW()
		WHERE id = $6`
	_, err := r.db.Exec(ctx, query, req.Name, req.Role, req.Email, req.AgentConfig, req.IsActive, id)
	if err != nil {
		return fmt.Errorf("update member: %w", err)
	}
	return nil
}

func (r *MemberRepository) GetByUserAndProject(ctx context.Context, userID, projectID uuid.UUID) (*model.Member, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	query := `SELECT id, project_id, user_id, name, type, role, email, avatar_url, agent_config, skills, is_active, created_at, updated_at
		FROM members WHERE user_id = $1 AND project_id = $2`
	m := &model.Member{}
	err := r.db.QueryRow(ctx, query, userID, projectID).Scan(
		&m.ID, &m.ProjectID, &m.UserID, &m.Name, &m.Type, &m.Role, &m.Email,
		&m.AvatarURL, &m.AgentConfig, &m.Skills, &m.IsActive, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get member by user and project: %w", err)
	}
	return m, nil
}
