package knowledge

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
)

// jsonText is a helper GORM type that stores/loads JSON strings as TEXT/JSONB columns.
type jsonText string

func (j jsonText) String(def string) string {
	s := string(j)
	if s == "" {
		return def
	}
	return s
}

func newJSONText(value, def string) jsonText {
	if value == "" {
		return jsonText(def)
	}
	return jsonText(value)
}

func (j jsonText) Value() (driver.Value, error) {
	s := string(j)
	if s == "" {
		return nil, nil
	}
	return s, nil
}

func (j *jsonText) Scan(src any) error {
	if src == nil {
		*j = ""
		return nil
	}
	switch v := src.(type) {
	case string:
		*j = jsonText(v)
	case []byte:
		*j = jsonText(v)
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Errorf("jsonText.Scan: %w", err)
		}
		*j = jsonText(b)
	}
	return nil
}

// knowledgeAssetRecord is the GORM representation of the knowledge_assets table.
type knowledgeAssetRecord struct {
	ID               uuid.UUID  `gorm:"column:id;primaryKey"`
	ProjectID        uuid.UUID  `gorm:"column:project_id"`
	WikiSpaceID      *uuid.UUID `gorm:"column:wiki_space_id"`
	ParentID         *uuid.UUID `gorm:"column:parent_id"`
	Kind             string     `gorm:"column:kind"`
	Title            string     `gorm:"column:title"`
	Path             string     `gorm:"column:path"`
	SortOrder        int        `gorm:"column:sort_order"`
	ContentJSON      jsonText   `gorm:"column:content_json;type:jsonb"`
	ContentText      string     `gorm:"column:content_text"`
	FileRef          string     `gorm:"column:file_ref"`
	FileSize         int64      `gorm:"column:file_size"`
	MimeType         string     `gorm:"column:mime_type"`
	IngestStatus     *string    `gorm:"column:ingest_status"`
	IngestChunkCount int        `gorm:"column:ingest_chunk_count"`
	TemplateCategory string     `gorm:"column:template_category"`
	IsSystemTemplate bool       `gorm:"column:is_system_template"`
	IsPinned         bool       `gorm:"column:is_pinned"`
	OwnerID          *uuid.UUID `gorm:"column:owner_id"`
	CreatedBy        *uuid.UUID `gorm:"column:created_by"`
	UpdatedBy        *uuid.UUID `gorm:"column:updated_by"`
	CreatedAt        time.Time  `gorm:"column:created_at"`
	UpdatedAt        time.Time  `gorm:"column:updated_at"`
	DeletedAt        *time.Time `gorm:"column:deleted_at"`
	Version          int64      `gorm:"column:version"`
}

func (knowledgeAssetRecord) TableName() string { return "knowledge_assets" }

func newKnowledgeAssetRecord(a *model.KnowledgeAsset) *knowledgeAssetRecord {
	if a == nil {
		return nil
	}
	var ingestStatus *string
	if a.IngestStatus != nil {
		s := string(*a.IngestStatus)
		ingestStatus = &s
	}
	templateCategory := strings.TrimSpace(a.TemplateCategory)
	var templateCategoryPtr *string
	if templateCategory != "" {
		templateCategoryPtr = &templateCategory
	}
	_ = templateCategoryPtr // stored as plain string
	return &knowledgeAssetRecord{
		ID:               a.ID,
		ProjectID:        a.ProjectID,
		WikiSpaceID:      cloneUUID(a.WikiSpaceID),
		ParentID:         cloneUUID(a.ParentID),
		Kind:             string(a.Kind),
		Title:            a.Title,
		Path:             a.Path,
		SortOrder:        a.SortOrder,
		ContentJSON:      newJSONText(a.ContentJSON, ""),
		ContentText:      a.ContentText,
		FileRef:          a.FileRef,
		FileSize:         a.FileSize,
		MimeType:         a.MimeType,
		IngestStatus:     ingestStatus,
		IngestChunkCount: a.IngestChunkCount,
		TemplateCategory: a.TemplateCategory,
		IsSystemTemplate: a.IsSystemTemplate,
		IsPinned:         a.IsPinned,
		OwnerID:          cloneUUID(a.OwnerID),
		CreatedBy:        cloneUUID(a.CreatedBy),
		UpdatedBy:        cloneUUID(a.UpdatedBy),
		CreatedAt:        a.CreatedAt,
		UpdatedAt:        a.UpdatedAt,
		DeletedAt:        cloneTime(a.DeletedAt),
		Version:          a.Version,
	}
}

func (r *knowledgeAssetRecord) toModel() *model.KnowledgeAsset {
	if r == nil {
		return nil
	}
	var ingestStatus *model.IngestStatus
	if r.IngestStatus != nil {
		s := model.IngestStatus(*r.IngestStatus)
		ingestStatus = &s
	}
	return &model.KnowledgeAsset{
		ID:               r.ID,
		ProjectID:        r.ProjectID,
		WikiSpaceID:      cloneUUID(r.WikiSpaceID),
		ParentID:         cloneUUID(r.ParentID),
		Kind:             model.KnowledgeAssetKind(r.Kind),
		Title:            r.Title,
		Path:             r.Path,
		SortOrder:        r.SortOrder,
		ContentJSON:      r.ContentJSON.String(""),
		ContentText:      r.ContentText,
		FileRef:          r.FileRef,
		FileSize:         r.FileSize,
		MimeType:         r.MimeType,
		IngestStatus:     ingestStatus,
		IngestChunkCount: r.IngestChunkCount,
		TemplateCategory: r.TemplateCategory,
		IsSystemTemplate: r.IsSystemTemplate,
		IsPinned:         r.IsPinned,
		OwnerID:          cloneUUID(r.OwnerID),
		CreatedBy:        cloneUUID(r.CreatedBy),
		UpdatedBy:        cloneUUID(r.UpdatedBy),
		CreatedAt:        r.CreatedAt,
		UpdatedAt:        r.UpdatedAt,
		DeletedAt:        cloneTime(r.DeletedAt),
		Version:          r.Version,
	}
}

