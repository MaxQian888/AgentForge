package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	rolepkg "github.com/react-go-quick-starter/server/internal/role"
)

type WorkflowPluginCatalog interface {
	GetByID(ctx context.Context, pluginID string) (*model.PluginRecord, error)
}

type WorkflowRunStore interface {
	Create(ctx context.Context, run *model.WorkflowPluginRun) error
	Update(ctx context.Context, run *model.WorkflowPluginRun) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.WorkflowPluginRun, error)
	ListByPluginID(ctx context.Context, pluginID string, limit int) ([]*model.WorkflowPluginRun, error)
}

type WorkflowStepExecutor interface {
	Execute(ctx context.Context, req WorkflowStepExecutionRequest) (*WorkflowStepExecutionResult, error)
}

type WorkflowExecutionRequest struct {
	Trigger map[string]any
}

type WorkflowStepExecutionRequest struct {
	RunID        uuid.UUID
	PluginID     string
	Process      model.WorkflowProcessMode
	Step         model.WorkflowStepDefinition
	RoleProfile  *model.RoleExecutionProfile
	Attempt      int
	Input        map[string]any
	StartedAt    time.Time
}

type WorkflowStepExecutionResult struct {
	Output map[string]any
}

type WorkflowExecutionService struct {
	plugins  WorkflowPluginCatalog
	runs     WorkflowRunStore
	roles    PluginRoleStore
	executor WorkflowStepExecutor
	now      func() time.Time
}

func NewWorkflowExecutionService(
	plugins WorkflowPluginCatalog,
	runs WorkflowRunStore,
	roles PluginRoleStore,
	executor WorkflowStepExecutor,
) *WorkflowExecutionService {
	return &WorkflowExecutionService{
		plugins:  plugins,
		runs:     runs,
		roles:    roles,
		executor: executor,
		now:      func() time.Time { return time.Now().UTC() },
	}
}

func (s *WorkflowExecutionService) WithClock(now func() time.Time) *WorkflowExecutionService {
	if now != nil {
		s.now = now
	}
	return s
}

