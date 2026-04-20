package nodetypes

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"

	"github.com/react-go-quick-starter/server/internal/model"
)

// ── Qianchuan applier interfaces (Spec 3D) ──────────────────────────────

// QianchuanProvider is the subset of adsplatform.Provider used by the
// metrics_fetcher and action_executor appliers. The qianchuan package's
// *Provider satisfies this structurally.
type QianchuanProvider interface {
	FetchMetrics(ctx context.Context, bindingRef QianchuanBindingRef, dims []string) (json.RawMessage, time.Time, error)
	ApplyAction(ctx context.Context, bindingRef QianchuanBindingRef, action QianchuanActionRequest) error
}

// QianchuanBindingRef carries the per-call binding context for the provider.
type QianchuanBindingRef struct {
	AdvertiserID string
	AwemeID      string
	AccessToken  string
}

// QianchuanActionRequest carries a single action to apply.
type QianchuanActionRequest struct {
	Kind       string          `json:"kind"`
	TargetAdID string          `json:"target_ad_id"`
	Params     json.RawMessage `json:"params"`
}

// QianchuanSecretsResolver is the subset of secrets.Service the appliers call.
type QianchuanSecretsResolver interface {
	Resolve(ctx context.Context, projectID uuid.UUID, name string) (string, error)
}

// QianchuanSnapshotRepo is the snapshot persistence contract for the applier.
type QianchuanSnapshotRepo interface {
	Upsert(ctx context.Context, bindingID uuid.UUID, bucket time.Time, payload json.RawMessage) error
}

// QianchuanActionLogRepo is the action log persistence contract for the applier.
type QianchuanActionLogRepo interface {
	Create(ctx context.Context, log *model.QianchuanActionLog) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.QianchuanActionLog, error)
	MarkApplied(ctx context.Context, id uuid.UUID) error
	MarkFailed(ctx context.Context, id uuid.UUID, msg string) error
}

// QianchuanStrategyLoader loads the parsed spec for a strategy by ID.
type QianchuanStrategyLoader interface {
	Load(ctx context.Context, id uuid.UUID) (parsedSpec json.RawMessage, err error)
}

// QianchuanStrategyEvaluator evaluates strategy rules against a snapshot.
type QianchuanStrategyEvaluator interface {
	Evaluate(ctx context.Context, parsedSpec, snapshot json.RawMessage) ([]QianchuanRuleMatch, error)
}

// QianchuanRuleMatch is the output of evaluating a single rule.
type QianchuanRuleMatch struct {
	RuleName string                   `json:"rule_name"`
	Actions  []QianchuanEmittedAction `json:"actions"`
}

// QianchuanEmittedAction is a single action emitted by a rule evaluation.
type QianchuanEmittedAction struct {
	Type   string          `json:"type"`
	Target string          `json:"target"`
	Params json.RawMessage `json:"params"`
}

// QianchuanBindingLookup loads a binding record by ID.
type QianchuanBindingLookup interface {
	Get(ctx context.Context, id uuid.UUID) (*QianchuanBindingRecord, error)
}

// QianchuanBindingRecord is the minimal binding data the applier needs.
type QianchuanBindingRecord struct {
	ID                   uuid.UUID
	ProjectID            uuid.UUID
	AdvertiserID         string
	AwemeID              string
	AccessTokenSecretRef string
}

// ── Local interfaces (Go convention: keep import tree clean) ────────────

// BroadcastHub sends WebSocket events to project channels.
type BroadcastHub interface {
	BroadcastEvent(eventType, projectID string, payload map[string]any)
}

// TaskTransitioner updates task status.
type TaskTransitioner interface {
	TransitionStatus(ctx context.Context, id uuid.UUID, newStatus string) error
}

// NodeExecDeleter removes node execution records (for loops).
type NodeExecDeleter interface {
	DeleteNodeExecutionsByNodeIDs(ctx context.Context, execID uuid.UUID, ids []string) error
}

// ExecutionDataStoreWriter reads/writes execution-level DataStore (for loop counters).
type ExecutionDataStoreWriter interface {
	UpdateExecutionDataStore(ctx context.Context, id uuid.UUID, dataStore json.RawMessage) error
	GetExecution(ctx context.Context, id uuid.UUID) (*model.WorkflowExecution, error)
}

// AgentSpawner dispatches agent runs from workflow nodes.
type AgentSpawner interface {
	Spawn(ctx context.Context, taskID, memberID uuid.UUID, runtime, provider, modelName string, budgetUsd float64, roleID string) (*model.AgentRun, error)
}

