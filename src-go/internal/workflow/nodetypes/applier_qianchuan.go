package nodetypes

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

// ── EffectFetchQianchuanMetrics applier ──────────────────────────────────

func (a *EffectApplier) applyFetchQianchuanMetrics(ctx context.Context, exec *model.WorkflowExecution, node *model.WorkflowNode, raw json.RawMessage) error {
	if a.QianchuanProvider == nil || a.QianchuanSecrets == nil || a.QianchuanSnapshots == nil || a.QianchuanBindings == nil {
		return fmt.Errorf("qianchuan: not configured")
	}
	if a.DataStoreMerger == nil {
		return fmt.Errorf("qianchuan: DataStoreMerger not configured")
	}

	var p FetchQianchuanMetricsPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	bindingID, err := uuid.Parse(p.BindingID)
	if err != nil {
		return fmt.Errorf("qianchuan: invalid binding_id %q: %w", p.BindingID, err)
	}

	binding, err := a.QianchuanBindings.Get(ctx, bindingID)
	if err != nil {
		return fmt.Errorf("qianchuan: binding lookup: %w", err)
	}

	tok, err := a.QianchuanSecrets.Resolve(ctx, binding.ProjectID, binding.AccessTokenSecretRef)
	if err != nil {
		return fmt.Errorf("qianchuan: resolve token: %w", err)
	}

	ref := QianchuanBindingRef{
		AdvertiserID: binding.AdvertiserID,
		AwemeID:      binding.AwemeID,
		AccessToken:  tok,
	}

	snapshot, bucket, err := a.QianchuanProvider.FetchMetrics(ctx, ref, p.Dimensions)
	if err != nil {
		return fmt.Errorf("qianchuan: fetch metrics: %w", err)
	}

	bucket = bucket.UTC().Truncate(time.Minute)

	if err := a.QianchuanSnapshots.Upsert(ctx, bindingID, bucket, snapshot); err != nil {
		return fmt.Errorf("qianchuan: upsert snapshot: %w", err)
	}

	result := map[string]any{
		"snapshot": json.RawMessage(snapshot),
		"bucket":   bucket.Format(time.RFC3339),
	}
	if err := a.DataStoreMerger.MergeNodeResult(ctx, exec.ID, p.NodeID, result); err != nil {
		return fmt.Errorf("qianchuan: merge result: %w", err)
	}

	return nil
}

// ── EffectRunQianchuanStrategy applier ───────────────────────────────────

func (a *EffectApplier) applyRunQianchuanStrategy(ctx context.Context, exec *model.WorkflowExecution, node *model.WorkflowNode, raw json.RawMessage) error {
	if a.QianchuanStrategies == nil || a.QianchuanEvaluator == nil || a.QianchuanActions == nil {
		return fmt.Errorf("qianchuan: not configured")
	}
	if a.DataStoreMerger == nil {
		return fmt.Errorf("qianchuan: DataStoreMerger not configured")
	}

	var p RunQianchuanStrategyPayload
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

	parsedSpec, err := a.QianchuanStrategies.Load(ctx, strategyID)
	if err != nil {
		return fmt.Errorf("qianchuan: load strategy: %w", err)
	}

	matches, evalErr := a.QianchuanEvaluator.Evaluate(ctx, parsedSpec, p.SnapshotRef)
	if evalErr != nil {
		// Per spec §11 drift handling: treat unresolved templates as noop.
		// Write a record_event action_log row with status='failed'.
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
		_ = a.QianchuanActions.Create(ctx, noopLog)

		// Emit empty actions — do NOT short-circuit the DAG.
		result := map[string]any{
			"strategy_run_id": noopLog.StrategyRunID.String(),
			"actions":         []any{},
		}
		_ = a.DataStoreMerger.MergeNodeResult(ctx, exec.ID, p.NodeID, result)
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
			if err := a.QianchuanActions.Create(ctx, logRow); err != nil {
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
	if err := a.DataStoreMerger.MergeNodeResult(ctx, exec.ID, p.NodeID, result); err != nil {
		return fmt.Errorf("qianchuan: merge result: %w", err)
	}

	return nil
}

// ── EffectExecuteQianchuanAction applier ─────────────────────────────────

func (a *EffectApplier) applyExecuteQianchuanAction(ctx context.Context, exec *model.WorkflowExecution, node *model.WorkflowNode, raw json.RawMessage) error {
	if a.QianchuanProvider == nil || a.QianchuanSecrets == nil || a.QianchuanActions == nil || a.QianchuanBindings == nil {
		return fmt.Errorf("qianchuan: not configured")
	}
	if a.DataStoreMerger == nil {
		return fmt.Errorf("qianchuan: DataStoreMerger not configured")
	}

	var p ExecuteQianchuanActionPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	actionLogID, err := uuid.Parse(p.ActionLogID)
	if err != nil {
		return fmt.Errorf("qianchuan: invalid action_log_id %q: %w", p.ActionLogID, err)
	}

	logRow, err := a.QianchuanActions.GetByID(ctx, actionLogID)
	if err != nil {
		return fmt.Errorf("qianchuan: get action log: %w", err)
	}
	if logRow == nil {
		return fmt.Errorf("qianchuan: action log %s not found", actionLogID)
	}

	binding, err := a.QianchuanBindings.Get(ctx, logRow.BindingID)
	if err != nil {
		return fmt.Errorf("qianchuan: binding lookup: %w", err)
	}

	tok, err := a.QianchuanSecrets.Resolve(ctx, binding.ProjectID, binding.AccessTokenSecretRef)
	if err != nil {
		// Mark failed and continue.
		_ = a.QianchuanActions.MarkFailed(ctx, actionLogID, fmt.Sprintf("resolve token: %v", err))
		result := map[string]any{"success": false, "error": fmt.Sprintf("resolve token: %v", err)}
		_ = a.DataStoreMerger.MergeNodeResult(ctx, exec.ID, p.NodeID, result)
		return nil
	}

	ref := QianchuanBindingRef{
		AdvertiserID: binding.AdvertiserID,
		AwemeID:      binding.AwemeID,
		AccessToken:  tok,
	}

	actionReq := QianchuanActionRequest{
		Kind:       logRow.ActionType,
		TargetAdID: logRow.TargetAdID,
		Params:     logRow.Params,
	}

	applyErr := a.QianchuanProvider.ApplyAction(ctx, ref, actionReq)
	if applyErr != nil {
		_ = a.QianchuanActions.MarkFailed(ctx, actionLogID, applyErr.Error())
		result := map[string]any{"success": false, "error": applyErr.Error()}
		_ = a.DataStoreMerger.MergeNodeResult(ctx, exec.ID, p.NodeID, result)
		return nil // Don't fail the DAG for sibling actions.
	}

	_ = a.QianchuanActions.MarkApplied(ctx, actionLogID)
	result := map[string]any{"success": true}
	if err := a.DataStoreMerger.MergeNodeResult(ctx, exec.ID, p.NodeID, result); err != nil {
		return fmt.Errorf("qianchuan: merge result: %w", err)
	}
	return nil
}
