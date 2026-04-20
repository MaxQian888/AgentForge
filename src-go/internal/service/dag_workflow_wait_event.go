package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/agentforge/server/internal/workflow/nodetypes"
	"github.com/google/uuid"
)

// WaitEventDataStoreAdapter merges a resume payload into the parent
// execution's dataStore as `dataStore[nodeID] = payload`. Backed by the
// execRepo's UpdateExecutionDataStore through a load-merge-save cycle so
// the resumer does not need to know the storage shape.
type WaitEventDataStoreAdapter struct {
	Repo DAGWorkflowExecutionRepo
}

func (a *WaitEventDataStoreAdapter) MergeNodeResult(ctx context.Context, executionID uuid.UUID, nodeID string, payload map[string]any) error {
	if a == nil || a.Repo == nil {
		return fmt.Errorf("wait_event datastore adapter not configured")
	}
	exec, err := a.Repo.GetExecution(ctx, executionID)
	if err != nil {
		return err
	}
	ds := map[string]any{}
	if len(exec.DataStore) > 0 {
		_ = json.Unmarshal(exec.DataStore, &ds)
	}
	ds[nodeID] = payload
	encoded, err := json.Marshal(ds)
	if err != nil {
		return err
	}
	return a.Repo.UpdateExecutionDataStore(ctx, executionID, encoded)
}

// WaitEventResumer returns a fully wired resumer that the IM card-action
// router can call to wake parked wait_event nodes.
func (s *DAGWorkflowService) WaitEventResumer() *nodetypes.WaitEventResumer {
	return &nodetypes.WaitEventResumer{
		ExecLookup: s.execRepo,
		NodeLookup: s.nodeRepo,
		NodeWriter: s.nodeRepo,
		DataStore:  &WaitEventDataStoreAdapter{Repo: s.execRepo},
		Advancer:   s,
	}
}
