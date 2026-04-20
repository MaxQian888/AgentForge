package nodetypes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
)

// ErrWaitEventNotWaiting is returned when Resume is called against an
// execution whose target node is not in the waiting state. Caller maps
// this to HTTP 409 with code "card_action:execution_not_waiting".
var ErrWaitEventNotWaiting = errors.New("wait_event: target node is not waiting")

// WaitEventExecLookup loads the parent execution so the resumer can refuse
// resume attempts against terminated executions before mutating any rows.
type WaitEventExecLookup interface {
	GetExecution(ctx context.Context, id uuid.UUID) (*model.WorkflowExecution, error)
}

// WaitEventNodeLookup lists node executions so the resumer can find the
// single waiting row for the supplied (executionID, nodeID).
type WaitEventNodeLookup interface {
	ListNodeExecutions(ctx context.Context, executionID uuid.UUID) ([]*model.WorkflowNodeExecution, error)
}

// WaitEventNodeWriter transitions the node execution to completed and
// stores the inbound payload as the node result.
type WaitEventNodeWriter interface {
	UpdateNodeExecution(ctx context.Context, id uuid.UUID, status string, result json.RawMessage, errorMessage string) error
}

// WaitEventDataStoreMerger writes the resume payload into the execution
// dataStore under the resumed nodeID so downstream nodes can reference it
// through {{$dataStore.<nodeID>.<field>}}.
type WaitEventDataStoreMerger interface {
	MergeNodeResult(ctx context.Context, executionID uuid.UUID, nodeID string, payload map[string]any) error
}

// WaitEventAdvancer kicks the DAG runner forward after the node flips.
// Backed at wiring time by *DAGWorkflowService.AdvanceExecution.
type WaitEventAdvancer interface {
	AdvanceExecution(ctx context.Context, executionID uuid.UUID) error
}

// WaitEventResumer is the public entry point card_action_router calls when
// a callback button correlation matches a parked wait_event node.
type WaitEventResumer struct {
	ExecLookup WaitEventExecLookup
	NodeLookup WaitEventNodeLookup
	NodeWriter WaitEventNodeWriter
	DataStore  WaitEventDataStoreMerger
	Advancer   WaitEventAdvancer
}

// Resume validates that exec is still alive and the named node is waiting,
// injects payload into dataStore, marks the node completed, and triggers
// a DAG advance. Returns ErrWaitEventNotWaiting when the precondition
// fails so the HTTP layer can surface a structured 409.
func (r *WaitEventResumer) Resume(ctx context.Context, executionID uuid.UUID, nodeID string, payload map[string]any) error {
	if r == nil || r.ExecLookup == nil || r.NodeLookup == nil || r.NodeWriter == nil || r.Advancer == nil {
		return fmt.Errorf("wait_event resumer is not fully wired")
	}

	exec, err := r.ExecLookup.GetExecution(ctx, executionID)
	if err != nil {
		return fmt.Errorf("load execution: %w", err)
	}
	// The execution itself may be running (the node is waiting), but if
	// it is already terminal the resume is a no-op and surfaces a 409 to
	// the caller so the IM toast says "workflow completed".
	if exec.Status == model.WorkflowExecStatusCompleted ||
		exec.Status == model.WorkflowExecStatusFailed ||
		exec.Status == model.WorkflowExecStatusCancelled {
		return ErrWaitEventNotWaiting
	}

	nodeExecs, err := r.NodeLookup.ListNodeExecutions(ctx, executionID)
	if err != nil {
		return fmt.Errorf("list node executions: %w", err)
	}
	var target *model.WorkflowNodeExecution
	for _, ne := range nodeExecs {
		if ne.NodeID == nodeID && ne.Status == model.NodeExecWaiting {
			target = ne
			break
		}
	}
	if target == nil {
		return ErrWaitEventNotWaiting
	}

	// Merge the payload into the execution dataStore so downstream nodes
	// can reference {{$dataStore.<nodeID>.action_id}}.
	if r.DataStore != nil && payload != nil {
		_ = r.DataStore.MergeNodeResult(ctx, executionID, nodeID, payload)
	}

	// Persist the payload on the node row as well, so /runs/<id> trace
	// viewers can see what input woke the node.
	var resultBytes json.RawMessage
	if payload != nil {
		if b, err := json.Marshal(payload); err == nil {
			resultBytes = b
		}
	}
	if err := r.NodeWriter.UpdateNodeExecution(ctx, target.ID, model.NodeExecCompleted, resultBytes, ""); err != nil {
		return fmt.Errorf("update node execution: %w", err)
	}

	return r.Advancer.AdvanceExecution(ctx, executionID)
}
