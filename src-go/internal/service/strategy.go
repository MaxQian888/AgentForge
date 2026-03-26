package service

import (
	"context"

	"github.com/react-go-quick-starter/server/internal/model"
)

// TeamStrategy defines a pluggable execution strategy for agent teams.
// Each strategy controls how a team transitions through phases (planning,
// executing, reviewing) and how agent runs are spawned and coordinated.
type TeamStrategy interface {
	// Name returns the canonical name used in team.Strategy.
	Name() string
	// Start is called once when the team is first created.
	// It should transition the team out of pending and spawn any initial agents.
	Start(ctx context.Context, svc *TeamService, team *model.AgentTeam, task *model.Task, input StartTeamInput) error
	// HandleRunCompletion is called whenever an agent run belonging to the team
	// reaches a terminal status. The strategy decides what to do next.
	HandleRunCompletion(ctx context.Context, svc *TeamService, team *model.AgentTeam, run *model.AgentRun) error
}
