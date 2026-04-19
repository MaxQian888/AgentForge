package model

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	MemoryScopeGlobal    = "global"
	MemoryScopeProject   = "project"
	MemoryScopeRole      = "role"
	MemoryScopeEmployee  = "employee"

	MemoryCategoryEpisodic   = "episodic"
	MemoryCategorySemantic   = "semantic"
	MemoryCategoryProcedural = "procedural"
	MemoryCategoryDocument   = "document"

	MemoryKindOperatorNote = "operator_note"
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
	EmployeeID     *uuid.UUID `db:"employee_id"`
	LastAccessedAt *time.Time `db:"last_accessed_at"`
	CreatedAt      time.Time  `db:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at"`
}

type AgentMemoryDTO struct {
	ID             string   `json:"id"`
	ProjectID      string   `json:"projectId"`
	Scope          string   `json:"scope"`
	RoleID         string   `json:"roleId"`
	Category       string   `json:"category"`
	Kind           string   `json:"kind,omitempty"`
	Tags           []string `json:"tags,omitempty"`
	Editable       bool     `json:"editable"`
	Key            string   `json:"key"`
	Content        string   `json:"content"`
	Metadata       string   `json:"metadata"`
	RelevanceScore float64  `json:"relevanceScore"`
	AccessCount    int      `json:"accessCount"`
	LastAccessedAt string   `json:"lastAccessedAt,omitempty"`
	CreatedAt      string   `json:"createdAt"`
	UpdatedAt      string   `json:"updatedAt"`
}

type AgentMemoryDetailDTO struct {
	AgentMemoryDTO
	MetadataObject map[string]any                 `json:"metadataObject,omitempty"`
	RelatedContext []AgentMemoryRelatedContextDTO `json:"relatedContext,omitempty"`
}

type AgentMemoryRelatedContextDTO struct {
	Type  string `json:"type"`
	ID    string `json:"id"`
	Label string `json:"label,omitempty"`
}

type AgentMemoryFilter struct {
	Query    string
	Scope    string
	Category string
	RoleID   string
	Tag      string
	StartAt  *time.Time
	EndAt    *time.Time
	Limit    int
}

type MemoryExplorerStatsDTO struct {
	TotalCount         int            `json:"totalCount"`
	ApproxStorageBytes int            `json:"approxStorageBytes"`
	ByCategory         map[string]int `json:"byCategory"`
	ByScope            map[string]int `json:"byScope"`
	OldestCreatedAt    string         `json:"oldestCreatedAt,omitempty"`
	NewestCreatedAt    string         `json:"newestCreatedAt,omitempty"`
	LastAccessedAt     string         `json:"lastAccessedAt,omitempty"`
}

type MemoryDeleteResultDTO struct {
	DeletedCount int64 `json:"deletedCount"`
}

func (m *AgentMemory) ToDTO() AgentMemoryDTO {
	metadata := parseAgentMemoryMetadata(m.Metadata)
	dto := AgentMemoryDTO{
		ID:             m.ID.String(),
		ProjectID:      m.ProjectID.String(),
		Scope:          m.Scope,
		RoleID:         m.RoleID,
		Category:       m.Category,
		Kind:           deriveAgentMemoryKind(metadata),
		Tags:           normalizeAgentMemoryTags(metadata["tags"]),
		Editable:       deriveAgentMemoryEditable(metadata),
		Key:            m.Key,
		Content:        m.Content,
		Metadata:       m.Metadata,
		RelevanceScore: m.RelevanceScore,
		AccessCount:    m.AccessCount,
		CreatedAt:      m.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      m.UpdatedAt.Format(time.RFC3339),
	}
	if m.LastAccessedAt != nil {
		dto.LastAccessedAt = m.LastAccessedAt.Format(time.RFC3339)
	}
	return dto
}

func (m *AgentMemory) ToDetailDTO() AgentMemoryDetailDTO {
	dto := AgentMemoryDetailDTO{AgentMemoryDTO: m.ToDTO()}
	if metadata := parseAgentMemoryMetadata(m.Metadata); len(metadata) > 0 {
		dto.MetadataObject = metadata
		dto.RelatedContext = deriveAgentMemoryRelatedContext(metadata)
	}
	return dto
}

func parseAgentMemoryMetadata(raw string) map[string]any {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var metadata map[string]any
	if err := json.Unmarshal([]byte(raw), &metadata); err != nil {
		return nil
	}
	return metadata
}

func deriveAgentMemoryRelatedContext(metadata map[string]any) []AgentMemoryRelatedContextDTO {
	if len(metadata) == 0 {
		return nil
	}
	related := make([]AgentMemoryRelatedContextDTO, 0, 2)
	if taskID, ok := metadata["taskId"].(string); ok && strings.TrimSpace(taskID) != "" {
		related = append(related, AgentMemoryRelatedContextDTO{
			Type:  "task",
			ID:    strings.TrimSpace(taskID),
			Label: "Related task",
		})
	}
	if sessionID, ok := metadata["sessionId"].(string); ok && strings.TrimSpace(sessionID) != "" {
		related = append(related, AgentMemoryRelatedContextDTO{
			Type:  "session",
			ID:    strings.TrimSpace(sessionID),
			Label: "Related session",
		})
	}
	return related
}

func deriveAgentMemoryKind(metadata map[string]any) string {
	if len(metadata) == 0 {
		return ""
	}
	kind, _ := metadata["kind"].(string)
	return strings.TrimSpace(kind)
}

func deriveAgentMemoryEditable(metadata map[string]any) bool {
	if len(metadata) == 0 {
		return false
	}
	if editable, ok := metadata["editable"].(bool); ok {
		return editable
	}
	return deriveAgentMemoryKind(metadata) == MemoryKindOperatorNote
}

func normalizeAgentMemoryTags(raw any) []string {
	var values []any
	switch typed := raw.(type) {
	case []any:
		values = typed
	case []string:
		values = make([]any, 0, len(typed))
		for _, value := range typed {
			values = append(values, value)
		}
	default:
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	tags := make([]string, 0, len(values))
	for _, item := range values {
		tag, ok := item.(string)
		if !ok {
			continue
		}
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		key := strings.ToLower(tag)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		tags = append(tags, tag)
	}
	if len(tags) == 0 {
		return nil
	}
	return tags
}
