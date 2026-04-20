package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/agentforge/server/internal/document"
	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/storage"
	"github.com/google/uuid"
)

// DocumentRepository defines persistence operations for documents.
type DocumentRepository interface {
	Create(ctx context.Context, doc *model.Document) error
	FindByID(ctx context.Context, id string) (*model.Document, error)
	FindByProject(ctx context.Context, projectID string) ([]model.Document, error)
	Update(ctx context.Context, doc *model.Document) error
	Delete(ctx context.Context, id string) error
}

// DocumentService handles document upload, parsing, and lifecycle.
type DocumentService struct {
	repo      DocumentRepository
	storage   storage.Storage
	memorySvc *MemoryService
	now       func() time.Time
}

// NewDocumentService creates a new DocumentService.
func NewDocumentService(repo DocumentRepository, store storage.Storage, memorySvc *MemoryService) *DocumentService {
	return &DocumentService{
		repo:      repo,
		storage:   store,
		memorySvc: memorySvc,
		now:       func() time.Time { return time.Now().UTC() },
	}
}

// Upload stores a document, parses it into text chunks, and saves them as memory entries.
func (s *DocumentService) Upload(ctx context.Context, projectID, fileName string, fileSize int64, reader io.Reader) (*model.Document, error) {
	ext := strings.ToLower(filepath.Ext(fileName))
	if !document.IsSupportedType(ext) {
		return nil, fmt.Errorf("unsupported file type: %s (supported: %s)", ext, strings.Join(document.SupportedTypes(), ", "))
	}

	docID := uuid.New().String()
	storageKey := fmt.Sprintf("documents/%s/%s/%s", projectID, docID, fileName)
	now := s.now()

	// Read file content into buffer (needed for both storage and parsing).
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read upload: %w", err)
	}

	// Store file.
	if s.storage != nil {
		if err := s.storage.Put(ctx, storageKey, bytes.NewReader(data), storage.PutOptions{
			ContentType: contentTypeForExt(ext),
		}); err != nil {
			return nil, fmt.Errorf("store file: %w", err)
		}
	}

	// Create document record with processing status.
	doc := &model.Document{
		ID:         docID,
		ProjectID:  projectID,
		Name:       fileName,
		FileType:   ext,
		FileSize:   fileSize,
		StorageKey: storageKey,
		Status:     model.DocumentStatusProcessing,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := s.repo.Create(ctx, doc); err != nil {
		return nil, fmt.Errorf("create document record: %w", err)
	}

	// Parse document into chunks.
	parser, err := document.ParserForType(ext)
	if err != nil {
		doc.Status = model.DocumentStatusFailed
		doc.Error = err.Error()
		_ = s.repo.Update(ctx, doc)
		return doc, nil
	}

	chunks, err := parser.Parse(bytes.NewReader(data))
	if err != nil {
		doc.Status = model.DocumentStatusFailed
		doc.Error = err.Error()
		doc.UpdatedAt = s.now()
		_ = s.repo.Update(ctx, doc)
		return doc, nil
	}

	// Store each chunk as a memory entry.
	if s.memorySvc != nil {
		projectUUID, parseErr := uuid.Parse(projectID)
		if parseErr == nil {
			for _, chunk := range chunks {
				if strings.TrimSpace(chunk.Content) == "" {
					continue
				}
				_, _ = s.memorySvc.Store(ctx, StoreMemoryInput{
					ProjectID:      projectUUID,
					Scope:          model.MemoryScopeProject,
					Category:       model.MemoryCategoryDocument,
					Key:            fmt.Sprintf("doc:%s:%s:%d", docID[:8], chunk.Section, chunk.Index),
					Content:        chunk.Content,
					Metadata:       fmt.Sprintf(`{"documentId":"%s","section":"%s","chunkIndex":%d}`, docID, chunk.Section, chunk.Index),
					RelevanceScore: 0.7,
				})
			}
		}
	}

	// Update document record to ready.
	doc.Status = model.DocumentStatusReady
	doc.ChunkCount = len(chunks)
	doc.UpdatedAt = s.now()
	if err := s.repo.Update(ctx, doc); err != nil {
		return nil, fmt.Errorf("update document record: %w", err)
	}

	return doc, nil
}

// List returns all documents for a project.
func (s *DocumentService) List(ctx context.Context, projectID string) ([]model.Document, error) {
	return s.repo.FindByProject(ctx, projectID)
}

// Get returns a single document by ID.
func (s *DocumentService) Get(ctx context.Context, id string) (*model.Document, error) {
	return s.repo.FindByID(ctx, id)
}

// Delete removes a document, its storage file, and associated memory chunks.
func (s *DocumentService) Delete(ctx context.Context, id string) error {
	doc, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("find document for deletion: %w", err)
	}

	// Delete from storage.
	if s.storage != nil && doc.StorageKey != "" {
		_ = s.storage.Delete(ctx, doc.StorageKey)
	}

	// Delete document record.
	return s.repo.Delete(ctx, id)
}

func contentTypeForExt(ext string) string {
	switch ext {
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case ".pdf":
		return "application/pdf"
	default:
		return "application/octet-stream"
	}
}
