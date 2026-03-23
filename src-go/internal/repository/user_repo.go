package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/react-go-quick-starter/server/internal/model"
)

// DBTX is the interface satisfied by *pgxpool.Pool, *pgx.Conn, and pgx.Tx.
type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type UserRepository struct {
	db DBTX
}

func NewUserRepository(db DBTX) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, user *model.User) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	query := `
		INSERT INTO users (id, email, password, name, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
	`
	_, err := r.db.Exec(ctx, query, user.ID, user.Email, user.Password, user.Name)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	query := `SELECT id, email, password, name, created_at, updated_at FROM users WHERE email = $1`
	user := &model.User{}
	err := r.db.QueryRow(ctx, query, email).Scan(
		&user.ID, &user.Email, &user.Password, &user.Name, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return user, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	query := `SELECT id, email, password, name, created_at, updated_at FROM users WHERE id = $1`
	user := &model.User{}
	err := r.db.QueryRow(ctx, query, id).Scan(
		&user.ID, &user.Email, &user.Password, &user.Name, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return user, nil
}

func (r *UserRepository) UpdateName(ctx context.Context, id uuid.UUID, name string) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	query := `UPDATE users SET name = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.Exec(ctx, query, name, id)
	if err != nil {
		return fmt.Errorf("update user name: %w", err)
	}
	return nil
}
