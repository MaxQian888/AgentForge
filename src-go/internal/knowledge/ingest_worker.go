package knowledge

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
)

// ingestParser extracts text chunks from a binary stream.
type ingestParser interface {
	Parse(r io.Reader) ([]parsedChunk, error)
}

type parsedChunk struct {
	Index   int
	Content string
}

// IngestWorker handles asynchronous ingestion of uploaded files.
type IngestWorker struct {
	assets  KnowledgeAssetRepository
	chunks  AssetIngestChunkRepository
	blobs   BlobStorage
	index   IndexPipeline
	parsers map[string]ingestParser // mime → parser
}

func NewIngestWorker(
	assets KnowledgeAssetRepository,
	chunks AssetIngestChunkRepository,
	blobs BlobStorage,
	index IndexPipeline,
) *IngestWorker {
	if index == nil {
		index = NoopIndexPipeline{}
	}
	return &IngestWorker{
		assets:  assets,
		chunks:  chunks,
		blobs:   blobs,
		index:   index,
		parsers: map[string]ingestParser{},
	}
}

// RegisterParser associates a MIME type with a parser.
func (w *IngestWorker) RegisterParser(mime string, p ingestParser) {
	w.parsers[mime] = p
}

// Ingest reads the file from blob storage, parses it, stores chunks, and
// transitions ingest_status to ready (or failed on error).
func (w *IngestWorker) Ingest(ctx context.Context, assetID uuid.UUID) error {
	a, err := w.assets.GetByID(ctx, assetID)
	if err != nil {
		return fmt.Errorf("ingest worker get asset: %w", err)
	}
	if a.Kind != model.KindIngestedFile {
		return fmt.Errorf("ingest worker: asset %s is not an ingested_file", assetID)
	}

	// Mark processing.
	status := model.IngestStatusProcessing
	if err := w.assets.UpdateIngestStatus(ctx, assetID, status, 0); err != nil {
		log.Printf("[ingest_worker] failed to mark processing for asset %s: %v", assetID, err)
	}

	// Publish ingest.status_changed → processing
	// (bus wiring is handled externally; just log for now)

	// Load blob.
	rc, err := w.blobs.Get(ctx, a.FileRef)
	if err != nil {
		return w.failIngest(ctx, assetID, fmt.Sprintf("open blob: %v", err))
	}
	defer rc.Close()

	// Select parser.
	parser, ok := w.parsers[a.MimeType]
	if !ok {
		// Fallback: treat as plain text.
		parser = &plainTextParser{}
	}

	chunks, err := parser.Parse(rc)
	if err != nil {
		return w.failIngest(ctx, assetID, fmt.Sprintf("parse: %v", err))
	}

	// Persist chunks (replace existing).
	_ = w.chunks.DeleteByAssetID(ctx, assetID)
	modelChunks := make([]*model.AssetIngestChunk, 0, len(chunks))
	for _, c := range chunks {
		modelChunks = append(modelChunks, &model.AssetIngestChunk{
			ID:         uuid.New(),
			AssetID:    assetID,
			ChunkIndex: c.Index,
			Content:    c.Content,
			CreatedAt:  time.Now().UTC(),
		})
	}
	if err := w.chunks.BulkCreate(ctx, modelChunks); err != nil {
		return w.failIngest(ctx, assetID, fmt.Sprintf("store chunks: %v", err))
	}

	// Mark ready.
	if err := w.assets.UpdateIngestStatus(ctx, assetID, model.IngestStatusReady, len(chunks)); err != nil {
		return fmt.Errorf("ingest worker mark ready: %w", err)
	}

	// Enqueue indexing.
	_ = w.index.EnqueueContentChanged(ctx, assetID, string(model.KindIngestedFile), a.ProjectID, a.Version)

	log.Printf("[ingest_worker] asset %s ingested %d chunks", assetID, len(chunks))
	return nil
}

func (w *IngestWorker) failIngest(ctx context.Context, assetID uuid.UUID, reason string) error {
	log.Printf("[ingest_worker] asset %s failed: %s", assetID, reason)
	_ = w.assets.UpdateIngestStatus(ctx, assetID, model.IngestStatusFailed, 0)
	return fmt.Errorf("ingest failed for asset %s: %s", assetID, reason)
}

// plainTextParser reads the entire reader as a single chunk.
type plainTextParser struct{}

func (p *plainTextParser) Parse(r io.Reader) ([]parsedChunk, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if len(b) == 0 {
		return nil, nil
	}
	return []parsedChunk{{Index: 0, Content: string(b)}}, nil
}
