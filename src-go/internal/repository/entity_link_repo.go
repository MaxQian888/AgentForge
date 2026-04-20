package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type EntityLinkRepository struct {
	db *gorm.DB
}

func NewEntityLinkRepository(db *gorm.DB) *EntityLinkRepository {
	return &EntityLinkRepository{db: db}
}

func (r *EntityLinkRepository) DB() *gorm.DB {
	if r == nil {
		return nil
	}
	return r.db
}

func (r *EntityLinkRepository) WithDB(db *gorm.DB) *EntityLinkRepository {
	return &EntityLinkRepository{db: db}
}

func (r *EntityLinkRepository) Create(ctx context.Context, link *model.EntityLink) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if link.ID == uuid.Nil {
		link.ID = uuid.New()
	}
	if link.CreatedAt.IsZero() {
		link.CreatedAt = time.Now().UTC()
	}
	if err := r.db.WithContext(ctx).Create(newEntityLinkRecord(link)).Error; err != nil {
		return fmt.Errorf("create entity link: %w", err)
	}
	return nil
}

func (r *EntityLinkRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.EntityLink, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var record entityLinkRecord
	if err := r.db.WithContext(ctx).Where("id = ?", id).Take(&record).Error; err != nil {
		return nil, fmt.Errorf("get entity link by id: %w", normalizeRepositoryError(err))
	}
	return record.toModel(), nil
}

func (r *EntityLinkRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	now := time.Now().UTC()
	if err := r.db.WithContext(ctx).
		Model(&entityLinkRecord{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Update("deleted_at", now).
		Error; err != nil {
		return fmt.Errorf("delete entity link: %w", err)
	}
	return nil
}

func (r *EntityLinkRepository) ListBySource(ctx context.Context, projectID uuid.UUID, sourceType string, sourceID uuid.UUID) ([]*model.EntityLink, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var records []entityLinkRecord
	if err := r.db.WithContext(ctx).
		Where("project_id = ? AND source_type = ? AND source_id = ? AND deleted_at IS NULL", projectID, sourceType, sourceID).
		Order("created_at ASC").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list entity links by source: %w", err)
	}
	return entityLinkRecordsToModels(records), nil
}

func (r *EntityLinkRepository) ListByTarget(ctx context.Context, projectID uuid.UUID, targetType string, targetID uuid.UUID) ([]*model.EntityLink, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var records []entityLinkRecord
	if err := r.db.WithContext(ctx).
		Where("project_id = ? AND target_type = ? AND target_id = ? AND deleted_at IS NULL", projectID, targetType, targetID).
		Order("created_at ASC").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list entity links by target: %w", err)
	}
	return entityLinkRecordsToModels(records), nil
}

func (r *EntityLinkRepository) UpsertMentionLinks(ctx context.Context, projectID uuid.UUID, sourceType string, sourceID uuid.UUID, createdBy uuid.UUID, targets []model.EntityLinkTarget) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if len(targets) == 0 {
		return nil
	}

	var existing []entityLinkRecord
	if err := r.db.WithContext(ctx).
		Where("project_id = ? AND source_type = ? AND source_id = ? AND link_type = ?", projectID, sourceType, sourceID, model.EntityLinkTypeMention).
		Find(&existing).Error; err != nil {
		return fmt.Errorf("load existing mention links: %w", err)
	}

	byKey := make(map[string]entityLinkRecord, len(existing))
	for _, record := range existing {
		byKey[entityLinkTargetKey(record.TargetType, record.TargetID)] = record
	}

	for _, target := range dedupeEntityLinkTargets(targets) {
		key := entityLinkTargetKey(target.EntityType, target.EntityID)
		if record, ok := byKey[key]; ok {
			if record.DeletedAt != nil {
				if err := r.db.WithContext(ctx).
					Model(&entityLinkRecord{}).
					Where("id = ?", record.ID).
					Update("deleted_at", nil).
					Error; err != nil {
					return fmt.Errorf("reactivate mention link: %w", err)
				}
			}
			continue
		}

		link := &model.EntityLink{
			ID:         uuid.New(),
			ProjectID:  projectID,
			SourceType: sourceType,
			SourceID:   sourceID,
			TargetType: target.EntityType,
			TargetID:   target.EntityID,
			LinkType:   model.EntityLinkTypeMention,
			CreatedBy:  createdBy,
			CreatedAt:  time.Now().UTC(),
		}
		if err := r.db.WithContext(ctx).Create(newEntityLinkRecord(link)).Error; err != nil {
			return fmt.Errorf("create mention link: %w", err)
		}
	}

	return nil
}

func (r *EntityLinkRepository) DeleteMentionLinksForSource(ctx context.Context, projectID uuid.UUID, sourceType string, sourceID uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	now := time.Now().UTC()
	if err := r.db.WithContext(ctx).
		Model(&entityLinkRecord{}).
		Where("project_id = ? AND source_type = ? AND source_id = ? AND link_type = ? AND deleted_at IS NULL", projectID, sourceType, sourceID, model.EntityLinkTypeMention).
		Update("deleted_at", now).
		Error; err != nil {
		return fmt.Errorf("delete mention links for source: %w", err)
	}
	return nil
}

func entityLinkRecordsToModels(records []entityLinkRecord) []*model.EntityLink {
	links := make([]*model.EntityLink, 0, len(records))
	for i := range records {
		links = append(links, records[i].toModel())
	}
	return links
}

func dedupeEntityLinkTargets(targets []model.EntityLinkTarget) []model.EntityLinkTarget {
	if len(targets) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(targets))
	deduped := make([]model.EntityLinkTarget, 0, len(targets))
	for _, target := range targets {
		key := entityLinkTargetKey(target.EntityType, target.EntityID)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		deduped = append(deduped, target)
	}
	return deduped
}

func entityLinkTargetKey(entityType string, entityID uuid.UUID) string {
	return entityType + ":" + entityID.String()
}
