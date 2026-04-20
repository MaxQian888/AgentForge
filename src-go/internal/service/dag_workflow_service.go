package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	eventbus "github.com/agentforge/server/internal/eventbus"
	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/workflow/nodetypes"
	"github.com/agentforge/server/internal/ws"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

// DAGWorkflowDefinitionRepo defines the definition persistence interface.
type DAGWorkflowDefinitionRepo interface {
	Create(ctx context.Context, def *model.WorkflowDefinition) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.WorkflowDefinition, error)
	ListByProject(ctx context.Context, projectID uuid.UUID) ([]*model.WorkflowDefinition, error)
	Update(ctx context.Context, id uuid.UUID, def *model.WorkflowDefinition) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// DAGWorkflowExecutionRepo defines the execution persistence interface.
type DAGWorkflowExecutionRepo interface {
	CreateExecution(ctx context.Context, exec *model.WorkflowExecution) error
	GetExecution(ctx context.Context, id uuid.UUID) (*model.WorkflowExecution, error)
	ListExecutions(ctx context.Context, workflowID uuid.UUID) ([]*model.WorkflowExecution, error)
	ListActiveByProject(ctx context.Context, projectID uuid.UUID) ([]*model.WorkflowExecution, error)
	UpdateExecution(ctx context.Context, id uuid.UUID, status string, currentNodes json.RawMessage, errorMessage string) error
	UpdateExecutionDataStore(ctx context.Context, id uuid.UUID, dataStore json.RawMessage) error
	CompleteExecution(ctx context.Context, id uuid.UUID, status string) error
}

// DAGWorkflowNodeExecRepo defines the node execution persistence interface.
type DAGWorkflowNodeExecRepo interface {
	CreateNodeExecution(ctx context.Context, nodeExec *model.WorkflowNodeExecution) error
	UpdateNodeExecution(ctx context.Context, id uuid.UUID, status string, result json.RawMessage, errorMessage string) error
	ListNodeExecutions(ctx context.Context, executionID uuid.UUID) ([]*model.WorkflowNodeExecution, error)
	DeleteNodeExecutionsByNodeIDs(ctx context.Context, executionID uuid.UUID, nodeIDs []string) error
}

// DAGWorkflowTaskRepo defines the task operations needed by DAG workflows.
// Retained because AdvanceExecution still calls EvaluateCondition on edges
// with a task-status resolver.
type DAGWorkflowTaskRepo interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Task, error)
}

// DAGWorkflowRunMappingRepo persists workflow-to-agent-run mappings.
type DAGWorkflowRunMappingRepo interface {
	Create(ctx context.Context, mapping *model.WorkflowRunMapping) error
	GetByAgentRunID(ctx context.Context, agentRunID uuid.UUID) (*model.WorkflowRunMapping, error)
	ListByExecution(ctx context.Context, executionID uuid.UUID) ([]*model.WorkflowRunMapping, error)
}

// DAGWorkflowReviewRepo persists human review requests.
type DAGWorkflowReviewRepo interface {
	Create(ctx context.Context, review *model.WorkflowPendingReview) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.WorkflowPendingReview, error)
	ListPendingByProject(ctx context.Context, projectID uuid.UUID) ([]*model.WorkflowPendingReview, error)
	Resolve(ctx context.Context, id uuid.UUID, decision, comment string) error
	FindPendingByExecutionAndNode(ctx context.Context, executionID uuid.UUID, nodeID string) (*model.WorkflowPendingReview, error)
}

// DAGWorkflowParentLinkRepo persists sub-workflow parent↔child linkage.
// Injected lazily via SetParentLinkRepo; when nil, sub-workflow resume paths
// become no-ops so legacy test wiring continues to build without this dep.
type DAGWorkflowParentLinkRepo interface {
	GetByChild(ctx context.Context, engineKind string, childRunID uuid.UUID) (*model.WorkflowRunParentLink, error)
	ListByParentExecution(ctx context.Context, parentExecutionID uuid.UUID) ([]*model.WorkflowRunParentLink, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status string) error
}

// PluginRunResumer is the narrow contract the DAG engine's terminal-state hook
// calls when a DAG child's parent link has parent_kind='plugin_run'. The
// implementation (WorkflowExecutionService) transitions the parked plugin step
// and continues the plugin run. Kept on the DAG side as an interface so the
// two services stay decoupled — the wiring layer injects the concrete
// runtime at startup.
//
// Introduced by bridge-legacy-to-dag-invocation so a legacy plugin invoking a
// DAG child can resume synchronously with the DAG engine's terminal path.
type PluginRunResumer interface {
	ResumeParkedDAGChild(ctx context.Context, parentRunID, childRunID uuid.UUID, outcome string, childOutputs map[string]any) error
	// CancelRun cancels the plugin run identified by runID. Invoked by the
	// cancellation cascade hook when the plugin parent needs to be stopped
	// because an invoking DAG ancestor was cancelled. A nil implementation is
	// permitted — it means the cascade drops the cancel silently.
	CancelRun(ctx context.Context, runID uuid.UUID) error
}

// DAGWorkflowService orchestrates workflow DAG execution.
type DAGWorkflowService struct {
	defRepo       DAGWorkflowDefinitionRepo
	execRepo      DAGWorkflowExecutionRepo
	nodeRepo      DAGWorkflowNodeExecRepo
	taskRepo      DAGWorkflowTaskRepo
	mappingRepo   DAGWorkflowRunMappingRepo
	reviewRepo    DAGWorkflowReviewRepo
	linkRepo      DAGWorkflowParentLinkRepo
	pluginResumer PluginRunResumer
	hub           *ws.Hub
	bus           eventbus.Publisher
	registry      *nodetypes.NodeTypeRegistry
	applier       *nodetypes.EffectApplier
	projectLookup DispatchProjectStatusLookup
	// runEmitter publishes canonical workflow.run.* events alongside the
	// engine-native workflow.execution.* channel. Nil turns the fan-out off
	// for legacy test wiring; production wiring constructs one in routes.go.
	runEmitter *WorkflowRunEventEmitter
}

