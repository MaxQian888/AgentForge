package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/ws"
	log "github.com/sirupsen/logrus"
)

var (
	ErrTeamNotFound      = errors.New("team not found")
	ErrTeamAlreadyActive = errors.New("team already active for this task")
	ErrTeamNotActive     = errors.New("team is not active")
	ErrTeamTaskNotFound  = errors.New("team task not found")
)

// TeamRunRepository defines persistence for agent teams.
type TeamRunRepository interface {
	Create(ctx context.Context, team *model.AgentTeam) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.AgentTeam, error)
	GetByTask(ctx context.Context, taskID uuid.UUID) (*model.AgentTeam, error)
	ListByProject(ctx context.Context, projectID uuid.UUID) ([]*model.AgentTeam, error)
	ListActive(ctx context.Context) ([]*model.AgentTeam, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status string) error
	UpdateStatusWithError(ctx context.Context, id uuid.UUID, status, errorMessage string) error
	UpdateSpent(ctx context.Context, id uuid.UUID, spent float64) error
	SetPlannerRun(ctx context.Context, id uuid.UUID, plannerRunID uuid.UUID) error
	SetReviewerRun(ctx context.Context, id uuid.UUID, reviewerRunID uuid.UUID) error
}

// TeamAgentRunRepository defines run persistence needed by the team service.
type TeamAgentRunRepository interface {
	ListByTeam(ctx context.Context, teamID uuid.UUID) ([]*model.AgentRun, error)
	SetTeamFields(ctx context.Context, id uuid.UUID, teamID uuid.UUID, teamRole string) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.AgentRun, error)
}

// TeamAgentSpawner spawns and cancels agent runs.
type TeamAgentSpawner interface {
	Spawn(ctx context.Context, taskID, memberID uuid.UUID, runtime, provider, modelName string, budgetUsd float64, roleID string) (*model.AgentRun, error)
	Cancel(ctx context.Context, id uuid.UUID, reason string) error
}

// TeamTaskRepository defines task persistence needed by the team service.
type TeamTaskRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Task, error)
	HasChildren(ctx context.Context, parentID uuid.UUID) (bool, error)
	CreateChildren(ctx context.Context, inputs []model.TaskChildInput) ([]*model.Task, error)
}

// TeamProjectRepository defines project persistence needed by the team service.
type TeamProjectRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Project, error)
}

// StartTeamInput is the input for starting a new team run.
type StartTeamInput struct {
	TaskID         uuid.UUID `json:"taskId"`
	MemberID       uuid.UUID `json:"memberId"`
	Name           string    `json:"name"`
	Strategy       string    `json:"strategy"`
	Runtime        string    `json:"runtime"`
	Provider       string    `json:"provider"`
	Model          string    `json:"model"`
	TotalBudgetUsd float64   `json:"totalBudgetUsd"`
}

type TeamService struct {
	teamRepo    TeamRunRepository
	runRepo     TeamAgentRunRepository
	spawner     TeamAgentSpawner
	taskRepo    TeamTaskRepository
	projectRepo TeamProjectRepository
	memorySvc   *MemoryService
	hub         *ws.Hub
}

func NewTeamService(
	teamRepo TeamRunRepository,
	runRepo TeamAgentRunRepository,
	spawner TeamAgentSpawner,
	taskRepo TeamTaskRepository,
	projectRepo TeamProjectRepository,
	memorySvc *MemoryService,
	hub *ws.Hub,
) *TeamService {
	return &TeamService{
		teamRepo:    teamRepo,
		runRepo:     runRepo,
		spawner:     spawner,
		taskRepo:    taskRepo,
		projectRepo: projectRepo,
		memorySvc:   memorySvc,
		hub:         hub,
	}
}

