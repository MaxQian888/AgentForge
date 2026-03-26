package service

import (
	"context"
	"fmt"

	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/ws"
	log "github.com/sirupsen/logrus"
)

// SwarmStrategy spawns all subtasks in parallel immediately after planning.
// There is no dependency ordering - every subtask runs concurrently.
// After all coders complete, a reviewer is spawned.
type SwarmStrategy struct{}

func (SwarmStrategy) Name() string { return "swarm" }

func (SwarmStrategy) Start(ctx context.Context, svc *TeamService, team *model.AgentTeam, task *model.Task, input StartTeamInput) error {
	if err := svc.teamRepo.UpdateStatus(ctx, team.ID, model.TeamStatusPlanning); err != nil {
		return fmt.Errorf("transition team to planning: %w", err)
	}
	team.Status = model.TeamStatusPlanning
	svc.broadcastEvent(ws.EventTeamPlanning, task.ProjectID.String(), team.ToDTO())

	selection := team.CodingAgentSelection()
	plannerBudget := team.TotalBudgetUsd * 0.2
	if plannerBudget < 1 {
		plannerBudget = 1
	}

	plannerRun, err := svc.spawner.SpawnForTeam(ctx, team.ID, model.TeamRolePlanner, task.ID, input.MemberID, selection.Runtime, selection.Provider, selection.Model, plannerBudget, "planner-agent")
	if err != nil {
		_ = svc.teamRepo.UpdateStatusWithError(ctx, team.ID, model.TeamStatusFailed, fmt.Sprintf("failed to spawn planner: %v", err))
		team.Status = model.TeamStatusFailed
		svc.broadcastEvent(ws.EventTeamFailed, task.ProjectID.String(), team.ToDTO())
		return fmt.Errorf("spawn planner: %w", err)
	}

	if err := svc.runRepo.SetTeamFields(ctx, plannerRun.ID, team.ID, model.TeamRolePlanner); err != nil {
		return fmt.Errorf("set planner team fields: %w", err)
	}
	if err := svc.teamRepo.SetPlannerRun(ctx, team.ID, plannerRun.ID); err != nil {
		return fmt.Errorf("set team planner run: %w", err)
	}
	team.PlannerRunID = &plannerRun.ID
	return nil
}

func (SwarmStrategy) HandleRunCompletion(ctx context.Context, svc *TeamService, team *model.AgentTeam, run *model.AgentRun) error {
	switch run.TeamRole {
	case model.TeamRolePlanner:
		handleSwarmPlannerDone(ctx, svc, team, run)
	case model.TeamRoleCoder:
		// Reuse the plan-code-review coder-done logic: wait for all, then review
		handlePCRCoderDone(ctx, svc, team, run)
	case model.TeamRoleReviewer:
		handlePCRReviewerDone(ctx, svc, team, run)
	default:
		log.WithFields(teamRunLogFields(team, run)).Warn("swarm: unknown team role")
	}
	return nil
}

func handleSwarmPlannerDone(ctx context.Context, svc *TeamService, team *model.AgentTeam, run *model.AgentRun) {
	if run.Status != model.AgentRunStatusCompleted {
		errMsg := "planner failed"
		if run.ErrorMessage != "" {
			errMsg = run.ErrorMessage
		}
		_ = svc.teamRepo.UpdateStatusWithError(ctx, team.ID, model.TeamStatusFailed, errMsg)
		team.Status = model.TeamStatusFailed
		svc.broadcastEvent(ws.EventTeamFailed, team.ProjectID.String(), team.ToDTO())
		return
	}

	if err := svc.teamRepo.UpdateStatus(ctx, team.ID, model.TeamStatusExecuting); err != nil {
		return
	}
	team.Status = model.TeamStatusExecuting
	svc.broadcastEvent(ws.EventTeamExecuting, team.ProjectID.String(), team.ToDTO())

	task, err := svc.taskRepo.GetByID(ctx, team.TaskID)
	if err != nil {
		_ = svc.teamRepo.UpdateStatusWithError(ctx, team.ID, model.TeamStatusFailed, "failed to get task")
		return
	}

	// Try to get children via ListChildren for full parallel spawn
	children, err := svc.taskRepo.ListChildren(ctx, task.ID)
	if err != nil || len(children) == 0 {
		// Fallback: check HasChildren / create default
		hasChildren, err := svc.taskRepo.HasChildren(ctx, task.ID)
		if err != nil || !hasChildren {
			children, err = svc.taskRepo.CreateChildren(ctx, []model.TaskChildInput{
				{
					ParentID:    task.ID,
					ProjectID:   task.ProjectID,
					SprintID:    task.SprintID,
					ReporterID:  task.ReporterID,
					Title:       task.Title + " - Implementation",
					Description: task.Description,
					Priority:    task.Priority,
					Labels:      task.Labels,
					BudgetUSD:   task.BudgetUsd * 0.6,
				},
			})
			if err != nil {
				_ = svc.teamRepo.UpdateStatusWithError(ctx, team.ID, model.TeamStatusFailed, "failed to create subtasks")
				return
			}
		} else {
			// Has children but ListChildren failed - fall back to single spawn
			svc.spawnCodersForTask(ctx, team, task)
			return
		}
	}

	log.WithFields(log.Fields{
		"teamId":     team.ID.String(),
		"childCount": len(children),
	}).Info("swarm: spawning all coders in parallel")
	svc.spawnCodersForTasks(ctx, team, task, children)
}