func (s *WorkflowExecutionService) Start(ctx context.Context, pluginID string, req WorkflowExecutionRequest) (*model.WorkflowPluginRun, error) {
	if s.plugins == nil {
		return nil, fmt.Errorf("workflow plugin catalog is not configured")
	}
	if s.runs == nil {
		return nil, fmt.Errorf("workflow run store is not configured")
	}
	if s.roles == nil {
		return nil, fmt.Errorf("workflow role store is not configured")
	}
	if s.executor == nil {
		return nil, fmt.Errorf("workflow step executor is not configured")
	}

	record, err := s.plugins.GetByID(ctx, pluginID)
	if err != nil {
		return nil, err
	}
	if record.Kind != model.PluginKindWorkflow || record.Spec.Workflow == nil {
		return nil, fmt.Errorf("plugin %s is not a workflow plugin", pluginID)
	}
	if record.LifecycleState == model.PluginStateDisabled {
		return nil, fmt.Errorf("workflow plugin %s is disabled", pluginID)
	}
	if record.LifecycleState != model.PluginStateEnabled && record.LifecycleState != model.PluginStateActive {
		return nil, fmt.Errorf("workflow plugin %s must be enabled before execution", pluginID)
	}
	if !isExecutableWorkflowProcess(record.Spec.Workflow.Process) {
		return nil, unsupportedWorkflowProcessError(record)
	}

	run := &model.WorkflowPluginRun{
		ID:        uuid.New(),
		PluginID:  record.Metadata.ID,
		Process:   record.Spec.Workflow.Process,
		Status:    model.WorkflowRunStatusRunning,
		Trigger:   cloneWorkflowPayload(req.Trigger),
		Steps:     initializeWorkflowSteps(record.Spec.Workflow.Steps),
		StartedAt: s.now(),
	}
	if err := s.runs.Create(ctx, run); err != nil {
		return nil, err
	}

	stepOutputs := make(map[string]map[string]any, len(run.Steps))
	maxRetries := 0
	if record.Spec.Workflow.Limits != nil && record.Spec.Workflow.Limits.MaxRetries > 0 {
		maxRetries = record.Spec.Workflow.Limits.MaxRetries
	}

	for index := range run.Steps {
		if err := ctx.Err(); err != nil {
			return s.cancelRun(ctx, run, err)
		}

		step := &run.Steps[index]
		run.CurrentStepID = step.StepID
		step.Status = model.WorkflowStepRunStatusRunning
		startedAt := s.now()
		step.StartedAt = &startedAt
		if err := s.runs.Update(ctx, run); err != nil {
			return nil, err
		}

		roleManifest, err := s.roles.Get(step.RoleID)
		if err != nil {
			return s.failRun(ctx, run, step, fmt.Errorf("resolve workflow role %s: %w", step.RoleID, err))
		}
		roleProfile := rolepkg.BuildExecutionProfile(roleManifest)

		var lastErr error
		for attempt := 1; attempt <= maxRetries+1; attempt++ {
			stepInput := buildWorkflowStepInput(run, *step, roleProfile, attempt, stepOutputs)
			step.Input = cloneWorkflowPayload(stepInput)
			step.RetryCount = attempt - 1

			attemptStartedAt := s.now()
			result, execErr := s.executor.Execute(ctx, WorkflowStepExecutionRequest{
				RunID:       run.ID,
				PluginID:    run.PluginID,
				Process:     run.Process,
				Step:        model.WorkflowStepDefinition{ID: step.StepID, Role: step.RoleID, Action: step.Action},
				RoleProfile: roleProfile,
				Attempt:     attempt,
				Input:       cloneWorkflowPayload(stepInput),
				StartedAt:   attemptStartedAt,
			})
			attemptCompletedAt := s.now()

			attemptRecord := model.WorkflowStepAttempt{
				Attempt:     attempt,
				Status:      model.WorkflowStepRunStatusFailed,
				Error:       "",
				StartedAt:   attemptStartedAt,
				CompletedAt: &attemptCompletedAt,
			}
			if execErr == nil {
				output := map[string]any{}
				if result != nil && result.Output != nil {
					output = cloneWorkflowPayload(result.Output)
				}
				attemptRecord.Status = model.WorkflowStepRunStatusCompleted
				attemptRecord.Output = cloneWorkflowPayload(output)
				step.Status = model.WorkflowStepRunStatusCompleted
				step.Output = cloneWorkflowPayload(output)
				step.Error = ""
				step.CompletedAt = &attemptCompletedAt
				step.Attempts = append(step.Attempts, attemptRecord)
				stepOutputs[step.StepID] = cloneWorkflowPayload(output)
				if err := s.runs.Update(ctx, run); err != nil {
					return nil, err
				}
				lastErr = nil
				break
			}

			attemptRecord.Error = execErr.Error()
			step.Attempts = append(step.Attempts, attemptRecord)
			lastErr = execErr
			if attempt > maxRetries {
				return s.failRun(ctx, run, step, execErr)
			}
			if err := s.runs.Update(ctx, run); err != nil {
				return nil, err
			}
		}

		if lastErr != nil {
			return nil, lastErr
		}
	}

	completedAt := s.now()
	run.Status = model.WorkflowRunStatusCompleted
	run.CurrentStepID = ""
	run.CompletedAt = &completedAt
	if err := s.runs.Update(ctx, run); err != nil {
		return nil, err
	}
	return s.runs.GetByID(ctx, run.ID)
}

func (s *WorkflowExecutionService) GetRun(ctx context.Context, id uuid.UUID) (*model.WorkflowPluginRun, error) {
	if s.runs == nil {
		return nil, fmt.Errorf("workflow run store is not configured")
	}
	return s.runs.GetByID(ctx, id)
}