// StartTeam creates a new team and spawns the planner agent.
func (s *TeamService) StartTeam(ctx context.Context, input StartTeamInput) (*model.AgentTeam, error) {
	task, err := s.taskRepo.GetByID(ctx, input.TaskID)
	if err != nil {
		return nil, ErrTeamTaskNotFound
	}
	project, err := s.projectRepo.GetByID(ctx, task.ProjectID)
	if err != nil {
		return nil, ErrAgentProjectNotFound
	}
	selection, err := ResolveProjectCodingAgentSelection(project, input.Runtime, input.Provider, input.Model)
	if err != nil {
		return nil, err
	}

	team := &model.AgentTeam{
		ID:             uuid.New(),
		ProjectID:      task.ProjectID,
		TaskID:         task.ID,
		Name:           input.Name,
		Status:         model.TeamStatusPending,
		Strategy:       input.Strategy,
		TotalBudgetUsd: input.TotalBudgetUsd,
		Config:         MarshalCodingAgentSelection(selection),
	}
	if strings.TrimSpace(team.Name) == "" {
		team.Name = "Team for " + task.Title
	}
	if strings.TrimSpace(team.Strategy) == "" {
		team.Strategy = "plan-code-review"
	}

	if err := s.teamRepo.Create(ctx, team); err != nil {
		return nil, fmt.Errorf("create team: %w", err)
	}
	log.WithFields(log.Fields{
		"teamId":    team.ID.String(),
		"projectId": task.ProjectID.String(),
		"taskId":    task.ID.String(),
		"strategy":  team.Strategy,
		"teamName":  team.Name,
		"runtime":   selection.Runtime,
		"provider":  selection.Provider,
		"model":     selection.Model,
		"budgetUsd": team.TotalBudgetUsd,
	}).Info("team created")

	s.broadcastEvent(ws.EventTeamCreated, task.ProjectID.String(), team.ToDTO())

	// Transition to planning and spawn planner
	if err := s.teamRepo.UpdateStatus(ctx, team.ID, model.TeamStatusPlanning); err != nil {
		return nil, fmt.Errorf("transition team to planning: %w", err)
	}
	team.Status = model.TeamStatusPlanning
	log.WithFields(teamLogFields(team)).Info("team transitioned to planning")
	s.broadcastEvent(ws.EventTeamPlanning, task.ProjectID.String(), team.ToDTO())

	plannerBudget := team.TotalBudgetUsd * 0.2
	if plannerBudget < 1 {
		plannerBudget = 1
	}

	plannerRun, err := s.spawner.Spawn(ctx, task.ID, input.MemberID, selection.Runtime, selection.Provider, selection.Model, plannerBudget, "planner-agent")
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
		_ = s.teamRepo.UpdateStatusWithError(ctx, team.ID, model.TeamStatusFailed, fmt.Sprintf("failed to spawn planner: %v", err))
		team.Status = model.TeamStatusFailed
		s.broadcastEvent(ws.EventTeamFailed, task.ProjectID.String(), team.ToDTO())
		return nil, fmt.Errorf("spawn planner: %w", err)
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

	if err := s.runRepo.SetTeamFields(ctx, plannerRun.ID, team.ID, model.TeamRolePlanner); err != nil {
		return nil, fmt.Errorf("set planner team fields: %w", err)
	}
	if err := s.teamRepo.SetPlannerRun(ctx, team.ID, plannerRun.ID); err != nil {
		return nil, fmt.Errorf("set team planner run: %w", err)
	}
	team.PlannerRunID = &plannerRun.ID

	return team, nil
}

// ProcessRunCompletion is called when an agent run in a team completes.
// It routes to the appropriate handler based on team_role.
func (s *TeamService) ProcessRunCompletion(ctx context.Context, run *model.AgentRun) {
	if run == nil || run.TeamID == nil {
		return
	}

	team, err := s.teamRepo.GetByID(ctx, *run.TeamID)
	if err != nil {
		log.WithError(err).WithField("teamId", run.TeamID.String()).Error("team service: failed to get team for run completion")
		return
	}

	if model.IsTerminalTeamStatus(team.Status) {
		return
	}
	log.WithFields(teamRunLogFields(team, run)).Info("team run completion received")

	// Update team cost
	s.updateTeamCost(ctx, team)

	switch run.TeamRole {
	case model.TeamRolePlanner:
		s.handlePlannerDone(ctx, team, run)
	case model.TeamRoleCoder:
		s.handleCoderDone(ctx, team, run)
	case model.TeamRoleReviewer:
		s.handleReviewerDone(ctx, team, run)
	default:
		log.WithFields(teamRunLogFields(team, run)).Warn("team service: unknown team role")
	}
}

