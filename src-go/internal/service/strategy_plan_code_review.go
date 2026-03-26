package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/ws"
	log "github.com/sirupsen/logrus"
)

// PlanCodeReviewStrategy implements the original planner -> coders -> reviewer pipeline.
// This is the default strategy and preserves the exact behavior that existed before
// the strategy abstraction was introduced.
type PlanCodeReviewStrategy struct{}

func (PlanCodeReviewStrategy) Name() string { return "plan-code-review" }

func (PlanCodeReviewStrategy) Start(ctx context.Context, svc *TeamService, team *model.AgentTeam, task *model.Task, input StartTeamInput) error {
	// Transition to planning
	if err := svc.teamRepo.UpdateStatus(ctx, team.ID, model.TeamStatusPlanning); err != nil {
		return fmt.Errorf("transition team to planning: %w", err)
	}
	team.Status = model.TeamStatusPlanning
	log.WithFields(teamLogFields(team)).Info("team transitioned to planning")
	svc.broadcastEvent(ws.EventTeamPlanning, task.ProjectID.String(), team.ToDTO())

	selection := team.CodingAgentSelection()
	plannerBudget := team.TotalBudgetUsd * 0.2
	if plannerBudget < 1 {
		plannerBudget = 1
	}

	plannerRun, err := svc.spawner.SpawnForTeam(ctx, team.ID, model.TeamRolePlanner, task.ID, input.MemberID, selection.Runtime, selection.Provider, selection.Model, plannerBudget, "planner-agent")
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"teamId":    team.ID.String(),
			"projectId": task.ProjectID.String(),
			"taskId":    task.ID.String(),
			"runtime":   selection.Runtime,
			"provider":  selection.Provider,
			"model":     selection.Model,
			"budgetUsd": plannerBudget,
		}).Error("team failed to spawn planner")
		_ = svc.teamRepo.UpdateStatusWithError(ctx, team.ID, model.TeamStatusFailed, fmt.Sprintf("failed to spawn planner: %v", err))
		team.Status = model.TeamStatusFailed
		svc.broadcastEvent(ws.EventTeamFailed, task.ProjectID.String(), team.ToDTO())
		return fmt.Errorf("spawn planner: %w", err)
	}
	log.WithFields(log.Fields{
		"teamId":    team.ID.String(),
		"projectId": task.ProjectID.String(),
		"taskId":    task.ID.String(),
		"runId":     plannerRun.ID.String(),
		"role":      model.TeamRolePlanner,
		"runtime":   selection.Runtime,
		"provider":  selection.Provider,
		"model":     selection.Model,
		"budgetUsd": plannerBudget,
	}).Info("team spawned planner")

	if err := svc.runRepo.SetTeamFields(ctx, plannerRun.ID, team.ID, model.TeamRolePlanner); err != nil {
		return fmt.Errorf("set planner team fields: %w", err)
	}
	if err := svc.teamRepo.SetPlannerRun(ctx, team.ID, plannerRun.ID); err != nil {
		return fmt.Errorf("set team planner run: %w", err)
	}
	team.PlannerRunID = &plannerRun.ID
	return nil
}

func (PlanCodeReviewStrategy) HandleRunCompletion(ctx context.Context, svc *TeamService, team *model.AgentTeam, run *model.AgentRun) error {
	switch run.TeamRole {
	case model.TeamRolePlanner:
		handlePCRPlannerDone(ctx, svc, team, run)
	case model.TeamRoleCoder:
		handlePCRCoderDone(ctx, svc, team, run)
	case model.TeamRoleReviewer:
		handlePCRReviewerDone(ctx, svc, team, run)
	default:
		log.WithFields(teamRunLogFields(team, run)).Warn("plan-code-review: unknown team role")
	}
	return nil
}

