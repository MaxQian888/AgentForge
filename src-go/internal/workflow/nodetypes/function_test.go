package nodetypes

import (
	"context"
	"encoding/json"
	"testing"
)

func TestFunctionHandler_EmptyConfig_ReturnsNull(t *testing.T) {
	h := FunctionHandler{}
	result, err := h.Execute(context.Background(), &NodeExecRequest{Config: map[string]any{}})
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if result == nil || string(result.Result) != "null" {
		t.Errorf("Execute() result = %v, want null JSON", result)
	}
	if len(result.Effects) != 0 {
		t.Errorf("Execute() effects = %v, want none", result.Effects)
	}
	assertCapsCoverEffects(t, h, result.Effects)
}

func TestFunctionHandler_PassThroughInput(t *testing.T) {
	h := FunctionHandler{}
	cfg := map[string]any{
		"input": map[string]any{"foo": "bar", "n": float64(2)},
	}
	result, err := h.Execute(context.Background(), &NodeExecRequest{Config: cfg})
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(result.Result, &got); err != nil {
		t.Fatalf("result not valid JSON: %v", err)
	}
	if got["foo"] != "bar" {
		t.Errorf("expected foo=bar, got %v", got["foo"])
	}
	if got["n"] != float64(2) {
		t.Errorf("expected n=2, got %v", got["n"])
	}
	assertCapsCoverEffects(t, h, result.Effects)
}

func TestFunctionHandler_TemplateResolution(t *testing.T) {
	h := FunctionHandler{}
	cfg := map[string]any{
		"expression": "{{node1.output.greeting}}",
	}
	ds := map[string]any{
		"node1": map[string]any{
			"output": map[string]any{"greeting": "hi"},
		},
	}
	result, err := h.Execute(context.Background(), &NodeExecRequest{Config: cfg, DataStore: ds})
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	// "hi" is not valid JSON on its own (no quotes), so it goes through
	// EvaluateExpression which returns it as a string, then Marshal wraps it.
	if string(result.Result) != `"hi"` {
		t.Errorf("Execute() result = %s, want \"hi\"", result.Result)
	}
	assertCapsCoverEffects(t, h, result.Effects)
}

func TestFunctionHandler_JSONPassthrough(t *testing.T) {
	h := FunctionHandler{}
	cfg := map[string]any{
		"expression": `{"a":1,"b":[true,false]}`,
	}
	result, err := h.Execute(context.Background(), &NodeExecRequest{Config: cfg})
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if !json.Valid(result.Result) {
		t.Fatalf("expected valid JSON, got %s", result.Result)
	}
	var got map[string]any
	if err := json.Unmarshal(result.Result, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got["a"] != float64(1) {
		t.Errorf("expected a=1, got %v", got["a"])
	}
	assertCapsCoverEffects(t, h, result.Effects)
}

func TestFunctionHandler_ExpressionEvaluation(t *testing.T) {
	h := FunctionHandler{}
	cfg := map[string]any{
		"expression": "len(items)",
	}
	ds := map[string]any{
		"items": []any{"a", "b", "c", "d"},
	}
	result, err := h.Execute(context.Background(), &NodeExecRequest{Config: cfg, DataStore: ds})
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if string(result.Result) != "4" {
		t.Errorf("Execute() result = %s, want 4", result.Result)
	}
	assertCapsCoverEffects(t, h, result.Effects)
}

func TestFunctionHandler_ConfigSchema(t *testing.T) {
	h := FunctionHandler{}
	schema := h.ConfigSchema()
	if len(schema) == 0 {
		t.Fatal("ConfigSchema() returned empty")
	}
	if !json.Valid(schema) {
		t.Errorf("ConfigSchema() returned invalid JSON: %s", schema)
	}
}

func TestFunctionHandler_CapabilitiesNil(t *testing.T) {
	h := FunctionHandler{}
	if caps := h.Capabilities(); len(caps) != 0 {
		t.Errorf("Capabilities() = %v, want nil/empty", caps)
	}
}
