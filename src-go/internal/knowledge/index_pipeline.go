package knowledge

import (
	"context"
	"log"

	"github.com/google/uuid"
)

// IndexPipeline enqueues an async re-indexing job when asset content changes.
type IndexPipeline interface {
	EnqueueContentChanged(ctx context.Context, assetID uuid.UUID, kind string, projectID uuid.UUID, version int64) error
}

// NoopIndexPipeline logs the event and does nothing else.
// Use it when no external search engine integration is configured.
type NoopIndexPipeline struct{}

func (NoopIndexPipeline) EnqueueContentChanged(ctx context.Context, assetID uuid.UUID, kind string, projectID uuid.UUID, version int64) error {
	log.Printf("[knowledge] noop index pipeline: asset %s kind=%s project=%s version=%d", assetID, kind, projectID, version)
	return nil
}
