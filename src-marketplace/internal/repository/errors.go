package repository

import (
	"database/sql"
	"errors"

	"gorm.io/gorm"
)

var (
	// ErrDatabaseUnavailable is returned when PostgreSQL is not connected.
	ErrDatabaseUnavailable = errors.New("database unavailable")
	// ErrNotFound is returned when a requested record does not exist.
	ErrNotFound = errors.New("record not found")
)

func normalizeRepositoryError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}