// EmployeeSpawner dispatches agent runs on behalf of persistent Employees.
// When a spawn_agent effect carries a non-empty EmployeeID, the applier
// prefers this seam over the raw AgentSpawner so the resulting agent_run
// row's employee_id and all employee-specific setup (skills, runtime prefs,
// system prompt override) are resolved inside the Employee service.
type EmployeeSpawner interface {
	Invoke(ctx context.Context, in EmployeeInvokeInput) (*EmployeeInvokeResult, error)
}

// EmployeeInvokeInput carries parameters for the EmployeeSpawner.Invoke call.
type EmployeeInvokeInput struct {
	EmployeeID  uuid.UUID
	TaskID      uuid.UUID
	ExecutionID uuid.UUID
	NodeID      string
	BudgetUsd   float64
}

// EmployeeInvokeResult is returned by a successful EmployeeSpawner.Invoke call.
type EmployeeInvokeResult struct {
	AgentRunID uuid.UUID
}

// RunMappingRepo persists workflow-to-agent-run mappings so async agent completion
// callbacks can resume the originating node.
type RunMappingRepo interface {
	Create(ctx context.Context, mapping *model.WorkflowRunMapping) error
}

// ReviewRepo persists human review requests.
type ReviewRepo interface {
	Create(ctx context.Context, review *model.WorkflowPendingReview) error
}

// SecretResolver renders {{secrets.X}} templates on http_call config strings.
type SecretResolver interface {
	Resolve(ctx context.Context, projectID uuid.UUID, name string) (string, error)
}

// IMSendDispatcher posts the rendered card to IM Bridge /im/send.
type IMSendDispatcher interface {
	Send(ctx context.Context, replyTarget map[string]any, card json.RawMessage) (messageID string, err error)
}

// CorrelationsCreator mints card_action_correlations rows.
type CorrelationsCreator interface {
	Create(ctx context.Context, in *CorrelationCreateInput) (uuid.UUID, error)
}

// CorrelationCreateInput carries the data for creating a correlation row.
type CorrelationCreateInput struct {
	ExecutionID uuid.UUID
	NodeID      string
	ActionID    string
	Payload     map[string]any
	ExpiresAt   time.Time
}

// ExecutionMetaWriter merges keys into system_metadata.
type ExecutionMetaWriter interface {
	MergeSystemMetadata(ctx context.Context, executionID uuid.UUID, patch map[string]any) error
}

// AuditRecorder records audit events.
type AuditRecorder interface {
	Record(ctx context.Context, kind string, payload map[string]any) error
}

// ── EffectApplier ───────────────────────────────────────────────────────

// EffectApplier executes the structured effects returned by node handlers.
// Uses exported fields for easy test construction via composite literal.
type EffectApplier struct {
	Hub      BroadcastHub
	TaskRepo TaskTransitioner
	NodeRepo NodeExecDeleter
	ExecRepo ExecutionDataStoreWriter
	// Park-effect deps (Task 4)
	AgentSpawner    AgentSpawner
	EmployeeSpawner EmployeeSpawner
	MappingRepo     RunMappingRepo
	ReviewRepo      ReviewRepo
	// Sub-workflow invocation deps. SubWorkflowEngines routes an
	// EffectInvokeSubWorkflow to the correct child-runtime adapter;
	// SubWorkflowLinks persists parent↔child linkage rows;
	// SubWorkflowGuard rejects cycles and depth violations before dispatch.
	// All three MUST be wired for sub_workflow nodes to execute — if any is
	// nil the applier surfaces a structured "not configured" error at
	// dispatch time rather than parking silently.
	SubWorkflowEngines *SubWorkflowEngineRegistry
	SubWorkflowLinks   SubWorkflowLinkRepo
	SubWorkflowGuard   *RecursionGuard

	// SecretResolver renders {{secrets.X}} templates on http_call config
	// strings. Wired in production by 1B's *secrets.Service. Nil disables
	// http_call (applier returns a structured error at dispatch time).
	SecretResolver SecretResolver

	// DataStoreMerger writes node results into the parent execution's
	// dataStore. Reuses WaitEventDataStoreAdapter so the http_call applier
	// and the wait_event resumer share one merge implementation.
	DataStoreMerger WaitEventDataStoreMerger

	// IMSendDispatcher posts the rendered card to IM Bridge /im/send.
	// Wired by the IM Bridge HTTP client at startup. Nil disables im_send.
	IMSendDispatcher IMSendDispatcher

	// CorrelationsCreator mints card_action_correlations rows for each
	// callback action on an im_send card.
	CorrelationsCreator CorrelationsCreator

	// ExecutionMetaWriter merges system_metadata.im_dispatched=true after a
	// successful im_send.
	ExecutionMetaWriter ExecutionMetaWriter

	// AuditSink records applier-level audit events (e.g. http_call executions).
	AuditSink AuditRecorder

	// Qianchuan is the plugin-provided hook bundle for Spec 3D effects
	// (FetchQianchuanMetrics, RunQianchuanStrategy, ExecuteQianchuanAction).
	// Nil when the qianchuan plugin is disabled; Apply() returns
	// "qianchuan: not configured" for those effect kinds in that case.
	// The plugin constructs EffectHooks with all of the QianchuanProvider
	// / QianchuanSecrets / QianchuanSnapshots / etc. dependencies wired
	// in and assigns it onto EffectApplier from its Install() function.
	Qianchuan QianchuanEffectHooks
}

