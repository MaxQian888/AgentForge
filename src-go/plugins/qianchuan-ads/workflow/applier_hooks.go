package qcworkflow

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/agentforge/server/internal/model"
	nodetypes "github.com/agentforge/server/internal/workflow/nodetypes"
	"github.com/google/uuid"
)

// EffectHooks implements nodetypes.QianchuanEffectHooks by composing
// the qianchuan plugin's provider, secrets, persistence, and strategy
// dependencies. The plugin's Install() constructs one and wires it on
// to EffectApplier.Qianchuan so the generic workflow runtime can
// dispatch Spec 3D effects without knowing anything about qianchuan.
type EffectHooks struct {
	Provider   nodetypes.QianchuanProvider
	Secrets    nodetypes.QianchuanSecretsResolver
	Snapshots  nodetypes.QianchuanSnapshotRepo
	Actions    nodetypes.QianchuanActionLogRepo
	Strategies nodetypes.QianchuanStrategyLoader
	Evaluator  nodetypes.QianchuanStrategyEvaluator
	Bindings   nodetypes.QianchuanBindingLookup
}

// Compile-time contract check.
var _ nodetypes.QianchuanEffectHooks = (*EffectHooks)(nil)

// ── EffectFetchQianchuanMetrics applier ──────────────────────────────────

func (h *EffectHooks) FetchMetrics(
	ctx context.Context,
	merger nodetypes.WaitEventDataStoreMerger,
	exec *model.WorkflowExecution,
	node *model.WorkflowNode,
	raw json.RawMessage,
) error {
	if h.Provider == nil || h.Secrets == nil || h.Snapshots == nil || h.Bindings == nil {
		return fmt.Errorf("qianchuan: not configured")
	}
	if merger == nil {
		return fmt.Errorf("qianchuan: DataStoreMerger not configured")
	}

	var p nodetypes.FetchQianchuanMetricsPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	bindingID, err := uuid.Parse(p.BindingID)
	if err != nil {
		return fmt.Errorf("qianchuan: invalid binding_id %q: %w", p.BindingID, err)
	}

	binding, err := h.Bindings.Get(ctx, bindingID)
	if err != nil {
		return fmt.Errorf("qianchuan: binding lookup: %w", err)
	}

	tok, err := h.Secrets.Resolve(ctx, binding.ProjectID, binding.AccessTokenSecretRef)
	if err != nil {
		return fmt.Errorf("qianchuan: resolve token: %w", err)
	}

	ref := nodetypes.QianchuanBindingRef{
		AdvertiserID: binding.AdvertiserID,
		AwemeID:      binding.AwemeID,
		AccessToken:  tok,
	}

	snapshot, bucket, err := h.Provider.FetchMetrics(ctx, ref, p.Dimensions)
	if err != nil {
		return fmt.Errorf("qianchuan: fetch metrics: %w", err)
	}

	bucket = bucket.UTC().Truncate(time.Minute)

	if err := h.Snapshots.Upsert(ctx, bindingID, bucket, snapshot); err != nil {
		return fmt.Errorf("qianchuan: upsert snapshot: %w", err)
	}

	result := map[string]any{
		"snapshot": json.RawMessage(snapshot),
		"bucket":   bucket.Format(time.RFC3339),
	}
	if err := merger.MergeNodeResult(ctx, exec.ID, p.NodeID, result); err != nil {
		return fmt.Errorf("qianchuan: merge result: %w", err)
	}
	return nil
}

// ── EffectRunQianchuanStrategy applier ───────────────────────────────────

