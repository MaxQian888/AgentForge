//go:build integration

package repository_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
	"github.com/react-go-quick-starter/server/migrations"
	"github.com/react-go-quick-starter/server/pkg/database"
)

func TestAgentMemoryRepository_TimeRangeAndRetention_Postgres(t *testing.T) {
	url := os.Getenv("TEST_POSTGRES_URL")
	if url == "" {
		t.Skip("TEST_POSTGRES_URL not set - skipping integration test")
	}

	if err := database.RunMigrations(url, migrations.FS); err != nil {
		t.Fatalf("RunMigrations() error = %v", err)
	}

	db, err := database.NewPostgres(url)
	if err != nil {
		t.Fatalf("NewPostgres() error = %v", err)
	}
	defer func() {
		if err := database.ClosePostgres(db); err != nil {
			t.Fatalf("ClosePostgres() error = %v", err)
		}
	}()

	repo := repository.NewAgentMemoryRepository(db)
	projectID := uuid.New()
	now := time.Now().UTC()
	projectSlug := "agent-memory-" + projectID.String()[:8]

	if err := db.WithContext(context.Background()).Exec(
		`INSERT INTO projects (id, name, slug, description, repo_url, default_branch, settings) VALUES (?, ?, ?, '', '', 'main', '{}')`,
		projectID,
		"Agent Memory Integration",
		projectSlug,
	).Error; err != nil {
		t.Fatalf("seed project: %v", err)
	}
	t.Cleanup(func() {
		_ = db.WithContext(context.Background()).Exec("DELETE FROM projects WHERE id = ?", projectID).Error
	})

	memories := []*model.AgentMemory{
		{
			ID:        uuid.New(),
			ProjectID: projectID,
			Scope:     model.MemoryScopeProject,
			Category:  model.MemoryCategoryEpisodic,
			Key:       "old-turn",
			Content:   "old turn",
			CreatedAt: now.Add(-48 * time.Hour),
			UpdatedAt: now.Add(-48 * time.Hour),
		},
		{
			ID:        uuid.New(),
			ProjectID: projectID,
			Scope:     model.MemoryScopeProject,
			Category:  model.MemoryCategoryEpisodic,
			Key:       "recent-turn",
			Content:   "recent turn",
			CreatedAt: now.Add(-time.Hour),
			UpdatedAt: now.Add(-time.Hour),
		},
	}

	ctx := context.Background()
	for _, memory := range memories {
		if err := repo.Create(ctx, memory); err != nil {
			t.Fatalf("Create(%s) error = %v", memory.Key, err)
		}
		t.Cleanup(func() {
			_ = db.WithContext(ctx).Exec("DELETE FROM agent_memory WHERE id = ?", memory.ID).Error
		})
	}

	start := now.Add(-2 * time.Hour)
	end := now
	got, err := repo.ListByProjectAndTimeRange(ctx, projectID, model.MemoryCategoryEpisodic, model.MemoryScopeProject, "", &start, &end, 10)
	if err != nil {
		t.Fatalf("ListByProjectAndTimeRange() error = %v", err)
	}
	if len(got) != 1 || got[0].Key != "recent-turn" {
		t.Fatalf("ListByProjectAndTimeRange() = %#v, want recent-turn only", got)
	}

	deleted, err := repo.DeleteOlderThan(ctx, projectID, model.MemoryCategoryEpisodic, now.Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("DeleteOlderThan() error = %v", err)
	}
	if deleted != 1 {
		t.Fatalf("DeleteOlderThan() deleted = %d, want 1", deleted)
	}

	fmt.Fprint(os.Stdout, "")
}
