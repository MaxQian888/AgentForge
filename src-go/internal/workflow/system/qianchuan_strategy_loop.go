// Package system provides canonical system workflow definitions that are
// seeded into the workflow_definitions table at server startup.
//
// This file defines the system:qianchuan_strategy_loop DAG — the canonical
// strategy execution loop for the e-commerce streaming digital employee.
//
// Spec 3E inserts a `qianchuan_policy_gate` node on the `run_strategy →
// actions_loop` edge; do not add it here.
package system

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

// QianchuanStrategyLoopName is the canonical name for the strategy loop.
const QianchuanStrategyLoopName = "system:qianchuan_strategy_loop"

// QianchuanStrategyLoopNodes returns the 7 node definitions for the DAG.
func QianchuanStrategyLoopNodes() []model.WorkflowNode {
	return []model.WorkflowNode{
		{
			ID:   "trigger",
			Type: "trigger",
			Label: "Schedule Trigger",
			Position: model.WorkflowPos{X: 0, Y: 0},
			Config: mustJSON(map[string]any{"source": "schedule"}),
		},
		{
			ID:   "fetch_metrics",
			Type: "qianchuan_metrics_fetcher",
			Label: "Fetch Metrics",
			Position: model.WorkflowPos{X: 200, Y: 0},
			Config: mustJSON(map[string]any{
				"binding_id_template": "{{$context.binding_id}}",
				"dimensions":          []string{"ads", "live", "materials"},
			}),
		},
		{
			ID:   "run_strategy",
			Type: "qianchuan_strategy_runner",
			Label: "Run Strategy",
			Position: model.WorkflowPos{X: 400, Y: 0},
			Config: mustJSON(map[string]any{
				"strategy_id_template": "{{$context.strategy_id}}",
				"snapshot_ref":         "{{$dataStore.fetch_metrics.snapshot}}",
				"binding_id_template":  "{{$context.binding_id}}",
			}),
		},
		{
			ID:   "has_actions",
			Type: "condition",
			Label: "Has Actions?",
			Position: model.WorkflowPos{X: 600, Y: 0},
			Config: mustJSON(map[string]any{
				"expression": "len($dataStore.run_strategy.actions) > 0",
			}),
		},
		{
			ID:   "actions_loop",
			Type: "loop",
			Label: "Actions Loop",
			Position: model.WorkflowPos{X: 800, Y: -50},
			Config: mustJSON(map[string]any{
				"target_node":    "execute_action",
				"max_iterations": 64,
				"exit_condition": "$dataStore.actions_loop._iter >= len($dataStore.run_strategy.actions)",
			}),
		},
		{
			ID:   "execute_action",
			Type: "qianchuan_action_executor",
			Label: "Execute Action",
			Position: model.WorkflowPos{X: 1000, Y: -50},
			Config: mustJSON(map[string]any{
				"action_log_id_template": "{{$dataStore.run_strategy.actions[$dataStore.actions_loop._iter].action_log_id}}",
				"binding_id_template":    "{{$context.binding_id}}",
			}),
		},
		{
			ID:   "summary_card",
			Type: "im_send",
			Label: "Summary Card",
			Position: model.WorkflowPos{X: 1200, Y: 0},
			Config: mustJSON(map[string]any{
				"target": "reply_to_trigger",
				"card": map[string]any{
					"title":   "Qianchuan Strategy Run Summary",
					"summary": "Run {{$dataStore.run_strategy.strategy_run_id}} completed.",
				},
			}),
		},
	}
}

// QianchuanStrategyLoopEdges returns the DAG edges.
func QianchuanStrategyLoopEdges() []model.WorkflowEdge {
	return []model.WorkflowEdge{
		{ID: "e1", Source: "trigger", Target: "fetch_metrics"},
		{ID: "e2", Source: "fetch_metrics", Target: "run_strategy"},
		{ID: "e3", Source: "run_strategy", Target: "has_actions"},
		{ID: "e4", Source: "has_actions", Target: "actions_loop", Condition: "true", Label: "has actions"},
		{ID: "e5", Source: "has_actions", Target: "summary_card", Condition: "false", Label: "no actions"},
		{ID: "e6", Source: "actions_loop", Target: "execute_action"},
		{ID: "e7", Source: "execute_action", Target: "actions_loop", Label: "loop back"},
		{ID: "e8", Source: "actions_loop", Target: "summary_card", Condition: "exit", Label: "loop done"},
	}
}

// QianchuanStrategyLoopDefinition builds the full WorkflowDefinition struct.
func QianchuanStrategyLoopDefinition() *model.WorkflowDefinition {
	nodes, _ := json.Marshal(QianchuanStrategyLoopNodes())
	edges, _ := json.Marshal(QianchuanStrategyLoopEdges())
	return &model.WorkflowDefinition{
		ID:          uuid.MustParse("00000000-0000-4000-8000-000000000003"),
		Name:        QianchuanStrategyLoopName,
		Description: "System-seeded DAG: per-binding strategy execution loop (Spec 3D). Spec 3E inserts policy_gate on run_strategy→actions_loop edge.",
		Status:      model.WorkflowDefStatusActive,
		Category:    model.WorkflowCategorySystem,
		Nodes:       nodes,
		Edges:       edges,
		Version:     1,
	}
}

// DefRepo is the subset of the workflow definition repository used for seeding.
type DefRepo interface {
	GetByName(ctx context.Context, name string) (*model.WorkflowDefinition, error)
	Create(ctx context.Context, def *model.WorkflowDefinition) error
	Update(ctx context.Context, def *model.WorkflowDefinition) error
}

// SeedQianchuanStrategyLoop upserts the canonical workflow definition.
// It is idempotent: creates on first boot, updates nodes/edges on subsequent
// boots if the seed has evolved.
func SeedQianchuanStrategyLoop(ctx context.Context, repo DefRepo) error {
	def := QianchuanStrategyLoopDefinition()

	existing, err := repo.GetByName(ctx, QianchuanStrategyLoopName)
	if err != nil {
		// Assume not-found means we should create.
		def.CreatedAt = time.Now().UTC()
		def.UpdatedAt = def.CreatedAt
		return repo.Create(ctx, def)
	}

	// Update nodes and edges to keep the seed authoritative.
	existing.Nodes = def.Nodes
	existing.Edges = def.Edges
	existing.Description = def.Description
	existing.Version = def.Version
	existing.UpdatedAt = time.Now().UTC()
	return repo.Update(ctx, existing)
}

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