// QianchuanEffectHooks is the plugin-provided contract the EffectApplier
// dispatches Spec 3D qianchuan effects through. Implementations live in
// plugins/qianchuan-ads/workflow; leaving this field nil cleanly
// disables the three effect kinds.
type QianchuanEffectHooks interface {
	FetchMetrics(ctx context.Context, merger WaitEventDataStoreMerger, exec *model.WorkflowExecution, node *model.WorkflowNode, raw json.RawMessage) error
	RunStrategy(ctx context.Context, merger WaitEventDataStoreMerger, exec *model.WorkflowExecution, node *model.WorkflowNode, raw json.RawMessage) error
	ExecuteAction(ctx context.Context, merger WaitEventDataStoreMerger, exec *model.WorkflowExecution, node *model.WorkflowNode, raw json.RawMessage) error
}

// Apply iterates effects in order and executes each one.
// Returns parked=true iff a park effect was successfully applied (not in this task).
func (a *EffectApplier) Apply(
	ctx context.Context,
	exec *model.WorkflowExecution,
	nodeExecID uuid.UUID,
	node *model.WorkflowNode,
	effects []Effect,
) (parked bool, err error) {
	for _, e := range effects {
		switch e.Kind {
		case EffectBroadcastEvent:
			if err := a.applyBroadcast(exec, e.Payload); err != nil {
				return false, fmt.Errorf("broadcast_event: %w", err)
			}

		case EffectUpdateTaskStatus:
			if err := a.applyUpdateTaskStatus(ctx, exec, e.Payload); err != nil {
				return false, fmt.Errorf("update_task_status: %w", err)
			}

		case EffectResetNodes:
			if err := a.applyResetNodes(ctx, exec, e.Payload); err != nil {
				return false, fmt.Errorf("reset_nodes: %w", err)
			}

		case EffectSpawnAgent:
			if err := a.applySpawnAgent(ctx, exec, node, e.Payload); err != nil {
				return false, fmt.Errorf("spawn_agent: %w", err)
			}
			return true, nil

		case EffectRequestReview:
			if err := a.applyRequestReview(ctx, exec, node, e.Payload); err != nil {
				return false, fmt.Errorf("request_review: %w", err)
			}
			return true, nil

		case EffectWaitEvent:
			if err := a.applyWaitEvent(exec, node, nodeExecID, e.Payload); err != nil {
				return false, fmt.Errorf("wait_event: %w", err)
			}
			return true, nil

		case EffectInvokeSubWorkflow:
			if err := a.applyInvokeSubWorkflow(ctx, exec, node, e.Payload); err != nil {
				return false, fmt.Errorf("invoke_sub_workflow: %w", err)
			}
			return true, nil

		case EffectExecuteHTTPCall:
			if err := a.applyExecuteHTTPCall(ctx, exec, node, e.Payload); err != nil {
				return false, fmt.Errorf("execute_http_call: %w", err)
			}

		case EffectExecuteIMSend:
			if err := a.applyExecuteIMSend(ctx, exec, node, e.Payload); err != nil {
				return false, fmt.Errorf("execute_im_send: %w", err)
			}

		case EffectFetchQianchuanMetrics:
			if a.Qianchuan == nil {
				return false, fmt.Errorf("fetch_qianchuan_metrics: qianchuan: not configured")
			}
			if err := a.Qianchuan.FetchMetrics(ctx, a.DataStoreMerger, exec, node, e.Payload); err != nil {
				return false, fmt.Errorf("fetch_qianchuan_metrics: %w", err)
			}

		case EffectRunQianchuanStrategy:
			if a.Qianchuan == nil {
				return false, fmt.Errorf("run_qianchuan_strategy: qianchuan: not configured")
			}
			if err := a.Qianchuan.RunStrategy(ctx, a.DataStoreMerger, exec, node, e.Payload); err != nil {
				return false, fmt.Errorf("run_qianchuan_strategy: %w", err)
			}

		case EffectExecuteQianchuanAction:
			if a.Qianchuan == nil {
				return false, fmt.Errorf("execute_qianchuan_action: qianchuan: not configured")
			}
			if err := a.Qianchuan.ExecuteAction(ctx, a.DataStoreMerger, exec, node, e.Payload); err != nil {
				return false, fmt.Errorf("execute_qianchuan_action: %w", err)
			}

		default:
			return false, fmt.Errorf("unknown effect kind %q", e.Kind)
		}
	}
	return false, nil
}

