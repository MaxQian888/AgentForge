package nodetypes

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
)

func newSubWorkflowNode(t *testing.T, config map[string]any) *NodeExecRequest {
	t.Helper()
	var raw json.RawMessage
	if config != nil {
		var err error
		raw, err = json.Marshal(config)
		if err != nil {
			t.Fatalf("marshal config: %v", err)
		}
	}
	return &NodeExecRequest{
		Node: &model.WorkflowNode{
			ID:     "sub-1",
			Type:   "sub_workflow",
			Config: raw,
		},
		NodeExecID: uuid.New(),
		ProjectID:  uuid.New(),
	}
}

func TestSubWorkflowHandler_Execute_EmitsEffect(t *testing.T) {
	h := SubWorkflowHandler{}
	req := newSubWorkflowNode(t, map[string]any{
		"targetKind":        "dag",
		"targetWorkflowId":  "11111111-1111-1111-1111-111111111111",
		"inputMapping":      map[string]any{"task_id": "{{$parent.dataStore.upstream.taskId}}"},
		"waitForCompletion": true,
	})
	result, err := h.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if result == nil || len(result.Effects) != 1 {
		t.Fatalf("Execute() result = %+v, want exactly one effect", result)
	}
	if result.Effects[0].Kind != EffectInvokeSubWorkflow {
		t.Errorf("effect kind = %s, want %s", result.Effects[0].Kind, EffectInvokeSubWorkflow)
	}
	var payload InvokeSubWorkflowPayload
	if err := json.Unmarshal(result.Effects[0].Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.TargetKind != SubWorkflowTargetDAG {
		t.Errorf("payload.TargetKind = %s, want dag", payload.TargetKind)
	}
	if payload.TargetWorkflowID != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("payload.TargetWorkflowID = %s, want the given UUID", payload.TargetWorkflowID)
	}
	if !payload.WaitForCompletion {
		t.Errorf("payload.WaitForCompletion = false, want true")
	}
	if len(payload.InputMapping) == 0 {
		t.Errorf("payload.InputMapping is empty, want non-empty")
	}
}

func TestSubWorkflowHandler_Execute_DefaultsTargetKindToDAG(t *testing.T) {
	h := SubWorkflowHandler{}
	req := newSubWorkflowNode(t, map[string]any{
		"targetWorkflowId": "some-id",
	})
	result, err := h.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	var payload InvokeSubWorkflowPayload
	_ = json.Unmarshal(result.Effects[0].Payload, &payload)
	if payload.TargetKind != SubWorkflowTargetDAG {
		t.Errorf("default TargetKind = %s, want dag", payload.TargetKind)
	}
	if !payload.WaitForCompletion {
		t.Errorf("default WaitForCompletion = false, want true")
	}
}

func TestSubWorkflowHandler_Execute_MissingTargetRejected(t *testing.T) {
	h := SubWorkflowHandler{}
	req := newSubWorkflowNode(t, map[string]any{
		"targetKind": "dag",
	})
	_, err := h.Execute(context.Background(), req)
	if err == nil {
		t.Fatal("Execute() returned nil error, want missing-target error")
	}
	if !strings.Contains(err.Error(), "targetWorkflowId") {
		t.Errorf("Execute() error = %q, want contains \"targetWorkflowId\"", err.Error())
	}
}

func TestSubWorkflowHandler_Execute_UnknownTargetKindRejected(t *testing.T) {
	h := SubWorkflowHandler{}
	req := newSubWorkflowNode(t, map[string]any{
		"targetKind":       "lambda",
		"targetWorkflowId": "some-id",
	})
	_, err := h.Execute(context.Background(), req)
	if err == nil {
		t.Fatal("Execute() returned nil error, want unknown-kind error")
	}
	if !strings.Contains(err.Error(), "unknown targetKind") {
		t.Errorf("Execute() error = %q, want contains \"unknown targetKind\"", err.Error())
	}
}

func TestSubWorkflowHandler_Execute_InvalidInputMappingRejected(t *testing.T) {
	h := SubWorkflowHandler{}
	req := &NodeExecRequest{
		Node: &model.WorkflowNode{
			ID:   "sub-1",
			Type: "sub_workflow",
			Config: json.RawMessage(
				`{"targetKind":"dag","targetWorkflowId":"id","inputMapping": {unclosed}`,
			),
		},
	}
	_, err := h.Execute(context.Background(), req)
	if err == nil {
		t.Fatal("Execute() returned nil error, want invalid-json error")
	}
}

func TestSubWorkflowHandler_Execute_LegacyWorkflowIDFallback(t *testing.T) {
	// Hand-authored legacy JSON using only workflowId (no targetWorkflowId).
	h := SubWorkflowHandler{}
	req := newSubWorkflowNode(t, map[string]any{
		"workflowId": "legacy-id",
	})
	result, err := h.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil for legacy-shape config", err)
	}
	var payload InvokeSubWorkflowPayload
	_ = json.Unmarshal(result.Effects[0].Payload, &payload)
	if payload.TargetWorkflowID != "legacy-id" {
		t.Errorf("legacy workflowId -> TargetWorkflowID = %q, want \"legacy-id\"", payload.TargetWorkflowID)
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
