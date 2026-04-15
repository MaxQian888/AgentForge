package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
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
type DAGWorkflowTaskRepo interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Task, error)
	TransitionStatus(ctx context.Context, id uuid.UUID, newStatus string) error
}

// DAGWorkflowAgentSpawner spawns agent runs from workflow nodes.
type DAGWorkflowAgentSpawner interface {
	Spawn(ctx context.Context, taskID, memberID uuid.UUID, runtime, provider, modelName string, budgetUsd float64, roleID string) (*model.AgentRun, error)
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
	defRepo      DAGWorkflowDefinitionRepo
	execRepo     DAGWorkflowExecutionRepo
	nodeRepo     DAGWorkflowNodeExecRepo
	taskRepo     DAGWorkflowTaskRepo
	agentSpawner DAGWorkflowAgentSpawner
	mappingRepo  DAGWorkflowRunMappingRepo
	reviewRepo   DAGWorkflowReviewRepo
	hub          *ws.Hub
}

// NewDAGWorkflowService creates a new DAG workflow execution service.
func NewDAGWorkflowService(
	defRepo DAGWorkflowDefinitionRepo,
	execRepo DAGWorkflowExecutionRepo,
	nodeRepo DAGWorkflowNodeExecRepo,
	hub *ws.Hub,
) *DAGWorkflowService {
	return &DAGWorkflowService{
		defRepo:  defRepo,
		execRepo: execRepo,
		nodeRepo: nodeRepo,
		hub:      hub,
	}
}

// SetTaskRepo sets the task repository for status transitions.
func (s *DAGWorkflowService) SetTaskRepo(r DAGWorkflowTaskRepo) { s.taskRepo = r }

