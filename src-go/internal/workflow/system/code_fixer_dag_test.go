package system

import (
	"encoding/json"
	"testing"

	"github.com/react-go-quick-starter/server/internal/model"
)

func TestSeedCodeFixer_RegistersDefinition(t *testing.T) {
	def := CodeFixerDefinition()
	if def.Name != TemplateCodeFixer {
		t.Errorf("name = %q, want %q", def.Name, TemplateCodeFixer)
	}

	var nodes []model.WorkflowNode
	if err := json.Unmarshal(def.Nodes, &nodes); err != nil {
		t.Fatalf("unmarshal nodes: %v", err)
	}

	got := make([]string, len(nodes))
	for i, n := range nodes {
		got[i] = n.ID
	}
	for i, want := range CodeFixerNodes {
		if i >= len(got) {
			t.Fatalf("missing node at index %d: want %q", i, want)
		}
		if got[i] != want {
			t.Errorf("node[%d] = %q, want %q", i, got[i], want)
		}
	}
	if len(got) != len(CodeFixerNodes) {
		t.Errorf("node count = %d, want %d", len(got), len(CodeFixerNodes))
	}
}

func TestSeedCodeFixer_NodeTypesMatchSpec(t *testing.T) {
	def := CodeFixerDefinition()
	var nodes []model.WorkflowNode
	_ = json.Unmarshal(def.Nodes, &nodes)

	typeMap := make(map[string]string)
	for _, n := range nodes {
		typeMap[n.ID] = n.Type
	}

	expectations := map[string]string{
		"trigger":            model.NodeTypeTrigger,
		"fetch_file":        model.NodeTypeHTTPCall,
		"has_prebaked":      model.NodeTypeCondition,
		"generate":          model.NodeTypeLLMAgent,
		"validate":          model.NodeTypeFunction,
		"decide":            model.NodeTypeCondition,
		"execute":           model.NodeTypeHTTPCall,
		"update_original_pr": model.NodeTypeHTTPCall,
		"card":              model.NodeTypeIMSend,
	}

	for nodeID, wantType := range expectations {
		if gotType := typeMap[nodeID]; gotType != wantType {
			t.Errorf("node %q: type = %q, want %q", nodeID, gotType, wantType)
		}
	}

	// Verify generate node has roleId=default-code-fixer
	for _, n := range nodes {
		if n.ID == "generate" {
			var cfg map[string]any
			_ = json.Unmarshal(n.Config, &cfg)
			if cfg["roleId"] != "default-code-fixer" {
				t.Errorf("generate roleId = %v, want default-code-fixer", cfg["roleId"])
			}
		}
	}
}

func TestSeedCodeFixer_PrebakedShortCircuit(t *testing.T) {
	def := CodeFixerDefinition()
	var edges []model.WorkflowEdge
	_ = json.Unmarshal(def.Edges, &edges)

	// has_prebaked --true--> validate (skip generate)
	foundTrueToValidate := false
	foundFalseToGenerate := false
	for _, e := range edges {
		if e.Source == "has_prebaked" && e.Target == "validate" && e.Condition == "true" {
			foundTrueToValidate = true
		}
		if e.Source == "has_prebaked" && e.Target == "generate" && e.Condition == "false" {
			foundFalseToGenerate = true
		}
	}
	if !foundTrueToValidate {
		t.Error("missing edge: has_prebaked --true--> validate")
	}
	if !foundFalseToGenerate {
		t.Error("missing edge: has_prebaked --false--> generate")
	}
}

func TestSeedCodeFixer_Idempotent(t *testing.T) {
	def1 := CodeFixerDefinition()
	def2 := CodeFixerDefinition()
	if def1.Name != def2.Name {
		t.Error("definitions have different names")
	}
	// IDs differ (uuid.New()) but name is stable for upsert
	if def1.Version != def2.Version {
		t.Error("versions differ")
	}
}
