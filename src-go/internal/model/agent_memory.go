package model

import (
	"time"

	"github.com/google/uuid"
)

const (
	MemoryScopeGlobal  = "global"
	MemoryScopeProject = "project"
	MemoryScopeRole    = "role"

	MemoryCategoryEpisodic   = "episodic"
	MemoryCategorySemantic   = "semantic"
	MemoryCategoryProcedural = "procedural"
)

type AgentMemory struct {
	ID             uuid.UUID  `db:"id"`
	ProjectID      uuid.UUID  `db:"project_id"`
	Scope          string     `db:"scope"`
	RoleID         string     `db:"role_id"`
	Category       string     `db:"category"`
	Key            string     `db:"key"`
	Content        string     `db:"content"`
	Metadata       string     `db:"metadata"`
	RelevanceScore float64    `db:"relevance_score"`
	AccessCount    int        `db:"access_count"`
	LastAccessedAt *time.Time `db:"last_accessed_at"`
	CreatedAt      time.Time  `db:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at"`
}

type AgentMemoryDTO struct {
	ID             string  `json:"id"`
	ProjectID      string  `json:"projectId"`
	Scope          string  `json:"scope"`
	RoleID         string  `json:"roleId"`
	Category       string  `json:"category"`
	Key            string  `json:"key"`
	Content        string  `json:"content"`
	Metadata       string  `json:"metadata"`
	RelevanceScore float64 `json:"relevanceScore"`
	AccessCount    int     `json:"accessCount"`
	CreatedAt      string  `json:"createdAt"`
}

func (m *AgentMemory) ToDTO() AgentMemoryDTO {
	return AgentMemoryDTO{
		ID:             m.ID.String(),
		ProjectID:      m.ProjectID.String(),
		Scope:          m.Scope,
		RoleID:         m.RoleID,
		Category:       m.Category,
		Key:            m.Key,
		Content:        m.Content,
		Metadata:       m.Metadata,
		RelevanceScore: m.RelevanceScore,
		AccessCount:    m.AccessCount,
		CreatedAt:      m.CreatedAt.Format(time.RFC3339),
	}
}
