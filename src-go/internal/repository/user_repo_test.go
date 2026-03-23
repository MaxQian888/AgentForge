package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
)

func TestNewUserRepository(t *testing.T) {
	repo := repository.NewUserRepository(nil)
	if repo == nil {
		t.Fatal("expected non-nil UserRepository")
	}
}

// --- Nil DB tests ---

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

// --- Mock DB tests ---

func TestUserRepository_Create_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	repo := repository.NewUserRepository(mock)
	user := &model.User{
		ID:       uuid.New(),
		Email:    "test@example.com",
		Password: "hash",
		Name:     "Test",
	}

	mock.ExpectExec("INSERT INTO users").
		WithArgs(user.ID, user.Email, user.Password, user.Name).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	err = repo.Create(context.Background(), user)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestUserRepository_Create_Error(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	repo := repository.NewUserRepository(mock)
	user := &model.User{
		ID:       uuid.New(),
		Email:    "test@example.com",
		Password: "hash",
		Name:     "Test",
	}

	mock.ExpectExec("INSERT INTO users").
		WithArgs(user.ID, user.Email, user.Password, user.Name).
		WillReturnError(context.DeadlineExceeded)

	err = repo.Create(context.Background(), user)
	if err == nil {
		t.Error("expected error")
	}
}

func TestUserRepository_GetByEmail_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	id := uuid.New()
	now := time.Now()

	repo := repository.NewUserRepository(mock)
	mock.ExpectQuery("SELECT .+ FROM users WHERE email").
		WithArgs("test@example.com").
		WillReturnRows(
			pgxmock.NewRows([]string{"id", "email", "password", "name", "created_at", "updated_at"}).
				AddRow(id, "test@example.com", "hash", "Test", now, now),
		)

	user, err := repo.GetByEmail(context.Background(), "test@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.Email != "test@example.com" {
		t.Errorf("expected email test@example.com, got %s", user.Email)
	}
	if user.ID != id {
		t.Errorf("expected ID %s, got %s", id, user.ID)
	}
}

func TestUserRepository_GetByEmail_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	repo := repository.NewUserRepository(mock)
	mock.ExpectQuery("SELECT .+ FROM users WHERE email").
		WithArgs("missing@example.com").
		WillReturnRows(pgxmock.NewRows([]string{"id", "email", "password", "name", "created_at", "updated_at"}))

	_, err = repo.GetByEmail(context.Background(), "missing@example.com")
	if err == nil {
		t.Error("expected error for missing user")
	}
}

func TestUserRepository_GetByID_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	id := uuid.New()
	now := time.Now()

	repo := repository.NewUserRepository(mock)
	mock.ExpectQuery("SELECT .+ FROM users WHERE id").
		WithArgs(id).
		WillReturnRows(
			pgxmock.NewRows([]string{"id", "email", "password", "name", "created_at", "updated_at"}).
				AddRow(id, "test@example.com", "hash", "Test", now, now),
		)

	user, err := repo.GetByID(context.Background(), id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.ID != id {
		t.Errorf("expected ID %s, got %s", id, user.ID)
	}
}

func TestUserRepository_GetByID_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	repo := repository.NewUserRepository(mock)
	id := uuid.New()
	mock.ExpectQuery("SELECT .+ FROM users WHERE id").
		WithArgs(id).
		WillReturnRows(pgxmock.NewRows([]string{"id", "email", "password", "name", "created_at", "updated_at"}))

	_, err = repo.GetByID(context.Background(), id)
	if err == nil {
		t.Error("expected error for missing user")
	}
}

func TestUserRepository_UpdateName_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	repo := repository.NewUserRepository(mock)
	id := uuid.New()

	mock.ExpectExec("UPDATE users SET name").
		WithArgs("New Name", id).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err = repo.UpdateName(context.Background(), id, "New Name")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestUserRepository_UpdateName_Error(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	repo := repository.NewUserRepository(mock)
	id := uuid.New()

	mock.ExpectExec("UPDATE users SET name").
		WithArgs("New Name", id).
		WillReturnError(context.DeadlineExceeded)

	err = repo.UpdateName(context.Background(), id, "New Name")
	if err == nil {
		t.Error("expected error")
	}
}
