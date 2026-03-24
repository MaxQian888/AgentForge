package database_test

import (
	"context"
	"testing"

	"github.com/react-go-quick-starter/server/pkg/database"
	"gorm.io/gorm"
)

func TestWithTx_NilDB(t *testing.T) {
	err := database.WithTx(context.Background(), nil, func(*gorm.DB) error {
		t.Fatal("callback should not run for nil db")
		return nil
	})
	if err == nil {
		t.Fatal("expected error for nil db")
	}
}

func TestClosePostgres_NilDB(t *testing.T) {
	if err := database.ClosePostgres(nil); err != nil {
		t.Fatalf("expected nil close error for nil db, got %v", err)
	}
}
