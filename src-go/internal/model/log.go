package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Log tab constants.
const (
	LogTabAgent  = "agent"
	LogTabSystem = "system"
)

// Log level constants.
const (
	LogLevelDebug = "debug"
	LogLevelInfo  = "info"
	LogLevelWarn  = "warn"
	LogLevelError = "error"
)

// Log represents a single log entry stored in the database.
type Log struct {
	ID           uuid.UUID       `db:"id"`
	ProjectID    uuid.UUID       `db:"project_id"`
	Tab          string          `db:"tab"`
	Level        string          `db:"level"`
	ActorType    string          `db:"actor_type"`
	ActorID      string          `db:"actor_id"`
	AgentID      *uuid.UUID      `db:"agent_id"`
	SessionID    string          `db:"session_id"`
	EventType    string          `db:"event_type"`
	Action       string          `db:"action"`
	ResourceType string          `db:"resource_type"`
	ResourceID   string          `db:"resource_id"`
	Summary      string          `db:"summary"`
	Detail       json.RawMessage `db:"detail"`
	CreatedAt    time.Time       `db:"created_at"`
}

// LogDTO is the API representation of a log entry.
type LogDTO struct {
	ID           string          `json:"id"`
	ProjectID    string          `json:"projectId"`
	Tab          string          `json:"tab"`
	Level        string          `json:"level"`
	ActorType    string          `json:"actorType"`
	ActorID      string          `json:"actorId"`
	AgentID      *string         `json:"agentId,omitempty"`
	SessionID    string          `json:"sessionId,omitempty"`
	EventType    string          `json:"eventType,omitempty"`
	Action       string          `json:"action,omitempty"`
	ResourceType string          `json:"resourceType,omitempty"`
	ResourceID   string          `json:"resourceId,omitempty"`
	Summary      string          `json:"summary"`
	Detail       json.RawMessage `json:"detail,omitempty"`
	CreatedAt    string          `json:"createdAt"`
}

// ToDTO converts a Log domain model to its API representation.
func (l *Log) ToDTO() LogDTO {
	dto := LogDTO{
		ID:           l.ID.String(),
		ProjectID:    l.ProjectID.String(),
		Tab:          l.Tab,
		Level:        l.Level,
		ActorType:    l.ActorType,
		ActorID:      l.ActorID,
		SessionID:    l.SessionID,
		EventType:    l.EventType,
		Action:       l.Action,
		ResourceType: l.ResourceType,
		ResourceID:   l.ResourceID,
		Summary:      l.Summary,
		Detail:       l.Detail,
		CreatedAt:    l.CreatedAt.Format(time.RFC3339),
	}
	if l.AgentID != nil {
		s := l.AgentID.String()
		dto.AgentID = &s
	}
	return dto
}

// LogListRequest contains parameters for listing logs.
type LogListRequest struct {
	ProjectID uuid.UUID
	Tab       string
	Page      int
	PageSize  int
	Level     string
	Search    string
	From      *time.Time
	To        *time.Time
}

// LogListResponse contains a paginated list of logs.
type LogListResponse struct {
	Items    []LogDTO `json:"items"`
	Total    int64    `json:"total"`
	Page     int      `json:"page"`
	PageSize int      `json:"pageSize"`
}

// CreateLogInput contains parameters for creating a log entry.
type CreateLogInput struct {
	ProjectID    uuid.UUID
	Tab          string
	Level        string
	ActorType    string
	ActorID      string
	AgentID      *uuid.UUID
	SessionID    string
	EventType    string
	Action       string
	ResourceType string
	ResourceID   string
	Summary      string
	Detail       map[string]any
}
