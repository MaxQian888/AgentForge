package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type FormDefinition struct {
	ID             uuid.UUID  `db:"id"`
	ProjectID      uuid.UUID  `db:"project_id"`
	Name           string     `db:"name"`
	Slug           string     `db:"slug"`
	Fields         string     `db:"fields"`
	TargetStatus   string     `db:"target_status"`
	TargetAssignee *uuid.UUID `db:"target_assignee"`
	IsPublic       bool       `db:"is_public"`
	CreatedAt      time.Time  `db:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at"`
	DeletedAt      *time.Time `db:"deleted_at"`
}

type FormSubmission struct {
	ID          uuid.UUID `db:"id"`
	FormID      uuid.UUID `db:"form_id"`
	TaskID      uuid.UUID `db:"task_id"`
	SubmittedBy string    `db:"submitted_by"`
	SubmittedAt time.Time `db:"submitted_at"`
	IPAddress   string    `db:"ip_address"`
}

type FormDefinitionDTO struct {
	ID             string          `json:"id"`
	ProjectID      string          `json:"projectId"`
	Name           string          `json:"name"`
	Slug           string          `json:"slug"`
	Fields         json.RawMessage `json:"fields"`
	TargetStatus   string          `json:"targetStatus"`
	TargetAssignee *string         `json:"targetAssignee,omitempty"`
	IsPublic       bool            `json:"isPublic"`
	CreatedAt      string          `json:"createdAt"`
	UpdatedAt      string          `json:"updatedAt"`
	DeletedAt      *string         `json:"deletedAt,omitempty"`
}

type FormSubmissionDTO struct {
	ID          string `json:"id"`
	FormID      string `json:"formId"`
	TaskID      string `json:"taskId"`
	SubmittedBy string `json:"submittedBy"`
	SubmittedAt string `json:"submittedAt"`
	IPAddress   string `json:"ipAddress"`
}

type CreateFormRequest struct {
	Name           string          `json:"name" validate:"required,min=1,max=100"`
	Slug           string          `json:"slug" validate:"required,min=1,max=120"`
	Fields         json.RawMessage `json:"fields" validate:"required"`
	TargetStatus   string          `json:"targetStatus"`
	TargetAssignee *string         `json:"targetAssignee"`
	IsPublic       bool            `json:"isPublic"`
}

type UpdateFormRequest struct {
	Name           *string         `json:"name"`
	Slug           *string         `json:"slug"`
	Fields         json.RawMessage `json:"fields"`
	TargetStatus   *string         `json:"targetStatus"`
	TargetAssignee *string         `json:"targetAssignee"`
	IsPublic       *bool           `json:"isPublic"`
}

type SubmitFormRequest struct {
	SubmittedBy string            `json:"submittedBy"`
	Values      map[string]string `json:"values" validate:"required"`
}

func (f *FormDefinition) ToDTO() FormDefinitionDTO {
	dto := FormDefinitionDTO{
		ID:           f.ID.String(),
		ProjectID:    f.ProjectID.String(),
		Name:         f.Name,
		Slug:         f.Slug,
		Fields:       normalizeJSONRawMessage(f.Fields, []byte("[]")),
		TargetStatus: f.TargetStatus,
		IsPublic:     f.IsPublic,
		CreatedAt:    f.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    f.UpdatedAt.Format(time.RFC3339),
	}
	if f.TargetAssignee != nil {
		value := f.TargetAssignee.String()
		dto.TargetAssignee = &value
	}
	if f.DeletedAt != nil {
		value := f.DeletedAt.Format(time.RFC3339)
		dto.DeletedAt = &value
	}
	return dto
}

func (s *FormSubmission) ToDTO() FormSubmissionDTO {
	return FormSubmissionDTO{
		ID:          s.ID.String(),
		FormID:      s.FormID.String(),
		TaskID:      s.TaskID.String(),
		SubmittedBy: s.SubmittedBy,
		SubmittedAt: s.SubmittedAt.Format(time.RFC3339),
		IPAddress:   s.IPAddress,
	}
}
