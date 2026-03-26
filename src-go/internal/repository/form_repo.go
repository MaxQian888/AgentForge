package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"gorm.io/gorm"
)

type FormRepository struct {
	db *gorm.DB
}

func NewFormRepository(db *gorm.DB) *FormRepository {
	return &FormRepository{db: db}
}

func (r *FormRepository) CreateDefinition(ctx context.Context, form *model.FormDefinition) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	record := newFormDefinitionRecord(form)
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}
	if record.UpdatedAt.IsZero() {
		record.UpdatedAt = record.CreatedAt
	}
	if err := r.db.WithContext(ctx).Create(record).Error; err != nil {
		return fmt.Errorf("create form definition: %w", err)
	}
	return nil
}

func (r *FormRepository) GetDefinition(ctx context.Context, id uuid.UUID) (*model.FormDefinition, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var record formDefinitionRecord
	if err := r.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).Take(&record).Error; err != nil {
		return nil, fmt.Errorf("get form definition: %w", normalizeRepositoryError(err))
	}
	return record.toModel(), nil
}

func (r *FormRepository) GetBySlug(ctx context.Context, slug string) (*model.FormDefinition, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var record formDefinitionRecord
	if err := r.db.WithContext(ctx).Where("slug = ? AND deleted_at IS NULL", slug).Take(&record).Error; err != nil {
		return nil, fmt.Errorf("get form by slug: %w", normalizeRepositoryError(err))
	}
	return record.toModel(), nil
}

func (r *FormRepository) ListByProject(ctx context.Context, projectID uuid.UUID) ([]*model.FormDefinition, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var records []formDefinitionRecord
	if err := r.db.WithContext(ctx).
		Where("project_id = ? AND deleted_at IS NULL", projectID).
		Order("created_at ASC").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list forms by project: %w", err)
	}
	result := make([]*model.FormDefinition, 0, len(records))
	for i := range records {
		result = append(result, records[i].toModel())
	}
	return result, nil
}

func (r *FormRepository) UpdateDefinition(ctx context.Context, form *model.FormDefinition) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	result := r.db.WithContext(ctx).
		Model(&formDefinitionRecord{}).
		Where("id = ? AND deleted_at IS NULL", form.ID).
		Updates(map[string]any{
			"name":            form.Name,
			"slug":            form.Slug,
			"fields":          newJSONText(form.Fields, "[]"),
			"target_status":   form.TargetStatus,
			"target_assignee": cloneUUIDPointer(form.TargetAssignee),
			"is_public":       form.IsPublic,
			"updated_at":      time.Now().UTC(),
		})
	if result.Error != nil {
		return fmt.Errorf("update form definition: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *FormRepository) DeleteDefinition(ctx context.Context, id uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	now := time.Now().UTC()
	result := r.db.WithContext(ctx).
		Model(&formDefinitionRecord{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]any{"deleted_at": now, "updated_at": now})
	if result.Error != nil {
		return fmt.Errorf("delete form definition: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *FormRepository) CreateSubmission(ctx context.Context, submission *model.FormSubmission) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	record := newFormSubmissionRecord(submission)
	if record.SubmittedAt.IsZero() {
		record.SubmittedAt = time.Now().UTC()
	}
	if err := r.db.WithContext(ctx).Create(record).Error; err != nil {
		return fmt.Errorf("create form submission: %w", err)
	}
	return nil
}

func (r *FormRepository) ListSubmissionsByForm(ctx context.Context, formID uuid.UUID) ([]*model.FormSubmission, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var records []formSubmissionRecord
	if err := r.db.WithContext(ctx).
		Where("form_id = ?", formID).
		Order("submitted_at DESC").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list form submissions: %w", err)
	}
	result := make([]*model.FormSubmission, 0, len(records))
	for i := range records {
		result = append(result, records[i].toModel())
	}
	return result, nil
}
