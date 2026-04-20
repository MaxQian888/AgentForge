package nodetypes

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

func TestHTTPCallHandler_EmitsExecuteEffect(t *testing.T) {
	h := HTTPCallHandler{}
	result, err := h.Execute(context.Background(), &NodeExecRequest{
		Execution: &model.WorkflowExecution{ID: uuid.New(), ProjectID: uuid.New()},
		Node:      &model.WorkflowNode{ID: "http-1"},
		Config: map[string]any{
			"method": "POST",
			"url":    "https://api.example.com/v1/things",
			"headers": map[string]any{
				"Authorization": "Bearer {{secrets.GITHUB_TOKEN}}",
				"Content-Type":  "application/json",
			},
			"body":            `{"hello":"world"}`,
			"timeout_seconds": 10.0,
		},
		DataStore: map[string]any{},
		ProjectID: uuid.New(),
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(result.Effects) != 1 {
		t.Fatalf("effects=%d", len(result.Effects))
	}
	if result.Effects[0].Kind != EffectExecuteHTTPCall {
		t.Errorf("kind = %s, want execute_http_call", result.Effects[0].Kind)
	}
	var p ExecuteHTTPCallPayload
	if err := json.Unmarshal(result.Effects[0].Payload, &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p.Method != "POST" {
		t.Errorf("method = %s", p.Method)
	}
	if p.URL != "https://api.example.com/v1/things" {
		t.Errorf("url = %s", p.URL)
	}
	if p.Headers["Authorization"] != "Bearer {{secrets.GITHUB_TOKEN}}" {
		t.Error("headers not preserved verbatim for applier-side resolution")
	}
}

func TestHTTPCallHandler_DefaultMethodAndTimeout(t *testing.T) {
	h := HTTPCallHandler{}
	result, _ := h.Execute(context.Background(), &NodeExecRequest{
		Execution: &model.WorkflowExecution{},
		Node:      &model.WorkflowNode{ID: "http-1"},
		Config:    map[string]any{"url": "https://api.example.com"},
		DataStore: map[string]any{},
		ProjectID: uuid.New(),
	})
	var p ExecuteHTTPCallPayload
	_ = json.Unmarshal(result.Effects[0].Payload, &p)
	if p.Method != "GET" {
		t.Errorf("default method = %s", p.Method)
	}
	if p.TimeoutSeconds != 30 {
		t.Errorf("default timeout = %d", p.TimeoutSeconds)
	}
}
