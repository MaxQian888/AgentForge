// Package repository — project_template_repo.go persists project_templates.
//
// Scope: CRUD + a ListVisible helper that returns the union of
// (system) ∪ (user's own user-source) ∪ (user's installed marketplace copies).
// The repo does NOT know about snapshot schema versioning or sanitization —
// those are service-layer concerns.
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"gorm.io/gorm"
)

type ProjectTemplateRepository struct {
	db *gorm.DB
}

func NewProjectTemplateRepository(db *gorm.DB) *ProjectTemplateRepository {
	return &ProjectTemplateRepository{db: db}
}

type projectTemplateRecord struct {
	ID              uuid.UUID  `gorm:"column:id;primaryKey"`
	Source          string     `gorm:"column:source"`
	OwnerUserID     *uuid.UUID `gorm:"column:owner_user_id"`
	Name            string     `gorm:"column:name"`
	Description     string     `gorm:"column:description"`
	SnapshotJSON    jsonText   `gorm:"column:snapshot_json;type:jsonb"`
	SnapshotVersion int        `gorm:"column:snapshot_version"`
	CreatedAt       time.Time  `gorm:"column:created_at"`
	UpdatedAt       time.Time  `gorm:"column:updated_at"`
}

func (projectTemplateRecord) TableName() string { return "project_templates" }

func newProjectTemplateRecord(t *model.ProjectTemplate) *projectTemplateRecord {
	if t == nil {
		return nil
	}
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	if t.SnapshotVersion == 0 {
		t.SnapshotVersion = model.CurrentProjectTemplateSnapshotVersion
	}
	return &projectTemplateRecord{
		ID:              t.ID,
		Source:          t.Source,
		OwnerUserID:     t.OwnerUserID,
		Name:            t.Name,
		Description:     t.Description,
		SnapshotJSON:    newJSONText(t.SnapshotJSON, "{}"),
		SnapshotVersion: t.SnapshotVersion,
		CreatedAt:       t.CreatedAt,
		UpdatedAt:       t.UpdatedAt,
	}
}

func (r *projectTemplateRecord) toModel() *model.ProjectTemplate {
	if r == nil {
		return nil
	}
	return &model.ProjectTemplate{
		ID:              r.ID,
		Source:          r.Source,
		OwnerUserID:     cloneUUIDPointer(r.OwnerUserID),
		Name:            r.Name,
		Description:     r.Description,
		SnapshotJSON:    r.SnapshotJSON.String("{}"),
		SnapshotVersion: r.SnapshotVersion,
		CreatedAt:       r.CreatedAt,
		UpdatedAt:       r.UpdatedAt,
	}
}

// Insert persists a new project template. Upserts on ID when one already
// exists so that builtin system template bundle registration is idempotent.
func (r *ProjectTemplateRepository) Insert(ctx context.Context, t *model.ProjectTemplate) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if t == nil {
		return fmt.Errorf("insert project template: template is nil")
	}
	record := newProjectTemplateRecord(t)
	if err := r.db.WithContext(ctx).Create(record).Error; err != nil {
		return fmt.Errorf("insert project template: %w", err)
	}
	t.ID = record.ID
	return nil
}

// Upsert replaces the row keyed by ID or inserts a new one. Used by builtin
// bundle registration so repeated server starts do not create duplicates.
func (r *ProjectTemplateRepository) Upsert(ctx context.Context, t *model.ProjectTemplate) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if t == nil {
		return fmt.Errorf("upsert project template: template is nil")
	}
	record := newProjectTemplateRecord(t)
	if err := r.db.WithContext(ctx).Save(record).Error; err != nil {
		return fmt.Errorf("upsert project template: %w", err)
	}
	return nil
}

func (r *ProjectTemplateRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.ProjectTemplate, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var record projectTemplateRecord
	if err := r.db.WithContext(ctx).Where("id = ?", id).Take(&record).Error; err != nil {
		return nil, fmt.Errorf("get project template: %w", normalizeRepositoryError(err))
	}
	return record.toModel(), nil
}

// ListVisible returns (system) ∪ (user's own) ∪ (user's marketplace installs).
// Passing uuid.Nil for userID narrows the result to system-only.
func (r *ProjectTemplateRepository) ListVisible(ctx context.Context, userID uuid.UUID) ([]*model.ProjectTemplate, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	q := r.db.WithContext(ctx).Model(&projectTemplateRecord{})
	if userID == uuid.Nil {
		q = q.Where("source = ?", model.ProjectTemplateSourceSystem)
	} else {
		q = q.Where(
			"source = ? OR (source IN ? AND owner_user_id = ?)",
			model.ProjectTemplateSourceSystem,
			[]string{model.ProjectTemplateSourceUser, model.ProjectTemplateSourceMarketplace},
			userID,
		)
	}

	var records []projectTemplateRecord
	if err := q.Order("source ASC").Order("name ASC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list project templates: %w", err)
	}
	out := make([]*model.ProjectTemplate, 0, len(records))
	for i := range records {
		out = append(out, records[i].toModel())
	}
	return out, nil
}

// UpdateMetadata edits Name/Description on a user-source template. Snapshot
// content is immutable post-creation; callers that want a refreshed snapshot
// create a new template. Returns ErrNotFound if no row matches.
func (r *ProjectTemplateRepository) UpdateMetadata(
	ctx context.Context,
	id uuid.UUID,
	name *string,
	description *string,
) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	updates := map[string]any{}
	if name != nil {
		updates["name"] = *name
	}
	if description != nil {
		updates["description"] = *description
	}
	if len(updates) == 0 {
		return nil
	}
	res := r.db.WithContext(ctx).Model(&projectTemplateRecord{}).
		Where("id = ?", id).
		Updates(updates)
	if res.Error != nil {
		return fmt.Errorf("update project template: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete removes a template row. Callers MUST enforce ownership/source rules
// before invoking — the repo accepts any id.
func (r *ProjectTemplateRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	res := r.db.WithContext(ctx).Where("id = ?", id).Delete(&projectTemplateRecord{})
	if res.Error != nil {
		return fmt.Errorf("delete project template: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
