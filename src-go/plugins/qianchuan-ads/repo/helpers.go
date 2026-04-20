package qcrepo

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository helpers copied from internal/repository so the plugin can
// live in its own package tree without requiring cross-package access
// to unexported symbols. Keep this set intentionally small — only
// helpers the qianchuan repos actually use. Any divergence from the
// core repo helpers should be flagged and reconciled.

var (
	ErrDatabaseUnavailable = errors.New("database unavailable")
	ErrNotFound            = errors.New("record not found")
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

type jsonText string

func newJSONText(raw string, fallback string) jsonText {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return jsonText(fallback)
	}
	return jsonText(trimmed)
}

func (j jsonText) String(defaultValue string) string {
	trimmed := strings.TrimSpace(string(j))
	if trimmed == "" {
		return defaultValue
	}
	return trimmed
}

func (j jsonText) Value() (driver.Value, error) {
	trimmed := strings.TrimSpace(string(j))
	if trimmed == "" {
		return nil, nil
	}
	return trimmed, nil
}

func (j *jsonText) Scan(src any) error {
	if j == nil {
		return fmt.Errorf("jsonText destination is nil")
	}
	switch value := src.(type) {
	case nil:
		*j = ""
	case string:
		*j = jsonText(value)
	case []byte:
		*j = jsonText(string(value))
	default:
		return fmt.Errorf("scan jsonText: unsupported type %T", src)
	}
	return nil
}

func cloneUUIDPointer(value *uuid.UUID) *uuid.UUID {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
