package qcrepo

import (
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// openFoundationRepoTestDB opens an in-memory SQLite database seeded
// with the given GORM models. Copied verbatim from
// internal/repository/foundation_repo_test_helpers_test.go so the
// plugin's repo tests don't depend on core test helpers.
func openFoundationRepoTestDB(t *testing.T, models ...any) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	if err := db.AutoMigrate(models...); err != nil {
		t.Fatalf("migrate foundation models: %v", err)
	}
	return db
}
