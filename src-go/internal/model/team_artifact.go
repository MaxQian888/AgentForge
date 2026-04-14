package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// TeamArtifact stores structured output from a team agent run, enabling
// downstream agents to consume results from upstream phases.
type TeamArtifact struct {
	ID        uuid.UUID       `db:"id"`
	TeamID    uuid.UUID       `db:"team_id"`
	RunID     uuid.UUID       `db:"run_id"`
	Role      string          `db:"role"`
	Key       string          `db:"key"`
	Value     json.RawMessage `db:"value"`
	CreatedAt time.Time       `db:"created_at"`
}

// TeamArtifactDTO is the JSON-serializable representation of a team artifact.
type TeamArtifactDTO struct {
	ID        string          `json:"id"`
	TeamID    string          `json:"teamId"`
	RunID     string          `json:"runId"`
	Role      string          `json:"role"`
	Key       string          `json:"key"`
	Value     json.RawMessage `json:"value"`
	CreatedAt string          `json:"createdAt"`
}

// ToDTO converts a TeamArtifact to its DTO representation.
func (a *TeamArtifact) ToDTO() TeamArtifactDTO {
	return TeamArtifactDTO{
		ID:        a.ID.String(),
		TeamID:    a.TeamID.String(),
		RunID:     a.RunID.String(),
		Role:      a.Role,
		Key:       a.Key,
		Value:     a.Value,
		CreatedAt: a.CreatedAt.Format(time.RFC3339),
	}
}
