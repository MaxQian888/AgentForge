package model

import (
	"time"

	"github.com/google/uuid"
)

const (
	EntityTypeTask     = "task"
	EntityTypeWikiPage = "wiki_page"
)

const (
	EntityLinkTypeRequirement = "requirement"
	EntityLinkTypeDesign      = "design"
	EntityLinkTypeTest        = "test"
	EntityLinkTypeRetro       = "retro"
	EntityLinkTypeReference   = "reference"
	EntityLinkTypeMention     = "mention"
)

type EntityLink struct {
	ID            uuid.UUID  `db:"id"`
	ProjectID     uuid.UUID  `db:"project_id"`
	SourceType    string     `db:"source_type"`
	SourceID      uuid.UUID  `db:"source_id"`
	TargetType    string     `db:"target_type"`
	TargetID      uuid.UUID  `db:"target_id"`
	LinkType      string     `db:"link_type"`
	AnchorBlockID *string    `db:"anchor_block_id"`
	CreatedBy     uuid.UUID  `db:"created_by"`
	CreatedAt     time.Time  `db:"created_at"`
	DeletedAt     *time.Time `db:"deleted_at"`
}

type EntityLinkTarget struct {
	EntityType string
	EntityID   uuid.UUID
}

type EntityLinkDTO struct {
	ID            string  `json:"id"`
	ProjectID     string  `json:"projectId"`
	SourceType    string  `json:"sourceType"`
	SourceID      string  `json:"sourceId"`
	TargetType    string  `json:"targetType"`
	TargetID      string  `json:"targetId"`
	LinkType      string  `json:"linkType"`
	AnchorBlockID *string `json:"anchorBlockId,omitempty"`
	CreatedBy     string  `json:"createdBy"`
	CreatedAt     string  `json:"createdAt"`
	DeletedAt     *string `json:"deletedAt,omitempty"`
}

func (l *EntityLink) ToDTO() EntityLinkDTO {
	dto := EntityLinkDTO{
		ID:         l.ID.String(),
		ProjectID:  l.ProjectID.String(),
		SourceType: l.SourceType,
		SourceID:   l.SourceID.String(),
		TargetType: l.TargetType,
		TargetID:   l.TargetID.String(),
		LinkType:   l.LinkType,
		CreatedBy:  l.CreatedBy.String(),
		CreatedAt:  l.CreatedAt.Format(time.RFC3339),
	}
	if l.AnchorBlockID != nil {
		anchor := *l.AnchorBlockID
		dto.AnchorBlockID = &anchor
	}
	if l.DeletedAt != nil {
		deletedAt := l.DeletedAt.Format(time.RFC3339)
		dto.DeletedAt = &deletedAt
	}
	return dto
}

type CreateEntityLinkRequest struct {
	SourceType    string  `json:"sourceType" validate:"required,oneof=task wiki_page"`
	SourceID      string  `json:"sourceId" validate:"required"`
	TargetType    string  `json:"targetType" validate:"required,oneof=task wiki_page"`
	TargetID      string  `json:"targetId" validate:"required"`
	LinkType      string  `json:"linkType" validate:"required,oneof=requirement design test retro reference mention"`
	AnchorBlockID *string `json:"anchorBlockId,omitempty"`
}
