package repository_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
)

func TestNewUserRepository(t *testing.T) {
	repo := repository.NewUserRepository(nil)
	if repo == nil {
		t.Fatal("expected non-nil UserRepository")
	}
}

func TestUserRepository_Create_NilDB(t *testing.T) {
	repo := repository.NewUserRepository(nil)
	err := repo.Create(context.Background(), &model.User{ID: uuid.New(), Email: "test@example.com"})
	if err != repository.ErrDatabaseUnavailable {
		t.Errorf("expected ErrDatabaseUnavailable, got %v", err)
	}
}

func TestUserRepository_GetByEmail_NilDB(t *testing.T) {
	repo := repository.NewUserRepository(nil)
	_, err := repo.GetByEmail(context.Background(), "test@example.com")
	if err != repository.ErrDatabaseUnavailable {
		t.Errorf("expected ErrDatabaseUnavailable, got %v", err)
	}
}

func TestUserRepository_GetByID_NilDB(t *testing.T) {
	repo := repository.NewUserRepository(nil)
	_, err := repo.GetByID(context.Background(), uuid.New())
	if err != repository.ErrDatabaseUnavailable {
		t.Errorf("expected ErrDatabaseUnavailable, got %v", err)
	}
}

func TestUserRepository_UpdateName_NilDB(t *testing.T) {
	repo := repository.NewUserRepository(nil)
	err := repo.UpdateName(context.Background(), uuid.New(), "newname")
	if err != repository.ErrDatabaseUnavailable {
		t.Errorf("expected ErrDatabaseUnavailable, got %v", err)
	}
}
