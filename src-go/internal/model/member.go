package model

import (
	"time"

	"github.com/google/uuid"
)

type Member struct {
	ID          uuid.UUID  `db:"id"`
	ProjectID   uuid.UUID  `db:"project_id"`
	UserID      *uuid.UUID `db:"user_id"` // nullable for agent members
	Name        string     `db:"name"`
	Type        string     `db:"type"` // "human" or "agent"
	Role        string     `db:"role"`
	Email       string     `db:"email"`
	AvatarURL   string     `db:"avatar_url"`
	AgentConfig string     `db:"agent_config"` // JSON string
	Skills      []string   `db:"skills"`
	IsActive    bool       `db:"is_active"`
	CreatedAt   time.Time  `db:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"`
}

const (
	MemberTypeHuman = "human"
	MemberTypeAgent = "agent"
)

type MemberDTO struct {
	ID        string   `json:"id"`
	ProjectID string   `json:"projectId"`
	UserID    *string  `json:"userId,omitempty"`
	Name      string   `json:"name"`
	Type      string   `json:"type"`
	Role      string   `json:"role"`
	Email     string   `json:"email"`
	AvatarURL string   `json:"avatarUrl"`
	AgentConfig string `json:"agentConfig,omitempty"`
	Skills    []string `json:"skills"`
	IsActive  bool     `json:"isActive"`
	CreatedAt string   `json:"createdAt"`
}

type CreateMemberRequest struct {
	Name        string   `json:"name" validate:"required,min=1,max=100"`
	Type        string   `json:"type" validate:"required,oneof=human agent"`
	Role        string   `json:"role"`
	Email       string   `json:"email"`
	AgentConfig string   `json:"agentConfig"`
	Skills      []string `json:"skills"`
}

type UpdateMemberRequest struct {
	Name        *string  `json:"name"`
	Role        *string  `json:"role"`
	Email       *string  `json:"email"`
	AgentConfig *string  `json:"agentConfig"`
	Skills      *[]string `json:"skills"`
	IsActive    *bool    `json:"isActive"`
}

func (m *Member) ToDTO() MemberDTO {
	dto := MemberDTO{
		ID:        m.ID.String(),
		ProjectID: m.ProjectID.String(),
		Name:      m.Name,
		Type:      m.Type,
		Role:      m.Role,
		Email:     m.Email,
		AvatarURL: m.AvatarURL,
		AgentConfig: m.AgentConfig,
		Skills:    m.Skills,
		IsActive:  m.IsActive,
		CreatedAt: m.CreatedAt.Format(time.RFC3339),
	}
	if m.UserID != nil {
		s := m.UserID.String()
		dto.UserID = &s
	}
	return dto
}