// SetRunEmitter wires the unified workflow-run WS fan-out emitter. The
// emitter is consulted inside every broadcastEvent site that represents a
// lifecycle transition; setting it to nil (the default) keeps the
// engine-native channel intact and simply skips the unified emission.
func (s *DAGWorkflowService) SetRunEmitter(e *WorkflowRunEventEmitter) { s.runEmitter = e }

// SetProjectStatusLookup wires the project repository used to reject
// StartExecution on archived projects. Reuses the DispatchProjectStatusLookup
// type to avoid duplicating the narrow interface.
func (s *DAGWorkflowService) SetProjectStatusLookup(lookup DispatchProjectStatusLookup) {
	s.projectLookup = lookup
}

// NewDAGWorkflowService creates a new DAG workflow execution service.
func NewDAGWorkflowService(
	defRepo DAGWorkflowDefinitionRepo,
	execRepo DAGWorkflowExecutionRepo,
	nodeRepo DAGWorkflowNodeExecRepo,
	hub *ws.Hub,
	bus eventbus.Publisher,
	registry *nodetypes.NodeTypeRegistry,
	applier *nodetypes.EffectApplier,
) *DAGWorkflowService {
	return &DAGWorkflowService{
		defRepo:  defRepo,
		execRepo: execRepo,
		nodeRepo: nodeRepo,
		hub:      hub,
		bus:      bus,
		registry: registry,
		applier:  applier,
	}
}

// SetTaskRepo sets the task repository used by AdvanceExecution for edge
// condition evaluation (task.status lookups).
func (s *DAGWorkflowService) SetTaskRepo(r DAGWorkflowTaskRepo) { s.taskRepo = r }

// SetRunMappingRepo sets the run mapping repository for async agent completion.
func (s *DAGWorkflowService) SetRunMappingRepo(r DAGWorkflowRunMappingRepo) { s.mappingRepo = r }

// SetReviewRepo sets the pending review repository for human review nodes.
func (s *DAGWorkflowService) SetReviewRepo(r DAGWorkflowReviewRepo) { s.reviewRepo = r }

// SetParentLinkRepo wires the sub-workflow parent-link repository used by the
// resume path. Must be set before the first sub_workflow invocation can
// complete successfully; nil at construction time is safe.
func (s *DAGWorkflowService) SetParentLinkRepo(r DAGWorkflowParentLinkRepo) { s.linkRepo = r }

// SetPluginRunResumer wires the plugin-runtime resume hook used when a DAG
// child's parent link has parent_kind='plugin_run'. Nil falls back to
// DAG-only behavior, skipping the cross-engine resume (introduced by
// bridge-legacy-to-dag-invocation).
func (s *DAGWorkflowService) SetPluginRunResumer(r PluginRunResumer) { s.pluginResumer = r }

// GetExecutionWorkflowID resolves a parent execution id to its workflow id.
// Satisfies nodetypes.WorkflowExecutionLookup so the sub-workflow recursion
// guard can walk the parent chain without pulling the full execution struct.
func (s *DAGWorkflowService) GetExecutionWorkflowID(ctx context.Context, executionID uuid.UUID) (uuid.UUID, error) {
	if s.execRepo == nil {
		return uuid.Nil, fmt.Errorf("dag workflow service: execution repo is not configured")
	}
	exec, err := s.execRepo.GetExecution(ctx, executionID)
	if err != nil {
		return uuid.Nil, err
	}
	return exec.WorkflowID, nil
}

// ---------------------------------------------------------------------------
// Execution lifecycle
// ---------------------------------------------------------------------------

// StartOptions is the optional-controls struct for StartExecution.
// Seed pre-populates the execution DataStore under the "$event" key so
// the first node advancement can consume external event data (IM payload,
// cron context, webhook body). TriggeredBy stamps the execution with the
// WorkflowTrigger id that fired it. ActingEmployeeID, when non-nil, is the
// run-level acting-employee default; nodes that do not override it attribute
// their spawned agent runs to this employee (see change
// bridge-employee-attribution-legacy).
type StartOptions struct {
	Seed             map[string]any
	TriggeredBy      *uuid.UUID
	ActingEmployeeID *uuid.UUID
}

// buildInitialDataStore constructs the DataStore JSON for a new execution.
// If seed is non-empty it is placed under the reserved "$event" key.
func buildInitialDataStore(seed map[string]any) json.RawMessage {
	if len(seed) == 0 {
		return json.RawMessage("{}")
	}
	seeded := map[string]any{"$event": seed}
	raw, err := json.Marshal(seeded)
	if err != nil {
		log.WithError(err).Warn("workflow start: failed to marshal seed data; proceeding with empty DataStore")
		return json.RawMessage("{}")
	}
	return raw
}

