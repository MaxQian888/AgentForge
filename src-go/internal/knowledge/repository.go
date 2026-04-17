package knowledge

import (
	"context"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

// KnowledgeAssetRepository is the data-access interface for knowledge_assets.
type KnowledgeAssetRepository interface {
	Create(ctx context.Context, a *model.KnowledgeAsset) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.KnowledgeAsset, error)
	Update(ctx context.Context, a *model.KnowledgeAsset) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
	Restore(ctx context.Context, id uuid.UUID) error
	ListByProject(ctx context.Context, projectID uuid.UUID, kind *model.KnowledgeAssetKind) ([]*model.KnowledgeAsset, error)
	ListTree(ctx context.Context, spaceID uuid.UUID) ([]*model.KnowledgeAsset, error)
	ListByParent(ctx context.Context, spaceID uuid.UUID, parentID *uuid.UUID) ([]*model.KnowledgeAsset, error)
	Move(ctx context.Context, id uuid.UUID, parentID *uuid.UUID, path string, sortOrder int) error
	UpdateIngestStatus(ctx context.Context, id uuid.UUID, status model.IngestStatus, chunkCount int) error
	// Descendants returns IDs of all assets rooted at id (for cascade soft-delete).
	Descendants(ctx context.Context, id uuid.UUID) ([]uuid.UUID, error)
}

// AssetVersionRepository manages asset_versions rows.
type AssetVersionRepository interface {
	Create(ctx context.Context, v *model.AssetVersion) error
	ListByAssetID(ctx context.Context, assetID uuid.UUID) ([]*model.AssetVersion, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.AssetVersion, error)
	MaxVersionNumber(ctx context.Context, assetID uuid.UUID) (int, error)
}

// AssetCommentRepository manages asset_comments rows.
type AssetCommentRepository interface {
	Create(ctx context.Context, c *model.AssetComment) error
	ListByAssetID(ctx context.Context, assetID uuid.UUID) ([]*model.AssetComment, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.AssetComment, error)
	Update(ctx context.Context, c *model.AssetComment) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
}

// AssetIngestChunkRepository manages asset_ingest_chunks rows.
type AssetIngestChunkRepository interface {
	BulkCreate(ctx context.Context, chunks []*model.AssetIngestChunk) error
	ListByAssetID(ctx context.Context, assetID uuid.UUID) ([]*model.AssetIngestChunk, error)
	DeleteByAssetID(ctx context.Context, assetID uuid.UUID) error
}