func handlePCRPlannerDone(ctx context.Context, svc *TeamService, team *model.AgentTeam, run *model.AgentRun) {
	if run.Status != model.AgentRunStatusCompleted {
		errMsg := "planner failed"
		if run.ErrorMessage != "" {
			errMsg = run.ErrorMessage
		}
		log.WithFields(teamRunLogFields(team, run)).WithField("errorMessage", errMsg).Warn("team planner run failed")
		_ = svc.teamRepo.UpdateStatusWithError(ctx, team.ID, model.TeamStatusFailed, errMsg)
		team.Status = model.TeamStatusFailed
		svc.broadcastEvent(ws.EventTeamFailed, team.ProjectID.String(), team.ToDTO())
		return
	}

	// Transition to executing
	if err := svc.teamRepo.UpdateStatus(ctx, team.ID, model.TeamStatusExecuting); err != nil {
		log.WithError(err).WithField("teamId", team.ID.String()).Error("team service: failed to transition to executing")
		return
	}
	team.Status = model.TeamStatusExecuting
	log.WithFields(teamRunLogFields(team, run)).Info("team transitioned to executing")
	svc.broadcastEvent(ws.EventTeamExecuting, team.ProjectID.String(), team.ToDTO())

	task, err := svc.taskRepo.GetByID(ctx, team.TaskID)
	if err != nil {
		log.WithError(err).WithField("teamId", team.ID.String()).Error("team service: failed to get task")
		_ = svc.teamRepo.UpdateStatusWithError(ctx, team.ID, model.TeamStatusFailed, "failed to get task for coder spawning")
		return
	}

	hasChildren, err := svc.taskRepo.HasChildren(ctx, task.ID)
	if err != nil {
		log.WithError(err).WithField("teamId", team.ID.String()).Error("team service: failed to check children")
		_ = svc.teamRepo.UpdateStatusWithError(ctx, team.ID, model.TeamStatusFailed, "failed to check subtasks")
		return
	}
	log.WithFields(log.Fields{
		"teamId":      team.ID.String(),
		"projectId":   team.ProjectID.String(),
		"taskId":      task.ID.String(),
		"hasChildren": hasChildren,
	}).Info("team planner output evaluated")

	if !hasChildren {
		children, err := svc.taskRepo.CreateChildren(ctx, []model.TaskChildInput{
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
			log.WithError(err).WithField("teamId", team.ID.String()).Error("team service: failed to create default subtask")
			_ = svc.teamRepo.UpdateStatusWithError(ctx, team.ID, model.TeamStatusFailed, "failed to create subtasks")
			return
		}
		log.WithFields(log.Fields{
			"teamId":     team.ID.String(),
			"projectId":  team.ProjectID.String(),
			"taskId":     task.ID.String(),
			"childCount": len(children),
		}).Info("team created default subtasks")
		svc.spawnCodersForTasks(ctx, team, task, children)
		return
	}

	svc.spawnCodersForTask(ctx, team, task)
}

func handlePCRCoderDone(ctx context.Context, svc *TeamService, team *model.AgentTeam, run *model.AgentRun) {
	runs, err := svc.runRepo.ListByTeam(ctx, team.ID)
	if err != nil {
		log.WithError(err).WithField("teamId", team.ID.String()).Error("team service: failed to list team runs")
		return
	}

	allCodersDone := true
	anyCoderFailed := false
	for _, r := range runs {
		if r.TeamRole != model.TeamRoleCoder {
			continue
		}
		if !isTerminalAgentStatus(r.Status) {
			allCodersDone = false
			break
		}
		if r.Status == model.AgentRunStatusFailed || r.Status == model.AgentRunStatusCancelled {
			anyCoderFailed = true
		}
	}

	if !allCodersDone {
		log.WithFields(teamRunLogFields(team, run)).Debug("team waiting for remaining coder runs")
		return
	}

	if anyCoderFailed {
		log.WithFields(teamRunLogFields(team, run)).Warn("team failed because one or more coder runs failed")
		_ = svc.teamRepo.UpdateStatusWithError(ctx, team.ID, model.TeamStatusFailed, "one or more coder runs failed")
		team.Status = model.TeamStatusFailed
		svc.broadcastEvent(ws.EventTeamFailed, team.ProjectID.String(), team.ToDTO())
		return
	}

	spawnReviewer(ctx, svc, team, run)
}

func handlePCRReviewerDone(ctx context.Context, svc *TeamService, team *model.AgentTeam, run *model.AgentRun) {
	if run.Status == model.AgentRunStatusCompleted {
		if err := svc.teamRepo.UpdateStatus(ctx, team.ID, model.TeamStatusCompleted); err != nil {
			log.WithError(err).WithField("teamId", team.ID.String()).Error("team service: failed to mark team completed")
			return
		}
		team.Status = model.TeamStatusCompleted
		log.WithFields(teamRunLogFields(team, run)).Info("team completed")
		svc.broadcastEvent(ws.EventTeamCompleted, team.ProjectID.String(), team.ToDTO())

		if svc.memorySvc != nil {
			runs, err := svc.runRepo.ListByTeam(ctx, team.ID)
			if err == nil {
				_ = svc.memorySvc.RecordTeamLearnings(ctx, team.ProjectID, team, runs)
			}
		}
	} else {
		errMsg := "reviewer failed"
		if run.ErrorMessage != "" {
			errMsg = run.ErrorMessage
		}
		log.WithFields(teamRunLogFields(team, run)).WithField("errorMessage", errMsg).Warn("team reviewer run failed")
		_ = svc.teamRepo.UpdateStatusWithError(ctx, team.ID, model.TeamStatusFailed, errMsg)
		team.Status = model.TeamStatusFailed
		svc.broadcastEvent(ws.EventTeamFailed, team.ProjectID.String(), team.ToDTO())
	}
}

// spawnReviewer is a shared helper used by multiple strategies to transition
// to reviewing and spawn the reviewer agent.
func spawnReviewer(ctx context.Context, svc *TeamService, team *model.AgentTeam, run *model.AgentRun) {
	if err := svc.teamRepo.UpdateStatus(ctx, team.ID, model.TeamStatusReviewing); err != nil {
		log.WithError(err).WithField("teamId", team.ID.String()).Error("team service: failed to transition to reviewing")
		return
	}
	team.Status = model.TeamStatusReviewing
	log.WithFields(teamRunLogFields(team, run)).Info("team transitioned to reviewing")
	svc.broadcastEvent(ws.EventTeamReviewing, team.ProjectID.String(), team.ToDTO())

	task, err := svc.taskRepo.GetByID(ctx, team.TaskID)
	if err != nil {
		log.WithError(err).WithField("teamId", team.ID.String()).Error("team service: failed to get task for reviewer")
		return
	}

	memberID := uuid.Nil
	if task.AssigneeID != nil {
		memberID = *task.AssigneeID
	}

	reviewerBudget := team.TotalBudgetUsd * 0.2
	if reviewerBudget < 1 {
		reviewerBudget = 1
	}
	selection := team.CodingAgentSelection()

	reviewerRun, err := svc.spawner.SpawnForTeam(ctx, team.ID, model.TeamRoleReviewer, task.ID, memberID, selection.Runtime, selection.Provider, selection.Model, reviewerBudget, "code-reviewer")
	if err != nil {
		log.WithError(err).WithField("teamId", team.ID.String()).Error("team service: failed to spawn reviewer")
		_ = svc.teamRepo.UpdateStatusWithError(ctx, team.ID, model.TeamStatusFailed, fmt.Sprintf("failed to spawn reviewer: %v", err))
		team.Status = model.TeamStatusFailed
		svc.broadcastEvent(ws.EventTeamFailed, team.ProjectID.String(), team.ToDTO())
		return
	}
	log.WithFields(log.Fields{
		"teamId":    team.ID.String(),
		"projectId": team.ProjectID.String(),
		"taskId":    task.ID.String(),
		"runId":     reviewerRun.ID.String(),
		"role":      model.TeamRoleReviewer,
		"budgetUsd": reviewerBudget,
		"runtime":   selection.Runtime,
		"provider":  selection.Provider,
		"model":     selection.Model,
	}).Info("team spawned reviewer")

	if err := svc.runRepo.SetTeamFields(ctx, reviewerRun.ID, team.ID, model.TeamRoleReviewer); err != nil {
		log.WithError(err).WithField("teamId", team.ID.String()).Error("team service: failed to set reviewer team fields")
	}
	if err := svc.teamRepo.SetReviewerRun(ctx, team.ID, reviewerRun.ID); err != nil {
		log.WithError(err).WithField("teamId", team.ID.String()).Error("team service: failed to set team reviewer run")
	}
}
