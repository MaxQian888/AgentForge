package service

import (
	"context"
	"encoding/json"
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
	GetTeamSummary(ctx context.Context, id uuid.UUID) (*model.AgentTeamSummaryDTO, error)
	ListByProject(ctx context.Context, projectID uuid.UUID, status string) ([]*model.AgentTeam, error)
	ListTeamSummaries(ctx context.Context, projectID uuid.UUID, status string) ([]*model.AgentTeamSummaryDTO, error)
	ListActive(ctx context.Context) ([]*model.AgentTeam, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status string) error
	UpdateStatusWithError(ctx context.Context, id uuid.UUID, status, errorMessage string) error
	UpdateSpent(ctx context.Context, id uuid.UUID, spent float64) error
	SetPlannerRun(ctx context.Context, id uuid.UUID, plannerRunID uuid.UUID) error
	SetReviewerRun(ctx context.Context, id uuid.UUID, reviewerRunID uuid.UUID) error
	Delete(ctx context.Context, id uuid.UUID) error
	Update(ctx context.Context, id uuid.UUID, req *model.UpdateTeamRequest) error
	SetWorkflowExecutionID(ctx context.Context, id uuid.UUID, execID uuid.UUID) error
}

// TeamAgentRunRepository defines run persistence needed by the team service.
type TeamAgentRunRepository interface {
	ListByTeam(ctx context.Context, teamID uuid.UUID) ([]*model.AgentRun, error)
	SetTeamFields(ctx context.Context, id uuid.UUID, teamID uuid.UUID, teamRole string) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.AgentRun, error)
	UpdateStructuredOutput(ctx context.Context, id uuid.UUID, output json.RawMessage) error
}

// TeamAgentSpawner spawns and cancels agent runs.
type TeamAgentSpawner interface {
	Spawn(ctx context.Context, taskID, memberID uuid.UUID, runtime, provider, modelName string, budgetUsd float64, roleID string) (*model.AgentRun, error)
	SpawnForTeam(ctx context.Context, teamID uuid.UUID, teamRole string, taskID, memberID uuid.UUID, runtime, provider, modelName string, budgetUsd float64, roleID string) (*model.AgentRun, error)
	Cancel(ctx context.Context, id uuid.UUID, reason string) error
}

// TeamTaskRepository defines task persistence needed by the team service.
type TeamTaskRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Task, error)
	HasChildren(ctx context.Context, parentID uuid.UUID) (bool, error)
	CreateChildren(ctx context.Context, inputs []model.TaskChildInput) ([]*model.Task, error)
	ListChildren(ctx context.Context, parentID uuid.UUID) ([]*model.Task, error)
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
	artifactSvc *TeamArtifactService
	templateSvc *WorkflowTemplateService
}

func NewTeamService(
	teamRepo TeamRunRepository,
	runRepo TeamAgentRunRepository,
	spawner TeamAgentSpawner,
	taskRepo TeamTaskRepository,
	projectRepo TeamProjectRepository,
	memorySvc *MemoryService,
	hub *ws.Hub,
	templateSvc *WorkflowTemplateService,
	artifactSvc ...*TeamArtifactService,
) *TeamService {
	svc := &TeamService{
		teamRepo:    teamRepo,
		runRepo:     runRepo,
		spawner:     spawner,
		taskRepo:    taskRepo,
		projectRepo: projectRepo,
		memorySvc:   memorySvc,
		hub:         hub,
		templateSvc: templateSvc,
	}
	if len(artifactSvc) > 0 {
		svc.artifactSvc = artifactSvc[0]
	}
	return svc
}

// StartTeam creates a new team and starts its workflow execution by mapping
// the requested strategy name to a seeded system workflow template.
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

	if s.templateSvc == nil {
		return nil, fmt.Errorf("team service: workflow template service not configured")
	}
	variables := map[string]any{
		"runtime":  selection.Runtime,
		"provider": selection.Provider,
		"model":    selection.Model,
	}
	exec, err := s.templateSvc.CreateFromStrategy(ctx, task.ProjectID, task.ID, team.Strategy, variables)
	if err != nil {
		return nil, fmt.Errorf("start team workflow: %w", err)
	}
	team.WorkflowExecutionID = &exec.ID
	if err := s.teamRepo.SetWorkflowExecutionID(ctx, team.ID, exec.ID); err != nil {
		log.WithError(err).WithFields(teamLogFields(team)).Warn("team service: failed to persist workflow execution id")
	}

	return team, nil
}

