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
	// ActingEmployeeID, when non-nil, is the run-level acting-employee
	// attribution that every subsequent step inherits unless it declares its
	// own explicit employee id (see change bridge-employee-attribution-legacy).
	ActingEmployeeID *uuid.UUID
	// ProjectID scopes the run to its originating project. Populated by the
	// trigger adapter from the WorkflowTrigger row; zero UUID means the run
	// was started outside a project-scoped entry point. Used by the unified
	// workflow-run view (bridge-unified-run-view) to filter plugin runs by
	// project without requiring a cross-table join.
	ProjectID uuid.UUID
	// TriggerID is the WorkflowTrigger.ID that dispatched this run, when
	// applicable. Mirrors run.TriggerID so the unified view can filter plugin
	// runs by trigger without walking the trigger payload map.
	TriggerID *uuid.UUID
}

type WorkflowStepExecutionRequest struct {
	RunID       uuid.UUID
	PluginID    string
	Process     model.WorkflowProcessMode
	Step        model.WorkflowStepDefinition
	RoleProfile *model.RoleExecutionProfile
	Attempt     int
	Input       map[string]any
	StartedAt   time.Time
	// RunActingEmployeeID is the workflow plugin run's run-level acting-employee
	// attribution default. When a step omits an explicit `trigger.employeeId`,
	// the step router uses this id (see change bridge-employee-attribution-legacy).
	RunActingEmployeeID *uuid.UUID
	// RunProjectID scopes the run to its originating project. Used by the
	// `workflow` action's DAG branch so cross-engine child invocations can
	// enforce same-project and carry project attribution onto the new
	// execution (bridge-legacy-to-dag-invocation).
	RunProjectID uuid.UUID
}

type WorkflowStepExecutionResult struct {
	Output map[string]any
}

// WorkflowPluginTerminalObserver is called whenever a WorkflowPluginRun reaches
// a terminal state (completed, failed, cancelled). The plugin runtime invokes
// each registered observer sequentially; observer errors are logged but never
// propagate back to the run's own transition, so terminal-state emission is
// best-effort. Wired by the sub-workflow invocation bridge so a plugin run
// that started as a DAG sub_workflow child resumes the parent DAG node.
type WorkflowPluginTerminalObserver interface {
	OnPluginRunTerminal(ctx context.Context, run *model.WorkflowPluginRun)
}

type WorkflowExecutionService struct {
	plugins   WorkflowPluginCatalog
	runs      WorkflowRunStore
	roles     PluginRoleStore
	executor  WorkflowStepExecutor
	now       func() time.Time
	observers []WorkflowPluginTerminalObserver
	// runEmitter publishes canonical workflow.run.* events alongside the
	// legacy per-run observers so the workflow workspace UI can consume one
	// cross-engine channel (bridge-unified-run-view).
	runEmitter *WorkflowRunEventEmitter
}

// SetRunEmitter wires the unified workflow.run.* emitter. Nil keeps the
// emission disabled for legacy test wiring.
func (s *WorkflowExecutionService) SetRunEmitter(e *WorkflowRunEventEmitter) { s.runEmitter = e }

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

// RegisterTerminalObserver appends an observer invoked when any workflow plugin
// run reaches a terminal state. Observers are called in registration order
// after the run has been persisted; their errors are not propagated to the
// run's transition itself. Safe to call from wiring at startup; NOT safe to
// call concurrently while runs are in flight.
func (s *WorkflowExecutionService) RegisterTerminalObserver(o WorkflowPluginTerminalObserver) {
	if o == nil {
		return
	}
	s.observers = append(s.observers, o)
}

// emitTerminal invokes every registered observer for a just-terminated run.
// Observer panics are isolated to protect the run's own transition.
func (s *WorkflowExecutionService) emitTerminal(ctx context.Context, run *model.WorkflowPluginRun) {
	if run == nil || len(s.observers) == 0 {
		return
	}
	for _, o := range s.observers {
		func(observer WorkflowPluginTerminalObserver) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[WARN] workflow plugin runtime: terminal observer panic: %v", r)
				}
			}()
			observer.OnPluginRunTerminal(ctx, run)
		}(o)
	}
}

func (s *WorkflowExecutionService) Start(ctx context.Context, pluginID string, req WorkflowExecutionRequest) (*model.WorkflowPluginRun, error) {
	record, err := s.loadExecutableWorkflowRecord(ctx, pluginID)
	if err != nil {
		return nil, err
	}
	return s.startWithRecord(ctx, record, req)
}