func (s *WorkflowExecutionService) ListRuns(ctx context.Context, pluginID string, limit int) ([]*model.WorkflowPluginRun, error) {
	if s.runs == nil {
		return nil, fmt.Errorf("workflow run store is not configured")
	}
	return s.runs.ListByPluginID(ctx, pluginID, limit)
}

func (s *WorkflowExecutionService) failRun(ctx context.Context, run *model.WorkflowPluginRun, step *model.WorkflowStepRun, cause error) (*model.WorkflowPluginRun, error) {
	completedAt := s.now()
	run.Status = model.WorkflowRunStatusFailed
	run.Error = cause.Error()
	run.CompletedAt = &completedAt
	if step != nil {
		step.Status = model.WorkflowStepRunStatusFailed
		step.Error = cause.Error()
		step.CompletedAt = &completedAt
	}
	if err := s.runs.Update(ctx, run); err != nil {
		return nil, err
	}
	stored, err := s.runs.GetByID(ctx, run.ID)
	if err != nil {
		return nil, err
	}
	return stored, cause
}

func (s *WorkflowExecutionService) cancelRun(ctx context.Context, run *model.WorkflowPluginRun, cause error) (*model.WorkflowPluginRun, error) {
	completedAt := s.now()
	run.Status = model.WorkflowRunStatusCancelled
	run.Error = cause.Error()
	run.CompletedAt = &completedAt
	if err := s.runs.Update(ctx, run); err != nil {
		return nil, err
	}
	stored, err := s.runs.GetByID(ctx, run.ID)
	if err != nil {
		return nil, err
	}
	return stored, cause
}

func initializeWorkflowSteps(steps []model.WorkflowStepDefinition) []model.WorkflowStepRun {
	runs := make([]model.WorkflowStepRun, len(steps))
	for index, step := range steps {
		runs[index] = model.WorkflowStepRun{
			StepID: step.ID,
			RoleID: step.Role,
			Action: step.Action,
			Status: model.WorkflowStepRunStatusPending,
		}
	}
	return runs
}

func buildWorkflowStepInput(
	run *model.WorkflowPluginRun,
	step model.WorkflowStepRun,
	roleProfile *model.RoleExecutionProfile,
	attempt int,
	stepOutputs map[string]map[string]any,
) map[string]any {
	input := map[string]any{
		"trigger": cloneWorkflowPayload(run.Trigger),
		"workflow": map[string]any{
			"run_id":    run.ID.String(),
			"plugin_id": run.PluginID,
			"process":   string(run.Process),
			"step_id":   step.StepID,
			"attempt":   attempt,
		},
	}
	if roleProfile != nil {
		input["role"] = map[string]any{
			"role_id":         roleProfile.RoleID,
			"name":            roleProfile.Name,
			"role":            roleProfile.Role,
			"goal":            roleProfile.Goal,
			"backstory":       roleProfile.Backstory,
			"system_prompt":   roleProfile.SystemPrompt,
			"allowed_tools":   append([]string(nil), roleProfile.AllowedTools...),
			"max_budget_usd":  roleProfile.MaxBudgetUsd,
			"max_turns":       roleProfile.MaxTurns,
			"permission_mode": roleProfile.PermissionMode,
		}
	}
	if len(stepOutputs) > 0 {
		steps := make(map[string]any, len(stepOutputs))
		for stepID, output := range stepOutputs {
			steps[stepID] = cloneWorkflowPayload(output)
		}
		input["steps"] = steps
	}
	return input
}

func cloneWorkflowPayload(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	cloned := make(map[string]any, len(input))
	for key, value := range input {
		switch typed := value.(type) {
		case map[string]any:
			cloned[key] = cloneWorkflowPayload(typed)
		case []string:
			cloned[key] = append([]string(nil), typed...)
		case []any:
			items := make([]any, len(typed))
			for index, item := range typed {
				if nested, ok := item.(map[string]any); ok {
					items[index] = cloneWorkflowPayload(nested)
					continue
				}
				items[index] = item
			}
			cloned[key] = items
		default:
			cloned[key] = value
		}
	}
	return cloned
}
