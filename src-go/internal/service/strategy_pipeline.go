package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/ws"
	log "github.com/sirupsen/logrus"
)

// PipelineStrategy executes subtasks strictly one at a time in order.
// After planning, it spawns a coder for the first subtask. When that coder
// completes, it spawns the next one. After all subtasks are done, the
// reviewer is spawned.
type PipelineStrategy struct{}

func (PipelineStrategy) Name() string { return "pipeline" }

func (PipelineStrategy) Start(ctx context.Context, svc *TeamService, team *model.AgentTeam, task *model.Task, input StartTeamInput) error {
	// Identical planning phase - spawn planner first
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

func (PipelineStrategy) HandleRunCompletion(ctx context.Context, svc *TeamService, team *model.AgentTeam, run *model.AgentRun) error {
	switch run.TeamRole {
	case model.TeamRolePlanner:
		handlePipelinePlannerDone(ctx, svc, team, run)
	case model.TeamRoleCoder:
		handlePipelineCoderDone(ctx, svc, team, run)
	case model.TeamRoleReviewer:
		handlePCRReviewerDone(ctx, svc, team, run)
	default:
		log.WithFields(teamRunLogFields(team, run)).Warn("pipeline: unknown team role")
	}
	return nil
}

func handlePipelinePlannerDone(ctx context.Context, svc *TeamService, team *model.AgentTeam, run *model.AgentRun) {
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

	// Try to create subtasks from planner structured output first.
	if children, ok := svc.tryCreateSubtasksFromStructuredOutput(ctx, team, task, run); ok && len(children) > 0 {
		spawnSingleCoder(ctx, svc, team, task, children[0])
		return
	}

	children, err := svc.taskRepo.ListChildren(ctx, task.ID)
	if err != nil || len(children) == 0 {
		// Fallback: spawn single coder
		svc.spawnCodersForTask(ctx, team, task)
		return
	}

	// Spawn only the first child
	spawnSingleCoder(ctx, svc, team, task, children[0])
}

func handlePipelineCoderDone(ctx context.Context, svc *TeamService, team *model.AgentTeam, run *model.AgentRun) {
	if run.Status != model.AgentRunStatusCompleted {
		_ = svc.teamRepo.UpdateStatusWithError(ctx, team.ID, model.TeamStatusFailed, "coder run failed")
		team.Status = model.TeamStatusFailed
		svc.broadcastEvent(ws.EventTeamFailed, team.ProjectID.String(), team.ToDTO())
		return
	}

	task, err := svc.taskRepo.GetByID(ctx, team.TaskID)
	if err != nil {
		spawnReviewer(ctx, svc, team, run)
		return
	}

	children, err := svc.taskRepo.ListChildren(ctx, task.ID)
	if err != nil {
		spawnReviewer(ctx, svc, team, run)
		return
	}

	// Find already-spawned task IDs
	runs, err := svc.runRepo.ListByTeam(ctx, team.ID)
	if err != nil {
		spawnReviewer(ctx, svc, team, run)
		return
	}
	spawnedTaskIDs := make(map[uuid.UUID]bool)
	for _, r := range runs {
		if r.TeamRole == model.TeamRoleCoder {
			spawnedTaskIDs[r.TaskID] = true
		}
	}

	// Find the next child that hasn't been spawned yet
	for _, child := range children {
		if !spawnedTaskIDs[child.ID] {
			log.WithFields(log.Fields{
				"teamId": team.ID.String(),
				"taskId": child.ID.String(),
			}).Info("pipeline: spawning next sequential coder")
			spawnSingleCoder(ctx, svc, team, task, child)
			return
		}
	}

	// All children spawned and completed - go to review
	spawnReviewer(ctx, svc, team, run)
}

// spawnSingleCoder spawns a single coder for one subtask.
func spawnSingleCoder(ctx context.Context, svc *TeamService, team *model.AgentTeam, parentTask *model.Task, child *model.Task) {
	memberID := uuid.Nil
	if parentTask.AssigneeID != nil {
		memberID = *parentTask.AssigneeID
	}

	selection := team.CodingAgentSelection()
	coderBudget := team.TotalBudgetUsd * 0.6 / 3 // conservative per-coder budget
	if coderBudget < 1 {
		coderBudget = 1
	}

	coderRun, err := svc.spawner.SpawnForTeam(ctx, team.ID, model.TeamRoleCoder, child.ID, memberID, selection.Runtime, selection.Provider, selection.Model, coderBudget, "coding-agent")
	if err != nil {
		log.WithError(err).WithFields(log.Fields{"teamId": team.ID.String(), "taskId": child.ID.String()}).Error("pipeline: failed to spawn coder")
		_ = svc.teamRepo.UpdateStatusWithError(ctx, team.ID, model.TeamStatusFailed, fmt.Sprintf("failed to spawn coder: %v", err))
		team.Status = model.TeamStatusFailed
		svc.broadcastEvent(ws.EventTeamFailed, team.ProjectID.String(), team.ToDTO())
		return
	}
	if err := svc.runRepo.SetTeamFields(ctx, coderRun.ID, team.ID, model.TeamRoleCoder); err != nil {
		log.WithError(err).WithFields(log.Fields{"teamId": team.ID.String(), "runId": coderRun.ID.String()}).Error("pipeline: failed to set coder team fields")
	}
}