// ProcessRunCompletion is called when an agent run belonging to a team reaches
// a terminal status. Phase progression is now driven by the workflow engine
// (DAGWorkflowService.HandleAgentRunCompletion) — TeamService is only
// responsible for refreshing aggregate team cost and persisting any structured
// output as a team artifact.
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

	// Store structured output as team artifact.
	if s.artifactSvc != nil && run.StructuredOutput != nil && len(run.StructuredOutput) > 0 {
		if err := s.artifactSvc.StoreFromRun(ctx, *run.TeamID, run); err != nil {
			logArtifactStoreError(*run.TeamID, err)
		} else {
			s.broadcastEvent(ws.EventTeamArtifactCreated, team.ProjectID.String(), map[string]any{
				"teamId": team.ID.String(),
				"runId":  run.ID.String(),
				"role":   run.TeamRole,
			})
		}
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
		plannerRun, err := s.spawner.SpawnForTeam(ctx, teamID, model.TeamRolePlanner, task.ID, memberID, selection.Runtime, selection.Provider, selection.Model, plannerBudget, "planner-agent")
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
	reviewerRun, err := s.spawner.SpawnForTeam(ctx, teamID, model.TeamRoleReviewer, task.ID, memberID, selection.Runtime, selection.Provider, selection.Model, reviewerBudget, "code-reviewer")
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
	if summary, err := s.teamRepo.GetTeamSummary(ctx, teamID); err == nil && summary != nil {
		return summary, nil
	}

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

// ListByProject returns all teams for a project, optionally filtered by status.
func (s *TeamService) ListByProject(ctx context.Context, projectID uuid.UUID, status string) ([]*model.AgentTeam, error) {
	return s.teamRepo.ListByProject(ctx, projectID, status)
}

// ListSummaries returns enriched team summaries for a project, optionally filtered by status.
func (s *TeamService) ListSummaries(ctx context.Context, projectID uuid.UUID, status string) ([]*model.AgentTeamSummaryDTO, error) {
	summaries, err := s.teamRepo.ListTeamSummaries(ctx, projectID, status)
	if err == nil {
		return summaries, nil
	}

	teams, listErr := s.teamRepo.ListByProject(ctx, projectID, status)
	if listErr != nil {
		return nil, listErr
	}

	fallback := make([]*model.AgentTeamSummaryDTO, 0, len(teams))
	for _, team := range teams {
		summary, summaryErr := s.GetSummary(ctx, team.ID)
		if summaryErr == nil && summary != nil {
			fallback = append(fallback, summary)
			continue
		}
		fallback = append(fallback, &model.AgentTeamSummaryDTO{
			AgentTeamDTO: team.ToDTO(),
			CoderRuns:    make([]model.AgentRunDTO, 0),
		})
	}
	return fallback, nil
}

// DeleteTeam removes a team that is in terminal status.
func (s *TeamService) DeleteTeam(ctx context.Context, teamID uuid.UUID) error {
	team, err := s.teamRepo.GetByID(ctx, teamID)
	if err != nil {
		return ErrTeamNotFound
	}
	if !model.IsTerminalTeamStatus(team.Status) {
		return ErrTeamNotActive
	}
	if err := s.teamRepo.Delete(ctx, teamID); err != nil {
		return fmt.Errorf("delete team: %w", err)
	}
	log.WithFields(teamLogFields(team)).Info("team deleted")
	return nil
}

// UpdateTeam updates mutable fields on a team.
func (s *TeamService) UpdateTeam(ctx context.Context, teamID uuid.UUID, req *model.UpdateTeamRequest) (*model.AgentTeam, error) {
	team, err := s.teamRepo.GetByID(ctx, teamID)
	if err != nil {
		return nil, ErrTeamNotFound
	}
	if req.TotalBudgetUsd != nil && *req.TotalBudgetUsd < team.TotalSpentUsd {
		return nil, fmt.Errorf("budget cannot be less than already spent amount ($%.2f)", team.TotalSpentUsd)
	}
	if err := s.teamRepo.Update(ctx, teamID, req); err != nil {
		return nil, fmt.Errorf("update team: %w", err)
	}
	updated, err := s.teamRepo.GetByID(ctx, teamID)
	if err != nil {
		return nil, fmt.Errorf("fetch updated team: %w", err)
	}
	log.WithFields(teamLogFields(updated)).Info("team updated")
	return updated, nil
}

// ListArtifacts returns all artifacts for a team as DTOs.
func (s *TeamService) ListArtifacts(ctx context.Context, teamID uuid.UUID) ([]model.TeamArtifactDTO, error) {
	if s.artifactSvc == nil {
		return []model.TeamArtifactDTO{}, nil
	}
	artifacts, err := s.artifactSvc.ListByTeam(ctx, teamID)
	if err != nil {
		return nil, fmt.Errorf("list team artifacts: %w", err)
	}
	dtos := make([]model.TeamArtifactDTO, 0, len(artifacts))
	for _, a := range artifacts {
		dtos = append(dtos, a.ToDTO())
	}
	return dtos, nil
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
