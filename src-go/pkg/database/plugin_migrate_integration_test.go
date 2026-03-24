//go:build integration

package database_test

import (
	"context"
	"os"
	"testing"

	"github.com/react-go-quick-starter/server/migrations"
	"github.com/react-go-quick-starter/server/pkg/database"
)

func TestRunMigrations_PluginControlPlaneTablesPresent(t *testing.T) {
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

	for _, tableName := range []string{"plugins", "plugin_instances", "plugin_events"} {
		var exists bool
		query := `
			SELECT EXISTS (
				SELECT 1
				FROM information_schema.tables
				WHERE table_name = $1
			)
		`
		if err := sqlDB.QueryRowContext(context.Background(), query, tableName).Scan(&exists); err != nil {
			t.Fatalf("scan %s existence: %v", tableName, err)
		}
		if !exists {
			t.Fatalf("expected table %s after migrations", tableName)
		}
	}
}
