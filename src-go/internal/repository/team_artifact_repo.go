package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// teamArtifactRecord is the GORM persistence model for team artifacts.
type teamArtifactRecord struct {
	ID        uuid.UUID `gorm:"column:id;primaryKey"`
	TeamID    uuid.UUID `gorm:"column:team_id"`
	RunID     uuid.UUID `gorm:"column:run_id"`
	Role      string    `gorm:"column:role"`
	Key       string    `gorm:"column:key"`
	Value     rawJSON   `gorm:"column:value;type:jsonb"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (teamArtifactRecord) TableName() string { return "team_artifacts" }

func newTeamArtifactRecord(a *model.TeamArtifact) *teamArtifactRecord {
	if a == nil {
		return nil
	}
	return &teamArtifactRecord{
		ID:        a.ID,
		TeamID:    a.TeamID,
		RunID:     a.RunID,
		Role:      a.Role,
		Key:       a.Key,
		Value:     newRawJSON(a.Value, "{}"),
		CreatedAt: a.CreatedAt,
	}
}

func (r *teamArtifactRecord) toModel() *model.TeamArtifact {
	if r == nil {
		return nil
	}
	return &model.TeamArtifact{
		ID:        r.ID,
		TeamID:    r.TeamID,
		RunID:     r.RunID,
		Role:      r.Role,
		Key:       r.Key,
		Value:     json.RawMessage(r.Value.Bytes("{}")),
		CreatedAt: r.CreatedAt,
	}
}

// TeamArtifactRepository provides CRUD for team artifacts.
type TeamArtifactRepository struct {
	db *gorm.DB
}

// NewTeamArtifactRepository creates a new team artifact repository.
func NewTeamArtifactRepository(db *gorm.DB) *TeamArtifactRepository {
	return &TeamArtifactRepository{db: db}
}

// Create persists a new team artifact.
func (r *TeamArtifactRepository) Create(ctx context.Context, artifact *model.TeamArtifact) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).Create(newTeamArtifactRecord(artifact)).Error; err != nil {
		return fmt.Errorf("create team artifact: %w", err)
	}
	return nil
}

// ListByTeam returns all artifacts for a team ordered by creation time.
func (r *TeamArtifactRepository) ListByTeam(ctx context.Context, teamID uuid.UUID) ([]*model.TeamArtifact, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var records []teamArtifactRecord
	if err := r.db.WithContext(ctx).Where("team_id = ?", teamID).Order("created_at ASC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list team artifacts: %w", err)
	}
	artifacts := make([]*model.TeamArtifact, len(records))
	for i := range records {
		artifacts[i] = records[i].toModel()
	}
	return artifacts, nil
}

// ListByTeamAndRole returns artifacts for a team filtered by role.
func (r *TeamArtifactRepository) ListByTeamAndRole(ctx context.Context, teamID uuid.UUID, role string) ([]*model.TeamArtifact, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var records []teamArtifactRecord
	if err := r.db.WithContext(ctx).Where("team_id = ? AND role = ?", teamID, role).Order("created_at ASC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list team artifacts by role: %w", err)
	}
	artifacts := make([]*model.TeamArtifact, len(records))
	for i := range records {
		artifacts[i] = records[i].toModel()
	}
	return artifacts, nil
}

// GetByKey returns a single artifact by team ID and key.
func (r *TeamArtifactRepository) GetByKey(ctx context.Context, teamID uuid.UUID, key string) (*model.TeamArtifact, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var record teamArtifactRecord
	if err := r.db.WithContext(ctx).Where("team_id = ? AND key = ?", teamID, key).First(&record).Error; err != nil {
		return nil, fmt.Errorf("get team artifact by key: %w", normalizeRepositoryError(err))
	}
	return record.toModel(), nil
}
