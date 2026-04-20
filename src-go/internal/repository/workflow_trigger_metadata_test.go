package repository

import (
	"testing"

	"github.com/react-go-quick-starter/server/internal/model"
)

func TestWorkflowTriggerRecord_CreatedViaAndDisplayMetadata(t *testing.T) {
	rec := workflowTriggerRecord{
		CreatedVia:  string(model.TriggerCreatedViaManual),
		DisplayName: "ping echo",
		Description: "demo trigger created via FE",
	}
	m := rec.toModel()
	if m.CreatedVia != model.TriggerCreatedViaManual {
		t.Errorf("created_via: got %q want %q", m.CreatedVia, model.TriggerCreatedViaManual)
	}
	if m.DisplayName != "ping echo" {
		t.Errorf("display_name: got %q want ping echo", m.DisplayName)
	}
	if m.Description != "demo trigger created via FE" {
		t.Errorf("description: got %q", m.Description)
	}

	rec2 := workflowTriggerRecord{} // empty created_via must default to dag_node
	if m2 := rec2.toModel(); m2.CreatedVia != model.TriggerCreatedViaDAGNode {
		t.Errorf("default created_via: got %q want %q", m2.CreatedVia, model.TriggerCreatedViaDAGNode)
	}
}
