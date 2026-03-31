package database

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

func WithTx(ctx context.Context, db *gorm.DB, fn func(tx *gorm.DB) error) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}

	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(tx)
	})
}
