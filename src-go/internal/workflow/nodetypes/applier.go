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

// ── EffectApplier ───────────────────────────────────────────────────────

// EffectApplier executes the structured effects returned by node handlers.
// Uses exported fields for easy test construction via composite literal.
type EffectApplier struct {
	Hub      BroadcastHub
	TaskRepo TaskTransitioner
	NodeRepo NodeExecDeleter
	ExecRepo ExecutionDataStoreWriter
	// Park-effect deps added in Task 4
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

		case EffectSpawnAgent, EffectRequestReview, EffectWaitEvent, EffectInvokeSubWorkflow:
			// Park effects — stubbed until Task 4.
			return false, fmt.Errorf("park effect %s not yet implemented", e.Kind)

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
