package model

import (
	"time"

	"github.com/google/uuid"
)

type FalsePositive struct {
	ID          uuid.UUID  `db:"id"`
	ProjectID   uuid.UUID  `db:"project_id"`
	Pattern     string     `db:"pattern"`
	Category    string     `db:"category"`
	FilePattern string     `db:"file_pattern"`
	Reason      string     `db:"reason"`
	ReporterID  *uuid.UUID `db:"reporter_id"`
	Occurrences int        `db:"occurrences"`
	IsStrong    bool       `db:"is_strong"`
	CreatedAt   time.Time  `db:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"`
}

type FalsePositiveDTO struct {
	ID          string  `json:"id"`
	ProjectID   string  `json:"projectId"`
	Pattern     string  `json:"pattern"`
	Category    string  `json:"category"`
	FilePattern string  `json:"filePattern"`
	Reason      string  `json:"reason"`
	ReporterID  *string `json:"reporterId,omitempty"`
	Occurrences int     `json:"occurrences"`
	IsStrong    bool    `json:"isStrong"`
	CreatedAt   string  `json:"createdAt"`
}

type CreateFalsePositiveRequest struct {
	Pattern     string `json:"pattern" validate:"required"`
	Category    string `json:"category" validate:"required"`
	FilePattern string `json:"filePattern"`
	Reason      string `json:"reason"`
}

func (fp *FalsePositive) ToDTO() FalsePositiveDTO {
	dto := FalsePositiveDTO{
		ID:          fp.ID.String(),
		ProjectID:   fp.ProjectID.String(),
		Pattern:     fp.Pattern,
		Category:    fp.Category,
		FilePattern: fp.FilePattern,
		Reason:      fp.Reason,
		Occurrences: fp.Occurrences,
		IsStrong:    fp.IsStrong,
		CreatedAt:   fp.CreatedAt.Format(time.RFC3339),
	}
	if fp.ReporterID != nil {
		s := fp.ReporterID.String()
		dto.ReporterID = &s
	}
	return dto
}
