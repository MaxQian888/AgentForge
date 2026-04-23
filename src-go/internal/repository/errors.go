package repository

import (
	"database/sql"
	"errors"

	"gorm.io/gorm"
)

var (
	// ErrDatabaseUnavailable is returned when PostgreSQL is not connected.
	ErrDatabaseUnavailable = errors.New("database unavailable")
	// ErrCacheUnavailable is returned when Redis is not connected.
	ErrCacheUnavailable = errors.New("cache unavailable")
	// ErrNotFound is returned when a requested record does not exist.
	ErrNotFound = errors.New("record not found")
	// ErrAgentRunActiveConflict is returned when a task already has an active
	// starting/running agent run.
	ErrAgentRunActiveConflict = errors.New("task already has an active agent run")
	// ErrEmployeeNameConflict is returned when attempting to create an Employee
	// whose (project_id, name) pair already exists.
	ErrEmployeeNameConflict = errors.New("employee name already exists in project")
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
