package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

// TeamArtifactRepo defines persistence for team artifacts.
type TeamArtifactRepo interface {
	Create(ctx context.Context, artifact *model.TeamArtifact) error
	ListByTeam(ctx context.Context, teamID uuid.UUID) ([]*model.TeamArtifact, error)
	ListByTeamAndRole(ctx context.Context, teamID uuid.UUID, role string) ([]*model.TeamArtifact, error)
	GetByKey(ctx context.Context, teamID uuid.UUID, key string) (*model.TeamArtifact, error)
}

// TeamArtifactService manages structured artifacts produced by team agent runs.
type TeamArtifactService struct {
	repo TeamArtifactRepo
}

// NewTeamArtifactService creates a new team artifact service.
func NewTeamArtifactService(repo TeamArtifactRepo) *TeamArtifactService {
	return &TeamArtifactService{repo: repo}
}

// StoreFromRun extracts and stores artifacts from a completed agent run's structured output.
func (s *TeamArtifactService) StoreFromRun(ctx context.Context, teamID uuid.UUID, run *model.AgentRun) error {
	if run == nil || run.StructuredOutput == nil || len(run.StructuredOutput) == 0 {
		return nil
	}
	artifact := &model.TeamArtifact{
		ID:        uuid.New(),
		TeamID:    teamID,
		RunID:     run.ID,
		Role:      run.TeamRole,
		Key:       run.TeamRole + "_output",
		Value:     run.StructuredOutput,
		CreatedAt: time.Now().UTC(),
	}
	return s.repo.Create(ctx, artifact)
}

// BuildTeamContext builds a formatted context string from team artifacts for injecting into agent prompts.
// It excludes artifacts produced by the same role (forRole) to avoid self-referencing.
func (s *TeamArtifactService) BuildTeamContext(ctx context.Context, teamID uuid.UUID, forRole string) string {
	artifacts, err := s.repo.ListByTeam(ctx, teamID)
	if err != nil || len(artifacts) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Previous Team Results\n\n")
	included := 0
	for _, a := range artifacts {
		if a.Role == forRole {
			continue // don't include own previous output
		}
		sb.WriteString(fmt.Sprintf("### %s (%s)\n", strings.ToUpper(a.Role[:1])+a.Role[1:], a.Key))
		sb.WriteString(string(a.Value))
		sb.WriteString("\n\n")
		included++
	}
	if included == 0 {
		return ""
	}
	return sb.String()
}

// ListByTeam returns all artifacts for a team.
func (s *TeamArtifactService) ListByTeam(ctx context.Context, teamID uuid.UUID) ([]*model.TeamArtifact, error) {
	return s.repo.ListByTeam(ctx, teamID)
}

// ListByTeamAndRole returns artifacts for a team filtered by role.
func (s *TeamArtifactService) ListByTeamAndRole(ctx context.Context, teamID uuid.UUID, role string) ([]*model.TeamArtifact, error) {
	return s.repo.ListByTeamAndRole(ctx, teamID, role)
}

// logArtifactStoreError logs a warning when artifact storage fails.
func logArtifactStoreError(teamID uuid.UUID, err error) {
	log.WithError(err).WithField("teamId", teamID.String()).Warn("team service: failed to store team artifact")
}
