// Package repository — workflow_trigger_repo.go persists workflow_triggers rows.
package repository

import (
	"context"
	"crypto/md5" //nolint:gosec // MD5 is used only for config dedup hashing, not cryptography.
	"encoding/hex"
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
// the same (workflow_id, source, md5(config::text)) fingerprint.
//
// Because the unique constraint is expression-based, Postgres does not allow
// INSERT … ON CONFLICT against it directly.  We do a manual find-then-insert
// (or find-then-update) inside a transaction so the operation is atomic
// relative to concurrent writers on the same constraint.
//
// On return, t.ID, t.CreatedAt, and t.UpdatedAt are always set to the
// canonical values stored in the database.
func (r *WorkflowTriggerRepository) Upsert(ctx context.Context, t *model.WorkflowTrigger) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}

	// Compute the MD5 hex of the canonical config JSON so we can query with it.
	md5Hex := configMD5(t.Config)

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return upsertTriggerTx(ctx, tx, t, md5Hex)
	})
}

// upsertTriggerTx performs the find-then-insert-or-update inside tx.
// It is extracted so it can be called recursively once on a unique-violation race.
func upsertTriggerTx(ctx context.Context, tx *gorm.DB, t *model.WorkflowTrigger, md5Hex string) error {
	var existing workflowTriggerRecord
	err := tx.WithContext(ctx).
		Where("workflow_id = ? AND source = ? AND md5(config::text) = ?",
			t.WorkflowID, string(t.Source), md5Hex).
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
				return upsertTriggerTx(ctx, tx, t, md5Hex)
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

// configMD5 returns the lowercase hex MD5 digest of the raw JSON bytes.
// This must produce the same value as Postgres' md5(config::text).
// We pass the raw JSON through — any whitespace differences between what we
// stored and what Postgres casts back are fine because the index is built on
// the stored column value, not the Go-side representation.
func configMD5(raw json.RawMessage) string {
	//nolint:gosec // Not used for cryptographic purposes.
	sum := md5.Sum([]byte(raw)) //nolint:gosec
	return hex.EncodeToString(sum[:])
}
