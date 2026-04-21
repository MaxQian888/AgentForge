package service_test

import (
	"context"
	"sync"
	"testing"

	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/plugin"
	"github.com/agentforge/server/internal/service"
)

// routerStubMCPCaller satisfies plugin.MCPToolCaller for router wiring tests.
type routerStubMCPCaller struct {
	mu    sync.Mutex
	calls []string
}

func (s *routerStubMCPCaller) CallMCPTool(ctx context.Context, pluginID, toolName string, args map[string]any) (*model.PluginMCPToolCallResult, error) {
	s.mu.Lock()
	s.calls = append(s.calls, toolName)
	s.mu.Unlock()
	return &model.PluginMCPToolCallResult{
		PluginID:  pluginID,
		Operation: toolName,
		Result: model.MCPToolCallResult{
			Content: []model.MCPContentBlock{{Type: "text", Text: "router-result"}},
		},
	}, nil
}

func TestWorkflowStepRouter_ToolChainAction_Dispatches(t *testing.T) {
	caller := &routerStubMCPCaller{}
	toolChain := plugin.NewToolChainExecutor(caller, nil)
	router := service.NewWorkflowStepRouterExecutor(nil, nil, nil).
		WithToolChainExecutor(toolChain)

	req := service.WorkflowStepExecutionRequest{
		PluginID: "test-plugin",
		Step: model.WorkflowStepDefinition{
			ID:     "research",
			Role:   "coding-agent",
			Action: model.WorkflowActionToolChain,
			ToolChain: &model.ToolChainSpec{
				Steps: []model.ToolChainStep{
					{Tool: "web-search", Input: map[string]any{"query": "go modules"}, OutputAs: "results"},
				},
				OnError: "stop",
			},
		},
		Input: map[string]any{"trigger": map[string]any{}},
	}

	result, err := router.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if completed, _ := result.Output["completed_steps"].(int); completed != 1 {
		t.Errorf("completed_steps = %v, want 1", result.Output["completed_steps"])
	}
	caller.mu.Lock()
	defer caller.mu.Unlock()
	if len(caller.calls) != 1 || caller.calls[0] != "web-search" {
		t.Errorf("expected web-search dispatch, got %v", caller.calls)
	}
}

func TestWorkflowStepRouter_ToolChainAction_RequiresExecutor(t *testing.T) {
	router := service.NewWorkflowStepRouterExecutor(nil, nil, nil) // no WithToolChainExecutor
	req := service.WorkflowStepExecutionRequest{
		PluginID: "p",
		Step: model.WorkflowStepDefinition{
			ID:        "x",
			Action:    model.WorkflowActionToolChain,
			ToolChain: &model.ToolChainSpec{Steps: []model.ToolChainStep{{Tool: "t"}}},
		},
		Input: map[string]any{"trigger": map[string]any{}},
	}
	if _, err := router.Execute(context.Background(), req); err == nil {
		t.Error("expected error when tool_chain executor is not configured")
	}
}

func TestWorkflowStepRouter_ToolChainAction_RequiresSpec(t *testing.T) {
	caller := &routerStubMCPCaller{}
	router := service.NewWorkflowStepRouterExecutor(nil, nil, nil).
		WithToolChainExecutor(plugin.NewToolChainExecutor(caller, nil))
	req := service.WorkflowStepExecutionRequest{
		PluginID: "p",
		Step: model.WorkflowStepDefinition{
			ID:     "x",
			Action: model.WorkflowActionToolChain,
		},
		Input: map[string]any{"trigger": map[string]any{}},
	}
	if _, err := router.Execute(context.Background(), req); err == nil {
		t.Error("expected error when ToolChain spec is nil")
	}
}
