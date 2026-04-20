//go:build integration

package secrets_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"

	"github.com/agentforge/server/internal/secrets"
	"github.com/agentforge/server/migrations"
	"github.com/agentforge/server/pkg/database"
)

// TestMain runs migrations once before all integration tests in this package.
func TestMain(m *testing.M) {
	if url := os.Getenv("TEST_POSTGRES_URL"); url != "" {
		if err := database.RunMigrations(url, migrations.FS); err != nil {
			fmt.Fprintf(os.Stderr, "migration error: %v\n", err)
			os.Exit(1)
		}
	}
	os.Exit(m.Run())
}

func TestGormRepo_EndToEnd(t *testing.T) {
	url := os.Getenv("TEST_POSTGRES_URL")
	if url == "" {
		t.Skip("TEST_POSTGRES_URL not set — skipping integration test")
	}

	db, err := database.NewPostgres(url)
	if err != nil {
		t.Fatalf("NewPostgres: %v", err)
	}
	defer func() { _ = database.ClosePostgres(db) }()

	ctx := context.Background()
	projectID := uuid.New()
	if err := db.WithContext(ctx).Exec(
		"INSERT INTO projects (id, name, slug) VALUES (?, ?, ?)",
		projectID, "secrets-test-"+projectID.String(), "slug-"+projectID.String(),
	).Error; err != nil {
		t.Fatalf("insert project: %v", err)
	}
	t.Cleanup(func() {
		_ = db.WithContext(ctx).Exec("DELETE FROM secrets WHERE project_id = ?", projectID).Error
		_ = db.WithContext(ctx).Exec("DELETE FROM projects WHERE id = ?", projectID).Error
	})

	c, err := secrets.NewCipher(testKey)
	if err != nil {
		t.Fatalf("cipher: %v", err)
	}
	repo := secrets.NewGormRepo(db)
	svc := secrets.NewService(repo, c, nil)

	actor := uuid.New()

	rec, err := svc.CreateSecret(ctx, projectID, "GITHUB_TOKEN", "ghp_xyz", "review", actor)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if rec.ID == uuid.Nil {
		t.Fatal("expected ID to be populated")
	}

	// Conflict on second create.
	if _, err := svc.CreateSecret(ctx, projectID, "GITHUB_TOKEN", "x", "", actor); err == nil {
		t.Fatal("expected name conflict")
	}

	plain, err := svc.Resolve(ctx, projectID, "GITHUB_TOKEN")
	if err != nil || plain != "ghp_xyz" {
		t.Fatalf("resolve: %q err=%v", plain, err)
	}

	if err := svc.RotateSecret(ctx, projectID, "GITHUB_TOKEN", "ghp_new", actor); err != nil {
		t.Fatalf("rotate: %v", err)
	}
	plain2, _ := svc.Resolve(ctx, projectID, "GITHUB_TOKEN")
	if plain2 != "ghp_new" {
		t.Fatalf("rotate not reflected: got %q", plain2)
	}

	if err := svc.DeleteSecret(ctx, projectID, "GITHUB_TOKEN", actor); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := svc.Resolve(ctx, projectID, "GITHUB_TOKEN"); err == nil {
		t.Fatal("expected not_found after delete")
	}

	// List should be empty.
	rows, err := svc.ListSecrets(ctx, projectID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("expected 0 rows post-delete, got %d", len(rows))
	}
}