// assetVersionRecord is the GORM representation of the asset_versions table.
type assetVersionRecord struct {
	ID            uuid.UUID  `gorm:"column:id;primaryKey"`
	AssetID       uuid.UUID  `gorm:"column:asset_id"`
	VersionNumber int        `gorm:"column:version_number"`
	Name          string     `gorm:"column:name"`
	KindSnapshot  string     `gorm:"column:kind_snapshot"`
	ContentJSON   jsonText   `gorm:"column:content_json;type:jsonb"`
	FileRef       string     `gorm:"column:file_ref"`
	CreatedBy     *uuid.UUID `gorm:"column:created_by"`
	CreatedAt     time.Time  `gorm:"column:created_at"`
}

func (assetVersionRecord) TableName() string { return "asset_versions" }

func newAssetVersionRecord(v *model.AssetVersion) *assetVersionRecord {
	if v == nil {
		return nil
	}
	return &assetVersionRecord{
		ID:            v.ID,
		AssetID:       v.AssetID,
		VersionNumber: v.VersionNumber,
		Name:          v.Name,
		KindSnapshot:  v.KindSnapshot,
		ContentJSON:   newJSONText(v.ContentJSON, ""),
		FileRef:       v.FileRef,
		CreatedBy:     cloneUUID(v.CreatedBy),
		CreatedAt:     v.CreatedAt,
	}
}

func (r *assetVersionRecord) toModel() *model.AssetVersion {
	if r == nil {
		return nil
	}
	return &model.AssetVersion{
		ID:            r.ID,
		AssetID:       r.AssetID,
		VersionNumber: r.VersionNumber,
		Name:          r.Name,
		KindSnapshot:  r.KindSnapshot,
		ContentJSON:   r.ContentJSON.String(""),
		FileRef:       r.FileRef,
		CreatedBy:     cloneUUID(r.CreatedBy),
		CreatedAt:     r.CreatedAt,
	}
}

// assetCommentRecord is the GORM representation of the asset_comments table.
type assetCommentRecord struct {
	ID              uuid.UUID  `gorm:"column:id;primaryKey"`
	AssetID         uuid.UUID  `gorm:"column:asset_id"`
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

func (assetCommentRecord) TableName() string { return "asset_comments" }

func newAssetCommentRecord(c *model.AssetComment) *assetCommentRecord {
	if c == nil {
		return nil
	}
	return &assetCommentRecord{
		ID:              c.ID,
		AssetID:         c.AssetID,
		AnchorBlockID:   cloneString(c.AnchorBlockID),
		ParentCommentID: cloneUUID(c.ParentCommentID),
		Body:            c.Body,
		Mentions:        newJSONText(c.Mentions, "[]"),
		ResolvedAt:      cloneTime(c.ResolvedAt),
		CreatedBy:       cloneUUID(c.CreatedBy),
		CreatedAt:       c.CreatedAt,
		UpdatedAt:       c.UpdatedAt,
		DeletedAt:       cloneTime(c.DeletedAt),
	}
}

func (r *assetCommentRecord) toModel() *model.AssetComment {
	if r == nil {
		return nil
	}
	return &model.AssetComment{
		ID:              r.ID,
		AssetID:         r.AssetID,
		AnchorBlockID:   cloneString(r.AnchorBlockID),
		ParentCommentID: cloneUUID(r.ParentCommentID),
		Body:            r.Body,
		Mentions:        r.Mentions.String("[]"),
		ResolvedAt:      cloneTime(r.ResolvedAt),
		CreatedBy:       cloneUUID(r.CreatedBy),
		CreatedAt:       r.CreatedAt,
		UpdatedAt:       r.UpdatedAt,
		DeletedAt:       cloneTime(r.DeletedAt),
	}
}

// assetIngestChunkRecord is the GORM representation of the asset_ingest_chunks table.
type assetIngestChunkRecord struct {
	ID         uuid.UUID `gorm:"column:id;primaryKey"`
	AssetID    uuid.UUID `gorm:"column:asset_id"`
	ChunkIndex int       `gorm:"column:chunk_index"`
	Content    string    `gorm:"column:content"`
	CreatedAt  time.Time `gorm:"column:created_at"`
}

func (assetIngestChunkRecord) TableName() string { return "asset_ingest_chunks" }

func newAssetIngestChunkRecord(c *model.AssetIngestChunk) *assetIngestChunkRecord {
	if c == nil {
		return nil
	}
	return &assetIngestChunkRecord{
		ID:         c.ID,
		AssetID:    c.AssetID,
		ChunkIndex: c.ChunkIndex,
		Content:    c.Content,
		CreatedAt:  c.CreatedAt,
	}
}

func (r *assetIngestChunkRecord) toModel() *model.AssetIngestChunk {
	if r == nil {
		return nil
	}
	return &model.AssetIngestChunk{
		ID:         r.ID,
		AssetID:    r.AssetID,
		ChunkIndex: r.ChunkIndex,
		Content:    r.Content,
		CreatedAt:  r.CreatedAt,
	}
}

// ---- helpers ----

func cloneUUID(u *uuid.UUID) *uuid.UUID {
	if u == nil {
		return nil
	}
	c := *u
	return &c
}

func cloneTime(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	c := *t
	return &c
}

func cloneString(s *string) *string {
	if s == nil {
		return nil
	}
	c := *s
	return &c
}