func (s *TeamService) handlePlannerDone(ctx context.Context, team *model.AgentTeam, run *model.AgentRun) {
	if run.Status != model.AgentRunStatusCompleted {
		errMsg := "planner failed"
		if run.ErrorMessage != "" {
			errMsg = run.ErrorMessage
		}
		log.WithFields(teamRunLogFields(team, run)).WithField("errorMessage", errMsg).Warn("team planner run failed")
		_ = s.teamRepo.UpdateStatusWithError(ctx, team.ID, model.TeamStatusFailed, errMsg)
		team.Status = model.TeamStatusFailed
		s.broadcastEvent(ws.EventTeamFailed, team.ProjectID.String(), team.ToDTO())
		return
	}

	// Transition to executing
	if err := s.teamRepo.UpdateStatus(ctx, team.ID, model.TeamStatusExecuting); err != nil {
		log.WithError(err).WithField("teamId", team.ID.String()).Error("team service: failed to transition to executing")
		return
	}
	team.Status = model.TeamStatusExecuting
	log.WithFields(teamRunLogFields(team, run)).Info("team transitioned to executing")
	s.broadcastEvent(ws.EventTeamExecuting, team.ProjectID.String(), team.ToDTO())

	// Get the parent task to find child tasks (planner should have created subtasks via decomposition)
	task, err := s.taskRepo.GetByID(ctx, team.TaskID)
	if err != nil {
		log.WithError(err).WithField("teamId", team.ID.String()).Error("team service: failed to get task")
		_ = s.teamRepo.UpdateStatusWithError(ctx, team.ID, model.TeamStatusFailed, "failed to get task for coder spawning")
		return
	}

	hasChildren, err := s.taskRepo.HasChildren(ctx, task.ID)
	if err != nil {
		log.WithError(err).WithField("teamId", team.ID.String()).Error("team service: failed to check children")
		_ = s.teamRepo.UpdateStatusWithError(ctx, team.ID, model.TeamStatusFailed, "failed to check subtasks")
		return
	}
	log.WithFields(log.Fields{
		"teamId":      team.ID.String(),
		"projectId":   team.ProjectID.String(),
		"taskId":      task.ID.String(),
		"hasChildren": hasChildren,
	}).Info("team planner output evaluated")

	if !hasChildren {
		// If planner didn't create subtasks, create a single child task for the work
		children, err := s.taskRepo.CreateChildren(ctx, []model.TaskChildInput{
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
			_ = s.teamRepo.UpdateStatusWithError(ctx, team.ID, model.TeamStatusFailed, "failed to create subtasks")
			return
		}
		log.WithFields(log.Fields{
			"teamId":     team.ID.String(),
			"projectId":  team.ProjectID.String(),
			"taskId":     task.ID.String(),
			"childCount": len(children),
		}).Info("team created default subtasks")
		s.spawnCodersForTasks(ctx, team, task, children)
		return
	}

	// Planner already created subtasks - we need to find them and spawn coders
	// The subtasks were created as children of the team's main task
	// We don't have a direct ListChildren method, so we spawn coders for the task itself
	// In practice, the planner output creates subtasks via the decomposition bridge
	s.spawnCodersForTask(ctx, team, task)
}

func (s *TeamService) spawnCodersForTasks(ctx context.Context, team *model.AgentTeam, parentTask *model.Task, children []*model.Task) {
	memberID := uuid.Nil
	if parentTask.AssigneeID != nil {
		memberID = *parentTask.AssigneeID
	}

	coderBudget := team.TotalBudgetUsd * 0.6 / float64(len(children))
	if coderBudget < 1 {
		coderBudget = 1
	}

	selection := team.CodingAgentSelection()
	log.WithFields(log.Fields{
		"teamId":       team.ID.String(),
		"projectId":    team.ProjectID.String(),
		"parentTaskId": parentTask.ID.String(),
		"childCount":   len(children),
		"budgetUsd":    coderBudget,
		"runtime":      selection.Runtime,
		"provider":     selection.Provider,
		"model":        selection.Model,
	}).Info("team spawning coders for subtasks")
	for _, child := range children {
		coderRun, err := s.spawner.Spawn(ctx, child.ID, memberID, selection.Runtime, selection.Provider, selection.Model, coderBudget, "coding-agent")
		if err != nil {
			log.WithError(err).WithFields(log.Fields{"teamId": team.ID.String(), "taskId": child.ID.String()}).Error("team service: failed to spawn coder")
			continue
		}
		log.WithFields(log.Fields{
			"teamId":    team.ID.String(),
			"projectId": team.ProjectID.String(),
			"taskId":    child.ID.String(),
			"runId":     coderRun.ID.String(),
			"role":      model.TeamRoleCoder,
			"budgetUsd": coderBudget,
			"runtime":   selection.Runtime,
			"provider":  selection.Provider,
			"model":     selection.Model,
		}).Info("team spawned coder")
		if err := s.runRepo.SetTeamFields(ctx, coderRun.ID, team.ID, model.TeamRoleCoder); err != nil {
			log.WithError(err).WithFields(log.Fields{"teamId": team.ID.String(), "runId": coderRun.ID.String()}).Error("team service: failed to set coder team fields")
		}
	}
}

func (s *TeamService) spawnCodersForTask(ctx context.Context, team *model.AgentTeam, task *model.Task) {
	memberID := uuid.Nil
	if task.AssigneeID != nil {
		memberID = *task.AssigneeID
	}

	selection := team.CodingAgentSelection()
	coderBudget := team.TotalBudgetUsd * 0.6
	if coderBudget < 1 {
		coderBudget = 1
	}

	coderRun, err := s.spawner.Spawn(ctx, task.ID, memberID, selection.Runtime, selection.Provider, selection.Model, coderBudget, "coding-agent")
	if err != nil {
		log.WithError(err).WithFields(log.Fields{"teamId": team.ID.String(), "taskId": task.ID.String()}).Error("team service: failed to spawn coder for task")
		_ = s.teamRepo.UpdateStatusWithError(ctx, team.ID, model.TeamStatusFailed, fmt.Sprintf("failed to spawn coder: %v", err))
		team.Status = model.TeamStatusFailed
		s.broadcastEvent(ws.EventTeamFailed, team.ProjectID.String(), team.ToDTO())
		return
	}
	log.WithFields(log.Fields{
		"teamId":    team.ID.String(),
		"projectId": team.ProjectID.String(),
		"taskId":    task.ID.String(),
		"runId":     coderRun.ID.String(),
		"role":      model.TeamRoleCoder,
		"budgetUsd": coderBudget,
		"runtime":   selection.Runtime,
		"provider":  selection.Provider,
		"model":     selection.Model,
	}).Info("team spawned coder for task")
	if err := s.runRepo.SetTeamFields(ctx, coderRun.ID, team.ID, model.TeamRoleCoder); err != nil {
		log.WithError(err).WithFields(log.Fields{"teamId": team.ID.String(), "runId": coderRun.ID.String()}).Error("team service: failed to set coder team fields")
	}
}

func (s *TeamService) handleCoderDone(ctx context.Context, team *model.AgentTeam, run *model.AgentRun) {
	// Check if all coder runs for this team are done
	runs, err := s.runRepo.ListByTeam(ctx, team.ID)
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
		_ = s.teamRepo.UpdateStatusWithError(ctx, team.ID, model.TeamStatusFailed, "one or more coder runs failed")
		team.Status = model.TeamStatusFailed
		s.broadcastEvent(ws.EventTeamFailed, team.ProjectID.String(), team.ToDTO())
		return
	}

	// All coders done successfully - spawn reviewer
	if err := s.teamRepo.UpdateStatus(ctx, team.ID, model.TeamStatusReviewing); err != nil {
		log.WithError(err).WithField("teamId", team.ID.String()).Error("team service: failed to transition to reviewing")
		return
	}
	team.Status = model.TeamStatusReviewing
	log.WithFields(teamRunLogFields(team, run)).Info("team transitioned to reviewing")
	s.broadcastEvent(ws.EventTeamReviewing, team.ProjectID.String(), team.ToDTO())

	task, err := s.taskRepo.GetByID(ctx, team.TaskID)
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

	reviewerRun, err := s.spawner.Spawn(ctx, task.ID, memberID, selection.Runtime, selection.Provider, selection.Model, reviewerBudget, "code-reviewer")
	if err != nil {
		log.WithError(err).WithField("teamId", team.ID.String()).Error("team service: failed to spawn reviewer")
		_ = s.teamRepo.UpdateStatusWithError(ctx, team.ID, model.TeamStatusFailed, fmt.Sprintf("failed to spawn reviewer: %v", err))
		team.Status = model.TeamStatusFailed
		s.broadcastEvent(ws.EventTeamFailed, team.ProjectID.String(), team.ToDTO())
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

	if err := s.runRepo.SetTeamFields(ctx, reviewerRun.ID, team.ID, model.TeamRoleReviewer); err != nil {
		log.WithError(err).WithField("teamId", team.ID.String()).Error("team service: failed to set reviewer team fields")
	}
	if err := s.teamRepo.SetReviewerRun(ctx, team.ID, reviewerRun.ID); err != nil {
		log.WithError(err).WithField("teamId", team.ID.String()).Error("team service: failed to set team reviewer run")
	}
}

func (s *TeamService) handleReviewerDone(ctx context.Context, team *model.AgentTeam, run *model.AgentRun) {
	if run.Status == model.AgentRunStatusCompleted {
		if err := s.teamRepo.UpdateStatus(ctx, team.ID, model.TeamStatusCompleted); err != nil {
			log.WithError(err).WithField("teamId", team.ID.String()).Error("team service: failed to mark team completed")
			return
		}
		team.Status = model.TeamStatusCompleted
		log.WithFields(teamRunLogFields(team, run)).Info("team completed")
		s.broadcastEvent(ws.EventTeamCompleted, team.ProjectID.String(), team.ToDTO())

		// Record learnings if memory service is available
		if s.memorySvc != nil {
			runs, err := s.runRepo.ListByTeam(ctx, team.ID)
			if err == nil {
				_ = s.memorySvc.RecordTeamLearnings(ctx, team.ProjectID, team, runs)
			}
		}
	} else {
		errMsg := "reviewer failed"
		if run.ErrorMessage != "" {
			errMsg = run.ErrorMessage
		}
		log.WithFields(teamRunLogFields(team, run)).WithField("errorMessage", errMsg).Warn("team reviewer run failed")
		_ = s.teamRepo.UpdateStatusWithError(ctx, team.ID, model.TeamStatusFailed, errMsg)
		team.Status = model.TeamStatusFailed
		s.broadcastEvent(ws.EventTeamFailed, team.ProjectID.String(), team.ToDTO())
	}
}

func (s *TeamService) updateTeamCost(ctx context.Context, team *model.AgentTeam) {
	runs, err := s.runRepo.ListByTeam(ctx, team.ID)
	if err != nil {
		return
	}
	var totalSpent float64
	for _, r := range runs {
		totalSpent += r.CostUsd
	}
	if err := s.teamRepo.UpdateSpent(ctx, team.ID, totalSpent); err != nil {
		log.WithError(err).WithField("teamId", team.ID.String()).Error("team service: failed to update team cost")
	}
	log.WithFields(log.Fields{
		"teamId":    team.ID.String(),
		"projectId": team.ProjectID.String(),
		"taskId":    team.TaskID.String(),
		"spentUsd":  totalSpent,
		"budgetUsd": team.TotalBudgetUsd,
	}).Info("team cost updated")
	s.broadcastEvent(ws.EventTeamCostUpdate, team.ProjectID.String(), map[string]any{
		"teamId": team.ID.String(),
		"spent":  totalSpent,
		"budget": team.TotalBudgetUsd,
	})
}

// CancelTeam cancels all active runs in a team.
func (s *TeamService) CancelTeam(ctx context.Context, teamID uuid.UUID) error {
	team, err := s.teamRepo.GetByID(ctx, teamID)
	if err != nil {
		return ErrTeamNotFound
	}
	if model.IsTerminalTeamStatus(team.Status) {
		return ErrTeamNotActive
	}

	runs, err := s.runRepo.ListByTeam(ctx, teamID)
	if err != nil {
		return fmt.Errorf("list team runs: %w", err)
	}

	for _, run := range runs {
		if isTerminalAgentStatus(run.Status) {
			continue
		}
		if err := s.spawner.Cancel(ctx, run.ID, "team_cancelled"); err != nil {
			log.WithError(err).WithFields(log.Fields{"teamId": teamID.String(), "runId": run.ID.String()}).Error("team service: failed to cancel run")
		}
	}

	if err := s.teamRepo.UpdateStatus(ctx, teamID, model.TeamStatusCancelled); err != nil {
		return fmt.Errorf("cancel team: %w", err)
	}
	team.Status = model.TeamStatusCancelled
	log.WithFields(teamLogFields(team)).Info("team cancelled")
	s.broadcastEvent(ws.EventTeamCancelled, team.ProjectID.String(), team.ToDTO())
	return nil
}

// RetryTeam restarts a failed team from its current phase.
func (s *TeamService) RetryTeam(ctx context.Context, teamID uuid.UUID) error {
	team, err := s.teamRepo.GetByID(ctx, teamID)
	if err != nil {
		return ErrTeamNotFound
	}
	if team.Status != model.TeamStatusFailed {
		return fmt.Errorf("can only retry failed teams, current status: %s", team.Status)
	}

	task, err := s.taskRepo.GetByID(ctx, team.TaskID)
	if err != nil {
		return ErrTeamTaskNotFound
	}

	memberID := uuid.Nil
	if task.AssigneeID != nil {
		memberID = *task.AssigneeID
	}

	// Determine which phase to retry based on what has completed
	runs, err := s.runRepo.ListByTeam(ctx, teamID)
	if err != nil {
		return fmt.Errorf("list team runs: %w", err)
	}

	hasCompletedPlanner := false
	hasCompletedAllCoders := true
	hasAnyCoders := false
	for _, r := range runs {
		switch r.TeamRole {
		case model.TeamRolePlanner:
			if r.Status == model.AgentRunStatusCompleted {
				hasCompletedPlanner = true
			}
		case model.TeamRoleCoder:
			hasAnyCoders = true
			if !isTerminalAgentStatus(r.Status) || r.Status != model.AgentRunStatusCompleted {
				hasCompletedAllCoders = false
			}
		}
	}

	if !hasCompletedPlanner {
		// Retry from planning
		if err := s.teamRepo.UpdateStatus(ctx, teamID, model.TeamStatusPlanning); err != nil {
			return fmt.Errorf("retry team planning: %w", err)
		}
		plannerBudget := team.TotalBudgetUsd * 0.2
		if plannerBudget < 1 {
			plannerBudget = 1
		}
		selection := team.CodingAgentSelection()
		plannerRun, err := s.spawner.Spawn(ctx, task.ID, memberID, selection.Runtime, selection.Provider, selection.Model, plannerBudget, "planner-agent")
		if err != nil {
			return fmt.Errorf("retry spawn planner: %w", err)
		}
		_ = s.runRepo.SetTeamFields(ctx, plannerRun.ID, teamID, model.TeamRolePlanner)
		_ = s.teamRepo.SetPlannerRun(ctx, teamID, plannerRun.ID)
		log.WithFields(log.Fields{
			"teamId":    team.ID.String(),
			"projectId": team.ProjectID.String(),
			"taskId":    task.ID.String(),
			"runId":     plannerRun.ID.String(),
			"role":      model.TeamRolePlanner,
			"budgetUsd": plannerBudget,
			"runtime":   selection.Runtime,
			"provider":  selection.Provider,
			"model":     selection.Model,
		}).Info("team retry spawned planner")
		s.broadcastEvent(ws.EventTeamPlanning, team.ProjectID.String(), team.ToDTO())
		return nil
	}

	if hasAnyCoders && !hasCompletedAllCoders {
		// Retry from executing - re-spawn failed coders
		if err := s.teamRepo.UpdateStatus(ctx, teamID, model.TeamStatusExecuting); err != nil {
			return fmt.Errorf("retry team executing: %w", err)
		}
		log.WithFields(teamLogFields(team)).Info("team retry resumed executing phase")
		s.broadcastEvent(ws.EventTeamExecuting, team.ProjectID.String(), team.ToDTO())
		return nil
	}

	// Retry from reviewing
	if err := s.teamRepo.UpdateStatus(ctx, teamID, model.TeamStatusReviewing); err != nil {
		return fmt.Errorf("retry team reviewing: %w", err)
	}
	reviewerBudget := team.TotalBudgetUsd * 0.2
	if reviewerBudget < 1 {
		reviewerBudget = 1
	}
	selection := team.CodingAgentSelection()
	reviewerRun, err := s.spawner.Spawn(ctx, task.ID, memberID, selection.Runtime, selection.Provider, selection.Model, reviewerBudget, "code-reviewer")
	if err != nil {
		return fmt.Errorf("retry spawn reviewer: %w", err)
	}
	_ = s.runRepo.SetTeamFields(ctx, reviewerRun.ID, teamID, model.TeamRoleReviewer)
	_ = s.teamRepo.SetReviewerRun(ctx, teamID, reviewerRun.ID)
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
	}).Info("team retry spawned reviewer")
	s.broadcastEvent(ws.EventTeamReviewing, team.ProjectID.String(), team.ToDTO())
	return nil
}