// ── Fire-and-forget effects ─────────────────────────────────────────────

func (a *EffectApplier) applyBroadcast(exec *model.WorkflowExecution, raw json.RawMessage) error {
	if a.Hub == nil {
		log.Printf("[WARN] EffectApplier: Hub is nil, skipping broadcast_event")
		return nil
	}
	var p BroadcastEventPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}
	a.Hub.BroadcastEvent(p.EventType, exec.ProjectID.String(), p.Payload)
	return nil
}

func (a *EffectApplier) applyUpdateTaskStatus(ctx context.Context, exec *model.WorkflowExecution, raw json.RawMessage) error {
	if a.TaskRepo == nil {
		return fmt.Errorf("TaskRepo is nil")
	}
	if exec.TaskID == nil {
		return fmt.Errorf("execution has no TaskID")
	}
	var p UpdateTaskStatusPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}
	return a.TaskRepo.TransitionStatus(ctx, *exec.TaskID, p.TargetStatus)
}

func (a *EffectApplier) applyResetNodes(ctx context.Context, exec *model.WorkflowExecution, raw json.RawMessage) error {
	if a.NodeRepo == nil {
		return fmt.Errorf("NodeRepo is nil")
	}
	var p ResetNodesPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	if err := a.NodeRepo.DeleteNodeExecutionsByNodeIDs(ctx, exec.ID, p.NodeIDs); err != nil {
		return err
	}

	// If a counter key is specified and ExecRepo is available, update the DataStore.
	if p.CounterKey != "" && a.ExecRepo != nil {
		current, err := a.ExecRepo.GetExecution(ctx, exec.ID)
		if err != nil {
			return fmt.Errorf("get execution for counter update: %w", err)
		}

		ds := make(map[string]any)
		if len(current.DataStore) > 0 {
			if err := json.Unmarshal(current.DataStore, &ds); err != nil {
				return fmt.Errorf("unmarshal datastore: %w", err)
			}
		}
		ds[p.CounterKey] = p.CounterValue

		updated, err := json.Marshal(ds)
		if err != nil {
			return fmt.Errorf("marshal datastore: %w", err)
		}
		if err := a.ExecRepo.UpdateExecutionDataStore(ctx, exec.ID, updated); err != nil {
			return fmt.Errorf("update datastore: %w", err)
		}
	}

	return nil
}

// ── Park effects ────────────────────────────────────────────────────────
//
// Invariant: the applier records intent only — it MUST NOT mutate node
// execution state. The caller (DAG service) inspects parked=true and flips
// NodeExec.Status / WorkflowExecution.Status appropriately.

