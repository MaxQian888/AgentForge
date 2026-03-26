package model

import (
	"time"

	"github.com/google/uuid"
)

type WikiSpace struct {
	ID        uuid.UUID  `db:"id"`
	ProjectID uuid.UUID  `db:"project_id"`
	CreatedAt time.Time  `db:"created_at"`
	DeletedAt *time.Time `db:"deleted_at"`
}

type WikiPage struct {
	ID               uuid.UUID  `db:"id"`
	SpaceID          uuid.UUID  `db:"space_id"`
	ParentID         *uuid.UUID `db:"parent_id"`
	Title            string     `db:"title"`
	Content          string     `db:"content"`
	ContentText      string     `db:"content_text"`
	Path             string     `db:"path"`
	SortOrder        int        `db:"sort_order"`
	IsTemplate       bool       `db:"is_template"`
	TemplateCategory string     `db:"template_category"`
	IsSystem         bool       `db:"is_system"`
	IsPinned         bool       `db:"is_pinned"`
	CreatedBy        *uuid.UUID `db:"created_by"`
	UpdatedBy        *uuid.UUID `db:"updated_by"`
	CreatedAt        time.Time  `db:"created_at"`
	UpdatedAt        time.Time  `db:"updated_at"`
	DeletedAt        *time.Time `db:"deleted_at"`
}

type PageVersion struct {
	ID            uuid.UUID  `db:"id"`
	PageID        uuid.UUID  `db:"page_id"`
	VersionNumber int        `db:"version_number"`
	Name          string     `db:"name"`
	Content       string     `db:"content"`
	CreatedBy     *uuid.UUID `db:"created_by"`
	CreatedAt     time.Time  `db:"created_at"`
}

type PageComment struct {
	ID              uuid.UUID  `db:"id"`
	PageID          uuid.UUID  `db:"page_id"`
	AnchorBlockID   *string    `db:"anchor_block_id"`
	ParentCommentID *uuid.UUID `db:"parent_comment_id"`
	Body            string     `db:"body"`
	Mentions        string     `db:"mentions"`
	ResolvedAt      *time.Time `db:"resolved_at"`
	CreatedBy       *uuid.UUID `db:"created_by"`
	CreatedAt       time.Time  `db:"created_at"`
	UpdatedAt       time.Time  `db:"updated_at"`
	DeletedAt       *time.Time `db:"deleted_at"`
}

type PageFavorite struct {
	PageID    uuid.UUID `db:"page_id"`
	UserID    uuid.UUID `db:"user_id"`
	CreatedAt time.Time `db:"created_at"`
}

type PageRecentAccess struct {
	PageID     uuid.UUID `db:"page_id"`
	UserID     uuid.UUID `db:"user_id"`
	AccessedAt time.Time `db:"accessed_at"`
}

type WikiSpaceDTO struct {
	ID        string  `json:"id"`
	ProjectID string  `json:"projectId"`
	CreatedAt string  `json:"createdAt"`
	DeletedAt *string `json:"deletedAt,omitempty"`
}

func (s *WikiSpace) ToDTO() WikiSpaceDTO {
	return WikiSpaceDTO{
		ID:        s.ID.String(),
		ProjectID: s.ProjectID.String(),
		CreatedAt: s.CreatedAt.Format(time.RFC3339),
		DeletedAt: formatOptionalTime(s.DeletedAt),
	}
}

type WikiPageDTO struct {
	ID               string  `json:"id"`
	SpaceID          string  `json:"spaceId"`
	ParentID         *string `json:"parentId,omitempty"`
	Title            string  `json:"title"`
	Content          string  `json:"content"`
	ContentText      string  `json:"contentText"`
	Path             string  `json:"path"`
	SortOrder        int     `json:"sortOrder"`
	IsTemplate       bool    `json:"isTemplate"`
	TemplateCategory string  `json:"templateCategory,omitempty"`
	IsSystem         bool    `json:"isSystem"`
	IsPinned         bool    `json:"isPinned"`
	CreatedBy        *string `json:"createdBy,omitempty"`
	UpdatedBy        *string `json:"updatedBy,omitempty"`
	CreatedAt        string  `json:"createdAt"`
	UpdatedAt        string  `json:"updatedAt"`
	DeletedAt        *string `json:"deletedAt,omitempty"`
}

func (p *WikiPage) ToDTO() WikiPageDTO {
	return WikiPageDTO{
		ID:               p.ID.String(),
		SpaceID:          p.SpaceID.String(),
		ParentID:         formatOptionalUUID(p.ParentID),
		Title:            p.Title,
		Content:          p.Content,
		ContentText:      p.ContentText,
		Path:             p.Path,
		SortOrder:        p.SortOrder,
		IsTemplate:       p.IsTemplate,
		TemplateCategory: p.TemplateCategory,
		IsSystem:         p.IsSystem,
		IsPinned:         p.IsPinned,
		CreatedBy:        formatOptionalUUID(p.CreatedBy),
		UpdatedBy:        formatOptionalUUID(p.UpdatedBy),
		CreatedAt:        p.CreatedAt.Format(time.RFC3339),
		UpdatedAt:        p.UpdatedAt.Format(time.RFC3339),
		DeletedAt:        formatOptionalTime(p.DeletedAt),
	}
}

type WikiPageContextDTO struct {
	ProjectID string      `json:"projectId"`
	Page      WikiPageDTO `json:"page"`
}