// GetSummary builds a full team summary DTO.
func (s *TeamService) GetSummary(ctx context.Context, teamID uuid.UUID) (*model.AgentTeamSummaryDTO, error) {
	team, err := s.teamRepo.GetByID(ctx, teamID)
	if err != nil {
		return nil, ErrTeamNotFound
	}

	task, err := s.taskRepo.GetByID(ctx, team.TaskID)
	if err != nil {
		return nil, ErrTeamTaskNotFound
	}

	runs, err := s.runRepo.ListByTeam(ctx, teamID)
	if err != nil {
		return nil, fmt.Errorf("list team runs: %w", err)
	}

	summary := &model.AgentTeamSummaryDTO{
		AgentTeamDTO: team.ToDTO(),
		TaskTitle:    task.Title,
		CoderRuns:    make([]model.AgentRunDTO, 0),
	}

	for _, r := range runs {
		switch r.TeamRole {
		case model.TeamRolePlanner:
			summary.PlannerStatus = r.Status
		case model.TeamRoleCoder:
			summary.CoderRuns = append(summary.CoderRuns, r.ToDTO())
			summary.CoderTotal++
			if r.Status == model.AgentRunStatusCompleted {
				summary.CoderCompleted++
			}
		case model.TeamRoleReviewer:
			summary.ReviewerStatus = r.Status
		}
	}

	return summary, nil
}

