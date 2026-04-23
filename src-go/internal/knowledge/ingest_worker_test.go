package knowledge_test

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/agentforge/server/internal/knowledge"
	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
)

// --- stub blob storage ---

type stubBlobStorage struct {
	data map[string]string
}

func newStubBlobStorage(key, content string) *stubBlobStorage {
	return &stubBlobStorage{data: map[string]string{key: content}}
}

func (s *stubBlobStorage) Put(_ context.Context, _ uuid.UUID, fileName string, r io.Reader) (string, error) {
	b, _ := io.ReadAll(r)
	key := fileName
	s.data[key] = string(b)
	return key, nil
}

func (s *stubBlobStorage) Get(_ context.Context, key string) (io.ReadCloser, error) {
	content, ok := s.data[key]
	if !ok {
		return nil, knowledge.ErrAssetNotFound
	}
	return io.NopCloser(strings.NewReader(content)), nil
}

func (s *stubBlobStorage) Delete(_ context.Context, key string) error {
	delete(s.data, key)
	return nil
}

// --- stub chunk repo ---

type captureChunkRepo struct {
	created []*model.AssetIngestChunk
}

func (r *captureChunkRepo) BulkCreate(_ context.Context, chunks []*model.AssetIngestChunk) error {
	r.created = append(r.created, chunks...)
	return nil
}
func (r *captureChunkRepo) ListByAssetID(_ context.Context, id uuid.UUID) ([]*model.AssetIngestChunk, error) {
	return nil, nil
}
func (r *captureChunkRepo) DeleteByAssetID(_ context.Context, id uuid.UUID) error { return nil }

// --- stub asset repo that captures ingest status updates ---

type statusCaptureAssetRepo struct {
	stubAssetRepo
	lastStatus model.IngestStatus
	lastCount  int
}

func newStatusCaptureRepo(assets ...*model.KnowledgeAsset) *statusCaptureAssetRepo {
	r := &statusCaptureAssetRepo{stubAssetRepo: stubAssetRepo{store: make(map[uuid.UUID]*model.KnowledgeAsset)}}
	for _, a := range assets {
		r.store[a.ID] = a
	}
	return r
}

func (r *statusCaptureAssetRepo) UpdateIngestStatus(_ context.Context, id uuid.UUID, status model.IngestStatus, count int) error {
	r.lastStatus = status
	r.lastCount = count
	if a, ok := r.store[id]; ok {
		a.IngestStatus = &status
		a.IngestChunkCount = count
	}
	return nil
}

func TestIngestWorker_PlainText_Success(t *testing.T) {
	fileRef := "project/file.txt"
	assetID := uuid.New()
	status := model.IngestStatusPending
	asset := &model.KnowledgeAsset{
		ID:           assetID,
		ProjectID:    uuid.New(),
		Kind:         model.KindIngestedFile,
		FileRef:      fileRef,
		MimeType:     "text/plain",
		IngestStatus: &status,
		Version:      1,
	}

	assetRepo := newStatusCaptureRepo(asset)
	chunkRepo := &captureChunkRepo{}
	blobs := newStubBlobStorage(fileRef, "hello world content")

	worker := knowledge.NewIngestWorker(assetRepo, chunkRepo, blobs, nil)

	if err := worker.Ingest(context.Background(), assetID); err != nil {
		t.Fatalf("ingest failed: %v", err)
	}

	if assetRepo.lastStatus != model.IngestStatusReady {
		t.Fatalf("expected ready status, got: %v", assetRepo.lastStatus)
	}
	if len(chunkRepo.created) == 0 {
		t.Fatal("expected chunks to be created")
	}
	if chunkRepo.created[0].Content != "hello world content" {
		t.Fatalf("unexpected chunk content: %v", chunkRepo.created[0].Content)
	}
}

func TestIngestWorker_MissingBlob_MarksFailure(t *testing.T) {
	fileRef := "project/missing.txt"
	assetID := uuid.New()
	status := model.IngestStatusPending
	asset := &model.KnowledgeAsset{
		ID:           assetID,
		ProjectID:    uuid.New(),
		Kind:         model.KindIngestedFile,
		FileRef:      fileRef,
		IngestStatus: &status,
		Version:      1,
	}

	assetRepo := newStatusCaptureRepo(asset)
	chunkRepo := &captureChunkRepo{}
	blobs := newStubBlobStorage("other/key", "nothing")

	worker := knowledge.NewIngestWorker(assetRepo, chunkRepo, blobs, nil)

	err := worker.Ingest(context.Background(), assetID)
	if err == nil {
		t.Fatal("expected ingest error for missing blob")
	}

	if assetRepo.lastStatus != model.IngestStatusFailed {
		t.Fatalf("expected failed status, got: %v", assetRepo.lastStatus)
	}
}

func TestIngestWorker_RejectsOversizedFile(t *testing.T) {
	fileRef := "project/huge.txt"
	assetID := uuid.New()
	status := model.IngestStatusPending
	asset := &model.KnowledgeAsset{
		ID:           assetID,
		ProjectID:    uuid.New(),
		Kind:         model.KindIngestedFile,
		FileRef:      fileRef,
		FileSize:     101,
		MimeType:     "text/plain",
		IngestStatus: &status,
		Version:      1,
	}

	assetRepo := newStatusCaptureRepo(asset)
	chunkRepo := &captureChunkRepo{}
	blobs := newStubBlobStorage(fileRef, "this should never be parsed")

	worker := knowledge.NewIngestWorker(assetRepo, chunkRepo, blobs, nil).WithMaxFileSize(100)

	err := worker.Ingest(context.Background(), assetID)
	if err == nil {
		t.Fatal("expected ingest error for oversized file")
	}
	if assetRepo.lastStatus != model.IngestStatusFailed {
		t.Fatalf("expected failed status, got: %v", assetRepo.lastStatus)
	}
	if len(chunkRepo.created) != 0 {
		t.Fatalf("expected no chunks for oversized file, got %d", len(chunkRepo.created))
	}
}
