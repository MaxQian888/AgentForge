package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/ws"
	log "github.com/sirupsen/logrus"
)

// WaveBasedStrategy implements dependency-aware wave execution.
// After planning, it inspects subtask labels for dependency metadata
// (e.g. "dep:0", "dep:2", "scope:path/to/file", "role:backend") and
// spawns agents in waves: first all tasks with no unmet dependencies,
// then when a wave completes, the next set of unblocked tasks, and so on.
// After all waves complete, a reviewer is spawned.
type WaveBasedStrategy struct{}

func (WaveBasedStrategy) Name() string { return "wave-based" }

func (WaveBasedStrategy) Start(ctx context.Context, svc *TeamService, team *model.AgentTeam, task *model.Task, input StartTeamInput) error {
	// Same as plan-code-review: start with a planner
	if err := svc.teamRepo.UpdateStatus(ctx, team.ID, model.TeamStatusPlanning); err != nil {
		return fmt.Errorf("transition team to planning: %w", err)
	}
	team.Status = model.TeamStatusPlanning
	log.WithFields(teamLogFields(team)).Info("wave-based: team transitioned to planning")
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

func (WaveBasedStrategy) HandleRunCompletion(ctx context.Context, svc *TeamService, team *model.AgentTeam, run *model.AgentRun) error {
	switch run.TeamRole {
	case model.TeamRolePlanner:
		handleWavePlannerDone(ctx, svc, team, run)
	case model.TeamRoleCoder:
		handleWaveCoderDone(ctx, svc, team, run)
	case model.TeamRoleReviewer:
		handlePCRReviewerDone(ctx, svc, team, run) // reuse reviewer logic
	default:
		log.WithFields(teamRunLogFields(team, run)).Warn("wave-based: unknown team role")
	}
	return nil
}

func handleWavePlannerDone(ctx context.Context, svc *TeamService, team *model.AgentTeam, run *model.AgentRun) {
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

	// Transition to executing
	if err := svc.teamRepo.UpdateStatus(ctx, team.ID, model.TeamStatusExecuting); err != nil {
		log.WithError(err).WithField("teamId", team.ID.String()).Error("wave-based: failed to transition to executing")
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
	if children, ok := svc.tryCreateSubtasksFromStructuredOutput(ctx, team, task, run); ok {
		unblocked := findUnblockedTasks(children, nil)
		if len(unblocked) == 0 {
			svc.spawnCodersForTasks(ctx, team, task, children)
		} else {
			svc.spawnCodersForTasks(ctx, team, task, unblocked)
		}
		return
	}

	hasChildren, err := svc.taskRepo.HasChildren(ctx, task.ID)
	if err != nil {
		_ = svc.teamRepo.UpdateStatusWithError(ctx, team.ID, model.TeamStatusFailed, "failed to check subtasks")
		return
	}

	if !hasChildren {
		// No subtasks from planner - create a single child and spawn directly
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
			_ = svc.teamRepo.UpdateStatusWithError(ctx, team.ID, model.TeamStatusFailed, "failed to create subtasks")
			return
		}
		svc.spawnCodersForTasks(ctx, team, task, children)
		return
	}

	// Planner created subtasks - load them via ListChildren and spawn wave 1
	children, err := svc.taskRepo.ListChildren(ctx, task.ID)
	if err != nil {
		log.WithError(err).WithField("teamId", team.ID.String()).Error("wave-based: failed to list children")
		_ = svc.teamRepo.UpdateStatusWithError(ctx, team.ID, model.TeamStatusFailed, "failed to list subtasks")
		return
	}
	if len(children) == 0 {
		// Fallback: spawn single coder for the task itself
		svc.spawnCodersForTask(ctx, team, task)
		return
	}

	unblocked := findUnblockedTasks(children, nil)
	if len(unblocked) == 0 {
		// All tasks have deps that can never be satisfied - spawn all
		log.WithField("teamId", team.ID.String()).Warn("wave-based: no unblocked tasks, spawning all")
		svc.spawnCodersForTasks(ctx, team, task, children)
		return
	}

	log.WithFields(log.Fields{
		"teamId":        team.ID.String(),
		"wave":          1,
		"unblockedCount": len(unblocked),
		"totalChildren": len(children),
	}).Info("wave-based: spawning wave 1")
	svc.spawnCodersForTasks(ctx, team, task, unblocked)
}

func handleWaveCoderDone(ctx context.Context, svc *TeamService, team *model.AgentTeam, run *model.AgentRun) {
	runs, err := svc.runRepo.ListByTeam(ctx, team.ID)
	if err != nil {
		log.WithError(err).WithField("teamId", team.ID.String()).Error("wave-based: failed to list team runs")
		return
	}

	// Check if all current coders are done
	allCodersDone := true
	anyCoderFailed := false
	completedTaskIDs := make(map[uuid.UUID]bool)

	for _, r := range runs {
		if r.TeamRole != model.TeamRoleCoder {
			continue
		}
		if !isTerminalAgentStatus(r.Status) {
			allCodersDone = false
		}
		if r.Status == model.AgentRunStatusFailed || r.Status == model.AgentRunStatusCancelled {
			anyCoderFailed = true
		}
		if r.Status == model.AgentRunStatusCompleted {
			completedTaskIDs[r.TaskID] = true
		}
	}

	if !allCodersDone {
		log.WithFields(teamRunLogFields(team, run)).Debug("wave-based: waiting for current wave to finish")
		return
	}

	if anyCoderFailed {
		_ = svc.teamRepo.UpdateStatusWithError(ctx, team.ID, model.TeamStatusFailed, "one or more coder runs failed")
		team.Status = model.TeamStatusFailed
		svc.broadcastEvent(ws.EventTeamFailed, team.ProjectID.String(), team.ToDTO())
		return
	}

	// Current wave done - check if there are more unblocked tasks
	task, err := svc.taskRepo.GetByID(ctx, team.TaskID)
	if err != nil {
		log.WithError(err).WithField("teamId", team.ID.String()).Error("wave-based: failed to get task")
		return
	}

	children, err := svc.taskRepo.ListChildren(ctx, task.ID)
	if err != nil {
		log.WithError(err).WithField("teamId", team.ID.String()).Error("wave-based: failed to list children")
		spawnReviewer(ctx, svc, team, run)
		return
	}

	// Find tasks that are now unblocked but haven't been spawned yet
	unblocked := findUnblockedTasks(children, completedTaskIDs)

	// Filter out tasks that already have a coder run
	spawnedTaskIDs := make(map[uuid.UUID]bool)
	for _, r := range runs {
		if r.TeamRole == model.TeamRoleCoder {
			spawnedTaskIDs[r.TaskID] = true
		}
	}
	var nextWave []*model.Task
	for _, child := range unblocked {
		if !spawnedTaskIDs[child.ID] {
			nextWave = append(nextWave, child)
		}
	}

	if len(nextWave) == 0 {
		// No more tasks to spawn - go to review
		log.WithFields(log.Fields{
			"teamId":           team.ID.String(),
			"completedCoders":  len(completedTaskIDs),
		}).Info("wave-based: all waves complete, spawning reviewer")
		spawnReviewer(ctx, svc, team, run)
		return
	}

	log.WithFields(log.Fields{
		"teamId":    team.ID.String(),
		"waveSize":  len(nextWave),
	}).Info("wave-based: spawning next wave")
	svc.spawnCodersForTasks(ctx, team, task, nextWave)
}

// subtaskMeta holds parsed metadata from task labels.
type subtaskMeta struct {
	deps  []int  // indices of tasks this one depends on (from "dep:N" labels)
	scope string // file/path scope (from "scope:PATH" labels)
	role  string // role hint (from "role:HINT" labels)
}

// parseSubtaskMeta extracts dependency, scope, and role metadata from task labels.
func parseSubtaskMeta(labels []string) subtaskMeta {
	var meta subtaskMeta
	for _, label := range labels {
		if strings.HasPrefix(label, "dep:") {
			if idx, err := strconv.Atoi(strings.TrimPrefix(label, "dep:")); err == nil {
				meta.deps = append(meta.deps, idx)
			}
		} else if strings.HasPrefix(label, "scope:") {
			meta.scope = strings.TrimPrefix(label, "scope:")
		} else if strings.HasPrefix(label, "role:") {
			meta.role = strings.TrimPrefix(label, "role:")
		}
	}
	return meta
}

// findUnblockedTasks returns the subset of children whose dependencies
// (expressed as "dep:INDEX" labels referencing sibling indices) are all
// satisfied by completedTaskIDs. Tasks with no deps are always unblocked.
func findUnblockedTasks(children []*model.Task, completedTaskIDs map[uuid.UUID]bool) []*model.Task {
	// Build index -> task ID mapping
	taskIDByIndex := make(map[int]uuid.UUID, len(children))
	for i, child := range children {
		taskIDByIndex[i] = child.ID
	}

	var unblocked []*model.Task
	for _, child := range children {
		// Skip tasks already completed
		if completedTaskIDs != nil && completedTaskIDs[child.ID] {
			continue
		}

		meta := parseSubtaskMeta(child.Labels)
		if len(meta.deps) == 0 {
			unblocked = append(unblocked, child)
			continue
		}

		allDepsSatisfied := true
		for _, depIdx := range meta.deps {
			depTaskID, exists := taskIDByIndex[depIdx]
			if !exists {
				continue // unknown dep index, treat as satisfied
			}
			if completedTaskIDs == nil || !completedTaskIDs[depTaskID] {
				allDepsSatisfied = false
				break
			}
		}
		if allDepsSatisfied {
			unblocked = append(unblocked, child)
		}
	}
	return unblocked
}
