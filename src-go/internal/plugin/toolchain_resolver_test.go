package plugin_test

import (
	"context"
	"errors"
	"testing"

	"github.com/agentforge/server/internal/plugin"
)

func TestResolver_WorkflowInput(t *testing.T) {
	r := plugin.NewToolChainResolver(
		map[string]any{"topic": "golang concurrency"},
		nil,
		nil,
	)
	result, err := r.Resolve(context.Background(), "{{workflow.input.topic}}")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if result != "golang concurrency" {
		t.Errorf("got %q", result)
	}
}

func TestResolver_StepOutput(t *testing.T) {
	r := plugin.NewToolChainResolver(
		map[string]any{},
		map[string]map[string]any{
			"search_results": {"top_result": "github.com/golang/go"},
		},
		nil,
	)
	result, err := r.Resolve(context.Background(), "{{steps.search_results.top_result}}")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if result != "github.com/golang/go" {
		t.Errorf("got %q", result)
	}
}

func TestResolver_EnvSecret(t *testing.T) {
	r := plugin.NewToolChainResolver(
		map[string]any{},
		nil,
		func(ctx context.Context, name string) (string, error) {
			if name == "API_KEY" {
				return "secret-value", nil
			}
			return "", errors.New("unknown secret: " + name)
		},
	)
	result, err := r.Resolve(context.Background(), "{{env.API_KEY}}")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if result != "secret-value" {
		t.Errorf("got %q", result)
	}
}

func TestResolver_EnvWithoutLookupErrors(t *testing.T) {
	r := plugin.NewToolChainResolver(map[string]any{}, nil, nil)
	if _, err := r.Resolve(context.Background(), "{{env.API_KEY}}"); err == nil {
		t.Error("expected error when SecretLookup is nil")
	}
}

func TestResolver_UnknownVariable(t *testing.T) {
	r := plugin.NewToolChainResolver(map[string]any{}, nil, nil)
	if _, err := r.Resolve(context.Background(), "{{unknown.path}}"); err == nil {
		t.Error("expected error for unknown variable prefix")
	}
}

func TestResolver_NonTemplate(t *testing.T) {
	r := plugin.NewToolChainResolver(map[string]any{}, nil, nil)
	result, err := r.Resolve(context.Background(), "plain text value")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if result != "plain text value" {
		t.Errorf("got %q", result)
	}
}

func TestResolver_ResolveMap(t *testing.T) {
	r := plugin.NewToolChainResolver(
		map[string]any{"topic": "rust"},
		nil,
		nil,
	)
	input := map[string]any{
		"query":  "{{workflow.input.topic}}",
		"limit":  10,
		"nested": map[string]any{"key": "{{workflow.input.topic}}"},
	}
	resolved, err := r.ResolveMap(context.Background(), input)
	if err != nil {
		t.Fatalf("ResolveMap: %v", err)
	}
	if resolved["query"] != "rust" {
		t.Errorf("query = %v", resolved["query"])
	}
	if resolved["limit"] != 10 {
		t.Errorf("limit = %v", resolved["limit"])
	}
	nested, _ := resolved["nested"].(map[string]any)
	if nested["key"] != "rust" {
		t.Errorf("nested.key = %v", nested["key"])
	}
}

func TestResolver_SetStepOutput(t *testing.T) {
	r := plugin.NewToolChainResolver(map[string]any{}, nil, nil)
	r.SetStepOutput("first", map[string]any{"value": "hello"})
	got, err := r.Resolve(context.Background(), "{{steps.first.value}}")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != "hello" {
		t.Errorf("got %q", got)
	}
}

func TestResolver_MissingWorkflowKey(t *testing.T) {
	r := plugin.NewToolChainResolver(map[string]any{}, nil, nil)
	if _, err := r.Resolve(context.Background(), "{{workflow.input.missing}}"); err == nil {
		t.Error("expected error for missing workflow input key")
	}
}

func TestResolver_MissingStep(t *testing.T) {
	r := plugin.NewToolChainResolver(map[string]any{}, nil, nil)
	if _, err := r.Resolve(context.Background(), "{{steps.nonexistent.value}}"); err == nil {
		t.Error("expected error for missing step")
	}
}
