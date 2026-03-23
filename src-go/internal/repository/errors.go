package repository

import "errors"

var (
	// ErrDatabaseUnavailable is returned when PostgreSQL is not connected.
	ErrDatabaseUnavailable = errors.New("database unavailable")
	// ErrCacheUnavailable is returned when Redis is not connected.
	ErrCacheUnavailable = errors.New("cache unavailable")
	// ErrNotFound is returned when a requested record does not exist.
	ErrNotFound = errors.New("record not found")
)
