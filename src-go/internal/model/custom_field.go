package model

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	CustomFieldTypeText        = "text"
	CustomFieldTypeNumber      = "number"
	CustomFieldTypeSelect      = "select"
	CustomFieldTypeMultiSelect = "multi_select"
	CustomFieldTypeDate        = "date"
	CustomFieldTypeUser        = "user"
	CustomFieldTypeURL         = "url"
	CustomFieldTypeCheckbox    = "checkbox"
)

type CustomFieldDefinition struct {
	ID        uuid.UUID  `db:"id"`
	ProjectID uuid.UUID  `db:"project_id"`
	Name      string     `db:"name"`
	FieldType string     `db:"field_type"`
	Options   string     `db:"options"`
	SortOrder int        `db:"sort_order"`
	Required  bool       `db:"required"`
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt time.Time  `db:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at"`
}

type CustomFieldValue struct {
	ID         uuid.UUID `db:"id"`
	TaskID     uuid.UUID `db:"task_id"`
	FieldDefID uuid.UUID `db:"field_def_id"`
	Value      string    `db:"value"`
	CreatedAt  time.Time `db:"created_at"`
	UpdatedAt  time.Time `db:"updated_at"`
}

type CustomFieldDefinitionDTO struct {
	ID        string          `json:"id"`
	ProjectID string          `json:"projectId"`
	Name      string          `json:"name"`
	FieldType string          `json:"fieldType"`
	Options   json.RawMessage `json:"options"`
	SortOrder int             `json:"sortOrder"`
	Required  bool            `json:"required"`
	CreatedAt string          `json:"createdAt"`
	UpdatedAt string          `json:"updatedAt"`
	DeletedAt *string         `json:"deletedAt,omitempty"`
}

type CustomFieldValueDTO struct {
	ID         string          `json:"id"`
	TaskID     string          `json:"taskId"`
	FieldDefID string          `json:"fieldDefId"`
	Value      json.RawMessage `json:"value"`
	CreatedAt  string          `json:"createdAt"`
	UpdatedAt  string          `json:"updatedAt"`
}

type CreateCustomFieldRequest struct {
	Name      string          `json:"name" validate:"required,min=1,max=100"`
	FieldType string          `json:"fieldType" validate:"required"`
	Options   json.RawMessage `json:"options"`
	Required  bool            `json:"required"`
}

type UpdateCustomFieldRequest struct {
	Name      *string         `json:"name"`
	FieldType *string         `json:"fieldType"`
	Options   json.RawMessage `json:"options"`
	Required  *bool           `json:"required"`
	SortOrder *int            `json:"sortOrder"`
}

type ReorderCustomFieldsRequest struct {
	FieldIDs []string `json:"fieldIds" validate:"required,min=1,dive,required"`
}

type SetCustomFieldValueRequest struct {
	Value json.RawMessage `json:"value"`
}

func (d *CustomFieldDefinition) ToDTO() CustomFieldDefinitionDTO {
	dto := CustomFieldDefinitionDTO{
		ID:        d.ID.String(),
		ProjectID: d.ProjectID.String(),
		Name:      d.Name,
		FieldType: d.FieldType,
		Options:   normalizeJSONRawMessage(d.Options, []byte("[]")),
		SortOrder: d.SortOrder,
		Required:  d.Required,
		CreatedAt: d.CreatedAt.Format(time.RFC3339),
		UpdatedAt: d.UpdatedAt.Format(time.RFC3339),
	}
	if d.DeletedAt != nil {
		value := d.DeletedAt.Format(time.RFC3339)
		dto.DeletedAt = &value
	}
	return dto
}

func (v *CustomFieldValue) ToDTO() CustomFieldValueDTO {
	return CustomFieldValueDTO{
		ID:         v.ID.String(),
		TaskID:     v.TaskID.String(),
		FieldDefID: v.FieldDefID.String(),
		Value:      normalizeJSONRawMessage(v.Value, []byte("null")),
		CreatedAt:  v.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  v.UpdatedAt.Format(time.RFC3339),
	}
}

func normalizeJSONRawMessage(raw string, fallback []byte) json.RawMessage {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return append(json.RawMessage(nil), fallback...)
	}
	return json.RawMessage(trimmed)
}
