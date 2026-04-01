package service

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
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

type workflowRoleSkillRootProvider interface {
	SkillsDir() string
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
	if err := FirstMissingWorkflowRoleError(record, s.roles); err != nil {
		return nil, err
	}
	if err := s.validateWorkflowRolePluginDependencies(ctx, record); err != nil {
		return nil, err
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

	maxRetries := 0
	if record.Spec.Workflow.Limits != nil && record.Spec.Workflow.Limits.MaxRetries > 0 {
		maxRetries = record.Spec.Workflow.Limits.MaxRetries
	}

	switch run.Process {
	case model.WorkflowProcessSequential:
		return s.executeSequential(ctx, run, record, maxRetries)
	case model.WorkflowProcessHierarchical:
		return s.executeHierarchical(ctx, run, record, maxRetries)
	case model.WorkflowProcessEventDriven:
		return s.executeEventDriven(ctx, run, record, maxRetries)
	case model.WorkflowProcessWave:
		return s.executeWave(ctx, run, record, maxRetries)
	default:
		return s.failRun(ctx, run, nil, fmt.Errorf("unsupported workflow process mode: %s", run.Process))
	}
}

// executeSequential processes steps one at a time in order (the original behaviour).
func (s *WorkflowExecutionService) executeSequential(ctx context.Context, run *model.WorkflowPluginRun, record *model.PluginRecord, maxRetries int) (*model.WorkflowPluginRun, error) {
	stepOutputs := make(map[string]map[string]any, len(run.Steps))

	for index := range run.Steps {
		result, err := s.executeStep(ctx, run, record, index, maxRetries, stepOutputs)
		if err != nil {
			return result, err
		}
		// If the step produced an awaiting_approval status, pause the run.
		if run.Steps[index].Output != nil {
			if status, _ := run.Steps[index].Output["status"].(string); status == "awaiting_approval" {
				return s.pauseRun(ctx, run)
			}
		}
	}

	return s.completeRun(ctx, run)
}

// executeHierarchical processes steps sequentially. When a step's action is
// "workflow", the executor will recursively start a child workflow via the
// WorkflowChildSpawner wired into the step router.
func (s *WorkflowExecutionService) executeHierarchical(ctx context.Context, run *model.WorkflowPluginRun, record *model.PluginRecord, maxRetries int) (*model.WorkflowPluginRun, error) {
	stepOutputs := make(map[string]map[string]any, len(run.Steps))

	for index := range run.Steps {
		result, err := s.executeStep(ctx, run, record, index, maxRetries, stepOutputs)
		if err != nil {
			return result, err
		}
		if run.Steps[index].Output != nil {
			if status, _ := run.Steps[index].Output["status"].(string); status == "awaiting_approval" {
				return s.pauseRun(ctx, run)
			}
		}
	}

	return s.completeRun(ctx, run)
}

// executeEventDriven is a simplified event-driven mode. It iterates all steps
// and only executes those whose step ID appears in the trigger's "events" list.
// Steps that do not match are skipped. Full pub/sub event-driven execution is
// not yet implemented.
func (s *WorkflowExecutionService) executeEventDriven(ctx context.Context, run *model.WorkflowPluginRun, record *model.PluginRecord, maxRetries int) (*model.WorkflowPluginRun, error) {
	log.Printf("[workflow] event-driven mode for run %s: simplified trigger-match implementation; full event-driven execution is pending", run.ID)

	stepOutputs := make(map[string]map[string]any, len(run.Steps))

	// Determine which step IDs should fire based on the trigger payload.
	triggerEvents := extractTriggerEvents(run.Trigger)

	for index := range run.Steps {
		step := &run.Steps[index]

		if len(triggerEvents) > 0 && !triggerEvents[step.StepID] {
			skippedAt := s.now()
			step.Status = model.WorkflowStepRunStatusSkipped
			step.CompletedAt = &skippedAt
			continue
		}

		result, err := s.executeStep(ctx, run, record, index, maxRetries, stepOutputs)
		if err != nil {
			return result, err
		}
		if run.Steps[index].Output != nil {
			if status, _ := run.Steps[index].Output["status"].(string); status == "awaiting_approval" {
				return s.pauseRun(ctx, run)
			}
		}
	}

	return s.completeRun(ctx, run)
}

// executeWave groups steps by a "wave_number" value from the step definition's
// Config or Metadata. Steps within the same wave execute in parallel using
// goroutines. Waves are processed in ascending order; the next wave starts only
// after all steps in the current wave complete. Steps without a wave_number
// default to wave 0.
func (s *WorkflowExecutionService) executeWave(ctx context.Context, run *model.WorkflowPluginRun, record *model.PluginRecord, maxRetries int) (*model.WorkflowPluginRun, error) {
	stepOutputs := make(map[string]map[string]any, len(run.Steps))

	// Build wave groups from the original step definitions.
	waves := groupStepsByWave(record.Spec.Workflow.Steps, run.Steps)
	waveNumbers := sortedWaveNumbers(waves)

	for _, waveNum := range waveNumbers {
		if err := ctx.Err(); err != nil {
			return s.cancelRun(ctx, run, err)
		}

		indices := waves[waveNum]
		if len(indices) == 1 {
			// Single step in the wave; run inline (no goroutine overhead).
			result, err := s.executeStep(ctx, run, record, indices[0], maxRetries, stepOutputs)
			if err != nil {
				return result, err
			}
			if run.Steps[indices[0]].Output != nil {
				if status, _ := run.Steps[indices[0]].Output["status"].(string); status == "awaiting_approval" {
					return s.pauseRun(ctx, run)
				}
			}
			continue
		}

		// Multiple steps: execute in parallel.
		type stepResult struct {
			index  int
			run    *model.WorkflowPluginRun
			err    error
		}

		var mu sync.Mutex
		var wg sync.WaitGroup
		results := make([]stepResult, len(indices))

		for i, idx := range indices {
			wg.Add(1)
			go func(slot, stepIndex int) {
				defer wg.Done()
				r, err := s.executeStep(ctx, run, record, stepIndex, maxRetries, stepOutputs)
				mu.Lock()
				results[slot] = stepResult{index: stepIndex, run: r, err: err}
				mu.Unlock()
			}(i, idx)
		}
		wg.Wait()

		// Check for failures in the wave.
		for _, res := range results {
			if res.err != nil {
				return res.run, res.err
			}
		}
		// Check for approval pauses.
		for _, idx := range indices {
			if run.Steps[idx].Output != nil {
				if status, _ := run.Steps[idx].Output["status"].(string); status == "awaiting_approval" {
					return s.pauseRun(ctx, run)
				}
			}
		}
	}

	return s.completeRun(ctx, run)
}

// executeStep runs a single step with retries. It is the shared core used by
// all execution modes. The caller must hold no locks; the method updates the
// run record in the store after each attempt.
func (s *WorkflowExecutionService) executeStep(
	ctx context.Context,
	run *model.WorkflowPluginRun,
	record *model.PluginRecord,
	index int,
	maxRetries int,
	stepOutputs map[string]map[string]any,
) (*model.WorkflowPluginRun, error) {
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
	roleProfile := rolepkg.BuildExecutionProfile(roleManifest, rolepkg.WithSkillRoot(resolveWorkflowRoleSkillsDir(s.roles)))
	if rolepkg.HasBlockingSkillDiagnostics(roleProfile) {
		return s.failRun(ctx, run, step, fmt.Errorf("resolve workflow role %s runtime skills: %s", step.RoleID, joinWorkflowBlockingSkillMessages(roleProfile.SkillDiagnostics)))
	}
	if err := s.validateSingleWorkflowRolePluginDependencies(ctx, step.RoleID); err != nil {
		return s.failRun(ctx, run, step, err)
	}

	// Resolve step definition to pass Config through.
	stepDef := model.WorkflowStepDefinition{ID: step.StepID, Role: step.RoleID, Action: step.Action}
	if record.Spec.Workflow != nil {
		for _, def := range record.Spec.Workflow.Steps {
			if def.ID == step.StepID {
				stepDef = def
				break
			}
		}
	}

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
			Step:        stepDef,
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
	return nil, nil
}

func resolveWorkflowRoleSkillsDir(store PluginRoleStore) string {
	provider, ok := store.(workflowRoleSkillRootProvider)
	if !ok {
		return ""
	}
	return provider.SkillsDir()
}

func (s *WorkflowExecutionService) validateWorkflowRolePluginDependencies(ctx context.Context, record *model.PluginRecord) error {
	if record == nil {
		return nil
	}
	for _, dependency := range BuildPluginRoleDependencies(record, s.roles) {
		if dependency.Blocking {
			return fmt.Errorf("unknown workflow role reference: %s", dependency.RoleID)
		}
		if err := s.validateSingleWorkflowRolePluginDependencies(ctx, dependency.RoleID); err != nil {
			return err
		}
	}
	return nil
}

func (s *WorkflowExecutionService) validateSingleWorkflowRolePluginDependencies(ctx context.Context, roleID string) error {
	catalog, ok := s.plugins.(pluginCatalogListProvider)
	if !ok {
		return nil
	}
	plugins, err := ListDependencyPlugins(ctx, catalog)
	if err != nil {
		return fmt.Errorf("load workflow plugin dependency catalog: %w", err)
	}
	roleManifest, err := s.roles.Get(roleID)
	if err != nil {
		return fmt.Errorf("resolve workflow role %s: %w", roleID, err)
	}
	dependencies := BuildRolePluginDependencies(roleManifest, plugins)
	if HasBlockingRolePluginDependencies(dependencies) {
		return fmt.Errorf("resolve workflow role %s plugin dependencies: %s", roleID, JoinBlockingRolePluginDependencyMessages(dependencies))
	}
	return nil
}

func joinWorkflowBlockingSkillMessages(diagnostics []model.RoleExecutionSkillDiagnostic) string {
	messages := make([]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		if diagnostic.Blocking {
			messages = append(messages, diagnostic.Message)
		}
	}
	return strings.Join(messages, "; ")
}

func (s *WorkflowExecutionService) completeRun(ctx context.Context, run *model.WorkflowPluginRun) (*model.WorkflowPluginRun, error) {
	completedAt := s.now()
	run.Status = model.WorkflowRunStatusCompleted
	run.CurrentStepID = ""
	run.CompletedAt = &completedAt
	if err := s.runs.Update(ctx, run); err != nil {
		return nil, err
	}
	return s.runs.GetByID(ctx, run.ID)
}

func (s *WorkflowExecutionService) pauseRun(ctx context.Context, run *model.WorkflowPluginRun) (*model.WorkflowPluginRun, error) {
	run.Status = model.WorkflowRunStatusPaused
	if err := s.runs.Update(ctx, run); err != nil {
		return nil, err
	}
	return s.runs.GetByID(ctx, run.ID)
}

// extractTriggerEvents reads a "events" key from the trigger payload and
// returns a set of step IDs that should be executed.
func extractTriggerEvents(trigger map[string]any) map[string]bool {
	if trigger == nil {
		return nil
	}
	raw, ok := trigger["events"]
	if !ok {
		return nil
	}
	result := make(map[string]bool)
	switch typed := raw.(type) {
	case []string:
		for _, s := range typed {
			result[s] = true
		}
	case []any:
		for _, item := range typed {
			if s, ok := item.(string); ok {
				result[s] = true
			}
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// groupStepsByWave maps wave numbers to step indices. The wave_number is read
// from the step definition's Config["wave_number"] or Metadata["wave_number"].
// Steps without a wave_number default to wave 0.
func groupStepsByWave(defs []model.WorkflowStepDefinition, steps []model.WorkflowStepRun) map[int][]int {
	waves := make(map[int][]int)
	for i, step := range steps {
		waveNum := 0
		// Try to find the matching definition for Config/Metadata.
		for _, def := range defs {
			if def.ID == step.StepID {
				waveNum = readWaveNumber(def)
				break
			}
		}
		waves[waveNum] = append(waves[waveNum], i)
	}
	return waves
}

func readWaveNumber(def model.WorkflowStepDefinition) int {
	if n := extractIntFromMap(def.Config, "wave_number"); n > 0 {
		return n
	}
	if n := extractIntFromMap(def.Metadata, "wave_number"); n > 0 {
		return n
	}
	return 0
}

func extractIntFromMap(m map[string]any, key string) int {
	if m == nil {
		return 0
	}
	switch v := m[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case float32:
		return int(v)
	}
	return 0
}

func sortedWaveNumbers(waves map[int][]int) []int {
	nums := make([]int, 0, len(waves))
	for n := range waves {
		nums = append(nums, n)
	}
	sort.Ints(nums)
	return nums
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
