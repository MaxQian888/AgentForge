package model

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestToolChainSpec_Unmarshal(t *testing.T) {
	raw := `
id: research_and_store
role: coding-agent
action: tool_chain
tool_chain:
  steps:
    - tool: web-search
      input:
        query: "{{workflow.input.topic}}"
      output_as: search_results
    - tool: github-tool
      input:
        query: "{{steps.search_results.top_result}}"
      output_as: github_data
  on_error: stop
next: [summarize]
`
	var step WorkflowStepDefinition
	if err := yaml.Unmarshal([]byte(raw), &step); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if step.Action != WorkflowActionToolChain {
		t.Errorf("Action = %q, want tool_chain", step.Action)
	}
	if step.ToolChain == nil {
		t.Fatal("expected ToolChain to be non-nil")
	}
	if len(step.ToolChain.Steps) != 2 {
		t.Errorf("ToolChain.Steps len = %d, want 2", len(step.ToolChain.Steps))
	}
	if step.ToolChain.Steps[0].Tool != "web-search" {
		t.Errorf("Steps[0].Tool = %q", step.ToolChain.Steps[0].Tool)
	}
	if step.ToolChain.Steps[0].OutputAs != "search_results" {
		t.Errorf("Steps[0].OutputAs = %q", step.ToolChain.Steps[0].OutputAs)
	}
	if step.ToolChain.Steps[1].OutputAs != "github_data" {
		t.Errorf("Steps[1].OutputAs = %q", step.ToolChain.Steps[1].OutputAs)
	}
	if step.ToolChain.OnError != "stop" {
		t.Errorf("OnError = %q", step.ToolChain.OnError)
	}
}

func TestWorkflowActionToolChain_Constant(t *testing.T) {
	if WorkflowActionToolChain != WorkflowActionType("tool_chain") {
		t.Errorf("WorkflowActionToolChain = %q, want tool_chain", WorkflowActionToolChain)
	}
}
