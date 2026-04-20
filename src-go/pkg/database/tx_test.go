package database_test

import (
	"context"
	"errors"
	"testing"

	"github.com/agentforge/server/pkg/database"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type txWidget struct {
	ID   int `gorm:"primaryKey"`
	Name string
}

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

func TestWithTx_CommitsOnSuccess(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:withtx-commit?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&txWidget{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	if err := database.WithTx(context.Background(), db, func(tx *gorm.DB) error {
		return tx.Create(&txWidget{ID: 1, Name: "committed"}).Error
	}); err != nil {
		t.Fatalf("WithTx() commit error = %v", err)
	}

	var count int64
	if err := db.Model(&txWidget{}).Where("id = ?", 1).Count(&count).Error; err != nil {
		t.Fatalf("Count() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("committed row count = %d, want 1", count)
	}
}

func TestWithTx_RollsBackOnError(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:withtx-rollback?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&txWidget{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	sentinel := errors.New("rollback me")
	if err := database.WithTx(context.Background(), db, func(tx *gorm.DB) error {
		if err := tx.Create(&txWidget{ID: 2, Name: "rolled-back"}).Error; err != nil {
			return err
		}
		return sentinel
	}); !errors.Is(err, sentinel) {
		t.Fatalf("WithTx() error = %v, want sentinel", err)
	}

	var count int64
	if err := db.Model(&txWidget{}).Where("id = ?", 2).Count(&count).Error; err != nil {
		t.Fatalf("Count() error = %v", err)
	}
	if count != 0 {
		t.Fatalf("rolled back row count = %d, want 0", count)
	}
}
