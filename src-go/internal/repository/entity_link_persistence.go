package repository

import (
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
)

type entityLinkRecord struct {
	ID            uuid.UUID  `gorm:"column:id;primaryKey"`
	ProjectID     uuid.UUID  `gorm:"column:project_id"`
	SourceType    string     `gorm:"column:source_type"`
	SourceID      uuid.UUID  `gorm:"column:source_id"`
	TargetType    string     `gorm:"column:target_type"`
	TargetID      uuid.UUID  `gorm:"column:target_id"`
	LinkType      string     `gorm:"column:link_type"`
	AnchorBlockID *string    `gorm:"column:anchor_block_id"`
	CreatedBy     uuid.UUID  `gorm:"column:created_by"`
	CreatedAt     time.Time  `gorm:"column:created_at"`
	DeletedAt     *time.Time `gorm:"column:deleted_at"`
}

func (entityLinkRecord) TableName() string { return "entity_links" }

func newEntityLinkRecord(link *model.EntityLink) *entityLinkRecord {
	if link == nil {
		return nil
	}
	return &entityLinkRecord{
		ID:            link.ID,
		ProjectID:     link.ProjectID,
		SourceType:    link.SourceType,
		SourceID:      link.SourceID,
		TargetType:    link.TargetType,
		TargetID:      link.TargetID,
		LinkType:      link.LinkType,
		AnchorBlockID: cloneStringPointer(link.AnchorBlockID),
		CreatedBy:     link.CreatedBy,
		CreatedAt:     link.CreatedAt,
		DeletedAt:     cloneTimePointer(link.DeletedAt),
	}
}

func (r *entityLinkRecord) toModel() *model.EntityLink {
	if r == nil {
		return nil
	}
	return &model.EntityLink{
		ID:            r.ID,
		ProjectID:     r.ProjectID,
		SourceType:    r.SourceType,
		SourceID:      r.SourceID,
		TargetType:    r.TargetType,
		TargetID:      r.TargetID,
		LinkType:      r.LinkType,
		AnchorBlockID: cloneStringPointer(r.AnchorBlockID),
		CreatedBy:     r.CreatedBy,
		CreatedAt:     r.CreatedAt,
		DeletedAt:     cloneTimePointer(r.DeletedAt),
	}
}
