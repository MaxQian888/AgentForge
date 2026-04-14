package model

import "time"

// DocumentStatus represents the processing state of an uploaded document.
type DocumentStatus string

const (
	DocumentStatusPending    DocumentStatus = "pending"
	DocumentStatusProcessing DocumentStatus = "processing"
	DocumentStatusReady      DocumentStatus = "ready"
	DocumentStatusFailed     DocumentStatus = "failed"
)

// Document represents an uploaded Office document that has been parsed into
// memory chunks for agent consumption.
type Document struct {
	ID         string         `json:"id"`
	ProjectID  string         `json:"project_id"`
	Name       string         `json:"name"`
	FileType   string         `json:"file_type"`
	FileSize   int64          `json:"file_size"`
	StorageKey string         `json:"-"`
	Status     DocumentStatus `json:"status"`
	ChunkCount int            `json:"chunk_count"`
	Error      string         `json:"error,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

// DocumentDTO is the JSON-serializable representation returned by the API.
type DocumentDTO struct {
	ID         string `json:"id"`
	ProjectID  string `json:"projectId"`
	Name       string `json:"name"`
	FileType   string `json:"fileType"`
	FileSize   int64  `json:"fileSize"`
	Status     string `json:"status"`
	ChunkCount int    `json:"chunkCount"`
	Error      string `json:"error,omitempty"`
	CreatedAt  string `json:"createdAt"`
	UpdatedAt  string `json:"updatedAt"`
}

// ToDTO converts a Document to its API representation.
func (d *Document) ToDTO() DocumentDTO {
	return DocumentDTO{
		ID:         d.ID,
		ProjectID:  d.ProjectID,
		Name:       d.Name,
		FileType:   d.FileType,
		FileSize:   d.FileSize,
		Status:     string(d.Status),
		ChunkCount: d.ChunkCount,
		Error:      d.Error,
		CreatedAt:  d.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  d.UpdatedAt.Format(time.RFC3339),
	}
}
