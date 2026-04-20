package database_test

import (
	"testing"
	"testing/fstest"

	"github.com/agentforge/server/pkg/database"
)

func TestRunMigrations_InvalidURL(t *testing.T) {
	fs := fstest.MapFS{
		"001_test.up.sql": &fstest.MapFile{Data: []byte("CREATE TABLE test (id int);")},
	}

	err := database.RunMigrations("postgres://invalid:invalid@127.0.0.1:59999/noexist?connect_timeout=1", fs)
	if err == nil {
		t.Fatal("expected error for invalid postgres URL")
	}
}

func TestRunMigrations_EmptyFS(t *testing.T) {
	fs := fstest.MapFS{}

	err := database.RunMigrations("postgres://user:pass@127.0.0.1:59999/db?connect_timeout=1", fs)
	if err == nil {
		t.Fatal("expected error for empty migrations FS")
	}
}
