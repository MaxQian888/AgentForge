package database_test

import (
	"testing"

	"github.com/react-go-quick-starter/server/pkg/database"
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