// StartExecution creates a new execution and advances from trigger nodes.
func (s *DAGWorkflowService) StartExecution(ctx context.Context, workflowID uuid.UUID, taskID *uuid.UUID, opts StartOptions) (*model.WorkflowExecution, error) {
	def, err := s.defRepo.GetByID(ctx, workflowID)
	if err != nil {
		return nil, fmt.Errorf("load workflow definition: %w", err)
	}
	if def.Status != model.WorkflowDefStatusActive {
		return nil, fmt.Errorf("workflow %s is not active (status: %s)", workflowID, def.Status)
	}
	if s.projectLookup != nil {
		if project, projErr := s.projectLookup.GetByID(ctx, def.ProjectID); projErr == nil && project != nil {
			if project.Status == model.ProjectStatusArchived {
				return nil, fmt.Errorf("workflow execution rejected: project is archived")
			}
		}
	}

	var nodes []model.WorkflowNode
	var edges []model.WorkflowEdge
	if err := json.Unmarshal(def.Nodes, &nodes); err != nil {
		return nil, fmt.Errorf("parse workflow nodes: %w", err)
	}
	if err := json.Unmarshal(def.Edges, &edges); err != nil {
		return nil, fmt.Errorf("parse workflow edges: %w", err)
	}

	// Find trigger nodes (nodes with no incoming edges or type=trigger)
	incomingSet := make(map[string]bool)
	for _, edge := range edges {
		incomingSet[edge.Target] = true
	}
	var triggerNodeIDs []string
	for _, node := range nodes {
		if node.Type == model.NodeTypeTrigger || !incomingSet[node.ID] {
			triggerNodeIDs = append(triggerNodeIDs, node.ID)
		}
	}
	if len(triggerNodeIDs) == 0 {
		return nil, fmt.Errorf("workflow has no trigger nodes")
	}

	now := time.Now().UTC()
	execContext := json.RawMessage("{}")
	if taskID != nil {
		execContext, _ = json.Marshal(map[string]any{"taskId": taskID.String()})
	}
	currentNodesJSON, _ := json.Marshal(triggerNodeIDs)
	dataStore := buildInitialDataStore(opts.Seed)

	exec := &model.WorkflowExecution{
		ID:           uuid.New(),
		WorkflowID:   workflowID,
		ProjectID:    def.ProjectID,
		TaskID:       taskID,
		Status:       model.WorkflowExecStatusRunning,
		CurrentNodes: currentNodesJSON,
		Context:      execContext,
		DataStore:    dataStore,
		StartedAt:    &now,
	}
	if opts.TriggeredBy != nil {
		exec.TriggeredBy = opts.TriggeredBy
	}
	if opts.ActingEmployeeID != nil {
		exec.ActingEmployeeID = opts.ActingEmployeeID
	}
	if err := s.execRepo.CreateExecution(ctx, exec); err != nil {
		return nil, fmt.Errorf("create execution: %w", err)
	}

	s.broadcastEvent(ctx, ws.EventWorkflowExecutionStarted, def.ProjectID.String(), map[string]any{
		"executionId": exec.ID.String(),
		"workflowId":  workflowID.String(),
	})
	s.runEmitter.EmitDAGStatusChanged(ctx, exec)

	// Mark trigger nodes as completed
	for _, nodeID := range triggerNodeIDs {
		nodeExec := &model.WorkflowNodeExecution{
			ID:          uuid.New(),
			ExecutionID: exec.ID,
			NodeID:      nodeID,
			Status:      model.NodeExecCompleted,
			StartedAt:   &now,
			CompletedAt: &now,
		}
		if err := s.nodeRepo.CreateNodeExecution(ctx, nodeExec); err != nil {
			log.WithError(err).WithField("nodeId", nodeID).Warn("failed to create trigger node execution")
		}
	}

	if err := s.AdvanceExecution(ctx, exec.ID); err != nil {
		log.WithError(err).WithField("executionId", exec.ID).Warn("failed to advance execution after start")
	}

	return s.execRepo.GetExecution(ctx, exec.ID)
}