// SetAgentSpawner sets the agent spawner for agent_dispatch and llm_agent nodes.
func (s *DAGWorkflowService) SetAgentSpawner(spawner DAGWorkflowAgentSpawner) {
	s.agentSpawner = spawner
}

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
				if edge.Condition != "" && !s.evaluateCondition(exec, edge.Condition, dataStore) {
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
// Node execution
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

	// Resolve node config with template variables
	config := s.resolveNodeConfig(node, dataStore)

	var execErr error
	var nodeResult json.RawMessage
	asyncNode := false

	switch node.Type {
	case model.NodeTypeLLMAgent:
		execErr = s.executeLLMAgent(ctx, exec, node, nodeExec.ID, config)
		asyncNode = true // Don't mark completed — wait for HandleAgentRunCompletion
	case model.NodeTypeAgentDispatch:
		execErr = s.executeLLMAgent(ctx, exec, node, nodeExec.ID, config) // Treat as llm_agent
		asyncNode = true
	case model.NodeTypeFunction:
		nodeResult, execErr = s.executeFunction(ctx, exec, node, config, dataStore)
	case model.NodeTypeNotification:
		execErr = s.executeNotification(ctx, exec, node, config)
	case model.NodeTypeStatusTransition:
		execErr = s.executeStatusTransition(ctx, exec, node, config)
	case model.NodeTypeCondition:
		execErr = s.executeCondition(ctx, exec, node, config, dataStore)
	case model.NodeTypeLoop:
		execErr = s.executeLoop(ctx, exec, node, config, dataStore)
	case model.NodeTypeHumanReview:
		execErr = s.executeHumanReview(ctx, exec, node, nodeExec.ID, config)
		asyncNode = true // Wait for external resolution
	case model.NodeTypeWaitEvent:
		execErr = s.executeWaitEvent(ctx, exec, node, nodeExec.ID, config)
		asyncNode = true
	case model.NodeTypeGate:
		execErr = nil
	case model.NodeTypeParallelSplit:
		execErr = nil
	case model.NodeTypeParallelJoin:
		execErr = nil
	case model.NodeTypeTrigger:
		execErr = nil
	default:
		execErr = fmt.Errorf("unknown node type: %s", node.Type)
	}

	if execErr != nil {
		_ = s.nodeRepo.UpdateNodeExecution(ctx, nodeExec.ID, model.NodeExecFailed, nil, execErr.Error())
		return execErr
	}

	if asyncNode {
		// Node stays in running/waiting state until external callback
		return nil
	}

	// Synchronous completion
	_ = s.nodeRepo.UpdateNodeExecution(ctx, nodeExec.ID, model.NodeExecCompleted, nodeResult, "")

	// Store result in DataStore
	if len(nodeResult) > 0 {
		s.storeNodeResult(ctx, exec.ID, node.ID, nodeResult)
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
// Node type executors
// ---------------------------------------------------------------------------

// executeLLMAgent spawns an agent run via the bridge and registers it for async callback.
func (s *DAGWorkflowService) executeLLMAgent(ctx context.Context, exec *model.WorkflowExecution, node *model.WorkflowNode, nodeExecID uuid.UUID, config map[string]any) error {
	if s.agentSpawner == nil {
		log.WithField("nodeId", node.ID).Warn("workflow: llm_agent skipped - agent spawner not available")
		return nil
	}
	if exec.TaskID == nil {
		return fmt.Errorf("llm_agent requires a task ID in the execution context")
	}

	runtime, _ := config["runtime"].(string)
	provider, _ := config["provider"].(string)
	modelName, _ := config["model"].(string)
	roleID, _ := config["roleId"].(string)
	budgetUsd := 5.0
	if b, ok := config["budgetUsd"].(float64); ok && b > 0 {
		budgetUsd = b
	}

	memberID := uuid.Nil
	if mid, ok := config["memberId"].(string); ok {
		if parsed, err := uuid.Parse(mid); err == nil {
			memberID = parsed
		}
	}

	run, err := s.agentSpawner.Spawn(ctx, *exec.TaskID, memberID, runtime, provider, modelName, budgetUsd, roleID)
	if err != nil {
		return fmt.Errorf("llm_agent spawn: %w", err)
	}

	// Register mapping for async completion callback
	if s.mappingRepo != nil {
		mapping := &model.WorkflowRunMapping{
			ID:          uuid.New(),
			ExecutionID: exec.ID,
			NodeID:      node.ID,
			AgentRunID:  run.ID,
		}
		if err := s.mappingRepo.Create(ctx, mapping); err != nil {
			log.WithError(err).Warn("workflow: failed to create run mapping")
		}
	}

	log.WithFields(log.Fields{
		"executionId": exec.ID.String(),
		"nodeId":      node.ID,
		"runId":       run.ID.String(),
		"runtime":     runtime,
		"model":       modelName,
		"budgetUsd":   budgetUsd,
	}).Info("workflow: llm_agent dispatched")
	return nil
}

// executeFunction evaluates an expression and produces a result.
func (s *DAGWorkflowService) executeFunction(_ context.Context, _ *model.WorkflowExecution, node *model.WorkflowNode, config map[string]any, dataStore map[string]any) (json.RawMessage, error) {
	expression, _ := config["expression"].(string)
	if expression == "" {
		// If no expression, pass through input as output
		if input, ok := config["input"]; ok {
			result, _ := json.Marshal(input)
			return result, nil
		}
		return json.RawMessage("null"), nil
	}

	// Resolve template vars in expression
	resolved := nodetypes.ResolveTemplateVars(expression, dataStore)

	// Try to parse as JSON and return directly
	if json.Valid([]byte(resolved)) {
		return json.RawMessage(resolved), nil
	}

	// Evaluate simple expressions
	val := nodetypes.EvaluateExpression(resolved, dataStore)
	result, _ := json.Marshal(val)
	return result, nil
}

// executeNotification logs and broadcasts a WebSocket event.
func (s *DAGWorkflowService) executeNotification(_ context.Context, exec *model.WorkflowExecution, node *model.WorkflowNode, config map[string]any) error {
	message := "Workflow notification"
	if msg, ok := config["message"].(string); ok {
		message = msg
	}
	log.WithFields(log.Fields{
		"executionId": exec.ID.String(),
		"nodeId":      node.ID,
		"message":     message,
	}).Info("workflow: notification sent")

	s.broadcastEvent("workflow.notification", exec.ProjectID.String(), map[string]any{
		"executionId": exec.ID.String(),
		"nodeId":      node.ID,
		"message":     message,
	})
	return nil
}

// executeStatusTransition updates the task status.
func (s *DAGWorkflowService) executeStatusTransition(ctx context.Context, exec *model.WorkflowExecution, node *model.WorkflowNode, config map[string]any) error {
	if s.taskRepo == nil {
		log.WithField("nodeId", node.ID).Warn("workflow: status_transition skipped - task repo not available")
		return nil
	}
	if exec.TaskID == nil {
		return fmt.Errorf("status_transition requires a task ID in the execution context")
	}
	targetStatus, ok := config["targetStatus"].(string)
	if !ok || targetStatus == "" {
		return fmt.Errorf("status_transition node %s missing targetStatus config", node.ID)
	}
	if err := s.taskRepo.TransitionStatus(ctx, *exec.TaskID, targetStatus); err != nil {
		return fmt.Errorf("status_transition to %s: %w", targetStatus, err)
	}
	return nil
}

// executeCondition evaluates a condition expression.
func (s *DAGWorkflowService) executeCondition(_ context.Context, exec *model.WorkflowExecution, _ *model.WorkflowNode, config map[string]any, dataStore map[string]any) error {
	expression, _ := config["expression"].(string)
	if expression == "" {
		return nil // No condition = pass through
	}
	if !s.evaluateCondition(exec, expression, dataStore) {
		return fmt.Errorf("condition not met: %s", expression)
	}
	return nil
}

// executeLoop handles a feedback loop by resetting downstream nodes and re-advancing.
func (s *DAGWorkflowService) executeLoop(ctx context.Context, exec *model.WorkflowExecution, node *model.WorkflowNode, config map[string]any, dataStore map[string]any) error {
	targetNode, _ := config["target_node"].(string)
	maxIterations := 3
	if m, ok := config["max_iterations"].(float64); ok && m > 0 {
		maxIterations = int(m)
	}
	exitCondition, _ := config["exit_condition"].(string)

	if targetNode == "" {
		return fmt.Errorf("loop node %s missing target_node config", node.ID)
	}

	// Track iteration count in DataStore
	loopKey := "_loop_" + node.ID + "_count"
	currentIter := 0
	if v, ok := dataStore[loopKey]; ok {
		if f, ok := v.(float64); ok {
			currentIter = int(f)
		}
	}

	// Check exit conditions
	if currentIter >= maxIterations {
		log.WithFields(log.Fields{
			"nodeId":    node.ID,
			"iteration": currentIter,
			"max":       maxIterations,
		}).Info("workflow: loop max iterations reached")
		return nil // Exit loop, continue DAG
	}
	if exitCondition != "" && s.evaluateCondition(exec, exitCondition, dataStore) {
		log.WithFields(log.Fields{
			"nodeId":    node.ID,
			"iteration": currentIter,
			"condition": exitCondition,
		}).Info("workflow: loop exit condition met")
		return nil
	}

	// Increment iteration count
	currentIter++
	dataStore[loopKey] = float64(currentIter)
	dsJSON, _ := json.Marshal(dataStore)
	if err := s.execRepo.UpdateExecutionDataStore(ctx, exec.ID, dsJSON); err != nil {
		log.WithError(err).Warn("workflow: failed to update DataStore for loop")
	}

	// Find all nodes between target_node and this loop node, reset them
	def, err := s.defRepo.GetByID(ctx, exec.WorkflowID)
	if err != nil {
		return fmt.Errorf("load definition for loop: %w", err)
	}
	var allNodes []model.WorkflowNode
	var allEdges []model.WorkflowEdge
	_ = json.Unmarshal(def.Nodes, &allNodes)
	_ = json.Unmarshal(def.Edges, &allEdges)

	nodesToReset := s.findNodesBetween(targetNode, node.ID, allNodes, allEdges)
	nodesToReset = append(nodesToReset, targetNode) // Include the target itself

	if len(nodesToReset) > 0 {
		if err := s.nodeRepo.DeleteNodeExecutionsByNodeIDs(ctx, exec.ID, nodesToReset); err != nil {
			return fmt.Errorf("reset nodes for loop: %w", err)
		}
	}

	log.WithFields(log.Fields{
		"nodeId":       node.ID,
		"iteration":    currentIter,
		"targetNode":   targetNode,
		"resetNodes":   len(nodesToReset),
	}).Info("workflow: loop iteration, resetting nodes")

	// Re-advance from the beginning — AdvanceExecution will find the reset nodes
	return nil // The caller (AdvanceExecution) will recurse and pick up the reset nodes
}

// executeHumanReview pauses execution and waits for human input.
func (s *DAGWorkflowService) executeHumanReview(ctx context.Context, exec *model.WorkflowExecution, node *model.WorkflowNode, nodeExecID uuid.UUID, config map[string]any) error {
	_ = s.nodeRepo.UpdateNodeExecution(ctx, nodeExecID, model.NodeExecWaiting, nil, "")
	_ = s.execRepo.UpdateExecution(ctx, exec.ID, model.WorkflowExecStatusPaused, nil, "")

	prompt, _ := config["prompt"].(string)
	if prompt == "" {
		prompt = "Review required"
	}

	// Persist the review request
	if s.reviewRepo != nil {
		reviewCtx, _ := json.Marshal(config)
		review := &model.WorkflowPendingReview{
			ID:          uuid.New(),
			ExecutionID: exec.ID,
			NodeID:      node.ID,
			ProjectID:   exec.ProjectID,
			Prompt:      prompt,
			Context:     reviewCtx,
			Decision:    model.ReviewDecisionPending,
		}
		if err := s.reviewRepo.Create(ctx, review); err != nil {
			log.WithError(err).Warn("workflow: failed to persist pending review")
		}
	}

	s.broadcastEvent(ws.EventWorkflowReviewRequested, exec.ProjectID.String(), map[string]any{
		"executionId": exec.ID.String(),
		"nodeId":      node.ID,
		"nodeExecId":  nodeExecID.String(),
		"prompt":      prompt,
	})

	log.WithFields(log.Fields{
		"executionId": exec.ID.String(),
		"nodeId":      node.ID,
		"prompt":      prompt,
	}).Info("workflow: human review requested, execution paused")

	return nil
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

// executeWaitEvent sets a node to waiting for an external event.
func (s *DAGWorkflowService) executeWaitEvent(_ context.Context, exec *model.WorkflowExecution, node *model.WorkflowNode, nodeExecID uuid.UUID, config map[string]any) error {
	_ = s.nodeRepo.UpdateNodeExecution(context.Background(), nodeExecID, model.NodeExecWaiting, nil, "")

	eventType, _ := config["event_type"].(string)
	s.broadcastEvent(ws.EventWorkflowNodeWaiting, exec.ProjectID.String(), map[string]any{
		"executionId": exec.ID.String(),
		"nodeId":      node.ID,
		"nodeExecId":  nodeExecID.String(),
		"eventType":   eventType,
	})
	return nil
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
// Template variable resolution & expression evaluation
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

// evaluateCondition evaluates a condition expression against execution context and DataStore.
func (s *DAGWorkflowService) evaluateCondition(exec *model.WorkflowExecution, expression string, dataStore map[string]any) bool {
	expression = strings.TrimSpace(expression)
	if expression == "" || expression == "true" {
		return true
	}
	if expression == "false" {
		return false
	}

	// Resolve template variables first
	expression = nodetypes.ResolveTemplateVars(expression, dataStore)

	// Re-check after resolution
	expression = strings.TrimSpace(expression)
	if expression == "true" {
		return true
	}
	if expression == "false" {
		return false
	}

	// Comparison operators: ==, !=, >, <, >=, <=
	for _, op := range []string{"==", "!=", ">=", "<=", ">", "<"} {
		if strings.Contains(expression, op) {
			parts := strings.SplitN(expression, op, 2)
			if len(parts) == 2 {
				left := strings.TrimSpace(parts[0])
				right := strings.Trim(strings.TrimSpace(parts[1]), "\"'")

				// Resolve left side: e.g., "task.status"
				if strings.HasPrefix(left, "task.status") && exec.TaskID != nil && s.taskRepo != nil {
					task, err := s.taskRepo.GetByID(context.Background(), *exec.TaskID)
					if err == nil {
						left = task.Status
					}
				}
				// Try resolving left as DataStore path
				if val := nodetypes.LookupPath(dataStore, left); val != nil {
					left = fmt.Sprintf("%v", val)
				}

				return compareValues(left, op, right)
			}
		}
	}

	log.WithField("expression", expression).Warn("workflow: unrecognized condition, defaulting to true")
	return true
}

// compareValues compares two string values using the given operator.
func compareValues(left, op, right string) bool {
	// Try numeric comparison
	leftNum, leftErr := strconv.ParseFloat(left, 64)
	rightNum, rightErr := strconv.ParseFloat(right, 64)

	if leftErr == nil && rightErr == nil {
		switch op {
		case "==":
			return leftNum == rightNum
		case "!=":
			return leftNum != rightNum
		case ">":
			return leftNum > rightNum
		case "<":
			return leftNum < rightNum
		case ">=":
			return leftNum >= rightNum
		case "<=":
			return leftNum <= rightNum
		}
	}

	// String comparison
	switch op {
	case "==":
		return left == right
	case "!=":
		return left != right
	case ">":
		return left > right
	case "<":
		return left < right
	case ">=":
		return left >= right
	case "<=":
		return left <= right
	}
	return false
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

// findNodesBetween returns node IDs reachable from `fromID` that lead to `toID` (exclusive of toID).
func (s *DAGWorkflowService) findNodesBetween(fromID, toID string, nodes []model.WorkflowNode, edges []model.WorkflowEdge) []string {
	// BFS from fromID, collect nodes that are on paths to toID
	adjacency := make(map[string][]string)
	for _, e := range edges {
		adjacency[e.Source] = append(adjacency[e.Source], e.Target)
	}

	visited := make(map[string]bool)
	var result []string
	queue := []string{fromID}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if visited[current] || current == toID {
			continue
		}
		visited[current] = true
		if current != fromID {
			result = append(result, current)
		}
		for _, next := range adjacency[current] {
			if !visited[next] {
				queue = append(queue, next)
			}
		}
	}
	return result
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