func (h *EffectHooks) RunStrategy(
	ctx context.Context,
	merger nodetypes.WaitEventDataStoreMerger,
	exec *model.WorkflowExecution,
	node *model.WorkflowNode,
	raw json.RawMessage,
) error {
	if h.Strategies == nil || h.Evaluator == nil || h.Actions == nil {
		return fmt.Errorf("qianchuan: not configured")
	}
	if merger == nil {
		return fmt.Errorf("qianchuan: DataStoreMerger not configured")
	}

	var p nodetypes.RunQianchuanStrategyPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	strategyID, err := uuid.Parse(p.StrategyID)
	if err != nil {
		return fmt.Errorf("qianchuan: invalid strategy_id %q: %w", p.StrategyID, err)
	}
	bindingID, err := uuid.Parse(p.BindingID)
	if err != nil {
		return fmt.Errorf("qianchuan: invalid binding_id %q: %w", p.BindingID, err)
	}

	parsedSpec, err := h.Strategies.Load(ctx, strategyID)
	if err != nil {
		return fmt.Errorf("qianchuan: load strategy: %w", err)
	}

	matches, evalErr := h.Evaluator.Evaluate(ctx, parsedSpec, p.SnapshotRef)
	if evalErr != nil {
		// Per spec §11 drift handling: treat unresolved templates as noop.
		noopLog := &model.QianchuanActionLog{
			BindingID:     bindingID,
			StrategyID:    &strategyID,
			StrategyRunID: uuid.New(),
			RuleName:      "record_event",
			ActionType:    "noop",
			Params:        json.RawMessage(`{}`),
			Status:        model.ActionLogStatusFailed,
			ErrorMessage:  evalErr.Error(),
		}
		_ = h.Actions.Create(ctx, noopLog)

		result := map[string]any{
			"strategy_run_id": noopLog.StrategyRunID.String(),
			"actions":         []any{},
		}
		_ = merger.MergeNodeResult(ctx, exec.ID, p.NodeID, result)
		return nil
	}

	runID := uuid.New()
	var actions []map[string]any

	for _, match := range matches {
		for _, action := range match.Actions {
			logRow := &model.QianchuanActionLog{
				BindingID:     bindingID,
				StrategyID:    &strategyID,
				StrategyRunID: runID,
				RuleName:      match.RuleName,
				ActionType:    action.Type,
				TargetAdID:    action.Target,
				Params:        action.Params,
				Status:        model.ActionLogStatusPending,
			}
			if err := h.Actions.Create(ctx, logRow); err != nil {
				return fmt.Errorf("qianchuan: create action log: %w", err)
			}
			actions = append(actions, map[string]any{
				"action_log_id": logRow.ID.String(),
				"action_type":   action.Type,
				"target":        action.Target,
			})
		}
	}

	result := map[string]any{
		"strategy_run_id": runID.String(),
		"actions":         actions,
	}
	if err := merger.MergeNodeResult(ctx, exec.ID, p.NodeID, result); err != nil {
		return fmt.Errorf("qianchuan: merge result: %w", err)
	}
	return nil
}

// ── EffectExecuteQianchuanAction applier ─────────────────────────────────

func (h *EffectHooks) ExecuteAction(
	ctx context.Context,
	merger nodetypes.WaitEventDataStoreMerger,
	exec *model.WorkflowExecution,
	node *model.WorkflowNode,
	raw json.RawMessage,
) error {
	if h.Provider == nil || h.Secrets == nil || h.Actions == nil || h.Bindings == nil {
		return fmt.Errorf("qianchuan: not configured")
	}
	if merger == nil {
		return fmt.Errorf("qianchuan: DataStoreMerger not configured")
	}

	var p nodetypes.ExecuteQianchuanActionPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	actionLogID, err := uuid.Parse(p.ActionLogID)
	if err != nil {
		return fmt.Errorf("qianchuan: invalid action_log_id %q: %w", p.ActionLogID, err)
	}

	logRow, err := h.Actions.GetByID(ctx, actionLogID)
	if err != nil {
		return fmt.Errorf("qianchuan: get action log: %w", err)
	}
	if logRow == nil {
		return fmt.Errorf("qianchuan: action log %s not found", actionLogID)
	}

	binding, err := h.Bindings.Get(ctx, logRow.BindingID)
	if err != nil {
		return fmt.Errorf("qianchuan: binding lookup: %w", err)
	}

	tok, err := h.Secrets.Resolve(ctx, binding.ProjectID, binding.AccessTokenSecretRef)
	if err != nil {
		_ = h.Actions.MarkFailed(ctx, actionLogID, fmt.Sprintf("resolve token: %v", err))
		result := map[string]any{"success": false, "error": fmt.Sprintf("resolve token: %v", err)}
		_ = merger.MergeNodeResult(ctx, exec.ID, p.NodeID, result)
		return nil
	}

	ref := nodetypes.QianchuanBindingRef{
		AdvertiserID: binding.AdvertiserID,
		AwemeID:      binding.AwemeID,
		AccessToken:  tok,
	}

	actionReq := nodetypes.QianchuanActionRequest{
		Kind:       logRow.ActionType,
		TargetAdID: logRow.TargetAdID,
		Params:     logRow.Params,
	}

	if err := h.Provider.ApplyAction(ctx, ref, actionReq); err != nil {
		_ = h.Actions.MarkFailed(ctx, actionLogID, err.Error())
		result := map[string]any{"success": false, "error": err.Error()}
		_ = merger.MergeNodeResult(ctx, exec.ID, p.NodeID, result)
		return nil
	}

	_ = h.Actions.MarkApplied(ctx, actionLogID)
	result := map[string]any{"success": true}
	if err := merger.MergeNodeResult(ctx, exec.ID, p.NodeID, result); err != nil {
		return fmt.Errorf("qianchuan: merge result: %w", err)
	}
	return nil
}
