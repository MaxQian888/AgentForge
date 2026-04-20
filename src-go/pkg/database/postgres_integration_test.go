//go:build integration

package database_test

import (
	"os"
	"testing"

	"github.com/agentforge/server/pkg/database"
)

func TestNewPostgres_Connect(t *testing.T) {
	url := os.Getenv("TEST_POSTGRES_URL")
	if url == "" {
		t.Skip("TEST_POSTGRES_URL not set — skipping integration test")
	}

	pool, err := database.NewPostgres(url)
	if err != nil {
		t.Fatalf("NewPostgres() error: %v", err)
	}
	defer func() {
		if err := database.ClosePostgres(pool); err != nil {
			t.Fatalf("ClosePostgres() error: %v", err)
		}
	}()

	// Ping is already done inside NewPostgres; reaching here means it worked.
	t.Log("PostgreSQL connection OK")
}
