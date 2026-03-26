package repository

import (
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

type wikiSpaceRecord struct {
	ID        uuid.UUID  `gorm:"column:id;primaryKey"`
	ProjectID uuid.UUID  `gorm:"column:project_id"`
	CreatedAt time.Time  `gorm:"column:created_at"`
	DeletedAt *time.Time `gorm:"column:deleted_at"`
}

func (wikiSpaceRecord) TableName() string { return "wiki_spaces" }

func newWikiSpaceRecord(space *model.WikiSpace) *wikiSpaceRecord {
	if space == nil {
		return nil
	}
	return &wikiSpaceRecord{
		ID:        space.ID,
		ProjectID: space.ProjectID,
		CreatedAt: space.CreatedAt,
		DeletedAt: cloneTimePointer(space.DeletedAt),
	}
}

func (r *wikiSpaceRecord) toModel() *model.WikiSpace {
	if r == nil {
		return nil
	}
	return &model.WikiSpace{
		ID:        r.ID,
		ProjectID: r.ProjectID,
		CreatedAt: r.CreatedAt,
		DeletedAt: cloneTimePointer(r.DeletedAt),
	}
}

type wikiPageRecord struct {
	ID               uuid.UUID  `gorm:"column:id;primaryKey"`
	SpaceID          uuid.UUID  `gorm:"column:space_id"`
	ParentID         *uuid.UUID `gorm:"column:parent_id"`
	Title            string     `gorm:"column:title"`
	Content          jsonText   `gorm:"column:content;type:jsonb"`
	ContentText      string     `gorm:"column:content_text"`
	Path             string     `gorm:"column:path"`
	SortOrder        int        `gorm:"column:sort_order"`
	IsTemplate       bool       `gorm:"column:is_template"`
	TemplateCategory *string    `gorm:"column:template_category"`
	IsSystem         bool       `gorm:"column:is_system"`
	IsPinned         bool       `gorm:"column:is_pinned"`
	CreatedBy        *uuid.UUID `gorm:"column:created_by"`
	UpdatedBy        *uuid.UUID `gorm:"column:updated_by"`
	CreatedAt        time.Time  `gorm:"column:created_at"`
	UpdatedAt        time.Time  `gorm:"column:updated_at"`
	DeletedAt        *time.Time `gorm:"column:deleted_at"`
}

func (wikiPageRecord) TableName() string { return "wiki_pages" }

func newWikiPageRecord(page *model.WikiPage) *wikiPageRecord {
	if page == nil {
		return nil
	}
	return &wikiPageRecord{
		ID:               page.ID,
		SpaceID:          page.SpaceID,
		ParentID:         cloneUUIDPointer(page.ParentID),
		Title:            page.Title,
		Content:          newJSONText(page.Content, "[]"),
		ContentText:      page.ContentText,
		Path:             page.Path,
		SortOrder:        page.SortOrder,
		IsTemplate:       page.IsTemplate,
		TemplateCategory: cloneStringPointer(optionalStringPointer(page.TemplateCategory)),
		IsSystem:         page.IsSystem,
		IsPinned:         page.IsPinned,
		CreatedBy:        cloneUUIDPointer(page.CreatedBy),
		UpdatedBy:        cloneUUIDPointer(page.UpdatedBy),
		CreatedAt:        page.CreatedAt,
		UpdatedAt:        page.UpdatedAt,
		DeletedAt:        cloneTimePointer(page.DeletedAt),
	}
}

func (r *wikiPageRecord) toModel() *model.WikiPage {
	if r == nil {
		return nil
	}
	return &model.WikiPage{
		ID:               r.ID,
		SpaceID:          r.SpaceID,
		ParentID:         cloneUUIDPointer(r.ParentID),
		Title:            r.Title,
		Content:          r.Content.String("[]"),
		ContentText:      r.ContentText,
		Path:             r.Path,
		SortOrder:        r.SortOrder,
		IsTemplate:       r.IsTemplate,
		TemplateCategory: valueOrEmpty(r.TemplateCategory),
		IsSystem:         r.IsSystem,
		IsPinned:         r.IsPinned,
		CreatedBy:        cloneUUIDPointer(r.CreatedBy),
		UpdatedBy:        cloneUUIDPointer(r.UpdatedBy),
		CreatedAt:        r.CreatedAt,
		UpdatedAt:        r.UpdatedAt,
		DeletedAt:        cloneTimePointer(r.DeletedAt),
	}
}

type pageVersionRecord struct {
	ID            uuid.UUID  `gorm:"column:id;primaryKey"`
	PageID        uuid.UUID  `gorm:"column:page_id"`
	VersionNumber int        `gorm:"column:version_number"`
	Name          string     `gorm:"column:name"`
	Content       jsonText   `gorm:"column:content;type:jsonb"`
	CreatedBy     *uuid.UUID `gorm:"column:created_by"`
	CreatedAt     time.Time  `gorm:"column:created_at"`
}

func (pageVersionRecord) TableName() string { return "page_versions" }

func newPageVersionRecord(version *model.PageVersion) *pageVersionRecord {
	if version == nil {
		return nil
	}
	return &pageVersionRecord{
		ID:            version.ID,
		PageID:        version.PageID,
		VersionNumber: version.VersionNumber,
		Name:          version.Name,
		Content:       newJSONText(version.Content, "[]"),
		CreatedBy:     cloneUUIDPointer(version.CreatedBy),
		CreatedAt:     version.CreatedAt,
	}
}

func (r *pageVersionRecord) toModel() *model.PageVersion {
	if r == nil {
		return nil
	}
	return &model.PageVersion{
		ID:            r.ID,
		PageID:        r.PageID,
		VersionNumber: r.VersionNumber,
		Name:          r.Name,
		Content:       r.Content.String("[]"),
		CreatedBy:     cloneUUIDPointer(r.CreatedBy),
		CreatedAt:     r.CreatedAt,
	}
}

type pageCommentRecord struct {
	ID              uuid.UUID  `gorm:"column:id;primaryKey"`
	PageID          uuid.UUID  `gorm:"column:page_id"`
	AnchorBlockID   *string    `gorm:"column:anchor_block_id"`
	ParentCommentID *uuid.UUID `gorm:"column:parent_comment_id"`
	Body            string     `gorm:"column:body"`
	Mentions        jsonText   `gorm:"column:mentions;type:jsonb"`
	ResolvedAt      *time.Time `gorm:"column:resolved_at"`
	CreatedBy       *uuid.UUID `gorm:"column:created_by"`
	CreatedAt       time.Time  `gorm:"column:created_at"`
	UpdatedAt       time.Time  `gorm:"column:updated_at"`
	DeletedAt       *time.Time `gorm:"column:deleted_at"`
}

func (pageCommentRecord) TableName() string { return "page_comments" }

func newPageCommentRecord(comment *model.PageComment) *pageCommentRecord {
	if comment == nil {
		return nil
	}
	return &pageCommentRecord{
		ID:              comment.ID,
		PageID:          comment.PageID,
		AnchorBlockID:   cloneStringPointer(comment.AnchorBlockID),
		ParentCommentID: cloneUUIDPointer(comment.ParentCommentID),
		Body:            comment.Body,
		Mentions:        newJSONText(comment.Mentions, "[]"),
		ResolvedAt:      cloneTimePointer(comment.ResolvedAt),
		CreatedBy:       cloneUUIDPointer(comment.CreatedBy),
		CreatedAt:       comment.CreatedAt,
		UpdatedAt:       comment.UpdatedAt,
		DeletedAt:       cloneTimePointer(comment.DeletedAt),
	}
}

func (r *pageCommentRecord) toModel() *model.PageComment {
	if r == nil {
		return nil
	}
	return &model.PageComment{
		ID:              r.ID,
		PageID:          r.PageID,
		AnchorBlockID:   cloneStringPointer(r.AnchorBlockID),
		ParentCommentID: cloneUUIDPointer(r.ParentCommentID),
		Body:            r.Body,
		Mentions:        r.Mentions.String("[]"),
		ResolvedAt:      cloneTimePointer(r.ResolvedAt),
		CreatedBy:       cloneUUIDPointer(r.CreatedBy),
		CreatedAt:       r.CreatedAt,
		UpdatedAt:       r.UpdatedAt,
		DeletedAt:       cloneTimePointer(r.DeletedAt),
	}
}

type pageFavoriteRecord struct {
	PageID    uuid.UUID `gorm:"column:page_id;primaryKey"`
	UserID    uuid.UUID `gorm:"column:user_id;primaryKey"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (pageFavoriteRecord) TableName() string { return "page_favorites" }

func newPageFavoriteRecord(favorite *model.PageFavorite) *pageFavoriteRecord {
	if favorite == nil {
		return nil
	}
	return &pageFavoriteRecord{
		PageID:    favorite.PageID,
		UserID:    favorite.UserID,
		CreatedAt: favorite.CreatedAt,
	}
}

func (r *pageFavoriteRecord) toModel() *model.PageFavorite {
	if r == nil {
		return nil
	}
	return &model.PageFavorite{
		PageID:    r.PageID,
		UserID:    r.UserID,
		CreatedAt: r.CreatedAt,
	}
}

type pageRecentAccessRecord struct {
	PageID     uuid.UUID `gorm:"column:page_id;primaryKey"`
	UserID     uuid.UUID `gorm:"column:user_id;primaryKey"`
	AccessedAt time.Time `gorm:"column:accessed_at"`
}

func (pageRecentAccessRecord) TableName() string { return "page_recent_access" }

func newPageRecentAccessRecord(access *model.PageRecentAccess) *pageRecentAccessRecord {
	if access == nil {
		return nil
	}
	return &pageRecentAccessRecord{
		PageID:     access.PageID,
		UserID:     access.UserID,
		AccessedAt: access.AccessedAt,
	}
}

func (r *pageRecentAccessRecord) toModel() *model.PageRecentAccess {
	if r == nil {
		return nil
	}
	return &model.PageRecentAccess{
		PageID:     r.PageID,
		UserID:     r.UserID,
		AccessedAt: r.AccessedAt,
	}
}