// StartTriggered is the entry point the unified trigger router uses to start a
// workflow plugin run in response to an external trigger event (IM command,
// schedule tick, future webhook). It wraps the existing plugin runtime start
// path with trigger-flavoured provenance tags (source="workflow.trigger",
// triggerId) so downstream run listings can distinguish trigger-initiated
// runs from task/manual starts.
//
// seed becomes the workflow plugin run's trigger payload under "$event" to
// keep parity with the DAG adapter's StartOptions.Seed contract.
//
// This seam intentionally stays narrow — it does not introduce new execution
// semantics; it only supplies a trigger-aware entry point so plugin runs are
// first-class trigger targets.
func (s *WorkflowExecutionService) StartTriggered(
	ctx context.Context,
	pluginID string,
	seed map[string]any,
	triggerID uuid.UUID,
) (*model.WorkflowPluginRun, error) {
	return s.StartTriggeredWithEmployee(ctx, pluginID, seed, triggerID, nil)
}

// StartTriggeredWithEmployee extends StartTriggered by stamping an optional
// run-level acting-employee identifier onto the new WorkflowPluginRun. Called
// by PluginEngineAdapter when a trigger row carries acting_employee_id.
func (s *WorkflowExecutionService) StartTriggeredWithEmployee(
	ctx context.Context,
	pluginID string,
	seed map[string]any,
	triggerID uuid.UUID,
	actingEmployeeID *uuid.UUID,
) (*model.WorkflowPluginRun, error) {
	return s.StartTriggeredForProject(ctx, pluginID, seed, triggerID, actingEmployeeID, uuid.Nil)
}

// StartTriggeredForProject is the project-scoped variant used by the unified
// trigger router once the WorkflowTrigger row's ProjectID is known. A zero
// projectID falls back to the pre-unified-view behavior (no project scope on
// the resulting plugin run).
func (s *WorkflowExecutionService) StartTriggeredForProject(
	ctx context.Context,
	pluginID string,
	seed map[string]any,
	triggerID uuid.UUID,
	actingEmployeeID *uuid.UUID,
	projectID uuid.UUID,
) (*model.WorkflowPluginRun, error) {
	record, err := s.loadExecutableWorkflowRecord(ctx, pluginID)
	if err != nil {
		return nil, err
	}
	trigger := map[string]any{
		"source":    "workflow.trigger",
		"triggerId": triggerID.String(),
	}
	if seed != nil {
		trigger["$event"] = cloneWorkflowPayload(seed)
	}
	triggerIDCopy := triggerID
	return s.startWithRecord(ctx, record, WorkflowExecutionRequest{
		Trigger:          trigger,
		ActingEmployeeID: actingEmployeeID,
		ProjectID:        projectID,
		TriggerID:        &triggerIDCopy,
	})
}

func (s *WorkflowExecutionService) StartTaskTriggered(
	ctx context.Context,
	pluginID string,
	profile string,
	taskID uuid.UUID,
	req WorkflowExecutionRequest,
) (*model.WorkflowPluginRun, error) {
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
	if taskID == uuid.Nil {
		return nil, fmt.Errorf("task-triggered workflow start requires task ID")
	}
	if strings.TrimSpace(profile) == "" {
		return nil, fmt.Errorf("task-triggered workflow start requires trigger profile")
	}

	record, err := s.loadExecutableWorkflowRecord(ctx, pluginID)
	if err != nil {
		return nil, err
	}
	if !supportsTaskTriggeredProfile(record, profile) {
		return nil, fmt.Errorf("workflow plugin %s does not support task-triggered profile %s", pluginID, profile)
	}
	duplicate, err := s.hasActiveTaskTriggeredRun(ctx, pluginID, profile, taskID)
	if err != nil {
		return nil, err
	}
	if duplicate {
		return nil, fmt.Errorf("workflow plugin %s already has an active workflow run for task %s and profile %s", pluginID, taskID, profile)
	}
	trigger := cloneWorkflowPayload(req.Trigger)
	if trigger == nil {
		trigger = map[string]any{}
	}
	trigger["source"] = "task.trigger"
	trigger["taskId"] = taskID.String()
	trigger["profile"] = profile

	return s.startWithRecord(ctx, record, WorkflowExecutionRequest{Trigger: trigger})
}

func (s *WorkflowExecutionService) loadExecutableWorkflowRecord(ctx context.Context, pluginID string) (*model.PluginRecord, error) {
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
	return record, nil
}