// ListByProject returns all teams for a project.
func (s *TeamService) ListByProject(ctx context.Context, projectID uuid.UUID) ([]*model.AgentTeam, error) {
	return s.teamRepo.ListByProject(ctx, projectID)
}

func (s *TeamService) broadcastEvent(eventType, projectID string, payload any) {
	if s.hub == nil {
		return
	}
	s.hub.BroadcastEvent(&ws.Event{
		Type:      eventType,
		ProjectID: projectID,
		Payload:   payload,
	})
}

func teamLogFields(team *model.AgentTeam) log.Fields {
	if team == nil {
		return log.Fields{}
	}

	return log.Fields{
		"teamId":     team.ID.String(),
		"projectId":  team.ProjectID.String(),
		"taskId":     team.TaskID.String(),
		"teamStatus": team.Status,
		"strategy":   team.Strategy,
	}
}

func teamRunLogFields(team *model.AgentTeam, run *model.AgentRun) log.Fields {
	fields := teamLogFields(team)
	if run == nil {
		return fields
	}

	fields["runId"] = run.ID.String()
	fields["role"] = run.TeamRole
	fields["runStatus"] = run.Status
	if run.TaskID != uuid.Nil {
		fields["runTaskId"] = run.TaskID.String()
	}
	return fields
}