// AdvanceExecution processes pending nodes and follows edges.
func (s *DAGWorkflowService) AdvanceExecution(ctx context.Context, executionID uuid.UUID) error {
	exec, err := s.execRepo.GetExecution(ctx, executionID)
	if err != nil {
		return fmt.Errorf("load execution: %w", err)
	}
	if exec.Status != model.WorkflowExecStatusRunning {
		return nil
	}

	def, err := s.defRepo.GetByID(ctx, exec.WorkflowID)
	if err != nil {
		return fmt.Errorf("load workflow definition: %w", err)
	}

	var nodes []model.WorkflowNode
	var edges []model.WorkflowEdge
	if err := json.Unmarshal(def.Nodes, &nodes); err != nil {
		return fmt.Errorf("parse nodes: %w", err)
	}
	if err := json.Unmarshal(def.Edges, &edges); err != nil {
		return fmt.Errorf("parse edges: %w", err)
	}

	nodeMap := make(map[string]*model.WorkflowNode, len(nodes))
	for i := range nodes {
		nodeMap[nodes[i].ID] = &nodes[i]
	}

	// Load DataStore for condition evaluation
	dataStore := s.loadDataStore(exec)

	nodeExecs, err := s.nodeRepo.ListNodeExecutions(ctx, executionID)
	if err != nil {
		return fmt.Errorf("list node executions: %w", err)
	}
	completedNodes := make(map[string]bool)
	runningNodes := make(map[string]bool)
	waitingNodes := make(map[string]bool)
	for _, ne := range nodeExecs {
		switch ne.Status {
		case model.NodeExecCompleted:
			completedNodes[ne.NodeID] = true
		case model.NodeExecRunning:
			runningNodes[ne.NodeID] = true
		case model.NodeExecWaiting, model.NodeExecAwaitingSubWorkflow:
			// Sub-workflow parks use a distinct status so operators can
			// distinguish them from human-review parks, but for the purposes of
			// the advance loop both are "the node is holding and must not be
			// re-dispatched" — treat them identically.
			waitingNodes[ne.NodeID] = true
		}
	}

	// Find next nodes: all predecessors completed and edge conditions met
	var nextNodeIDs []string
	for _, node := range nodes {
		if completedNodes[node.ID] || runningNodes[node.ID] || waitingNodes[node.ID] {
			continue
		}
		allPredCompleted := true
		hasPredecessors := false
		for _, edge := range edges {
			if edge.Target == node.ID {
				hasPredecessors = true
				if !completedNodes[edge.Source] {
					allPredCompleted = false
					break
				}
				if edge.Condition != "" && !nodetypes.EvaluateCondition(ctx, exec, edge.Condition, dataStore, s.taskRepo) {
					allPredCompleted = false
					break
				}
			}
		}
		if hasPredecessors && allPredCompleted {
			nextNodeIDs = append(nextNodeIDs, node.ID)
		}
	}

	if len(nextNodeIDs) == 0 {
		if len(runningNodes) > 0 || len(waitingNodes) > 0 {
			return nil // Wait for async/waiting nodes
		}
		if err := s.execRepo.CompleteExecution(ctx, executionID, model.WorkflowExecStatusCompleted); err != nil {
			return fmt.Errorf("complete execution: %w", err)
		}
		s.broadcastEvent(ctx, ws.EventWorkflowExecutionCompleted, exec.ProjectID.String(), map[string]any{
			"executionId": exec.ID.String(),
			"workflowId":  exec.WorkflowID.String(),
			"status":      model.WorkflowExecStatusCompleted,
		})
		if terminated, terminatedErr := s.execRepo.GetExecution(ctx, executionID); terminatedErr == nil {
			s.runEmitter.EmitDAGTerminal(ctx, terminated)
		} else {
			s.runEmitter.EmitDAGTerminal(ctx, exec)
		}
		// If this DAG execution is a child of a parked sub_workflow node,
		// resume the parent with the child's outputs.
		s.tryResumeParentFromDAGChild(ctx, exec, model.SubWorkflowLinkStatusCompleted)
		return nil
	}

	currentNodesJSON, _ := json.Marshal(nextNodeIDs)
	if err := s.execRepo.UpdateExecution(ctx, executionID, model.WorkflowExecStatusRunning, currentNodesJSON, ""); err != nil {
		return fmt.Errorf("update current nodes: %w", err)
	}

	s.broadcastEvent(ctx, ws.EventWorkflowExecutionAdvanced, exec.ProjectID.String(), map[string]any{
		"executionId":  exec.ID.String(),
		"currentNodes": nextNodeIDs,
	})
	s.runEmitter.EmitDAGStatusChanged(ctx, exec)

	for _, nodeID := range nextNodeIDs {
		node := nodeMap[nodeID]
		if node == nil {
			continue
		}
		if err := s.executeNode(ctx, exec, node, dataStore); err != nil {
			log.WithError(err).WithField("nodeId", nodeID).Warn("workflow node execution failed")
			if updateErr := s.execRepo.UpdateExecution(ctx, executionID, model.WorkflowExecStatusFailed, currentNodesJSON, err.Error()); updateErr != nil {
				log.WithError(updateErr).Warn("failed to update execution status to failed")
			}
			// Child failure must also unblock any parent that is parked.
			s.tryResumeParentFromDAGChild(ctx, exec, model.SubWorkflowLinkStatusFailed)
			return err
		}
	}

	return s.AdvanceExecution(ctx, executionID)
}

// CancelAllActiveForProject cancels every non-terminal workflow execution in
// the project. Used by the project lifecycle service as an archive cascade.
// Best-effort: individual failures are logged; the method returns nil unless
// the underlying list call itself fails.
func (s *DAGWorkflowService) CancelAllActiveForProject(ctx context.Context, projectID uuid.UUID, reason string) error {
	if s.execRepo == nil {
		return nil
	}
	execs, err := s.execRepo.ListActiveByProject(ctx, projectID)
	if err != nil {
		return fmt.Errorf("list active workflow executions for cascade cancel: %w", err)
	}
	if reason == "" {
		reason = "project_archived"
	}
	for _, exec := range execs {
		if exec == nil {
			continue
		}
		if err := s.CancelExecution(ctx, exec.ID); err != nil {
			log.WithError(err).WithFields(log.Fields{
				"projectId":   projectID.String(),
				"executionId": exec.ID.String(),
				"reason":      reason,
			}).Warn("dag workflow service: cascade cancel for archived project failed (best-effort)")
		}
	}
	return nil
}

// CancelExecution cancels a running execution.
func (s *DAGWorkflowService) CancelExecution(ctx context.Context, executionID uuid.UUID) error {
	exec, err := s.execRepo.GetExecution(ctx, executionID)
	if err != nil {
		return fmt.Errorf("load execution: %w", err)
	}
	if exec.Status != model.WorkflowExecStatusRunning && exec.Status != model.WorkflowExecStatusPending && exec.Status != model.WorkflowExecStatusPaused {
		return fmt.Errorf("cannot cancel execution in status: %s", exec.Status)
	}
	if err := s.execRepo.CompleteExecution(ctx, executionID, model.WorkflowExecStatusCancelled); err != nil {
		return fmt.Errorf("cancel execution: %w", err)
	}
	s.broadcastEvent(ctx, ws.EventWorkflowExecutionCompleted, exec.ProjectID.String(), map[string]any{
		"executionId": exec.ID.String(),
		"workflowId":  exec.WorkflowID.String(),
		"status":      model.WorkflowExecStatusCancelled,
	})
	if cancelled, cerr := s.execRepo.GetExecution(ctx, exec.ID); cerr == nil {
		s.runEmitter.EmitDAGTerminal(ctx, cancelled)
	} else {
		s.runEmitter.EmitDAGTerminal(ctx, exec)
	}
	// Cancellation of a child DAG run must fail-resume the parent node so the
	// parent doesn't hang indefinitely in awaiting_sub_workflow.
	s.tryResumeParentFromDAGChild(ctx, exec, model.SubWorkflowLinkStatusCancelled)
	return nil
}

