//go:build integration

package database_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/react-go-quick-starter/server/migrations"
	"github.com/react-go-quick-starter/server/pkg/database"
)

func TestRunMigrations_AgentRunsIncludesUpdatedAt(t *testing.T) {
	url := os.Getenv("TEST_POSTGRES_URL")
	if url == "" {
		t.Skip("TEST_POSTGRES_URL not set")
	}

	if err := database.RunMigrations(url, migrations.FS); err != nil {
		t.Fatalf("RunMigrations() error = %v", err)
	}

	pool, err := database.NewPostgres(url)
	if err != nil {
		t.Fatalf("NewPostgres() error = %v", err)
	}
	defer func() {
		if err := database.ClosePostgres(pool); err != nil {
			t.Fatalf("ClosePostgres() error = %v", err)
		}
	}()

	sqlDB, err := pool.DB()
	if err != nil {
		t.Fatalf("DB() error = %v", err)
	}

	for _, columnName := range []string{"provider", "model", "updated_at"} {
		var exists bool
		query := `
			SELECT EXISTS (
				SELECT 1
				FROM information_schema.columns
				WHERE table_name = 'agent_runs'
				  AND column_name = $1
			)
		`
		if err := sqlDB.QueryRowContext(context.Background(), query, columnName).Scan(&exists); err != nil {
			t.Fatalf("scan agent_runs.%s existence: %v", columnName, err)
		}
		if !exists {
			t.Fatalf("expected agent_runs.%s column after migrations", columnName)
		}
	}

	var triggerCount int
	if err := sqlDB.QueryRowContext(
		context.Background(),
		`SELECT COUNT(*) FROM pg_trigger WHERE tgname = 'agent_runs_updated_at_trigger'`,
	).Scan(&triggerCount); err != nil {
		t.Fatalf("scan agent_runs updated_at trigger count: %v", err)
	}
	if triggerCount == 0 {
		t.Fatal("expected agent_runs_updated_at_trigger to be installed")
	}

	fmt.Fprint(os.Stdout, "")
}
