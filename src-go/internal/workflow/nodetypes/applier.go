package nodetypes

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

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
			if err := a.applyInvokeSubWorkflow(exec, node, e.Payload); err != nil {
				return false, fmt.Errorf("invoke_sub_workflow: %w", err)
			}
			return true, nil

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

func (a *EffectApplier) applyInvokeSubWorkflow(exec *model.WorkflowExecution, node *model.WorkflowNode, raw json.RawMessage) error {
	var p InvokeSubWorkflowPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}
	// Track A stub: the sub_workflow handler itself fails fast (Task 6), so this
	// applier path is defensive. Park the node and let the caller surface the
	// lack-of-implementation via the handler contract.
	log.Printf("[WARN] EffectApplier: TODO: sub_workflow not wired (execution=%s, node=%s, targetWorkflow=%s)", exec.ID, node.ID, p.WorkflowID)
	return nil
}
