package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// KnowledgeAssetKind classifies the asset within the unified table.
type KnowledgeAssetKind string

const (
	KindWikiPage      KnowledgeAssetKind = "wiki_page"
	KindIngestedFile  KnowledgeAssetKind = "ingested_file"
	KindTemplate      KnowledgeAssetKind = "template"
)

// IngestStatus tracks the lifecycle of an ingested file.
type IngestStatus string

const (
	IngestStatusPending    IngestStatus = "pending"
	IngestStatusProcessing IngestStatus = "processing"
	IngestStatusReady      IngestStatus = "ready"
	IngestStatusFailed     IngestStatus = "failed"
)

// KnowledgeAsset is the unified model for wiki pages, ingested documents, and templates.
type KnowledgeAsset struct {
	ID                uuid.UUID          `db:"id"`
	ProjectID         uuid.UUID          `db:"project_id"`
	WikiSpaceID       *uuid.UUID         `db:"wiki_space_id"`
	ParentID          *uuid.UUID         `db:"parent_id"`
	Kind              KnowledgeAssetKind `db:"kind"`
	Title             string             `db:"title"`
	Path              string             `db:"path"`
	SortOrder         int                `db:"sort_order"`
	ContentJSON       string             `db:"content_json"`
	ContentText       string             `db:"content_text"`
	FileRef           string             `db:"file_ref"`
	FileSize          int64              `db:"file_size"`
	MimeType          string             `db:"mime_type"`
	IngestStatus      *IngestStatus      `db:"ingest_status"`
	IngestChunkCount  int                `db:"ingest_chunk_count"`
	TemplateCategory  string             `db:"template_category"`
	IsSystemTemplate  bool               `db:"is_system_template"`
	IsPinned          bool               `db:"is_pinned"`
	OwnerID           *uuid.UUID         `db:"owner_id"`
	CreatedBy         *uuid.UUID         `db:"created_by"`
	UpdatedBy         *uuid.UUID         `db:"updated_by"`
	CreatedAt         time.Time          `db:"created_at"`
	UpdatedAt         time.Time          `db:"updated_at"`
	DeletedAt         *time.Time         `db:"deleted_at"`
	Version           int64              `db:"version"`
}

// AssetVersion records a snapshot of a KnowledgeAsset at a point in time.
type AssetVersion struct {
	ID            uuid.UUID  `db:"id"`
	AssetID       uuid.UUID  `db:"asset_id"`
	VersionNumber int        `db:"version_number"`
	Name          string     `db:"name"`
	KindSnapshot  string     `db:"kind_snapshot"`
	ContentJSON   string     `db:"content_json"`
	FileRef       string     `db:"file_ref"`
	CreatedBy     *uuid.UUID `db:"created_by"`
	CreatedAt     time.Time  `db:"created_at"`
}

