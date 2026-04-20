// Command backfill-trigger-source is a one-shot maintenance utility that
// fills the workflow_triggers.created_via column with 'dag_node' for any
// pre-Spec-1C row whose value was inserted before migration 068 added the
// DEFAULT 'dag_node' clause.
//
// Migration 068 is idempotent and forward-only: it adds the column with a
// NOT NULL DEFAULT 'dag_node', so existing rows already inherit the
// correct value when the migration runs cleanly. This script is purely
// defensive — useful when a deployment skipped the migration window or
// when an out-of-band write inserted a row with NULL.
//
// Invoke after migrations have completed:
//
//	cd src-go && go run ./cmd/backfill-trigger-source
//
// The script is safe to re-run: rows with the correct value are not
// touched (the WHERE clause filters NULL or empty string only).
package main

import (
	"context"
	"log"
	"time"

	"github.com/react-go-quick-starter/server/internal/config"
	"github.com/react-go-quick-starter/server/pkg/database"
)

func main() {
	cfg := config.Load()
	if cfg.PostgresURL == "" {
		log.Fatal("POSTGRES_URL is required")
	}
	db, err := database.NewPostgres(cfg.PostgresURL)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer func() {
		if cerr := database.ClosePostgres(db); cerr != nil {
			log.Printf("close db: %v", cerr)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	res := db.WithContext(ctx).Exec(
		"UPDATE workflow_triggers SET created_via = 'dag_node' WHERE created_via IS NULL OR created_via = ''",
	)
	if res.Error != nil {
		log.Fatalf("backfill: %v", res.Error)
	}
	log.Printf("backfill complete: %d row(s) updated", res.RowsAffected)
}
