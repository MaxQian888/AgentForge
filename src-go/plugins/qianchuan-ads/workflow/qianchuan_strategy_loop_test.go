package qcworkflow

import (
	"encoding/json"
	"testing"

	"github.com/agentforge/server/internal/model"
)

func TestQianchuanStrategyLoopDefinition_Has7Nodes(t *testing.T) {
	def := QianchuanStrategyLoopDefinition()

	var nodes []model.WorkflowNode
	if err := json.Unmarshal(def.Nodes, &nodes); err != nil {
		t.Fatalf("unmarshal nodes: %v", err)
	}
	if len(nodes) != 7 {
		t.Fatalf("expected 7 nodes, got %d", len(nodes))
	}

	expectedIDs := map[string]bool{
		"trigger": true, "fetch_metrics": true, "run_strategy": true,
		"has_actions": true, "actions_loop": true, "execute_action": true,
		"summary_card": true,
	}
	for _, n := range nodes {
		if !expectedIDs[n.ID] {
			t.Errorf("unexpected node id %q", n.ID)
		}
		delete(expectedIDs, n.ID)
	}
	if len(expectedIDs) > 0 {
		t.Errorf("missing node ids: %v", expectedIDs)
	}
}

func TestQianchuanStrategyLoopDefinition_EdgesFormDAG(t *testing.T) {
	def := QianchuanStrategyLoopDefinition()

	var edges []model.WorkflowEdge
	if err := json.Unmarshal(def.Edges, &edges); err != nil {
		t.Fatalf("unmarshal edges: %v", err)
	}
	if len(edges) != 8 {
		t.Fatalf("expected 8 edges, got %d", len(edges))
	}

	// Verify trigger → fetch_metrics → run_strategy → has_actions path exists.
	edgeSet := make(map[string]bool)
	for _, e := range edges {
		edgeSet[e.Source+"→"+e.Target] = true
	}
	required := []string{
		"trigger→fetch_metrics",
		"fetch_metrics→run_strategy",
		"run_strategy→has_actions",
		"has_actions→actions_loop",
		"actions_loop→execute_action",
		"execute_action→actions_loop",
		"actions_loop→summary_card",
	}
	for _, r := range required {
		if !edgeSet[r] {
			t.Errorf("missing required edge %q", r)
		}
	}
}

func TestQianchuanStrategyLoopDefinition_IsSystemCategory(t *testing.T) {
	def := QianchuanStrategyLoopDefinition()
	if def.Category != model.WorkflowCategorySystem {
		t.Errorf("category = %q, want %q", def.Category, model.WorkflowCategorySystem)
	}
	if def.Status != model.WorkflowDefStatusActive {
		t.Errorf("status = %q, want %q", def.Status, model.WorkflowDefStatusActive)
	}
}

func TestQianchuanStrategyLoopDefinition_ContextTemplatesReferenceContext(t *testing.T) {
	nodes := QianchuanStrategyLoopNodes()
	fetchNode := findNode(nodes, "fetch_metrics")
	if fetchNode == nil {
		t.Fatal("fetch_metrics node not found")
	}
	var cfg map[string]any
	_ = json.Unmarshal(fetchNode.Config, &cfg)
	tpl, _ := cfg["binding_id_template"].(string)
	if tpl != "{{$context.binding_id}}" {
		t.Errorf("fetch_metrics binding_id_template = %q, want {{$context.binding_id}}", tpl)
	}

	runNode := findNode(nodes, "run_strategy")
	if runNode == nil {
		t.Fatal("run_strategy node not found")
	}
	_ = json.Unmarshal(runNode.Config, &cfg)
	tpl, _ = cfg["strategy_id_template"].(string)
	if tpl != "{{$context.strategy_id}}" {
		t.Errorf("run_strategy strategy_id_template = %q, want {{$context.strategy_id}}", tpl)
	}
}

func findNode(nodes []model.WorkflowNode, id string) *model.WorkflowNode {
	for i := range nodes {
		if nodes[i].ID == id {
			return &nodes[i]
		}
	}
	return nil
}