func (a *EffectApplier) applySpawnAgent(ctx context.Context, exec *model.WorkflowExecution, node *model.WorkflowNode, raw json.RawMessage) error {
	if a.AgentSpawner == nil {
		return fmt.Errorf("AgentSpawner is nil")
	}
	if exec.TaskID == nil {
		return fmt.Errorf("execution has no TaskID")
	}
	var p SpawnAgentPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	// Employee-backed spawn: route through EmployeeSpawner when EmployeeID is set.
	if p.EmployeeID != "" {
		if a.EmployeeSpawner == nil {
			return fmt.Errorf("EmployeeSpawner is nil but spawn payload carries employeeId")
		}
		empID, err := uuid.Parse(p.EmployeeID)
		if err != nil {
			return fmt.Errorf("invalid employeeId %q: %w", p.EmployeeID, err)
		}
		res, invokeErr := a.EmployeeSpawner.Invoke(ctx, EmployeeInvokeInput{
			EmployeeID:  empID,
			TaskID:      *exec.TaskID,
			ExecutionID: exec.ID,
			NodeID:      node.ID,
			BudgetUsd:   p.BudgetUsd,
		})
		if invokeErr != nil {
			return fmt.Errorf("employee invoke: %w", invokeErr)
		}
		// Persist the mapping so the node can be awoken when the run finishes.
		if a.MappingRepo != nil {
			if err := a.MappingRepo.Create(ctx, &model.WorkflowRunMapping{
				ID:          uuid.New(),
				ExecutionID: exec.ID,
				NodeID:      node.ID,
				AgentRunID:  res.AgentRunID,
			}); err != nil {
				// Mirror existing behavior: warn but don't fail the spawn.
				log.Printf("[WARN] EffectApplier: failed to create run mapping for node %s: %v", node.ID, err)
			}
		}
		return nil
	}

	memberID := uuid.Nil
	if p.MemberID != "" {
		if parsed, err := uuid.Parse(p.MemberID); err == nil {
			memberID = parsed
		} else {
			log.Printf("[WARN] EffectApplier: invalid memberId %q in spawn_agent: %v", p.MemberID, err)
		}
	}

	run, err := a.AgentSpawner.Spawn(ctx, *exec.TaskID, memberID, p.Runtime, p.Provider, p.Model, p.BudgetUsd, p.RoleID)
	if err != nil {
		return fmt.Errorf("spawn: %w", err)
	}

	// Register mapping so HandleAgentRunCompletion can resume this node.
	if a.MappingRepo == nil {
		log.Printf("[WARN] EffectApplier: MappingRepo is nil, skipping run mapping for node %s (async resume will not work)", node.ID)
		return nil
	}
	mapping := &model.WorkflowRunMapping{
		ID:          uuid.New(),
		ExecutionID: exec.ID,
		NodeID:      node.ID,
		AgentRunID:  run.ID,
	}
	if err := a.MappingRepo.Create(ctx, mapping); err != nil {
		// Match service-layer semantics: warn-log, do not fail the node — the
		// spawn already happened and will run; we just lose the async resume link.
		log.Printf("[WARN] EffectApplier: failed to create run mapping for node %s: %v", node.ID, err)
	}
	return nil
}

func (a *EffectApplier) applyRequestReview(ctx context.Context, exec *model.WorkflowExecution, node *model.WorkflowNode, raw json.RawMessage) error {
	var p RequestReviewPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	if a.ReviewRepo == nil {
		log.Printf("[WARN] EffectApplier: ReviewRepo is nil, skipping review persistence for node %s", node.ID)
		return nil
	}

	review := &model.WorkflowPendingReview{
		ID:          uuid.New(),
		ExecutionID: exec.ID,
		NodeID:      node.ID,
		ProjectID:   exec.ProjectID,
		Prompt:      p.Prompt,
		Context:     p.Context,
		Decision:    model.ReviewDecisionPending,
	}
	if err := a.ReviewRepo.Create(ctx, review); err != nil {
		// Match service-layer semantics: warn-log, do not fail the node.
		log.Printf("[WARN] EffectApplier: failed to persist pending review for node %s: %v", node.ID, err)
	}
	return nil
}

func (a *EffectApplier) applyWaitEvent(exec *model.WorkflowExecution, node *model.WorkflowNode, nodeExecID uuid.UUID, raw json.RawMessage) error {
	var p WaitEventPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	if a.Hub == nil {
		log.Printf("[WARN] EffectApplier: Hub is nil, skipping wait_event broadcast for node %s", node.ID)
		return nil
	}

	a.Hub.BroadcastEvent("workflow.node.waiting", exec.ProjectID.String(), map[string]any{
		"executionId": exec.ID.String(),
		"nodeId":      node.ID,
		"nodeExecId":  nodeExecID.String(),
		"eventType":   p.EventType,
		"matchKey":    p.MatchKey,
	})
	return nil
}

