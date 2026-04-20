package strategy

import (
	"time"

	"github.com/google/uuid"
)

// Status values for QianchuanStrategy.Status. Transitions:
//
//	draft -> published (immutable) -> archived
//
// Edits to a published row produce a NEW row (status=draft, version=max+1, same name).
const (
	StatusDraft     = "draft"
	StatusPublished = "published"
	StatusArchived  = "archived"
)

// QianchuanStrategy is the DB row in qianchuan_strategies. ProjectID is nil
// for system seed rows.
type QianchuanStrategy struct {
	ID          uuid.UUID
	ProjectID   *uuid.UUID
	Name        string
	Description string
	YAMLSource  string
	ParsedSpec  string // JSON-encoded ParsedSpec
	Version     int
	Status      string
	CreatedBy   uuid.UUID
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// IsSystem reports whether this row is a read-only system seed.
func (s *QianchuanStrategy) IsSystem() bool {
	return s != nil && s.ProjectID == nil
}