// HandleAgentRunCompletion is called when an agent run reaches terminal status.
// It maps the run back to the workflow node and advances execution.
func (s *DAGWorkflowService) HandleAgentRunCompletion(ctx context.Context, runID uuid.UUID, result json.RawMessage, status string) error {
	if s.mappingRepo == nil {
		return nil
	}
	mapping, err := s.mappingRepo.GetByAgentRunID(ctx, runID)
	if err != nil {
		return nil // No mapping = not a workflow-spawned run
	}

	nodeExecs, err := s.nodeRepo.ListNodeExecutions(ctx, mapping.ExecutionID)
	if err != nil {
		return fmt.Errorf("list node executions: %w", err)
	}

	// Find the running node execution for this node
	var targetNodeExec *model.WorkflowNodeExecution
	for _, ne := range nodeExecs {
		if ne.NodeID == mapping.NodeID && ne.Status == model.NodeExecRunning {
			targetNodeExec = ne
			break
		}
	}
	if targetNodeExec == nil {
		log.WithFields(log.Fields{
			"runId":       runID.String(),
			"executionId": mapping.ExecutionID.String(),
			"nodeId":      mapping.NodeID,
		}).Warn("workflow: no running node execution found for completed agent run")
		return nil
	}

	// Determine node status from agent status
	nodeStatus := model.NodeExecCompleted
	errorMsg := ""
	if status != model.AgentRunStatusCompleted {
		nodeStatus = model.NodeExecFailed
		errorMsg = fmt.Sprintf("agent run %s terminated with status: %s", runID, status)
	}

	if err := s.nodeRepo.UpdateNodeExecution(ctx, targetNodeExec.ID, nodeStatus, result, errorMsg); err != nil {
		return fmt.Errorf("update node execution: %w", err)
	}

	// Store result in DataStore
	if len(result) > 0 && nodeStatus == model.NodeExecCompleted {
		s.storeNodeResult(ctx, mapping.ExecutionID, mapping.NodeID, result)
	}

	s.broadcastEvent(ctx, ws.EventWorkflowNodeCompleted, "", map[string]any{
		"executionId": mapping.ExecutionID.String(),
		"nodeId":      mapping.NodeID,
		"status":      nodeStatus,
	})

	if nodeStatus == model.NodeExecFailed {
		if err := s.execRepo.UpdateExecution(ctx, mapping.ExecutionID, model.WorkflowExecStatusFailed, nil, fmt.Sprintf("agent run %s failed", runID)); err != nil {
			log.WithError(err).Warn("failed to mark workflow as failed")
		}
		return nil
	}

	return s.AdvanceExecution(ctx, mapping.ExecutionID)
}

// ---------------------------------------------------------------------------
// Node execution — registry + applier dispatch
// ---------------------------------------------------------------------------

func (s *DAGWorkflowService) executeNode(ctx context.Context, exec *model.WorkflowExecution, node *model.WorkflowNode, dataStore map[string]any) error {
	now := time.Now().UTC()
	nodeExec := &model.WorkflowNodeExecution{
		ID:          uuid.New(),
		ExecutionID: exec.ID,
		NodeID:      node.ID,
		Status:      model.NodeExecRunning,
		StartedAt:   &now,
	}
	if err := s.nodeRepo.CreateNodeExecution(ctx, nodeExec); err != nil {
		return fmt.Errorf("create node execution: %w", err)
	}

	// Resolve via two-layer registry (project overlay + global built-ins).
	entry, err := s.registry.Resolve(exec.ProjectID, node.Type)
	if err != nil {
		execErr := fmt.Errorf("unknown node type: %s", node.Type)
		_ = s.nodeRepo.UpdateNodeExecution(ctx, nodeExec.ID, model.NodeExecFailed, nil, execErr.Error())
		return execErr
	}

	config := s.resolveNodeConfig(node, dataStore)

	req := &nodetypes.NodeExecRequest{
		Execution:  exec,
		Node:       node,
		Config:     config,
		DataStore:  cloneDataStore(dataStore), // defensive copy: handlers must be pure
		NodeExecID: nodeExec.ID,
		ProjectID:  exec.ProjectID,
	}

	result, err := entry.Handler.Execute(ctx, req)
	if err != nil {
		_ = s.nodeRepo.UpdateNodeExecution(ctx, nodeExec.ID, model.NodeExecFailed, nil, err.Error())
		return err
	}
	if result == nil {
		result = &nodetypes.NodeExecResult{}
	}

	// Capability enforcement: emitted effects must be declared.
	if bad := firstUndeclaredEffect(entry.DeclaredCaps, result.Effects); bad != "" {
		execErr := fmt.Errorf("node type %q emitted undeclared effect %q", node.Type, bad)
		recordCapabilityViolation(ctx, s.nodeRepo, nodeExec.ID, execErr)
		return execErr
	}

	// At most one park effect per node.
	if result.ParkCount() > 1 {
		execErr := fmt.Errorf("node type %q emitted multiple park effects", node.Type)
		_ = s.nodeRepo.UpdateNodeExecution(ctx, nodeExec.ID, model.NodeExecFailed, nil, execErr.Error())
		return execErr
	}

	parked, err := s.applier.Apply(ctx, exec, nodeExec.ID, node, result.Effects)
	if err != nil {
		_ = s.nodeRepo.UpdateNodeExecution(ctx, nodeExec.ID, model.NodeExecFailed, nil, err.Error())
		return err
	}

	if parked {
		// Determine which kind of park to flip statuses correctly.
		park := firstParkEffect(result.Effects)
		switch park.Kind {
		case nodetypes.EffectRequestReview:
			_ = s.nodeRepo.UpdateNodeExecution(ctx, nodeExec.ID, model.NodeExecWaiting, nil, "")
			_ = s.execRepo.UpdateExecution(ctx, exec.ID, model.WorkflowExecStatusPaused, exec.CurrentNodes, "")
		case nodetypes.EffectWaitEvent:
			_ = s.nodeRepo.UpdateNodeExecution(ctx, nodeExec.ID, model.NodeExecWaiting, nil, "")
		case nodetypes.EffectInvokeSubWorkflow:
			// Parent parks in a distinct awaiting_sub_workflow status so
			// operators can visually distinguish agent-driven parks from
			// sub-workflow-driven parks. The resume path (tryResumeParentFromDAGChild
			// or ResumeParentFromPluginChild) flips the status back.
			_ = s.nodeRepo.UpdateNodeExecution(ctx, nodeExec.ID, model.NodeExecAwaitingSubWorkflow, nil, "")
			_ = s.execRepo.UpdateExecution(ctx, exec.ID, model.WorkflowExecStatusPaused, exec.CurrentNodes, "")
		case nodetypes.EffectSpawnAgent:
			// Stays running — async callback (HandleAgentRunCompletion) will resume.
		}
		return nil
	}

	// Synchronous completion path
	_ = s.nodeRepo.UpdateNodeExecution(ctx, nodeExec.ID, model.NodeExecCompleted, result.Result, "")
	if len(result.Result) > 0 {
		s.storeNodeResult(ctx, exec.ID, node.ID, result.Result)
	}
	s.broadcastEvent(ctx, ws.EventWorkflowNodeCompleted, exec.ProjectID.String(), map[string]any{
		"executionId": exec.ID.String(),
		"nodeId":      node.ID,
		"nodeType":    node.Type,
		"status":      model.NodeExecCompleted,
	})
	return nil
}

