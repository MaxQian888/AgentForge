package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/workflow/nodetypes"
	"github.com/react-go-quick-starter/server/internal/ws"
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

// DAGWorkflowService orchestrates workflow DAG execution.
type DAGWorkflowService struct {
	defRepo     DAGWorkflowDefinitionRepo
	execRepo    DAGWorkflowExecutionRepo
	nodeRepo    DAGWorkflowNodeExecRepo
	taskRepo    DAGWorkflowTaskRepo
	mappingRepo DAGWorkflowRunMappingRepo
	reviewRepo  DAGWorkflowReviewRepo
	hub         *ws.Hub
	registry    *nodetypes.NodeTypeRegistry
	applier     *nodetypes.EffectApplier
}

// NewDAGWorkflowService creates a new DAG workflow execution service.
func NewDAGWorkflowService(
	defRepo DAGWorkflowDefinitionRepo,
	execRepo DAGWorkflowExecutionRepo,
	nodeRepo DAGWorkflowNodeExecRepo,
	hub *ws.Hub,
	registry *nodetypes.NodeTypeRegistry,
	applier *nodetypes.EffectApplier,
) *DAGWorkflowService {
	return &DAGWorkflowService{
		defRepo:  defRepo,
		execRepo: execRepo,
		nodeRepo: nodeRepo,
		hub:      hub,
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

// ---------------------------------------------------------------------------
// Execution lifecycle
// ---------------------------------------------------------------------------

// StartExecution creates a new execution and advances from trigger nodes.
func (s *DAGWorkflowService) StartExecution(ctx context.Context, workflowID uuid.UUID, taskID *uuid.UUID) (*model.WorkflowExecution, error) {
	def, err := s.defRepo.GetByID(ctx, workflowID)
	if err != nil {
		return nil, fmt.Errorf("load workflow definition: %w", err)
	}
	if def.Status != model.WorkflowDefStatusActive {
		return nil, fmt.Errorf("workflow %s is not active (status: %s)", workflowID, def.Status)
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

	exec := &model.WorkflowExecution{
		ID:           uuid.New(),
		WorkflowID:   workflowID,
		ProjectID:    def.ProjectID,
		TaskID:       taskID,
		Status:       model.WorkflowExecStatusRunning,
		CurrentNodes: currentNodesJSON,
		Context:      execContext,
		DataStore:    json.RawMessage("{}"),
		StartedAt:    &now,
	}
	if err := s.execRepo.CreateExecution(ctx, exec); err != nil {
		return nil, fmt.Errorf("create execution: %w", err)
	}

	s.broadcastEvent(ws.EventWorkflowExecutionStarted, def.ProjectID.String(), map[string]any{
		"executionId": exec.ID.String(),
		"workflowId":  workflowID.String(),
	})

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
		case model.NodeExecWaiting:
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
		s.broadcastEvent(ws.EventWorkflowExecutionCompleted, exec.ProjectID.String(), map[string]any{
			"executionId": exec.ID.String(),
			"workflowId":  exec.WorkflowID.String(),
			"status":      model.WorkflowExecStatusCompleted,
		})
		return nil
	}

	currentNodesJSON, _ := json.Marshal(nextNodeIDs)
	if err := s.execRepo.UpdateExecution(ctx, executionID, model.WorkflowExecStatusRunning, currentNodesJSON, ""); err != nil {
		return fmt.Errorf("update current nodes: %w", err)
	}

	s.broadcastEvent(ws.EventWorkflowExecutionAdvanced, exec.ProjectID.String(), map[string]any{
		"executionId":  exec.ID.String(),
		"currentNodes": nextNodeIDs,
	})

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
			return err
		}
	}

	return s.AdvanceExecution(ctx, executionID)
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
	s.broadcastEvent(ws.EventWorkflowExecutionCompleted, exec.ProjectID.String(), map[string]any{
		"executionId": exec.ID.String(),
		"workflowId":  exec.WorkflowID.String(),
		"status":      model.WorkflowExecStatusCancelled,
	})
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

	s.broadcastEvent(ws.EventWorkflowNodeCompleted, "", map[string]any{
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
		case nodetypes.EffectSpawnAgent, nodetypes.EffectInvokeSubWorkflow:
			// Stays running — async callback (HandleAgentRunCompletion / sub-workflow completion) will resume.
		}
		return nil
	}

	// Synchronous completion path
	_ = s.nodeRepo.UpdateNodeExecution(ctx, nodeExec.ID, model.NodeExecCompleted, result.Result, "")
	if len(result.Result) > 0 {
		s.storeNodeResult(ctx, exec.ID, node.ID, result.Result)
	}
	s.broadcastEvent(ws.EventWorkflowNodeCompleted, exec.ProjectID.String(), map[string]any{
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

	s.broadcastEvent(ws.EventWorkflowReviewResolved, exec.ProjectID.String(), map[string]any{
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

func (s *DAGWorkflowService) broadcastEvent(eventType, projectID string, payload map[string]any) {
	if s.hub == nil {
		return
	}
	s.hub.BroadcastEvent(&ws.Event{
		Type:      eventType,
		ProjectID: projectID,
		Payload:   payload,
	})
}