func (s *WorkflowExecutionService) startWithRecord(ctx context.Context, record *model.PluginRecord, req WorkflowExecutionRequest) (*model.WorkflowPluginRun, error) {
	run := &model.WorkflowPluginRun{
		ID:               uuid.New(),
		PluginID:         record.Metadata.ID,
		ProjectID:        req.ProjectID,
		Process:          record.Spec.Workflow.Process,
		Status:           model.WorkflowRunStatusRunning,
		Trigger:          cloneWorkflowPayload(req.Trigger),
		Steps:            initializeWorkflowSteps(record.Spec.Workflow.Steps),
		ActingEmployeeID: req.ActingEmployeeID,
		TriggerID:        req.TriggerID,
		StartedAt:        s.now(),
	}
	if err := s.runs.Create(ctx, run); err != nil {
		return nil, err
	}
	s.runEmitter.EmitPluginStatusChanged(ctx, run)

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

func supportsTaskTriggeredProfile(record *model.PluginRecord, profile string) bool {
	if record == nil || record.Spec.Workflow == nil {
		return false
	}
	for _, trigger := range record.Spec.Workflow.Triggers {
		if trigger.Event == "task.transition" && strings.TrimSpace(trigger.Profile) == profile {
			return true
		}
	}
	return false
}

func (s *WorkflowExecutionService) hasActiveTaskTriggeredRun(ctx context.Context, pluginID, profile string, taskID uuid.UUID) (bool, error) {
	runs, err := s.runs.ListByPluginID(ctx, pluginID, 50)
	if err != nil {
		return false, err
	}
	taskIDString := taskID.String()
	for _, run := range runs {
		if run == nil || !isWorkflowRunActive(run.Status) {
			continue
		}
		if workflowTriggerString(run.Trigger, "taskId") != taskIDString {
			continue
		}
		if workflowTriggerString(run.Trigger, "profile") != profile {
			continue
		}
		return true, nil
	}
	return false, nil
}

func isWorkflowRunActive(status model.WorkflowRunStatus) bool {
	switch status {
	case model.WorkflowRunStatusPending, model.WorkflowRunStatusRunning, model.WorkflowRunStatusPaused:
		return true
	default:
		return false
	}
}

func workflowTriggerString(trigger map[string]any, key string) string {
	if trigger == nil {
		return ""
	}
	value, ok := trigger[key]
	if !ok {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprint(value)
}

// executeSequential processes steps one at a time in order (the original behaviour).
func (s *WorkflowExecutionService) executeSequential(ctx context.Context, run *model.WorkflowPluginRun, record *model.PluginRecord, maxRetries int) (*model.WorkflowPluginRun, error) {
	stepOutputs := make(map[string]map[string]any, len(run.Steps))

	for index := range run.Steps {
		result, err := s.executeStep(ctx, run, record, index, maxRetries, stepOutputs)
		if err != nil {
			return result, err
		}
		// If the step produced a parking status (awaiting_approval or
		// awaiting_sub_workflow), pause the run. A parked awaiting_sub_workflow
		// step stamps that status on the step record itself so resume hooks
		// can find it.
		if parkedRun, parked, parkErr := s.handleParkedStep(ctx, run, index); parked {
			return parkedRun, parkErr
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
		if parkedRun, parked, parkErr := s.handleParkedStep(ctx, run, index); parked {
			return parkedRun, parkErr
		}
	}

	return s.completeRun(ctx, run)
}

// handleParkedStep inspects a just-executed step for a parking status and
// transitions the run accordingly. Returns (run, true, err) when the step
// caused the run to pause and the caller should stop advancing; (nil, false,
// nil) otherwise. Pulled into a helper so every execution mode can share the
// awaiting_approval / awaiting_sub_workflow handling.
func (s *WorkflowExecutionService) handleParkedStep(ctx context.Context, run *model.WorkflowPluginRun, index int) (*model.WorkflowPluginRun, bool, error) {
	output := run.Steps[index].Output
	if output == nil {
		return nil, false, nil
	}
	status, _ := output["status"].(string)
	switch status {
	case "awaiting_approval":
		paused, err := s.pauseRun(ctx, run)
		return paused, true, err
	case string(model.WorkflowStepRunStatusAwaitingSubWorkflow):
		// Stamp the parking status on the step itself so the resume hook can
		// find it. completedAt is intentionally cleared — the step is not
		// done until the DAG child terminates.
		run.Steps[index].Status = model.WorkflowStepRunStatusAwaitingSubWorkflow
		run.Steps[index].CompletedAt = nil
		paused, err := s.pauseRun(ctx, run)
		return paused, true, err
	}
	return nil, false, nil
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
		if parkedRun, parked, parkErr := s.handleParkedStep(ctx, run, index); parked {
			return parkedRun, parkErr
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
			if parkedRun, parked, parkErr := s.handleParkedStep(ctx, run, indices[0]); parked {
				return parkedRun, parkErr
			}
			_ = result
			continue
		}

		// Multiple steps: execute in parallel.
		type stepResult struct {
			index int
			run   *model.WorkflowPluginRun
			err   error
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
		// Check for parking states (awaiting_approval or awaiting_sub_workflow).
		for _, idx := range indices {
			if parkedRun, parked, parkErr := s.handleParkedStep(ctx, run, idx); parked {
				return parkedRun, parkErr
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
			RunID:               run.ID,
			PluginID:            run.PluginID,
			Process:             run.Process,
			Step:                stepDef,
			RoleProfile:         roleProfile,
			Attempt:             attempt,
			Input:               cloneWorkflowPayload(stepInput),
			StartedAt:           attemptStartedAt,
			RunActingEmployeeID: run.ActingEmployeeID,
			RunProjectID:        run.ProjectID,
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
	stored, err := s.runs.GetByID(ctx, run.ID)
	if err != nil {
		return nil, err
	}
	s.emitTerminal(ctx, stored)
	s.runEmitter.EmitPluginTerminal(ctx, stored)
	return stored, nil
}

func (s *WorkflowExecutionService) pauseRun(ctx context.Context, run *model.WorkflowPluginRun) (*model.WorkflowPluginRun, error) {
	run.Status = model.WorkflowRunStatusPaused
	if err := s.runs.Update(ctx, run); err != nil {
		return nil, err
	}
	stored, err := s.runs.GetByID(ctx, run.ID)
	if err != nil {
		return nil, err
	}
	s.runEmitter.EmitPluginStatusChanged(ctx, stored)
	return stored, nil
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
	s.emitTerminal(ctx, stored)
	s.runEmitter.EmitPluginTerminal(ctx, stored)
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
	s.emitTerminal(ctx, stored)
	s.runEmitter.EmitPluginTerminal(ctx, stored)
	return stored, cause
}

// ResumeParkedDAGChild transitions a plugin run's `workflow` step parked with
// status awaiting_sub_workflow into a terminal step status based on the
// supplied outcome, then resumes the run by executing remaining steps. Invoked
// by the DAG service's terminal-state hook when a child run linked via
// parent_kind='plugin_run' reaches completion/failure/cancellation.
//
// Idempotent: a run whose parked step has already been transitioned is left
// unchanged. A plugin run that is no longer Paused (e.g. already failed for
// another reason) is not resumed.
func (s *WorkflowExecutionService) ResumeParkedDAGChild(
	ctx context.Context,
	parentRunID, childRunID uuid.UUID,
	outcome string,
	childOutputs map[string]any,
) error {
	if s.runs == nil {
		return fmt.Errorf("workflow run store is not configured")
	}
	run, err := s.runs.GetByID(ctx, parentRunID)
	if err != nil {
		return err
	}
	if run.Status != model.WorkflowRunStatusPaused {
		return nil // run is no longer parked; nothing to do
	}
	parkedIdx := findParkedDAGChildStep(run, childRunID)
	if parkedIdx < 0 {
		return nil
	}

	now := s.now()
	step := &run.Steps[parkedIdx]
	if step.Output == nil {
		step.Output = map[string]any{}
	}
	step.Output["child_outcome"] = outcome
	if childOutputs != nil {
		step.Output["child_outputs"] = cloneWorkflowPayload(childOutputs)
	}

	switch outcome {
	case model.SubWorkflowLinkStatusCompleted:
		step.Status = model.WorkflowStepRunStatusCompleted
		step.Output["status"] = string(model.WorkflowStepRunStatusCompleted)
		step.CompletedAt = &now
	case model.SubWorkflowLinkStatusFailed, model.SubWorkflowLinkStatusCancelled:
		step.Status = model.WorkflowStepRunStatusFailed
		step.CompletedAt = &now
		step.Error = fmt.Sprintf("dag child %s terminated with outcome %s", childRunID, outcome)
	default:
		return fmt.Errorf("unknown resume outcome %q for plugin run %s", outcome, parentRunID)
	}

	if err := s.runs.Update(ctx, run); err != nil {
		return err
	}

	if step.Status == model.WorkflowStepRunStatusFailed {
		_, _ = s.failRun(ctx, run, step, fmt.Errorf("%s", step.Error))
		return nil
	}

	// Step completed — continue executing remaining steps sequentially.
	record, err := s.loadExecutableWorkflowRecord(ctx, run.PluginID)
	if err != nil {
		return err
	}
	maxRetries := 0
	if record.Spec.Workflow.Limits != nil && record.Spec.Workflow.Limits.MaxRetries > 0 {
		maxRetries = record.Spec.Workflow.Limits.MaxRetries
	}

	run.Status = model.WorkflowRunStatusRunning
	if err := s.runs.Update(ctx, run); err != nil {
		return err
	}

	stepOutputs := make(map[string]map[string]any, len(run.Steps))
	for _, st := range run.Steps {
		if st.Status == model.WorkflowStepRunStatusCompleted && st.Output != nil {
			stepOutputs[st.StepID] = cloneWorkflowPayload(st.Output)
		}
	}

	for i := parkedIdx + 1; i < len(run.Steps); i++ {
		if _, execErr := s.executeStep(ctx, run, record, i, maxRetries, stepOutputs); execErr != nil {
			return execErr
		}
		if _, parked, parkErr := s.handleParkedStep(ctx, run, i); parked {
			return parkErr
		}
	}

	_, err = s.completeRun(ctx, run)
	return err
}

// CancelRun cancels a plugin run identified by id. Satisfies PluginRunResumer
// so the DAG service's cross-engine cancellation cascade can stop a plugin
// parent when its invoking DAG ancestor is cancelled.
func (s *WorkflowExecutionService) CancelRun(ctx context.Context, runID uuid.UUID) error {
	if s.runs == nil {
		return fmt.Errorf("workflow run store is not configured")
	}
	run, err := s.runs.GetByID(ctx, runID)
	if err != nil {
		return err
	}
	if !isWorkflowRunActive(run.Status) {
		return nil
	}
	// Cancellation cascade: if a `workflow` step is parked on a DAG child,
	// cancel that child first so its resources release before the parent
	// transitions. Failures are logged but do not block the parent's cancel.
	s.cancelParkedDAGChildren(ctx, run)
	if _, err := s.cancelRun(ctx, run, fmt.Errorf("workflow plugin run cancelled")); err != nil {
		// cancelRun returns the cancel cause as its error on success; surface
		// only store/persistence errors as a hard failure from CancelRun.
		if err.Error() != "workflow plugin run cancelled" {
			return err
		}
	}
	return nil
}

// cancelParkedDAGChildren walks the run's parked `workflow` steps and calls
// the step router's DAG cancel seam (when wired) for each. Idempotent: a
// child that has already reached a terminal state is left to the engine's
// own handling.
func (s *WorkflowExecutionService) cancelParkedDAGChildren(ctx context.Context, run *model.WorkflowPluginRun) {
	if run == nil {
		return
	}
	starter := s.resolveDAGChildStarter()
	if starter == nil {
		return
	}
	for _, step := range run.Steps {
		if step.Status != model.WorkflowStepRunStatusAwaitingSubWorkflow || step.Output == nil {
			continue
		}
		engine, _ := step.Output["child_engine"].(string)
		if engine != "dag" {
			continue
		}
		raw, _ := step.Output["child_run_id"].(string)
		if raw == "" {
			continue
		}
		childID, err := uuid.Parse(raw)
		if err != nil {
			continue
		}
		if err := starter.Cancel(ctx, childID); err != nil {
			log.Printf("[WARN] cancel parked DAG child %s for plugin run %s: %v", childID, run.ID, err)
		}
	}
}

// resolveDAGChildStarter returns the DAG child starter wired into the step
// executor (if any). Kept as a method on the service so tests can stub it by
// swapping the executor.
func (s *WorkflowExecutionService) resolveDAGChildStarter() WorkflowDAGChildStarter {
	if s.executor == nil {
		return nil
	}
	if provider, ok := s.executor.(interface {
		DAGChildStarter() WorkflowDAGChildStarter
	}); ok {
		return provider.DAGChildStarter()
	}
	return nil
}

// findParkedDAGChildStep scans run.Steps for the single `workflow` step parked
// on the given DAG child run id. Returns -1 if no parked step references it.
func findParkedDAGChildStep(run *model.WorkflowPluginRun, childRunID uuid.UUID) int {
	if run == nil {
		return -1
	}
	target := childRunID.String()
	for i, step := range run.Steps {
		if step.Status != model.WorkflowStepRunStatusAwaitingSubWorkflow {
			continue
		}
		if step.Output == nil {
			continue
		}
		raw, _ := step.Output["child_run_id"].(string)
		if raw == target {
			return i
		}
	}
	return -1
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
