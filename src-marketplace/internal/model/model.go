package model

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// Item type constants
const (
	ItemTypePlugin           = "plugin"
	ItemTypeSkill            = "skill"
	ItemTypeRole             = "role"
	ItemTypeWorkflowTemplate = "workflow_template"
)

// Pagination defaults
const (
	DefaultPageSize = 20
	MaxPageSize     = 100
)

// StringArray handles PostgreSQL text[] arrays with pq driver.
type StringArray []string

func (a StringArray) Value() (driver.Value, error) {
	return pq.Array([]string(a)).Value()
}

func (a *StringArray) Scan(src interface{}) error {
	return pq.Array((*[]string)(a)).Scan(src)
}

// MarketplaceItem maps to marketplace_items table.
type MarketplaceItem struct {
	ID            uuid.UUID       `json:"id" gorm:"type:uuid;primaryKey"`
	Type          string          `json:"type" gorm:"not null"`
	Slug          string          `json:"slug" gorm:"not null"`
	Name          string          `json:"name" gorm:"not null"`
	AuthorID      uuid.UUID       `json:"author_id" gorm:"not null"`
	AuthorName    string          `json:"author_name" gorm:"not null"`
	Description   string          `json:"description" gorm:"not null;default:''"`
	Category      string          `json:"category" gorm:"not null;default:''"`
	Tags          StringArray     `json:"tags" gorm:"type:text[];not null;default:'{}'"`
	IconURL       *string         `json:"icon_url,omitempty"`
	RepositoryURL *string         `json:"repository_url,omitempty"`
	License       string          `json:"license" gorm:"not null;default:'MIT'"`
	ExtraMetadata json.RawMessage `json:"extra_metadata" gorm:"type:jsonb;not null;default:'{}'"`
	LatestVersion *string         `json:"latest_version,omitempty"`
	DownloadCount int64           `json:"download_count" gorm:"not null;default:0"`
	AvgRating     float64         `json:"avg_rating" gorm:"type:numeric(3,2);not null;default:0"`
	RatingCount   int             `json:"rating_count" gorm:"not null;default:0"`
	IsVerified    bool            `json:"is_verified" gorm:"not null;default:false"`
	IsFeatured    bool            `json:"is_featured" gorm:"not null;default:false"`
	IsDeleted     bool            `json:"is_deleted" gorm:"not null;default:false"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
	SourceType    string          `json:"sourceType,omitempty" gorm:"-"`
	SkillPreview  *SkillPackagePreview `json:"skillPreview,omitempty" gorm:"-"`
	PreviewError  string          `json:"previewError,omitempty" gorm:"-"`
}

func (MarketplaceItem) TableName() string { return "marketplace_items" }

// MarketplaceItemVersion maps to marketplace_item_versions table.
type MarketplaceItemVersion struct {
	ID                uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	ItemID            uuid.UUID `json:"item_id" gorm:"type:uuid;not null"`
	Version           string    `json:"version" gorm:"not null"`
	Changelog         string    `json:"changelog" gorm:"not null;default:''"`
	ArtifactPath      string    `json:"artifact_path" gorm:"not null"`
	ArtifactSizeBytes int64     `json:"artifact_size_bytes" gorm:"not null;default:0"`
	ArtifactDigest    string    `json:"artifact_digest" gorm:"not null"`
	IsLatest          bool      `json:"is_latest" gorm:"not null;default:false"`
	IsYanked          bool      `json:"is_yanked" gorm:"not null;default:false"`
	CreatedAt         time.Time `json:"created_at"`
}

func (MarketplaceItemVersion) TableName() string { return "marketplace_item_versions" }

// MarketplaceReview maps to marketplace_reviews table.
type MarketplaceReview struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	ItemID    uuid.UUID `json:"item_id" gorm:"type:uuid;not null"`
	UserID    uuid.UUID `json:"user_id" gorm:"type:uuid;not null"`
	UserName  string    `json:"user_name" gorm:"not null"`
	Rating    int16     `json:"rating" gorm:"not null"`
	Comment   string    `json:"comment" gorm:"not null;default:''"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (MarketplaceReview) TableName() string { return "marketplace_reviews" }

// --------------------------------------------------------------------------
// Request types
// --------------------------------------------------------------------------

type CreateItemRequest struct {
	Type          string          `json:"type" validate:"required,oneof=plugin skill role workflow_template"`
	Slug          string          `json:"slug" validate:"required,min=2,max=100,alphanumdash"`
	Name          string          `json:"name" validate:"required,min=1,max=255"`
	Description   string          `json:"description"`
	Category      string          `json:"category"`
	Tags          []string        `json:"tags"`
	IconURL       *string         `json:"icon_url,omitempty"`
	RepositoryURL *string         `json:"repository_url,omitempty"`
	License       string          `json:"license"`
	ExtraMetadata json.RawMessage `json:"extra_metadata,omitempty"`
}

type UpdateItemRequest struct {
	Name          *string         `json:"name,omitempty"`
	Description   *string         `json:"description,omitempty"`
	Category      *string         `json:"category,omitempty"`
	Tags          []string        `json:"tags,omitempty"`
	IconURL       *string         `json:"icon_url,omitempty"`
	RepositoryURL *string         `json:"repository_url,omitempty"`
	License       *string         `json:"license,omitempty"`
	ExtraMetadata json.RawMessage `json:"extra_metadata,omitempty"`
}

type ListItemsQuery struct {
	Type     string   `query:"type"`
	Category string   `query:"category"`
	Tags     []string `query:"tags"`
	Sort     string   `query:"sort"`    // "downloads" | "rating" | "created_at"
	Page     int      `query:"page"`
	PageSize int      `query:"page_size"`
	Query    string   `query:"q"`
}

type CreateVersionRequest struct {
	Version   string `form:"version" validate:"required"`
	Changelog string `form:"changelog"`
}

type CreateReviewRequest struct {
	Rating  int16  `json:"rating" validate:"required,min=1,max=5"`
	Comment string `json:"comment"`
}

// --------------------------------------------------------------------------
// Response types
// --------------------------------------------------------------------------

type ItemListResponse struct {
	Items    []MarketplaceItem `json:"items"`
	Total    int64             `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
}

type ErrorResponse struct {
	Message string `json:"message"`
}

type SkillAgentConfigPreview struct {
	Path             string `json:"path"`
	Yaml             string `json:"yaml"`
	DisplayName      string `json:"displayName,omitempty"`
	ShortDescription string `json:"shortDescription,omitempty"`
	DefaultPrompt    string `json:"defaultPrompt,omitempty"`
}

type SkillPackagePreview struct {
	CanonicalPath   string                   `json:"canonicalPath"`
	Label           string                   `json:"label"`
	DisplayName     string                   `json:"displayName,omitempty"`
	Description     string                   `json:"description,omitempty"`
	DefaultPrompt   string                   `json:"defaultPrompt,omitempty"`
	MarkdownBody    string                   `json:"markdownBody"`
	FrontmatterYAML string                   `json:"frontmatterYaml"`
	Requires        []string                 `json:"requires,omitempty"`
	Tools           []string                 `json:"tools,omitempty"`
	AvailableParts  []string                 `json:"availableParts,omitempty"`
	ReferenceCount  int                      `json:"referenceCount,omitempty"`
	ScriptCount     int                      `json:"scriptCount,omitempty"`
	AssetCount      int                      `json:"assetCount,omitempty"`
	AgentConfigs    []SkillAgentConfigPreview `json:"agentConfigs,omitempty"`
}
