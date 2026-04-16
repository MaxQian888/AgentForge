package nodetypes

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestSubWorkflowHandler_ExecuteReturnsNotImplemented(t *testing.T) {
	h := SubWorkflowHandler{}
	ctx := context.Background()

	result, err := h.Execute(ctx, &NodeExecRequest{})
	if err == nil {
		t.Fatal("Execute() returned nil error, want non-nil")
	}
	if !strings.Contains(err.Error(), "sub_workflow not implemented") {
		t.Errorf("Execute() error = %q, want it to contain %q", err.Error(), "sub_workflow not implemented")
	}
	if result != nil {
		t.Errorf("Execute() result = %v, want nil", result)
	}
}

func TestSubWorkflowHandler_Capabilities(t *testing.T) {
	h := SubWorkflowHandler{}
	caps := h.Capabilities()
	if len(caps) != 1 {
		t.Fatalf("Capabilities() length = %d, want 1", len(caps))
	}
	if caps[0] != EffectInvokeSubWorkflow {
		t.Errorf("Capabilities()[0] = %s, want %s", caps[0], EffectInvokeSubWorkflow)
	}
}

func TestSubWorkflowHandler_ConfigSchemaIsValidJSON(t *testing.T) {
	h := SubWorkflowHandler{}
	schema := h.ConfigSchema()
	if len(schema) == 0 {
		t.Fatal("ConfigSchema() returned empty, want non-empty schema")
	}
	if !json.Valid(schema) {
		t.Errorf("ConfigSchema() returned invalid JSON: %s", schema)
	}
}
