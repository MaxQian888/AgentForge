package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/workflow/nodetypes"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
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

type activeWorkflowExecutionLister interface {
	ListActiveExecutions(ctx context.Context) ([]*model.WorkflowExecution, error)
}

func (s *DAGWorkflowService) SweepExpiredWaitEvents(ctx context.Context, now time.Time) (int, error) {
	if s == nil || s.defRepo == nil || s.execRepo == nil || s.nodeRepo == nil {
		return 0, fmt.Errorf("wait_event sweeper is not fully wired")
	}
	lister, ok := s.execRepo.(activeWorkflowExecutionLister)
	if !ok {
		return 0, fmt.Errorf("wait_event sweeper requires active execution listing")
	}

	execs, err := lister.ListActiveExecutions(ctx)
	if err != nil {
		return 0, fmt.Errorf("list active executions: %w", err)
	}

	expiredCount := 0
	for _, exec := range execs {
		if exec == nil {
			continue
		}
		def, err := s.defRepo.GetByID(ctx, exec.WorkflowID)
		if err != nil || def == nil {
			continue
		}
		timeoutByNode := waitEventTimeoutsByNode(def.Nodes)
		if len(timeoutByNode) == 0 {
			continue
		}

		nodeExecs, err := s.nodeRepo.ListNodeExecutions(ctx, exec.ID)
		if err != nil {
			continue
		}
		for _, nodeExec := range nodeExecs {
			if nodeExec == nil || nodeExec.Status != model.NodeExecWaiting {
				continue
			}
			timeout, ok := timeoutByNode[nodeExec.NodeID]
			if !ok || timeout <= 0 {
				continue
			}
			startedAt := nodeExec.CreatedAt
			if nodeExec.StartedAt != nil && !nodeExec.StartedAt.IsZero() {
				startedAt = nodeExec.StartedAt.UTC()
			}
			if now.UTC().Before(startedAt.Add(timeout)) {
				continue
			}

			msg := fmt.Sprintf("wait_event timed out after %s", timeout)
			_ = s.nodeRepo.UpdateNodeExecution(ctx, nodeExec.ID, model.NodeExecFailed, nil, msg)
			_ = s.execRepo.UpdateExecution(ctx, exec.ID, model.WorkflowExecStatusFailed, nil, msg)
			exec.Status = model.WorkflowExecStatusFailed
			exec.ErrorMessage = msg
			s.runEmitter.EmitDAGStatusChanged(ctx, exec)
			s.runEmitter.EmitDAGTerminal(ctx, exec)
			log.WithFields(log.Fields{
				"executionId": exec.ID.String(),
				"workflowId":  exec.WorkflowID.String(),
				"nodeId":      nodeExec.NodeID,
				"timeout":     timeout.String(),
			}).Warn("wait_event timed out")
			expiredCount++
			break
		}
	}
	return expiredCount, nil
}

func (s *DAGWorkflowService) RunWaitEventTimeoutSweeper(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Minute
	}
	_, _ = s.SweepExpiredWaitEvents(ctx, time.Now().UTC())
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case tickAt := <-ticker.C:
			if _, err := s.SweepExpiredWaitEvents(ctx, tickAt.UTC()); err != nil {
				log.WithError(err).Warn("wait_event timeout sweep failed")
			}
		}
	}
}

func waitEventTimeoutsByNode(raw json.RawMessage) map[string]time.Duration {
	if len(raw) == 0 {
		return nil
	}
	var nodes []model.WorkflowNode
	if err := json.Unmarshal(raw, &nodes); err != nil {
		return nil
	}
	out := make(map[string]time.Duration)
	for _, node := range nodes {
		if node.Type != model.NodeTypeWaitEvent || len(node.Config) == 0 {
			continue
		}
		var cfg struct {
			TimeoutSeconds float64 `json:"timeout_seconds"`
		}
		if err := json.Unmarshal(node.Config, &cfg); err != nil {
			continue
		}
		if cfg.TimeoutSeconds > 0 {
			out[node.ID] = time.Duration(cfg.TimeoutSeconds) * time.Second
		}
	}
	return out
}
