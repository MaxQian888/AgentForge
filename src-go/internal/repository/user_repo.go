package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"gorm.io/gorm"
)

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, user *model.User) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).Create(newUserRecord(user)).Error; err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}

	var record userRecord
	if err := r.db.WithContext(ctx).Where("email = ?", email).Take(&record).Error; err != nil {
		return nil, fmt.Errorf("get user by email: %w", normalizeRepositoryError(err))
	}
	return record.toModel(), nil
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}

	var record userRecord
	if err := r.db.WithContext(ctx).Where("id = ?", id).Take(&record).Error; err != nil {
		return nil, fmt.Errorf("get user by id: %w", normalizeRepositoryError(err))
	}
	return record.toModel(), nil
}

func (r *UserRepository) UpdateName(ctx context.Context, id uuid.UUID, name string) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).
		Model(&userRecord{}).
		Where("id = ?", id).
		Updates(map[string]any{"name": name}).
		Error; err != nil {
		return fmt.Errorf("update user name: %w", err)
	}
	return nil
}
