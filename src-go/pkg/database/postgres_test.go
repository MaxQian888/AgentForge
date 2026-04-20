package database_test

import (
	"testing"

	"github.com/agentforge/server/pkg/database"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestNewPostgres_EmptyURL(t *testing.T) {
	_, err := database.NewPostgres("")
	if err == nil {
		t.Fatal("expected error for empty URL")
	}
}

func TestNewPostgres_InvalidURL(t *testing.T) {
	_, err := database.NewPostgres("not-a-valid-url")
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestNewPostgres_UnreachableHost(t *testing.T) {
	// Valid URL format but unreachable host
	_, err := database.NewPostgres("postgres://user:pass@127.0.0.1:59999/dbname?connect_timeout=1")
	if err == nil {
		t.Fatal("expected error for unreachable postgres")
	}
}

func TestClosePostgres_ClosesSQLHandle(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:close-postgres-test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	if err := database.ClosePostgres(db); err != nil {
		t.Fatalf("ClosePostgres() error = %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB() error = %v", err)
	}
	if err := sqlDB.Ping(); err == nil {
		t.Fatal("expected closed sql handle to fail ping")
	}
}
