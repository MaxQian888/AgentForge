package plugin_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/plugin"
)

// stubMCPCaller satisfies plugin.MCPToolCaller and records every tool call.
type stubMCPCaller struct {
	mu     sync.Mutex
	calls  []string
	args   []map[string]any
	output map[string]any // optional extra fields injected into MCPToolCallResult.StructuredContent
}

func (s *stubMCPCaller) CallMCPTool(ctx context.Context, pluginID, toolName string, args map[string]any) (*model.PluginMCPToolCallResult, error) {
	s.mu.Lock()
	s.calls = append(s.calls, toolName)
	s.args = append(s.args, args)
	s.mu.Unlock()

	structured := map[string]any{}
	for k, v := range s.output {
		structured[k] = v
	}
	return &model.PluginMCPToolCallResult{
		PluginID:  pluginID,
		Operation: toolName,
		Result: model.MCPToolCallResult{
			Content: []model.MCPContentBlock{
				{Type: "text", Text: toolName + "_output"},
			},
			StructuredContent: structured,
		},
	}, nil
}

func (s *stubMCPCaller) snapshotCalls() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.calls))
	copy(out, s.calls)
	return out
}

func (s *stubMCPCaller) snapshotArgs() []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]map[string]any, len(s.args))
	copy(out, s.args)
	return out
}

