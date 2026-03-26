package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type SavedView struct {
	ID         uuid.UUID  `db:"id"`
	ProjectID  uuid.UUID  `db:"project_id"`
	Name       string     `db:"name"`
	OwnerID    *uuid.UUID `db:"owner_id"`
	IsDefault  bool       `db:"is_default"`
	SharedWith string     `db:"shared_with"`
	Config     string     `db:"config"`
	CreatedAt  time.Time  `db:"created_at"`
	UpdatedAt  time.Time  `db:"updated_at"`
	DeletedAt  *time.Time `db:"deleted_at"`
}

type SavedViewShareConfig struct {
	RoleIDs   []string `json:"roleIds"`
	MemberIDs []string `json:"memberIds"`
}

type SavedViewDTO struct {
	ID         string          `json:"id"`
	ProjectID  string          `json:"projectId"`
	Name       string          `json:"name"`
	OwnerID    *string         `json:"ownerId,omitempty"`
	IsDefault  bool            `json:"isDefault"`
	SharedWith json.RawMessage `json:"sharedWith"`
	Config     json.RawMessage `json:"config"`
	CreatedAt  string          `json:"createdAt"`
	UpdatedAt  string          `json:"updatedAt"`
	DeletedAt  *string         `json:"deletedAt,omitempty"`
}

type CreateSavedViewRequest struct {
	Name       string          `json:"name" validate:"required,min=1,max=100"`
	OwnerID    *string         `json:"ownerId"`
	IsDefault  bool            `json:"isDefault"`
	SharedWith json.RawMessage `json:"sharedWith"`
	Config     json.RawMessage `json:"config" validate:"required"`
}

type UpdateSavedViewRequest struct {
	Name       *string         `json:"name"`
	OwnerID    *string         `json:"ownerId"`
	IsDefault  *bool           `json:"isDefault"`
	SharedWith json.RawMessage `json:"sharedWith"`
	Config     json.RawMessage `json:"config"`
}

func (v *SavedView) ShareConfig() SavedViewShareConfig {
	if v == nil || v.SharedWith == "" {
		return SavedViewShareConfig{}
	}

	var cfg SavedViewShareConfig
	if err := json.Unmarshal([]byte(v.SharedWith), &cfg); err != nil {
		return SavedViewShareConfig{}
	}
	return cfg
}

func (v *SavedView) IsAccessibleTo(userID uuid.UUID, roles []string) bool {
	if v == nil {
		return false
	}
	if v.OwnerID != nil && *v.OwnerID == userID {
		return true
	}

	cfg := v.ShareConfig()
	if v.OwnerID == nil && len(cfg.MemberIDs) == 0 && len(cfg.RoleIDs) == 0 {
		return true
	}
	for _, memberID := range cfg.MemberIDs {
		if memberID == userID.String() {
			return true
		}
	}
	for _, roleID := range cfg.RoleIDs {
		for _, candidate := range roles {
			if roleID == candidate {
				return true
			}
		}
	}
	return false
}

func (v *SavedView) ToDTO() SavedViewDTO {
	dto := SavedViewDTO{
		ID:         v.ID.String(),
		ProjectID:  v.ProjectID.String(),
		Name:       v.Name,
		IsDefault:  v.IsDefault,
		SharedWith: normalizeJSONRawMessage(v.SharedWith, []byte("{}")),
		Config:     normalizeJSONRawMessage(v.Config, []byte("{}")),
		CreatedAt:  v.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  v.UpdatedAt.Format(time.RFC3339),
	}
	if v.OwnerID != nil {
		value := v.OwnerID.String()
		dto.OwnerID = &value
	}
	if v.DeletedAt != nil {
		value := v.DeletedAt.Format(time.RFC3339)
		dto.DeletedAt = &value
	}
	return dto
}
