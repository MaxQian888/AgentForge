package service

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

// --- mock document repository ---

type mockDocumentRepo struct {
	docs      map[string]*model.Document
	createErr error
}

func newMockDocumentRepo() *mockDocumentRepo {
	return &mockDocumentRepo{docs: make(map[string]*model.Document)}
}

func (m *mockDocumentRepo) Create(_ context.Context, doc *model.Document) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.docs[doc.ID] = doc
	return nil
}

func (m *mockDocumentRepo) FindByID(_ context.Context, id string) (*model.Document, error) {
	doc, ok := m.docs[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return doc, nil
}

func (m *mockDocumentRepo) FindByProject(_ context.Context, projectID string) ([]model.Document, error) {
	var result []model.Document
	for _, doc := range m.docs {
		if doc.ProjectID == projectID {
			result = append(result, *doc)
		}
	}
	return result, nil
}

func (m *mockDocumentRepo) Update(_ context.Context, doc *model.Document) error {
	m.docs[doc.ID] = doc
	return nil
}

func (m *mockDocumentRepo) Delete(_ context.Context, id string) error {
	delete(m.docs, id)
	return nil
}

// --- helpers ---

func createTestDocx(text string) []byte {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	docXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p><w:r><w:t>` + text + `</w:t></w:r></w:p>
  </w:body>
</w:document>`
	f, _ := w.Create("word/document.xml")
	f.Write([]byte(docXML))
	w.Close()
	return buf.Bytes()
}

// --- tests ---

func TestDocumentService_Upload(t *testing.T) {
	repo := newMockDocumentRepo()
	svc := NewDocumentService(repo, nil, nil)
	svc.now = func() time.Time { return time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC) }

	data := createTestDocx("Test document content")

	doc, err := svc.Upload(context.Background(), "550e8400-e29b-41d4-a716-446655440000", "test.docx", int64(len(data)), bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	if doc.Status != model.DocumentStatusReady {
		t.Errorf("Upload() status = %s, want %s", doc.Status, model.DocumentStatusReady)
	}
	if doc.ChunkCount == 0 {
		t.Error("Upload() chunkCount = 0, want > 0")
	}
	if doc.Name != "test.docx" {
		t.Errorf("Upload() name = %q, want 'test.docx'", doc.Name)
	}
	if doc.FileType != ".docx" {
		t.Errorf("Upload() fileType = %q, want '.docx'", doc.FileType)
	}
}

func TestDocumentService_Upload_UnsupportedType(t *testing.T) {
	repo := newMockDocumentRepo()
	svc := NewDocumentService(repo, nil, nil)

	_, err := svc.Upload(context.Background(), "proj-1", "test.txt", 100, strings.NewReader("hello"))
	if err == nil {
		t.Error("Upload() with .txt should return error")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Errorf("Upload() error = %v, want to contain 'unsupported'", err)
	}
}

func TestDocumentService_List(t *testing.T) {
	repo := newMockDocumentRepo()
	repo.docs["doc-1"] = &model.Document{ID: "doc-1", ProjectID: "proj-1", Name: "a.docx"}
	repo.docs["doc-2"] = &model.Document{ID: "doc-2", ProjectID: "proj-2", Name: "b.docx"}

	svc := NewDocumentService(repo, nil, nil)
	docs, err := svc.List(context.Background(), "proj-1")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(docs) != 1 {
		t.Errorf("List() returned %d docs, want 1", len(docs))
	}
}

func TestDocumentService_Get(t *testing.T) {
	repo := newMockDocumentRepo()
	repo.docs["doc-1"] = &model.Document{ID: "doc-1", ProjectID: "proj-1", Name: "test.docx"}

	svc := NewDocumentService(repo, nil, nil)
	doc, err := svc.Get(context.Background(), "doc-1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if doc.Name != "test.docx" {
		t.Errorf("Get() name = %q, want 'test.docx'", doc.Name)
	}
}

func TestDocumentService_Get_NotFound(t *testing.T) {
	repo := newMockDocumentRepo()
	svc := NewDocumentService(repo, nil, nil)

	_, err := svc.Get(context.Background(), "nonexistent")
	if err == nil {
		t.Error("Get() with nonexistent ID should return error")
	}
}

func TestDocumentService_Delete(t *testing.T) {
	repo := newMockDocumentRepo()
	repo.docs["doc-1"] = &model.Document{ID: "doc-1", ProjectID: "proj-1", Name: "test.docx", StorageKey: "documents/proj-1/doc-1/test.docx"}

	svc := NewDocumentService(repo, nil, nil)
	err := svc.Delete(context.Background(), "doc-1")
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if len(repo.docs) != 0 {
		t.Error("Delete() did not remove document from repo")
	}
}

func TestDocumentService_Upload_WithMemory(t *testing.T) {
	repo := newMockDocumentRepo()
	memoryRepo := &docTestMemoryRepo{}
	memorySvc := NewMemoryService(memoryRepo)

	svc := NewDocumentService(repo, nil, memorySvc)
	svc.now = func() time.Time { return time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC) }

	data := createTestDocx("Memory integration test")
	doc, err := svc.Upload(context.Background(), "550e8400-e29b-41d4-a716-446655440000", "test.docx", int64(len(data)), bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	if doc.Status != model.DocumentStatusReady {
		t.Errorf("Upload() status = %s, want %s", doc.Status, model.DocumentStatusReady)
	}
	if len(memoryRepo.memories) == 0 {
		t.Error("Upload() did not create memory entries")
	}
}

// docTestMemoryRepo implements MemoryRepository for testing document upload memory integration.
type docTestMemoryRepo struct {
	memories []*model.AgentMemory
}

func (m *docTestMemoryRepo) Create(_ context.Context, mem *model.AgentMemory) error {
	m.memories = append(m.memories, mem)
	return nil
}

func (m *docTestMemoryRepo) GetByID(_ context.Context, _ uuid.UUID) (*model.AgentMemory, error) {
	return nil, errors.New("not implemented")
}

func (m *docTestMemoryRepo) ListByProject(_ context.Context, _ uuid.UUID, _, _ string) ([]*model.AgentMemory, error) {
	return m.memories, nil
}

func (m *docTestMemoryRepo) Search(_ context.Context, _ uuid.UUID, _ string, _ int) ([]*model.AgentMemory, error) {
	return m.memories, nil
}

func (m *docTestMemoryRepo) IncrementAccess(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *docTestMemoryRepo) Update(_ context.Context, _ *model.AgentMemory) error {
	return nil
}

func (m *docTestMemoryRepo) Delete(_ context.Context, _ uuid.UUID) error {
	return nil
}