func TestToolChainExecutor_ExecutesStepsInOrder(t *testing.T) {
	caller := &stubMCPCaller{}
	exec := plugin.NewToolChainExecutor(caller, nil)

	spec := &model.ToolChainSpec{
		Steps: []model.ToolChainStep{
			{Tool: "web-search", Input: map[string]any{"query": "golang"}, OutputAs: "search_results"},
			{Tool: "github-tool", Input: map[string]any{"q": "golang"}, OutputAs: "github_data"},
		},
		OnError: "stop",
	}

	result, err := exec.Execute(context.Background(), "test-plugin", spec, map[string]any{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	calls := caller.snapshotCalls()
	if len(calls) != 2 || calls[0] != "web-search" || calls[1] != "github-tool" {
		t.Errorf("call order = %v", calls)
	}
	if result.CompletedSteps != 2 {
		t.Errorf("CompletedSteps = %d, want 2", result.CompletedSteps)
	}
	if result.FinalOutput == nil {
		t.Fatal("expected non-nil final output")
	}
}

func TestToolChainExecutor_TemplateResolution(t *testing.T) {
	caller := &stubMCPCaller{output: map[string]any{"top_result": "github.com/golang"}}
	exec := plugin.NewToolChainExecutor(caller, nil)

	spec := &model.ToolChainSpec{
		Steps: []model.ToolChainStep{
			{
				Tool:     "web-search",
				Input:    map[string]any{"query": "{{workflow.input.topic}}"},
				OutputAs: "search_results",
			},
			{
				Tool:  "github-tool",
				Input: map[string]any{"q": "{{steps.search_results.top_result}}"},
			},
		},
		OnError: "stop",
	}

	_, err := exec.Execute(context.Background(), "test-plugin", spec, map[string]any{"topic": "rust"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	args := caller.snapshotArgs()
	if args[0]["query"] != "rust" {
		t.Errorf("step 0 query = %v, want rust", args[0]["query"])
	}
	if args[1]["q"] != "github.com/golang" {
		t.Errorf("step 1 q = %v, want github.com/golang", args[1]["q"])
	}
}

func TestToolChainExecutor_OnError_Stop(t *testing.T) {
	caller := &failingMCPCaller{failAt: "web-search"}
	exec := plugin.NewToolChainExecutor(caller, nil)

	spec := &model.ToolChainSpec{
		Steps: []model.ToolChainStep{
			{Tool: "web-search", Input: map[string]any{}, OutputAs: "r1"},
			{Tool: "github-tool", Input: map[string]any{}, OutputAs: "r2"},
		},
		OnError: "stop",
	}

	_, err := exec.Execute(context.Background(), "test-plugin", spec, map[string]any{})
	if err == nil {
		t.Error("expected error from stopped chain")
	}
	if caller.snapshotCount() != 1 {
		t.Errorf("expected 1 call (stop after first failure), got %d", caller.snapshotCount())
	}
}

func TestToolChainExecutor_OnError_Skip(t *testing.T) {
	caller := &failingMCPCaller{failAt: "web-search"}
	exec := plugin.NewToolChainExecutor(caller, nil)

	spec := &model.ToolChainSpec{
		Steps: []model.ToolChainStep{
			{Tool: "web-search", Input: map[string]any{}, OutputAs: "r1"},
			{Tool: "github-tool", Input: map[string]any{}, OutputAs: "r2"},
		},
		OnError: "skip",
	}

	result, err := exec.Execute(context.Background(), "test-plugin", spec, map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error with skip policy: %v", err)
	}
	if caller.snapshotCount() != 2 {
		t.Errorf("expected 2 calls (skip and continue), got %d", caller.snapshotCount())
	}
	if result.CompletedSteps != 1 {
		t.Errorf("CompletedSteps = %d, want 1 (only the second step succeeded)", result.CompletedSteps)
	}
}

func TestToolChainExecutor_OnError_Retry(t *testing.T) {
	caller := &flakyMCPCaller{failuresBeforeSuccess: 2}
	exec := plugin.NewToolChainExecutor(caller, nil)

	spec := &model.ToolChainSpec{
		Steps: []model.ToolChainStep{
			{Tool: "flaky-tool", Input: map[string]any{}, OutputAs: "r"},
		},
		OnError: "retry(3)",
	}

	_, err := exec.Execute(context.Background(), "test-plugin", spec, map[string]any{})
	if err != nil {
		t.Fatalf("expected success after retries, got %v", err)
	}
	if caller.attempts != 3 {
		t.Errorf("expected 3 attempts (2 failures + 1 success), got %d", caller.attempts)
	}
}

func TestToolChainExecutor_OnError_RetryExhausted(t *testing.T) {
	caller := &flakyMCPCaller{failuresBeforeSuccess: 99}
	exec := plugin.NewToolChainExecutor(caller, nil)

	spec := &model.ToolChainSpec{
		Steps: []model.ToolChainStep{
			{Tool: "flaky-tool", Input: map[string]any{}, OutputAs: "r"},
		},
		OnError: "retry(2)",
	}

	_, err := exec.Execute(context.Background(), "test-plugin", spec, map[string]any{})
	if err == nil {
		t.Error("expected exhausted-retry error")
	}
	if caller.attempts != 3 {
		t.Errorf("expected 3 attempts (1 + 2 retries), got %d", caller.attempts)
	}
}

func TestToolChainExecutor_NilSpec(t *testing.T) {
	exec := plugin.NewToolChainExecutor(&stubMCPCaller{}, nil)
	_, err := exec.Execute(context.Background(), "p", nil, nil)
	if err == nil {
		t.Error("expected error for nil spec")
	}
}

// failingMCPCaller fails when toolName == failAt.
type failingMCPCaller struct {
	mu     sync.Mutex
	failAt string
	count  int
}

func (f *failingMCPCaller) CallMCPTool(ctx context.Context, pluginID, toolName string, args map[string]any) (*model.PluginMCPToolCallResult, error) {
	f.mu.Lock()
	f.count++
	f.mu.Unlock()
	if toolName == f.failAt {
		return nil, errors.New("simulated failure for " + toolName)
	}
	return &model.PluginMCPToolCallResult{
		PluginID:  pluginID,
		Operation: toolName,
		Result: model.MCPToolCallResult{
			Content: []model.MCPContentBlock{{Type: "text", Text: "ok"}},
		},
	}, nil
}

func (f *failingMCPCaller) snapshotCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.count
}

// flakyMCPCaller fails the first failuresBeforeSuccess attempts then succeeds.
type flakyMCPCaller struct {
	failuresBeforeSuccess int
	attempts              int
}

func (f *flakyMCPCaller) CallMCPTool(ctx context.Context, pluginID, toolName string, args map[string]any) (*model.PluginMCPToolCallResult, error) {
	f.attempts++
	if f.attempts <= f.failuresBeforeSuccess {
		return nil, errors.New("flaky")
	}
	return &model.PluginMCPToolCallResult{
		PluginID:  pluginID,
		Operation: toolName,
		Result: model.MCPToolCallResult{
			Content: []model.MCPContentBlock{{Type: "text", Text: "ok"}},
		},
	}, nil
}
