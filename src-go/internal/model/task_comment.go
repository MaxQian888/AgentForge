package model

import (
	"time"

	"github.com/google/uuid"
)

type TaskComment struct {
	ID              uuid.UUID  `db:"id"`
	TaskID          uuid.UUID  `db:"task_id"`
	ParentCommentID *uuid.UUID `db:"parent_comment_id"`
	Body            string     `db:"body"`
	Mentions        []string   `db:"mentions"`
	ResolvedAt      *time.Time `db:"resolved_at"`
	CreatedBy       uuid.UUID  `db:"created_by"`
	CreatedAt       time.Time  `db:"created_at"`
	UpdatedAt       time.Time  `db:"updated_at"`
	DeletedAt       *time.Time `db:"deleted_at"`
}

type TaskCommentDTO struct {
	ID              string   `json:"id"`
	TaskID          string   `json:"taskId"`
	ParentCommentID *string  `json:"parentCommentId,omitempty"`
	Body            string   `json:"body"`
	Mentions        []string `json:"mentions"`
	ResolvedAt      *string  `json:"resolvedAt,omitempty"`
	CreatedBy       string   `json:"createdBy"`
	CreatedAt       string   `json:"createdAt"`
	UpdatedAt       string   `json:"updatedAt"`
	DeletedAt       *string  `json:"deletedAt,omitempty"`
}

func (c *TaskComment) ToDTO() TaskCommentDTO {
	dto := TaskCommentDTO{
		ID:        c.ID.String(),
		TaskID:    c.TaskID.String(),
		Body:      c.Body,
		Mentions:  append([]string(nil), c.Mentions...),
		CreatedBy: c.CreatedBy.String(),
		CreatedAt: c.CreatedAt.Format(time.RFC3339),
		UpdatedAt: c.UpdatedAt.Format(time.RFC3339),
	}
	if c.ParentCommentID != nil {
		parentID := c.ParentCommentID.String()
		dto.ParentCommentID = &parentID
	}
	if c.ResolvedAt != nil {
		resolvedAt := c.ResolvedAt.Format(time.RFC3339)
		dto.ResolvedAt = &resolvedAt
	}
	if c.DeletedAt != nil {
		deletedAt := c.DeletedAt.Format(time.RFC3339)
		dto.DeletedAt = &deletedAt
	}
	return dto
}

type CreateTaskCommentRequest struct {
	Body            string  `json:"body" validate:"required,min=1,max=4000"`
	ParentCommentID *string `json:"parentCommentId,omitempty"`
}

type UpdateTaskCommentRequest struct {
	Resolved *bool `json:"resolved,omitempty"`
}