func (a *EffectApplier) applyInvokeSubWorkflow(ctx context.Context, exec *model.WorkflowExecution, node *model.WorkflowNode, raw json.RawMessage) error {
	var p InvokeSubWorkflowPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	target := p.TargetWorkflowID
	if target == "" {
		target = p.WorkflowID
	}
	if target == "" {
		return &SubWorkflowInvocationError{
			Reason:  SubWorkflowRejectUnknownTarget,
			Message: "payload missing targetWorkflowId",
		}
	}

	kind := p.TargetKind
	if kind == "" {
		kind = SubWorkflowTargetDAG
	}

	if a.SubWorkflowEngines == nil {
		return fmt.Errorf("sub_workflow: engine registry is not configured")
	}
	engine, ok := a.SubWorkflowEngines.Get(kind)
	if !ok {
		return &SubWorkflowInvocationError{
			Reason:  SubWorkflowRejectUnknownTarget,
			Message: fmt.Sprintf("no engine registered for target kind %q", kind),
		}
	}

	// Pre-dispatch validation (unknown target + cross-project rejection).
	// Engines return SubWorkflowInvocationError so the caller can classify
	// the failure without parsing error strings.
	invForValidate := SubWorkflowInvocation{
		ParentExecutionID: exec.ID,
		ParentNodeID:      node.ID,
		ProjectID:         exec.ProjectID,
		ActingEmployeeID:  exec.ActingEmployeeID,
	}
	if err := engine.Validate(ctx, target, invForValidate); err != nil {
		return err
	}

	// Recursion guard: only walk for DAG targets (plugin children cannot host
	// sub_workflow nodes, so no cyclic chain is possible through them).
	if kind == SubWorkflowTargetDAG && a.SubWorkflowGuard != nil {
		if err := a.SubWorkflowGuard.Check(ctx, exec.ID, target); err != nil {
			return err
		}
	}

	// Render input mapping against parent context. Unresolvable references
	// surface as structured errors so callers can classify them.
	parentDataStore := map[string]any{}
	if len(exec.DataStore) > 0 {
		_ = json.Unmarshal(exec.DataStore, &parentDataStore)
	}
	parentContext := map[string]any{}
	if len(exec.Context) > 0 {
		_ = json.Unmarshal(exec.Context, &parentContext)
	}
	seed, err := renderSubWorkflowMapping(p.InputMapping, parentDataStore, parentContext)
	if err != nil {
		return err
	}
	// Augment the seed with parent attribution so the child can correlate.
	seed["$parent"] = map[string]any{
		"executionId": exec.ID.String(),
		"nodeId":      node.ID,
		"projectId":   exec.ProjectID.String(),
	}

	// Start the child run through the engine adapter.
	childRunID, err := engine.Start(ctx, target, seed, invForValidate)
	if err != nil {
		return fmt.Errorf("sub_workflow: engine start: %w", err)
	}

	// Persist the parent-link row so the terminal-state hook on either engine
	// can resume this parent when the child completes. ParentKind is stamped
	// "dag_execution" here because the applier is only invoked for DAG parent
	// nodes; plugin-run parents insert their own rows via the legacy step
	// router (bridge-legacy-to-dag-invocation).
	if a.SubWorkflowLinks != nil {
		if err := a.SubWorkflowLinks.Create(ctx, &SubWorkflowLinkRecord{
			ID:                uuid.New(),
			ParentExecutionID: exec.ID,
			ParentKind:        "dag_execution",
			ParentNodeID:      node.ID,
			ChildEngineKind:   string(kind),
			ChildRunID:        childRunID,
			Status:            "running",
		}); err != nil {
			// Soft-fail parity with spawn_agent mapping: the child is already
			// started; log but don't fail the node. Loss of the link means
			// the parent won't auto-resume, which operators can reconcile by
			// manually completing the node.
			log.Printf("[WARN] EffectApplier: failed to persist parent link for node %s (child %s): %v", node.ID, childRunID, err)
		}
	} else {
		log.Printf("[WARN] EffectApplier: SubWorkflowLinks is nil, skipping parent link for node %s (async resume will not work)", node.ID)
	}

	// Broadcast a sub_workflow.started hint so the frontend can stitch parent
	// ↔ child into the execution detail view.
	if a.Hub != nil {
		a.Hub.BroadcastEvent("workflow.sub_workflow.started", exec.ProjectID.String(), map[string]any{
			"executionId": exec.ID.String(),
			"nodeId":      node.ID,
			"childEngine": string(kind),
			"childRunId":  childRunID.String(),
		})
	}
	return nil
}
