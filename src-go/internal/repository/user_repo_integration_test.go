// src-go/internal/repository/user_repo_integration_test.go
//go:build integration

package repository_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/repository"
	"github.com/agentforge/server/migrations"
	"github.com/agentforge/server/pkg/database"
	"github.com/google/uuid"
)

// TestMain runs migrations once before all tests in this package.
func TestMain(m *testing.M) {
	if url := os.Getenv("TEST_POSTGRES_URL"); url != "" {
		if err := database.RunMigrations(url, migrations.FS); err != nil {
			fmt.Fprintf(os.Stderr, "migration error: %v\n", err)
			os.Exit(1)
		}
	}
	os.Exit(m.Run())
}

func TestUserRepository_CreateAndGetByEmail(t *testing.T) {
	url := os.Getenv("TEST_POSTGRES_URL")
	if url == "" {
		t.Skip("TEST_POSTGRES_URL not set — skipping integration test")
	}

	db, err := database.NewPostgres(url)
	if err != nil {
		t.Fatalf("NewPostgres() error: %v", err)
	}
	defer func() {
		_ = database.ClosePostgres(db)
	}()

	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	user := &model.User{
		ID:       uuid.New(),
		Email:    "ci-test-" + uuid.NewString() + "@example.com",
		Password: "bcrypt-hash-placeholder",
		Name:     "CI Test User",
	}

	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	// Cleanup regardless of test outcome.
	t.Cleanup(func() {
		_ = db.WithContext(ctx).Exec("DELETE FROM users WHERE id = ?", user.ID).Error
	})

	found, err := repo.GetByEmail(ctx, user.Email)
	if err != nil {
		t.Fatalf("GetByEmail() error: %v", err)
	}
	if found.ID != user.ID {
		t.Errorf("ID: want %s, got %s", user.ID, found.ID)
	}
	if found.Name != user.Name {
		t.Errorf("Name: want %q, got %q", user.Name, found.Name)
	}
}