// ---------------------------------------------------------------------------
// Park-status / capability helpers
// ---------------------------------------------------------------------------

func cloneDataStore(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func firstUndeclaredEffect(declared map[nodetypes.EffectKind]bool, effects []nodetypes.Effect) nodetypes.EffectKind {
	if declared == nil {
		// Nothing declared = strictly nothing allowed.
		if len(effects) > 0 {
			return effects[0].Kind
		}
		return ""
	}
	for _, e := range effects {
		if !declared[e.Kind] {
			return e.Kind
		}
	}
	return ""
}

func firstParkEffect(effects []nodetypes.Effect) nodetypes.Effect {
	for _, e := range effects {
		switch e.Kind {
		case nodetypes.EffectSpawnAgent, nodetypes.EffectRequestReview,
			nodetypes.EffectWaitEvent, nodetypes.EffectInvokeSubWorkflow:
			return e
		}
	}
	return nodetypes.Effect{}
}

func recordCapabilityViolation(ctx context.Context, nodeRepo DAGWorkflowNodeExecRepo, nodeExecID uuid.UUID, err error) {
	_ = nodeRepo.UpdateNodeExecution(ctx, nodeExecID, model.NodeExecFailed, nil, err.Error())
}

// ResolveHumanReview resumes a paused workflow after human review.
func (s *DAGWorkflowService) ResolveHumanReview(ctx context.Context, executionID uuid.UUID, nodeID string, decision string, comment string) error {
	exec, err := s.execRepo.GetExecution(ctx, executionID)
	if err != nil {
		return fmt.Errorf("load execution: %w", err)
	}
	if exec.Status != model.WorkflowExecStatusPaused {
		return fmt.Errorf("execution is not paused (status: %s)", exec.Status)
	}

	// Find the waiting node execution
	nodeExecs, err := s.nodeRepo.ListNodeExecutions(ctx, executionID)
	if err != nil {
		return fmt.Errorf("list node executions: %w", err)
	}
	var waitingExec *model.WorkflowNodeExecution
	for _, ne := range nodeExecs {
		if ne.NodeID == nodeID && ne.Status == model.NodeExecWaiting {
			waitingExec = ne
			break
		}
	}
	if waitingExec == nil {
		return fmt.Errorf("no waiting node execution found for node %s", nodeID)
	}

	// Resolve the persistent review record
	if s.reviewRepo != nil {
		pendingReview, findErr := s.reviewRepo.FindPendingByExecutionAndNode(ctx, executionID, nodeID)
		if findErr == nil && pendingReview != nil {
			_ = s.reviewRepo.Resolve(ctx, pendingReview.ID, decision, comment)
		}
	}

	// Store decision as result
	result, _ := json.Marshal(map[string]any{
		"decision": decision,
		"comment":  comment,
	})
	_ = s.nodeRepo.UpdateNodeExecution(ctx, waitingExec.ID, model.NodeExecCompleted, result, "")

	// Store in DataStore
	s.storeNodeResult(ctx, executionID, nodeID, result)

	// Resume execution
	_ = s.execRepo.UpdateExecution(ctx, executionID, model.WorkflowExecStatusRunning, nil, "")

	s.broadcastEvent(ctx, ws.EventWorkflowReviewResolved, exec.ProjectID.String(), map[string]any{
		"executionId": executionID.String(),
		"nodeId":      nodeID,
		"decision":    decision,
	})

	return s.AdvanceExecution(ctx, executionID)
}

// HandleExternalEvent resumes a waiting node with external data.
func (s *DAGWorkflowService) HandleExternalEvent(ctx context.Context, executionID uuid.UUID, nodeID string, payload json.RawMessage) error {
	nodeExecs, err := s.nodeRepo.ListNodeExecutions(ctx, executionID)
	if err != nil {
		return fmt.Errorf("list node executions: %w", err)
	}

	var waitingExec *model.WorkflowNodeExecution
	for _, ne := range nodeExecs {
		if ne.NodeID == nodeID && ne.Status == model.NodeExecWaiting {
			waitingExec = ne
			break
		}
	}
	if waitingExec == nil {
		return fmt.Errorf("no waiting node execution found for node %s", nodeID)
	}

	_ = s.nodeRepo.UpdateNodeExecution(ctx, waitingExec.ID, model.NodeExecCompleted, payload, "")
	s.storeNodeResult(ctx, executionID, nodeID, payload)

	// If execution was paused, resume it
	exec, _ := s.execRepo.GetExecution(ctx, executionID)
	if exec != nil && exec.Status == model.WorkflowExecStatusPaused {
		_ = s.execRepo.UpdateExecution(ctx, executionID, model.WorkflowExecStatusRunning, nil, "")
	}

	return s.AdvanceExecution(ctx, executionID)
}

// ---------------------------------------------------------------------------
// Template variable resolution
// ---------------------------------------------------------------------------

// resolveNodeConfig parses node config JSON and resolves template variables.
func (s *DAGWorkflowService) resolveNodeConfig(node *model.WorkflowNode, dataStore map[string]any) map[string]any {
	config := make(map[string]any)
	if len(node.Config) > 0 {
		_ = json.Unmarshal(node.Config, &config)
	}
	// Deep-resolve string values
	resolveMapValues(config, dataStore)
	return config
}

// resolveMapValues recursively resolves template vars in string values of a map.
func resolveMapValues(m map[string]any, dataStore map[string]any) {
	for k, v := range m {
		switch val := v.(type) {
		case string:
			m[k] = nodetypes.ResolveTemplateVars(val, dataStore)
		case map[string]any:
			resolveMapValues(val, dataStore)
		}
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (s *DAGWorkflowService) loadDataStore(exec *model.WorkflowExecution) map[string]any {
	ds := make(map[string]any)
	if len(exec.DataStore) > 0 {
		_ = json.Unmarshal(exec.DataStore, &ds)
	}
	return ds
}

func (s *DAGWorkflowService) storeNodeResult(ctx context.Context, executionID uuid.UUID, nodeID string, result json.RawMessage) {
	exec, err := s.execRepo.GetExecution(ctx, executionID)
	if err != nil {
		return
	}
	ds := s.loadDataStore(exec)

	var resultVal any
	_ = json.Unmarshal(result, &resultVal)
	ds[nodeID] = map[string]any{"output": resultVal}

	dsJSON, _ := json.Marshal(ds)
	_ = s.execRepo.UpdateExecutionDataStore(ctx, executionID, dsJSON)
}

func (s *DAGWorkflowService) broadcastEvent(ctx context.Context, eventType, projectID string, payload map[string]any) {
	_ = eventbus.PublishLegacy(ctx, s.bus, eventType, projectID, payload)
}

// ---------------------------------------------------------------------------
// Sub-workflow resume paths
// ---------------------------------------------------------------------------

// tryResumeParentFromDAGChild is the best-effort hook called from every DAG
// terminal-state transition (complete, fail, cancel). If the completing
// execution is a child registered in workflow_run_parent_link, this resumes
// the parent with the child's outcome. Idempotent: already-terminated links
// are skipped silently. Never returns an error to the caller — the child
// transition has already happened; resume is a side effect.
func (s *DAGWorkflowService) tryResumeParentFromDAGChild(ctx context.Context, childExec *model.WorkflowExecution, linkStatus string) {
	if s.linkRepo == nil || childExec == nil {
		return
	}
	link, err := s.linkRepo.GetByChild(ctx, model.SubWorkflowEngineDAG, childExec.ID)
	if err != nil || link == nil {
		return // not a sub-workflow child; nothing to do.
	}
	if link.Status != model.SubWorkflowLinkStatusRunning {
		return // already resumed (idempotent)
	}

	outcome := outcomeForLinkStatus(linkStatus)
	// Route based on parent_kind. When the parent is a legacy plugin run, hand
	// off to the plugin runtime's resume hook; otherwise fall through to the
	// DAG-native resume that mutates parent node state and advances the DAG.
	parentKind := link.ParentKind
	if parentKind == "" {
		parentKind = model.SubWorkflowParentKindDAGExecution
	}
	switch parentKind {
	case model.SubWorkflowParentKindPluginRun:
		if s.pluginResumer != nil {
			childOutputs := map[string]any{}
			if len(childExec.DataStore) > 0 {
				_ = json.Unmarshal(childExec.DataStore, &childOutputs)
			}
			if err := s.pluginResumer.ResumeParkedDAGChild(ctx, link.ParentExecutionID, childExec.ID, outcome, childOutputs); err != nil {
				log.WithError(err).WithFields(log.Fields{
					"parentRunId": link.ParentExecutionID.String(),
					"parentStep":  link.ParentNodeID,
					"childRunId":  childExec.ID.String(),
				}).Warn("sub_workflow: plugin parent resume failed")
			}
		}
	default:
		if err := s.resumeParentDAG(ctx, link, outcome, childExec); err != nil {
			log.WithError(err).WithFields(log.Fields{
				"parentExecutionId": link.ParentExecutionID.String(),
				"parentNodeId":      link.ParentNodeID,
				"childRunId":        childExec.ID.String(),
			}).Warn("sub_workflow: parent resume failed")
		}
	}
	if err := s.linkRepo.UpdateStatus(ctx, link.ID, linkStatus); err != nil {
		log.WithError(err).Warn("sub_workflow: update parent link status failed")
	}
}

// ResumeParentFromPluginChild is the plugin-runtime's counterpart to
// tryResumeParentFromDAGChild. Called from workflow_execution_service on
// terminal transitions of a workflow plugin run. The caller supplies the
// child's final outputs as a JSON envelope.
func (s *DAGWorkflowService) ResumeParentFromPluginChild(ctx context.Context, childRunID uuid.UUID, linkStatus string, childOutputs json.RawMessage) {
	if s.linkRepo == nil {
		return
	}
	link, err := s.linkRepo.GetByChild(ctx, model.SubWorkflowEnginePlugin, childRunID)
	if err != nil || link == nil {
		return
	}
	if link.Status != model.SubWorkflowLinkStatusRunning {
		return
	}

	outcome := outcomeForLinkStatus(linkStatus)
	if err := s.resumeParentWithOutputs(ctx, link, outcome, childOutputs, map[string]any{
		"runId":  childRunID.String(),
		"engine": model.SubWorkflowEnginePlugin,
	}); err != nil {
		log.WithError(err).WithFields(log.Fields{
			"parentExecutionId": link.ParentExecutionID.String(),
			"parentNodeId":      link.ParentNodeID,
			"childRunId":        childRunID.String(),
		}).Warn("sub_workflow: plugin parent resume failed")
	}
	if err := s.linkRepo.UpdateStatus(ctx, link.ID, linkStatus); err != nil {
		log.WithError(err).Warn("sub_workflow: update plugin parent link status failed")
	}
}

// resumeParentDAG materializes a child DAG execution's outputs into the parent
// datastore, marks the parent node complete/failed, and advances the parent.
// The "outputs" envelope is the child's final DataStore (the accumulated node
// outputs) so downstream parent nodes can read `$.dataStore[<sub>].subWorkflow.outputs.*`.
func (s *DAGWorkflowService) resumeParentDAG(ctx context.Context, link *model.WorkflowRunParentLink, outcome string, childExec *model.WorkflowExecution) error {
	var outputs json.RawMessage
	if childExec != nil {
		outputs = childExec.DataStore
	}
	return s.resumeParentWithOutputs(ctx, link, outcome, outputs, map[string]any{
		"runId":  childExec.ID.String(),
		"engine": model.SubWorkflowEngineDAG,
	})
}

func (s *DAGWorkflowService) resumeParentWithOutputs(ctx context.Context, link *model.WorkflowRunParentLink, outcome string, outputs json.RawMessage, extra map[string]any) error {
	parentExec, err := s.execRepo.GetExecution(ctx, link.ParentExecutionID)
	if err != nil {
		return fmt.Errorf("load parent execution: %w", err)
	}

	// Find the parked node execution.
	nodeExecs, err := s.nodeRepo.ListNodeExecutions(ctx, link.ParentExecutionID)
	if err != nil {
		return fmt.Errorf("list parent node executions: %w", err)
	}
	var parked *model.WorkflowNodeExecution
	for _, ne := range nodeExecs {
		if ne.NodeID == link.ParentNodeID && (ne.Status == model.NodeExecAwaitingSubWorkflow || ne.Status == model.NodeExecRunning) {
			parked = ne
			break
		}
	}
	if parked == nil {
		// Parent may have been cancelled or otherwise resolved elsewhere.
		return nil
	}

	// Build subWorkflow outputs envelope keyed under parent node id.
	var outputsVal any
	if len(outputs) > 0 {
		_ = json.Unmarshal(outputs, &outputsVal)
	}
	envelope := map[string]any{
		"subWorkflow": mergeMaps(extra, map[string]any{
			"outputs": outputsVal,
			"outcome": outcome,
		}),
	}
	envelopeJSON, _ := json.Marshal(envelope)
	s.storeNodeResult(ctx, link.ParentExecutionID, link.ParentNodeID, envelopeJSON)

	nodeStatus := model.NodeExecCompleted
	errMsg := ""
	if outcome != model.SubWorkflowLinkStatusCompleted {
		nodeStatus = model.NodeExecFailed
		errMsg = fmt.Sprintf("child sub-workflow run terminated with outcome: %s", outcome)
	}
	if err := s.nodeRepo.UpdateNodeExecution(ctx, parked.ID, nodeStatus, envelopeJSON, errMsg); err != nil {
		return fmt.Errorf("update parent node execution: %w", err)
	}

	s.broadcastEvent(ctx, ws.EventWorkflowNodeCompleted, parentExec.ProjectID.String(), map[string]any{
		"executionId": link.ParentExecutionID.String(),
		"nodeId":      link.ParentNodeID,
		"status":      nodeStatus,
	})

	if nodeStatus == model.NodeExecFailed {
		if err := s.execRepo.UpdateExecution(ctx, link.ParentExecutionID, model.WorkflowExecStatusFailed, nil, errMsg); err != nil {
			log.WithError(err).Warn("sub_workflow: mark parent failed")
		}
		return nil
	}

	// Flip back to running if the parent was paused (e.g. awaiting sub-workflow).
	if parentExec.Status != model.WorkflowExecStatusRunning {
		_ = s.execRepo.UpdateExecution(ctx, link.ParentExecutionID, model.WorkflowExecStatusRunning, nil, "")
	}
	return s.AdvanceExecution(ctx, link.ParentExecutionID)
}

func outcomeForLinkStatus(status string) string {
	switch status {
	case model.SubWorkflowLinkStatusCompleted:
		return model.SubWorkflowLinkStatusCompleted
	case model.SubWorkflowLinkStatusFailed:
		return model.SubWorkflowLinkStatusFailed
	case model.SubWorkflowLinkStatusCancelled:
		return model.SubWorkflowLinkStatusCancelled
	default:
		return status
	}
}

func mergeMaps(a, b map[string]any) map[string]any {
	out := make(map[string]any, len(a)+len(b))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}
