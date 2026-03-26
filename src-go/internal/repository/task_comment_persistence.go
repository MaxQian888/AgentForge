package repository

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

type taskCommentRecord struct {
	ID              uuid.UUID  `gorm:"column:id;primaryKey"`
	TaskID          uuid.UUID  `gorm:"column:task_id"`
	ParentCommentID *uuid.UUID `gorm:"column:parent_comment_id"`
	Body            string     `gorm:"column:body"`
	Mentions        jsonText   `gorm:"column:mentions;type:jsonb"`
	ResolvedAt      *time.Time `gorm:"column:resolved_at"`
	CreatedBy       uuid.UUID  `gorm:"column:created_by"`
	CreatedAt       time.Time  `gorm:"column:created_at"`
	UpdatedAt       time.Time  `gorm:"column:updated_at"`
	DeletedAt       *time.Time `gorm:"column:deleted_at"`
}

func (taskCommentRecord) TableName() string { return "task_comments" }

func newTaskCommentRecord(comment *model.TaskComment) *taskCommentRecord {
	if comment == nil {
		return nil
	}
	return &taskCommentRecord{
		ID:              comment.ID,
		TaskID:          comment.TaskID,
		ParentCommentID: cloneUUIDPointer(comment.ParentCommentID),
		Body:            comment.Body,
		Mentions:        newJSONText(mustMarshalStringSlice(comment.Mentions), "[]"),
		ResolvedAt:      cloneTimePointer(comment.ResolvedAt),
		CreatedBy:       comment.CreatedBy,
		CreatedAt:       comment.CreatedAt,
		UpdatedAt:       comment.UpdatedAt,
		DeletedAt:       cloneTimePointer(comment.DeletedAt),
	}
}

func (r *taskCommentRecord) toModel() (*model.TaskComment, error) {
	if r == nil {
		return nil, nil
	}
	mentions, err := unmarshalStringSlice(r.Mentions.String("[]"))
	if err != nil {
		return nil, fmt.Errorf("unmarshal task comment mentions: %w", err)
	}
	return &model.TaskComment{
		ID:              r.ID,
		TaskID:          r.TaskID,
		ParentCommentID: cloneUUIDPointer(r.ParentCommentID),
		Body:            r.Body,
		Mentions:        mentions,
		ResolvedAt:      cloneTimePointer(r.ResolvedAt),
		CreatedBy:       r.CreatedBy,
		CreatedAt:       r.CreatedAt,
		UpdatedAt:       r.UpdatedAt,
		DeletedAt:       cloneTimePointer(r.DeletedAt),
	}, nil
}

func mustMarshalStringSlice(values []string) string {
	if len(values) == 0 {
		return "[]"
	}
	payload, err := json.Marshal(values)
	if err != nil {
		return "[]"
	}
	return string(payload)
}

func unmarshalStringSlice(raw string) ([]string, error) {
	if raw == "" {
		return []string{}, nil
	}
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil, err
	}
	if values == nil {
		return []string{}, nil
	}
	return values, nil
}