// AssetComment is a comment on a KnowledgeAsset (optionally inline for wiki_page).
type AssetComment struct {
	ID              uuid.UUID  `db:"id"`
	AssetID         uuid.UUID  `db:"asset_id"`
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

// AssetIngestChunk is a parsed text chunk for an ingested_file asset.
type AssetIngestChunk struct {
	ID         uuid.UUID `db:"id"`
	AssetID    uuid.UUID `db:"asset_id"`
	ChunkIndex int       `db:"chunk_index"`
	Content    string    `db:"content"`
	CreatedAt  time.Time `db:"created_at"`
}

// PrincipalContext carries the resolved user identity and their project-level role.
type PrincipalContext struct {
	UserID      uuid.UUID
	ProjectRole string // "viewer" | "editor" | "admin" | "owner"
}

func (p PrincipalContext) CanRead() bool {
	return p.ProjectRole != ""
}

func (p PrincipalContext) CanWrite() bool {
	switch p.ProjectRole {
	case "editor", "admin", "owner":
		return true
	}
	return false
}

func (p PrincipalContext) CanAdmin() bool {
	switch p.ProjectRole {
	case "admin", "owner":
		return true
	}
	return false
}

// --- DTOs ---

type KnowledgeAssetDTO struct {
	ID               string  `json:"id"`
	ProjectID        string  `json:"projectId"`
	WikiSpaceID      *string `json:"wikiSpaceId,omitempty"`
	ParentID         *string `json:"parentId,omitempty"`
	Kind             string  `json:"kind"`
	Title            string  `json:"title"`
	Path             string  `json:"path,omitempty"`
	SortOrder        int     `json:"sortOrder"`
	ContentJSON      string  `json:"contentJson,omitempty"`
	ContentText      string  `json:"contentText,omitempty"`
	FileRef          string  `json:"fileRef,omitempty"`
	FileSize         int64   `json:"fileSize,omitempty"`
	MimeType         string  `json:"mimeType,omitempty"`
	IngestStatus     *string `json:"ingestStatus,omitempty"`
	IngestChunkCount int     `json:"ingestChunkCount,omitempty"`
	TemplateCategory string  `json:"templateCategory,omitempty"`
	IsSystemTemplate bool    `json:"isSystemTemplate,omitempty"`
	IsPinned         bool    `json:"isPinned"`
	OwnerID          *string `json:"ownerId,omitempty"`
	CreatedBy        *string `json:"createdBy,omitempty"`
	UpdatedBy        *string `json:"updatedBy,omitempty"`
	CreatedAt        string  `json:"createdAt"`
	UpdatedAt        string  `json:"updatedAt"`
	DeletedAt        *string `json:"deletedAt,omitempty"`
	Version          int64   `json:"version"`
}

func (a *KnowledgeAsset) ToDTO() KnowledgeAssetDTO {
	var ingestStatus *string
	if a.IngestStatus != nil {
		s := string(*a.IngestStatus)
		ingestStatus = &s
	}
	return KnowledgeAssetDTO{
		ID:               a.ID.String(),
		ProjectID:        a.ProjectID.String(),
		WikiSpaceID:      formatOptionalUUID(a.WikiSpaceID),
		ParentID:         formatOptionalUUID(a.ParentID),
		Kind:             string(a.Kind),
		Title:            a.Title,
		Path:             a.Path,
		SortOrder:        a.SortOrder,
		ContentJSON:      a.ContentJSON,
		ContentText:      a.ContentText,
		FileRef:          a.FileRef,
		FileSize:         a.FileSize,
		MimeType:         a.MimeType,
		IngestStatus:     ingestStatus,
		IngestChunkCount: a.IngestChunkCount,
		TemplateCategory: a.TemplateCategory,
		IsSystemTemplate: a.IsSystemTemplate,
		IsPinned:         a.IsPinned,
		OwnerID:          formatOptionalUUID(a.OwnerID),
		CreatedBy:        formatOptionalUUID(a.CreatedBy),
		UpdatedBy:        formatOptionalUUID(a.UpdatedBy),
		CreatedAt:        a.CreatedAt.Format(time.RFC3339),
		UpdatedAt:        a.UpdatedAt.Format(time.RFC3339),
		DeletedAt:        formatOptionalTime(a.DeletedAt),
		Version:          a.Version,
	}
}

type KnowledgeAssetTreeNodeDTO struct {
	KnowledgeAssetDTO
	Children []KnowledgeAssetTreeNodeDTO `json:"children"`
}

type AssetVersionDTO struct {
	ID            string  `json:"id"`
	AssetID       string  `json:"assetId"`
	VersionNumber int     `json:"versionNumber"`
	Name          string  `json:"name"`
	KindSnapshot  string  `json:"kindSnapshot"`
	ContentJSON   string  `json:"contentJson,omitempty"`
	FileRef       string  `json:"fileRef,omitempty"`
	CreatedBy     *string `json:"createdBy,omitempty"`
	CreatedAt     string  `json:"createdAt"`
}

func (v *AssetVersion) ToDTO() AssetVersionDTO {
	return AssetVersionDTO{
		ID:            v.ID.String(),
		AssetID:       v.AssetID.String(),
		VersionNumber: v.VersionNumber,
		Name:          v.Name,
		KindSnapshot:  v.KindSnapshot,
		ContentJSON:   v.ContentJSON,
		FileRef:       v.FileRef,
		CreatedBy:     formatOptionalUUID(v.CreatedBy),
		CreatedAt:     v.CreatedAt.Format(time.RFC3339),
	}
}

type AssetCommentDTO struct {
	ID              string  `json:"id"`
	AssetID         string  `json:"assetId"`
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

func (c *AssetComment) ToDTO() AssetCommentDTO {
	return AssetCommentDTO{
		ID:              c.ID.String(),
		AssetID:         c.AssetID.String(),
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

// --- Request types ---

type CreateKnowledgeAssetRequest struct {
	Kind             string  `json:"kind" validate:"required,oneof=wiki_page ingested_file template"`
	Title            string  `json:"title" validate:"required,min=1,max=200"`
	WikiSpaceID      *string `json:"wikiSpaceId,omitempty"`
	ParentID         *string `json:"parentId,omitempty"`
	ContentJSON      string  `json:"contentJson,omitempty"`
	FileRef          string  `json:"fileRef,omitempty"`
	TemplateCategory string  `json:"templateCategory,omitempty"`
}

type UpdateKnowledgeAssetRequest struct {
	Title             string  `json:"title" validate:"required,min=1,max=200"`
	ContentJSON       string  `json:"contentJson,omitempty"`
	ContentText       string  `json:"contentText,omitempty"`
	TemplateCategory  *string `json:"templateCategory,omitempty"`
	ExpectedVersion   *int64  `json:"expectedVersion,omitempty"`
}

type MoveKnowledgeAssetRequest struct {
	ParentID  *string `json:"parentId,omitempty"`
	SortOrder int     `json:"sortOrder"`
}

type CreateAssetVersionRequest struct {
	Name string `json:"name" validate:"required,min=1,max=200"`
}

type CreateAssetCommentRequest struct {
	Body            string  `json:"body" validate:"required,min=1,max=4000"`
	AnchorBlockID   *string `json:"anchorBlockId,omitempty"`
	ParentCommentID *string `json:"parentCommentId,omitempty"`
	Mentions        string  `json:"mentions,omitempty"`
}

type UpdateAssetCommentRequest struct {
	Body     *string `json:"body,omitempty"`
	Resolved *bool   `json:"resolved,omitempty"`
}

type MaterializeAsWikiRequest struct {
	WikiSpaceID string  `json:"wikiSpaceId" validate:"required"`
	ParentID    *string `json:"parentId,omitempty"`
	Title       string  `json:"title,omitempty"`
}

type DecomposeTasksFromAssetRequest struct {
	BlockIDs     []string `json:"blockIds" validate:"required,min=1,dive,min=1"`
	ParentTaskID *string  `json:"parentTaskId,omitempty"`
}

type DecomposeTasksFromAssetResponse struct {
	AssetID  string    `json:"assetId"`
	BlockIDs []string  `json:"blockIds"`
	Tasks    []TaskDTO `json:"tasks"`
}

// KnowledgeSearchResult is a single hit from the search provider.
type KnowledgeSearchResult struct {
	Asset KnowledgeAssetDTO `json:"asset"`
	Rank  float64           `json:"rank"`
	// Snippet holds a highlighted excerpt (may be empty if FTS rank alone is used).
	Snippet string `json:"snippet,omitempty"`
}

// MarshalJSON ensures Mentions is always valid JSON.
func (a *AssetComment) MarshalJSON() ([]byte, error) {
	type Alias AssetComment
	if a.Mentions == "" {
		a.Mentions = "[]"
	}
	raw := (*Alias)(a)
	_ = raw
	return json.Marshal(a.ToDTO())
}
