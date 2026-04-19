// Package repository — workflow_trigger_repo.go persists workflow_triggers rows.
package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"gorm.io/gorm"
)

// WorkflowTriggerRepository handles persistence for WorkflowTrigger aggregates.
type WorkflowTriggerRepository struct {
	db *gorm.DB
}

// NewWorkflowTriggerRepository returns a new WorkflowTriggerRepository backed by db.
func NewWorkflowTriggerRepository(db *gorm.DB) *WorkflowTriggerRepository {
	return &WorkflowTriggerRepository{db: db}
}

// ---------------------------------------------------------------------------
// Record type
// ---------------------------------------------------------------------------

type workflowTriggerRecord struct {
	ID                     uuid.UUID  `gorm:"column:id;primaryKey"`
	WorkflowID             uuid.UUID  `gorm:"column:workflow_id"`
	ProjectID              uuid.UUID  `gorm:"column:project_id"`
	Source                 string     `gorm:"column:source"`
	Config                 jsonText   `gorm:"column:config;type:jsonb"`
	InputMapping           jsonText   `gorm:"column:input_mapping;type:jsonb"`
	IdempotencyKeyTemplate string     `gorm:"column:idempotency_key_template"`
	DedupeWindowSeconds    int        `gorm:"column:dedupe_window_seconds"`
	Enabled                bool       `gorm:"column:enabled"`
	CreatedBy              *uuid.UUID `gorm:"column:created_by"`
	CreatedAt              time.Time  `gorm:"column:created_at"`
	UpdatedAt              time.Time  `gorm:"column:updated_at"`
}

func (workflowTriggerRecord) TableName() string { return "workflow_triggers" }

// ---------------------------------------------------------------------------
// Constructors
// ---------------------------------------------------------------------------

func newWorkflowTriggerRecord(t *model.WorkflowTrigger) *workflowTriggerRecord {
	if t == nil {
		return nil
	}
	return &workflowTriggerRecord{
		ID:                     t.ID,
		WorkflowID:             t.WorkflowID,
		ProjectID:              t.ProjectID,
		Source:                 string(t.Source),
		Config:                 newJSONText(rawMessageToString(t.Config), "{}"),
		InputMapping:           newJSONText(rawMessageToString(t.InputMapping), "{}"),
		IdempotencyKeyTemplate: t.IdempotencyKeyTemplate,
		DedupeWindowSeconds:    t.DedupeWindowSeconds,
		Enabled:                t.Enabled,
		CreatedBy:              t.CreatedBy,
		CreatedAt:              t.CreatedAt,
		UpdatedAt:              t.UpdatedAt,
	}
}

func (r *workflowTriggerRecord) toModel() *model.WorkflowTrigger {
	if r == nil {
		return nil
	}
	return &model.WorkflowTrigger{
		ID:                     r.ID,
		WorkflowID:             r.WorkflowID,
		ProjectID:              r.ProjectID,
		Source:                 model.TriggerSource(r.Source),
		Config:                 json.RawMessage(r.Config.String("{}")),
		InputMapping:           json.RawMessage(r.InputMapping.String("{}")),
		IdempotencyKeyTemplate: r.IdempotencyKeyTemplate,
		DedupeWindowSeconds:    r.DedupeWindowSeconds,
		Enabled:                r.Enabled,
		CreatedBy:              r.CreatedBy,
		CreatedAt:              r.CreatedAt,
		UpdatedAt:              r.UpdatedAt,
	}
}

// ---------------------------------------------------------------------------
// Repository methods
// ---------------------------------------------------------------------------

// Upsert inserts a new trigger row or updates the existing one that shares
// the same (workflow_id, source, config) jsonb value.
//
// Postgres' jsonb equality canonicalizes both sides (keys sorted, whitespace
// normalized), so callers may pass arbitrarily-shaped input JSON and still
// deduplicate correctly.
//
// The unique index is expression-based on md5(config::text), so INSERT
// ON CONFLICT against it is not available. We do a find-then-insert under
// a transaction; the unique constraint plus a bounded retry (see retries
// parameter on upsertTriggerTx) handle the narrow race where two writers
// both see "no existing row" and both attempt INSERT.
//
// On return, t.ID, t.CreatedAt, and t.UpdatedAt carry the canonical values
// stored in the database.
func (r *WorkflowTriggerRepository) Upsert(ctx context.Context, t *model.WorkflowTrigger) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return upsertTriggerTx(ctx, tx, t, 1)
	})
}