type WikiPageTreeNodeDTO struct {
	WikiPageDTO
	Children []WikiPageTreeNodeDTO `json:"children"`
}

type PageVersionDTO struct {
	ID            string  `json:"id"`
	PageID        string  `json:"pageId"`
	VersionNumber int     `json:"versionNumber"`
	Name          string  `json:"name"`
	Content       string  `json:"content"`
	CreatedBy     *string `json:"createdBy,omitempty"`
	CreatedAt     string  `json:"createdAt"`
}

func (v *PageVersion) ToDTO() PageVersionDTO {
	return PageVersionDTO{
		ID:            v.ID.String(),
		PageID:        v.PageID.String(),
		VersionNumber: v.VersionNumber,
		Name:          v.Name,
		Content:       v.Content,
		CreatedBy:     formatOptionalUUID(v.CreatedBy),
		CreatedAt:     v.CreatedAt.Format(time.RFC3339),
	}
}

type PageCommentDTO struct {
	ID              string  `json:"id"`
	PageID          string  `json:"pageId"`
	AnchorBlockID   *string `json:"anchorBlockId,omitempty"`
	ParentCommentID *string `json:"parentCommentId,omitempty"`
	Body            string  `json:"body"`
	Mentions        string  `json:"mentions"`
	ResolvedAt      *string `json:"resolvedAt,omitempty"`
	CreatedBy       *string `json:"createdBy,omitempty"`
	CreatedAt       string  `json:"createdAt"`
	UpdatedAt       string  `json:"updatedAt"`
	DeletedAt       *string `json:"deletedAt,omitempty"`
}

func (c *PageComment) ToDTO() PageCommentDTO {
	return PageCommentDTO{
		ID:              c.ID.String(),
		PageID:          c.PageID.String(),
		AnchorBlockID:   cloneStringPointer(c.AnchorBlockID),
		ParentCommentID: formatOptionalUUID(c.ParentCommentID),
		Body:            c.Body,
		Mentions:        c.Mentions,
		ResolvedAt:      formatOptionalTime(c.ResolvedAt),
		CreatedBy:       formatOptionalUUID(c.CreatedBy),
		CreatedAt:       c.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       c.UpdatedAt.Format(time.RFC3339),
		DeletedAt:       formatOptionalTime(c.DeletedAt),
	}
}

type PageFavoriteDTO struct {
	PageID    string `json:"pageId"`
	UserID    string `json:"userId"`
	CreatedAt string `json:"createdAt"`
}

func (f *PageFavorite) ToDTO() PageFavoriteDTO {
	return PageFavoriteDTO{
		PageID:    f.PageID.String(),
		UserID:    f.UserID.String(),
		CreatedAt: f.CreatedAt.Format(time.RFC3339),
	}
}

type PageRecentAccessDTO struct {
	PageID     string `json:"pageId"`
	UserID     string `json:"userId"`
	AccessedAt string `json:"accessedAt"`
}

func (a *PageRecentAccess) ToDTO() PageRecentAccessDTO {
	return PageRecentAccessDTO{
		PageID:     a.PageID.String(),
		UserID:     a.UserID.String(),
		AccessedAt: a.AccessedAt.Format(time.RFC3339),
	}
}

type CreateWikiPageRequest struct {
	Title    string  `json:"title" validate:"required,min=1,max=200"`
	ParentID *string `json:"parentId,omitempty"`
	Content  string  `json:"content,omitempty"`
}

type UpdateWikiPageRequest struct {
	Title             string  `json:"title" validate:"required,min=1,max=200"`
	Content           string  `json:"content,omitempty"`
	ContentText       string  `json:"contentText,omitempty"`
	ExpectedUpdatedAt *string `json:"expectedUpdatedAt,omitempty"`
}

type MoveWikiPageRequest struct {
	ParentID  *string `json:"parentId,omitempty"`
	SortOrder int     `json:"sortOrder"`
}

type CreatePageVersionRequest struct {
	Name string `json:"name" validate:"required,min=1,max=200"`
}

type CreatePageCommentRequest struct {
	Body            string  `json:"body" validate:"required,min=1,max=4000"`
	AnchorBlockID   *string `json:"anchorBlockId,omitempty"`
	ParentCommentID *string `json:"parentCommentId,omitempty"`
	Mentions        string  `json:"mentions,omitempty"`
}

type UpdatePageCommentRequest struct {
	Body     *string `json:"body,omitempty"`
	Resolved *bool   `json:"resolved,omitempty"`
}

type CreatePageFromTemplateRequest struct {
	TemplateID string  `json:"templateId" validate:"required"`
	Title      string  `json:"title" validate:"required,min=1,max=200"`
	ParentID   *string `json:"parentId,omitempty"`
}

type CreateTemplateFromPageRequest struct {
	Name     string `json:"name" validate:"required,min=1,max=200"`
	Category string `json:"category" validate:"required,min=1,max=100"`
}

type ToggleFavoriteRequest struct {
	Favorite bool `json:"favorite"`
}

type TogglePinnedRequest struct {
	Pinned bool `json:"pinned"`
}

func formatOptionalUUID(value *uuid.UUID) *string {
	if value == nil {
		return nil
	}
	formatted := value.String()
	return &formatted
}

func formatOptionalTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.Format(time.RFC3339)
	return &formatted
}

func cloneStringPointer(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
