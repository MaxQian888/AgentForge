package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
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
	CompleteExecution(ctx context.Context, id uuid.UUID, status string) error
}

// DAGWorkflowNodeExecRepo defines the node execution persistence interface.
type DAGWorkflowNodeExecRepo interface {
	CreateNodeExecution(ctx context.Context, nodeExec *model.WorkflowNodeExecution) error
	UpdateNodeExecution(ctx context.Context, id uuid.UUID, status string, result json.RawMessage, errorMessage string) error
	ListNodeExecutions(ctx context.Context, executionID uuid.UUID) ([]*model.WorkflowNodeExecution, error)
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

// DAGWorkflowService orchestrates workflow DAG execution.
type DAGWorkflowService struct {
	defRepo      DAGWorkflowDefinitionRepo
	execRepo     DAGWorkflowExecutionRepo
	nodeRepo     DAGWorkflowNodeExecRepo
	taskRepo     DAGWorkflowTaskRepo
	agentSpawner DAGWorkflowAgentSpawner
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
func (s *DAGWorkflowService) SetTaskRepo(r DAGWorkflowTaskRepo) {
	s.taskRepo = r
}

// SetAgentSpawner sets the agent spawner for agent_dispatch nodes.
func (s *DAGWorkflowService) SetAgentSpawner(spawner DAGWorkflowAgentSpawner) {
	s.agentSpawner = spawner
}

// StartExecution creates a new execution and advances from trigger nodes.
func (s *DAGWorkflowService) StartExecution(ctx context.Context, workflowID uuid.UUID, taskID *uuid.UUID) (*model.WorkflowExecution, error) {
	// 1. Load workflow definition
	def, err := s.defRepo.GetByID(ctx, workflowID)
	if err != nil {
		return nil, fmt.Errorf("load workflow definition: %w", err)
	}
	if def.Status != model.WorkflowDefStatusActive {
		return nil, fmt.Errorf("workflow %s is not active (status: %s)", workflowID, def.Status)
	}

	// 2. Parse nodes and edges
	var nodes []model.WorkflowNode
	var edges []model.WorkflowEdge
	if err := json.Unmarshal(def.Nodes, &nodes); err != nil {
		return nil, fmt.Errorf("parse workflow nodes: %w", err)
	}
	if err := json.Unmarshal(def.Edges, &edges); err != nil {
		return nil, fmt.Errorf("parse workflow edges: %w", err)
	}

	// 3. Find trigger nodes (nodes with no incoming edges)
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

	// 4. Create execution record
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
		StartedAt:    &now,
	}
	if err := s.execRepo.CreateExecution(ctx, exec); err != nil {
		return nil, fmt.Errorf("create execution: %w", err)
	}

	// Broadcast start event
	s.broadcastEvent(ws.EventWorkflowExecutionStarted, def.ProjectID.String(), map[string]any{
		"executionId": exec.ID.String(),
		"workflowId":  workflowID.String(),
	})

	// 5. Mark trigger nodes as completed and create node execution records
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

	// 6. Advance to next nodes
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

	// Get completed nodes from node executions
	nodeExecs, err := s.nodeRepo.ListNodeExecutions(ctx, executionID)
	if err != nil {
		return fmt.Errorf("list node executions: %w", err)
	}
	completedNodes := make(map[string]bool)
	runningNodes := make(map[string]bool)
	for _, ne := range nodeExecs {
		if ne.Status == model.NodeExecCompleted {
			completedNodes[ne.NodeID] = true
		}
		if ne.Status == model.NodeExecRunning {
			runningNodes[ne.NodeID] = true
		}
	}

	// Find the next nodes to activate: nodes whose all incoming edges come from completed nodes
	var nextNodeIDs []string
	for _, node := range nodes {
		if completedNodes[node.ID] || runningNodes[node.ID] {
			continue
		}
		// Check if all predecessors (via incoming edges) are completed
		allPredCompleted := true
		hasPredecessors := false
		for _, edge := range edges {
			if edge.Target == node.ID {
				hasPredecessors = true
				if !completedNodes[edge.Source] {
					allPredCompleted = false
					break
				}
				// Evaluate condition if present
				if edge.Condition != "" && !s.evaluateCondition(exec, edge.Condition) {
					allPredCompleted = false
					break
				}
			}
		}
		if hasPredecessors && allPredCompleted {
			nextNodeIDs = append(nextNodeIDs, node.ID)
		}
	}

	// If no more nodes to process, check if workflow is complete
	if len(nextNodeIDs) == 0 {
		// Check if there are still running nodes
		if len(runningNodes) > 0 {
			return nil // Wait for running nodes to finish
		}
		// Workflow is complete
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

	// Execute each next node
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
		if err := s.executeNode(ctx, exec, node); err != nil {
			log.WithError(err).WithField("nodeId", nodeID).Warn("workflow node execution failed")
			// Mark execution as failed
			if updateErr := s.execRepo.UpdateExecution(ctx, executionID, model.WorkflowExecStatusFailed, currentNodesJSON, err.Error()); updateErr != nil {
				log.WithError(updateErr).Warn("failed to update execution status to failed")
			}
			return err
		}
	}

	// Recurse to advance further
	return s.AdvanceExecution(ctx, executionID)
}

// CancelExecution cancels a running execution.
func (s *DAGWorkflowService) CancelExecution(ctx context.Context, executionID uuid.UUID) error {
	exec, err := s.execRepo.GetExecution(ctx, executionID)
	if err != nil {
		return fmt.Errorf("load execution: %w", err)
	}
	if exec.Status != model.WorkflowExecStatusRunning && exec.Status != model.WorkflowExecStatusPending {
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

// executeNode handles a single node based on its type.
func (s *DAGWorkflowService) executeNode(ctx context.Context, exec *model.WorkflowExecution, node *model.WorkflowNode) error {
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

	var execErr error
	switch node.Type {
	case model.NodeTypeAgentDispatch:
		execErr = s.executeAgentDispatch(ctx, exec, node)
	case model.NodeTypeNotification:
		execErr = s.executeNotification(ctx, exec, node)
	case model.NodeTypeStatusTransition:
		execErr = s.executeStatusTransition(ctx, exec, node)
	case model.NodeTypeCondition:
		execErr = s.executeCondition(ctx, exec, node)
	case model.NodeTypeGate:
		// Gate waits for manual/external signal - mark as completed for now
		execErr = nil
	case model.NodeTypeParallelSplit:
		execErr = nil // Split just activates all outgoing edges, handled by advance
	case model.NodeTypeParallelJoin:
		execErr = nil // Join is handled by the predecessor check in AdvanceExecution
	case model.NodeTypeTrigger:
		execErr = nil // Triggers are already handled at start
	default:
		execErr = fmt.Errorf("unknown node type: %s", node.Type)
	}

	if execErr != nil {
		completedAt := time.Now().UTC()
		nodeExec.Status = model.NodeExecFailed
		nodeExec.ErrorMessage = execErr.Error()
		nodeExec.CompletedAt = &completedAt
		_ = s.nodeRepo.UpdateNodeExecution(ctx, nodeExec.ID, model.NodeExecFailed, nil, execErr.Error())
		return execErr
	}

	completedAt := time.Now().UTC()
	nodeExec.Status = model.NodeExecCompleted
	nodeExec.CompletedAt = &completedAt
	_ = s.nodeRepo.UpdateNodeExecution(ctx, nodeExec.ID, model.NodeExecCompleted, nil, "")

	s.broadcastEvent(ws.EventWorkflowNodeCompleted, exec.ProjectID.String(), map[string]any{
		"executionId": exec.ID.String(),
		"nodeId":      node.ID,
		"nodeType":    node.Type,
		"status":      model.NodeExecCompleted,
	})

	return nil
}

// executeAgentDispatch spawns an agent run using configuration from the node.
// Config fields: runtime, provider, model, roleId, budgetUsd.
// The task ID is taken from the workflow execution context.
func (s *DAGWorkflowService) executeAgentDispatch(ctx context.Context, exec *model.WorkflowExecution, node *model.WorkflowNode) error {
	if s.agentSpawner == nil {
		log.WithField("nodeId", node.ID).Warn("workflow: agent_dispatch skipped - agent spawner not available")
		return nil
	}
	if exec.TaskID == nil {
		return fmt.Errorf("agent_dispatch requires a task ID in the execution context")
	}

	var config map[string]any
	if len(node.Config) > 0 {
		_ = json.Unmarshal(node.Config, &config)
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
		return fmt.Errorf("agent_dispatch spawn: %w", err)
	}

	log.WithFields(log.Fields{
		"executionId": exec.ID.String(),
		"nodeId":      node.ID,
		"runId":       run.ID.String(),
		"taskId":      exec.TaskID.String(),
		"runtime":     runtime,
		"provider":    provider,
		"model":       modelName,
		"budgetUsd":   budgetUsd,
	}).Info("workflow: agent dispatched")
	return nil
}

// executeNotification logs and broadcasts a WebSocket event.
func (s *DAGWorkflowService) executeNotification(ctx context.Context, exec *model.WorkflowExecution, node *model.WorkflowNode) error {
	var config map[string]any
	if len(node.Config) > 0 {
		_ = json.Unmarshal(node.Config, &config)
	}
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

// executeStatusTransition parses config for target status and updates the task.
func (s *DAGWorkflowService) executeStatusTransition(ctx context.Context, exec *model.WorkflowExecution, node *model.WorkflowNode) error {
	if s.taskRepo == nil {
		log.WithField("nodeId", node.ID).Warn("workflow: status_transition skipped - task repo not available")
		return nil
	}
	if exec.TaskID == nil {
		return fmt.Errorf("status_transition requires a task ID in the execution context")
	}

	var config map[string]any
	if len(node.Config) > 0 {
		_ = json.Unmarshal(node.Config, &config)
	}
	targetStatus, ok := config["targetStatus"].(string)
	if !ok || targetStatus == "" {
		return fmt.Errorf("status_transition node %s missing targetStatus config", node.ID)
	}

	if err := s.taskRepo.TransitionStatus(ctx, *exec.TaskID, targetStatus); err != nil {
		return fmt.Errorf("status_transition to %s: %w", targetStatus, err)
	}
	log.WithFields(log.Fields{
		"executionId":  exec.ID.String(),
		"nodeId":       node.ID,
		"targetStatus": targetStatus,
		"taskId":       exec.TaskID.String(),
	}).Info("workflow: status transition completed")
	return nil
}

// executeCondition evaluates a simple condition against execution context.
func (s *DAGWorkflowService) executeCondition(ctx context.Context, exec *model.WorkflowExecution, node *model.WorkflowNode) error {
	var config map[string]any
	if len(node.Config) > 0 {
		_ = json.Unmarshal(node.Config, &config)
	}
	expression, _ := config["expression"].(string)
	if expression == "" {
		// No condition = pass through
		return nil
	}
	if !s.evaluateCondition(exec, expression) {
		return fmt.Errorf("condition not met: %s", expression)
	}
	return nil
}

// evaluateCondition evaluates a simple condition expression against the execution context.
// Supported: "task.status == \"done\"", "true", "false"
func (s *DAGWorkflowService) evaluateCondition(exec *model.WorkflowExecution, expression string) bool {
	expression = strings.TrimSpace(expression)
	if expression == "" || expression == "true" {
		return true
	}
	if expression == "false" {
		return false
	}

	// Simple expression evaluator: "task.status == \"value\""
	if strings.HasPrefix(expression, "task.status") && strings.Contains(expression, "==") {
		parts := strings.SplitN(expression, "==", 2)
		if len(parts) != 2 {
			return false
		}
		expectedStatus := strings.Trim(strings.TrimSpace(parts[1]), "\"'")

		if exec.TaskID != nil && s.taskRepo != nil {
			task, err := s.taskRepo.GetByID(context.Background(), *exec.TaskID)
			if err == nil {
				return task.Status == expectedStatus
			}
		}
		return false
	}

	// Default: any unrecognized expression passes
	log.WithField("expression", expression).Warn("workflow: unrecognized condition expression, defaulting to true")
	return true
}

// broadcastEvent sends a WebSocket event.
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