// upsertTriggerTx performs the find-then-insert-or-update inside tx.
// It is extracted so it can be called recursively with bounded retries on unique-violation race.
func upsertTriggerTx(ctx context.Context, tx *gorm.DB, t *model.WorkflowTrigger, retries int) error {
	if retries < 0 {
		return fmt.Errorf("upsert workflow trigger: unique-violation retry exhausted")
	}

	var existing workflowTriggerRecord
	err := tx.WithContext(ctx).
		Where("workflow_id = ? AND source = ? AND config = ?::jsonb",
			t.WorkflowID, string(t.Source), rawMessageToString(t.Config)).
		Take(&existing).Error

	switch {
	case err == nil:
		// Row exists — update mutable fields.
		now := time.Now().UTC()
		updates := map[string]any{
			"input_mapping":             newJSONText(rawMessageToString(t.InputMapping), "{}"),
			"idempotency_key_template":  t.IdempotencyKeyTemplate,
			"dedupe_window_seconds":     t.DedupeWindowSeconds,
			"enabled":                   t.Enabled,
			"updated_at":                now,
		}
		if updErr := tx.WithContext(ctx).
			Model(&workflowTriggerRecord{}).
			Where("id = ?", existing.ID).
			Updates(updates).Error; updErr != nil {
			return fmt.Errorf("upsert workflow trigger (update): %w", updErr)
		}
		// Write canonical identity fields back onto the caller's struct.
		t.ID = existing.ID
		t.CreatedAt = existing.CreatedAt
		t.UpdatedAt = now
		return nil

	case errors.Is(normalizeRepositoryError(err), ErrNotFound):
		// Row does not exist — insert.
		rec := newWorkflowTriggerRecord(t)
		if rec.CreatedAt.IsZero() {
			rec.CreatedAt = time.Now().UTC()
		}
		if rec.UpdatedAt.IsZero() {
			rec.UpdatedAt = rec.CreatedAt
		}
		if insErr := tx.WithContext(ctx).Create(rec).Error; insErr != nil {
			// Race: another writer inserted the same config between our SELECT
			// and our INSERT.  Retry the find-and-update path once.
			if isUniqueViolation(insErr) {
				return upsertTriggerTx(ctx, tx, t, retries-1)
			}
			return fmt.Errorf("upsert workflow trigger (insert): %w", insErr)
		}
		t.ID = rec.ID
		t.CreatedAt = rec.CreatedAt
		t.UpdatedAt = rec.UpdatedAt
		return nil

	default:
		return fmt.Errorf("upsert workflow trigger (lookup): %w", err)
	}
}

// ListEnabledBySource returns all enabled triggers for the given source, ordered
// by created_at ASC.
func (r *WorkflowTriggerRepository) ListEnabledBySource(ctx context.Context, source model.TriggerSource) ([]*model.WorkflowTrigger, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var records []workflowTriggerRecord
	if err := r.db.WithContext(ctx).
		Where("source = ? AND enabled = true", string(source)).
		Order("created_at ASC").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list enabled workflow triggers by source: %w", err)
	}
	out := make([]*model.WorkflowTrigger, 0, len(records))
	for i := range records {
		out = append(out, records[i].toModel())
	}
	return out, nil
}

// ListByWorkflow returns all triggers for the given workflow (enabled or not),
// ordered by created_at ASC.
func (r *WorkflowTriggerRepository) ListByWorkflow(ctx context.Context, workflowID uuid.UUID) ([]*model.WorkflowTrigger, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var records []workflowTriggerRecord
	if err := r.db.WithContext(ctx).
		Where("workflow_id = ?", workflowID).
		Order("created_at ASC").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list workflow triggers by workflow: %w", err)
	}
	out := make([]*model.WorkflowTrigger, 0, len(records))
	for i := range records {
		out = append(out, records[i].toModel())
	}
	return out, nil
}

// SetEnabled flips the enabled flag for a single trigger.
// Returns ErrNotFound when no row matched the ID.
func (r *WorkflowTriggerRepository) SetEnabled(ctx context.Context, id uuid.UUID, enabled bool) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	res := r.db.WithContext(ctx).
		Model(&workflowTriggerRecord{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"enabled":    enabled,
			"updated_at": time.Now().UTC(),
		})
	if res.Error != nil {
		return fmt.Errorf("set workflow trigger enabled: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete hard-deletes a trigger row.
// Returns ErrNotFound when no row matched the ID.
func (r *WorkflowTriggerRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	res := r.db.WithContext(ctx).Where("id = ?", id).Delete(&workflowTriggerRecord{})
	if res.Error != nil {
		return fmt.Errorf("delete workflow trigger: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------
