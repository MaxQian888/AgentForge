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
	Status      string     `db:"status"`
	Email       string     `db:"email"`
	IMPlatform  string     `db:"im_platform"`
	IMUserID    string     `db:"im_user_id"`
	AvatarURL   string     `db:"avatar_url"`
	AgentConfig string     `db:"agent_config"` // JSON string
	Skills      []string   `db:"skills"`
	IsActive    bool       `db:"is_active"`
	CreatedAt   time.Time  `db:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"`
}

const (
	MemberTypeHuman       = "human"
	MemberTypeAgent       = "agent"
	MemberStatusActive    = "active"
	MemberStatusInactive  = "inactive"
	MemberStatusSuspended = "suspended"
)

type MemberDTO struct {
	ID        string   `json:"id"`
	ProjectID string   `json:"projectId"`
	UserID    *string  `json:"userId,omitempty"`
	Name      string   `json:"name"`
	Type      string   `json:"type"`
	Role      string   `json:"role"`
	Status    string   `json:"status"`
	Email     string   `json:"email"`
	IMPlatform string  `json:"imPlatform,omitempty"`
	IMUserID  string   `json:"imUserId,omitempty"`
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
	Status      string   `json:"status" validate:"omitempty,oneof=active inactive suspended"`
	Email       string   `json:"email"`
	IMPlatform  string   `json:"imPlatform"`
	IMUserID    string   `json:"imUserId"`
	AgentConfig string   `json:"agentConfig"`
	Skills      []string `json:"skills"`
}

type UpdateMemberRequest struct {
	Name        *string   `json:"name"`
	Role        *string   `json:"role"`
	Status      *string   `json:"status" validate:"omitempty,oneof=active inactive suspended"`
	Email       *string   `json:"email"`
	IMPlatform  *string   `json:"imPlatform"`
	IMUserID    *string   `json:"imUserId"`
	AgentConfig *string   `json:"agentConfig"`
	Skills      *[]string `json:"skills"`
	IsActive    *bool     `json:"isActive"`
}

type BulkUpdateMembersRequest struct {
	MemberIDs []string `json:"memberIds" validate:"required,min=1,dive,required"`
	Status    string   `json:"status" validate:"required,oneof=active inactive suspended"`
}

type BulkUpdateMemberResult struct {
	MemberID string `json:"memberId"`
	Success  bool   `json:"success"`
	Status   string `json:"status,omitempty"`
	Error    string `json:"error,omitempty"`
}

type BulkUpdateMembersResponse struct {
	Results []BulkUpdateMemberResult `json:"results"`
}

func NormalizeMemberStatus(status string, isActive bool) string {
	switch status {
	case MemberStatusActive, MemberStatusInactive, MemberStatusSuspended:
		return status
	case "":
		if isActive {
			return MemberStatusActive
		}
		return MemberStatusInactive
	default:
		if isActive {
			return MemberStatusActive
		}
		return MemberStatusInactive
	}
}

func IsMemberStatusActive(status string) bool {
	return NormalizeMemberStatus(status, false) == MemberStatusActive
}

func (m *Member) ToDTO() MemberDTO {
	status := NormalizeMemberStatus(m.Status, m.IsActive)
	dto := MemberDTO{
		ID:        m.ID.String(),
		ProjectID: m.ProjectID.String(),
		Name:      m.Name,
		Type:      m.Type,
		Role:      m.Role,
		Status:    status,
		Email:     m.Email,
		IMPlatform: m.IMPlatform,
		IMUserID:  m.IMUserID,
		AvatarURL: m.AvatarURL,
		AgentConfig: m.AgentConfig,
		Skills:    m.Skills,
		IsActive:  IsMemberStatusActive(status),
		CreatedAt: m.CreatedAt.Format(time.RFC3339),
	}
	if m.UserID != nil {
		s := m.UserID.String()
		dto.UserID = &s
	}
	return dto
}
