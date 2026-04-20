package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/agentforge/server/internal/model"
	"gorm.io/gorm"
)

// documentRecord is the GORM persistence model for the documents table.
type documentRecord struct {
	ID         string    `gorm:"column:id;primaryKey"`
	ProjectID  string    `gorm:"column:project_id"`
	Name       string    `gorm:"column:name"`
	FileType   string    `gorm:"column:file_type"`
	FileSize   int64     `gorm:"column:file_size"`
	StorageKey string    `gorm:"column:storage_key"`
	Status     string    `gorm:"column:status"`
	ChunkCount int       `gorm:"column:chunk_count"`
	Error      string    `gorm:"column:error"`
	CreatedAt  time.Time `gorm:"column:created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at"`
}

func (documentRecord) TableName() string { return "documents" }

func newDocumentRecord(doc *model.Document) *documentRecord {
	if doc == nil {
		return nil
	}
	return &documentRecord{
		ID:         doc.ID,
		ProjectID:  doc.ProjectID,
		Name:       doc.Name,
		FileType:   doc.FileType,
		FileSize:   doc.FileSize,
		StorageKey: doc.StorageKey,
		Status:     string(doc.Status),
		ChunkCount: doc.ChunkCount,
		Error:      doc.Error,
		CreatedAt:  doc.CreatedAt,
		UpdatedAt:  doc.UpdatedAt,
	}
}

func (r *documentRecord) toModel() *model.Document {
	if r == nil {
		return nil
	}
	return &model.Document{
		ID:         r.ID,
		ProjectID:  r.ProjectID,
		Name:       r.Name,
		FileType:   r.FileType,
		FileSize:   r.FileSize,
		StorageKey: r.StorageKey,
		Status:     model.DocumentStatus(r.Status),
		ChunkCount: r.ChunkCount,
		Error:      r.Error,
		CreatedAt:  r.CreatedAt,
		UpdatedAt:  r.UpdatedAt,
	}
}

// DocumentRepo handles persistence for document records.
type DocumentRepo struct {
	db *gorm.DB
}

// NewDocumentRepo creates a new DocumentRepo.
func NewDocumentRepo(db *gorm.DB) *DocumentRepo {
	return &DocumentRepo{db: db}
}

// Create inserts a new document record.
func (r *DocumentRepo) Create(ctx context.Context, doc *model.Document) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).Create(newDocumentRecord(doc)).Error; err != nil {
		return fmt.Errorf("create document: %w", err)
	}
	return nil
}

// FindByID retrieves a document by its ID.
func (r *DocumentRepo) FindByID(ctx context.Context, id string) (*model.Document, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var record documentRecord
	if err := r.db.WithContext(ctx).Where("id = ?", id).Take(&record).Error; err != nil {
		return nil, fmt.Errorf("find document by id: %w", normalizeRepositoryError(err))
	}
	return record.toModel(), nil
}

// FindByProject retrieves all documents for a project.
func (r *DocumentRepo) FindByProject(ctx context.Context, projectID string) ([]model.Document, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var records []documentRecord
	if err := r.db.WithContext(ctx).Where("project_id = ?", projectID).Order("created_at DESC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("find documents by project: %w", err)
	}
	docs := make([]model.Document, len(records))
	for i := range records {
		docs[i] = *records[i].toModel()
	}
	return docs, nil
}

// Update saves changes to an existing document record.
func (r *DocumentRepo) Update(ctx context.Context, doc *model.Document) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).Save(newDocumentRecord(doc)).Error; err != nil {
		return fmt.Errorf("update document: %w", err)
	}
	return nil
}

// Delete removes a document record by ID.
func (r *DocumentRepo) Delete(ctx context.Context, id string) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).Delete(&documentRecord{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("delete document: %w", err)
	}
	return nil
}
