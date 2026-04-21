# Plugin ToolChain Primitive

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `action: tool_chain` as a new Workflow Step action type that declaratively chains MCP tool calls within a step, with `{{workflow.input.*}}` / `{{steps.<id>.*}}` / `{{env.*}}` template variable resolution.

**Architecture:** Three new files: `toolchain_executor.go` (orchestration loop), `toolchain_resolver.go` (template variable expansion), and model changes to `WorkflowStepDefinition`. `ToolChainExecutor` calls `PluginService.CallMCPTool()` **as a Go function** — not via HTTP. Template resolution runs server-side only. `on_error: stop | skip | retry(n)` is handled within the executor.

**Tech Stack:** Go 1.23+, `src-go/internal/service/workflow_step_router.go`, `src-go/internal/service/plugin_service.go` (`CallMCPTool`), `src-go/internal/secrets/service.go` (`GetPlaintext`), `src-go/internal/model/plugin.go`.

**Dependency:** Plan `2026-04-21-plugin-workflow-executor.md` should be completed first (adds `WorkflowActionToolChain` constant). If running this plan standalone, add the constant in Task 1.

---

## File Map

| Action | Path | Responsibility |
|--------|------|---------------|
| Modify | `src-go/internal/model/plugin.go` | Add `WorkflowActionToolChain` const, `ToolChainSpec` and `ToolChainStep` structs, `ToolChain` field to `WorkflowStepDefinition` |
| Create | `src-go/internal/plugin/toolchain_resolver.go` | Template variable resolver (`{{...}}` expansion) |
| Create | `src-go/internal/plugin/toolchain_resolver_test.go` | Unit tests for resolver |
| Create | `src-go/internal/plugin/toolchain_executor.go` | `ToolChainExecutor` (calls `PluginService.CallMCPTool` directly) |
| Create | `src-go/internal/plugin/toolchain_executor_test.go` | Unit tests for executor |
| Modify | `src-go/internal/service/workflow_step_router.go` | Add `case WorkflowActionToolChain` dispatch |

---

## Task 1: Add `ToolChainSpec` model and `WorkflowActionToolChain` constant

**Files:**
- Modify: `src-go/internal/model/plugin.go`

- [ ] **Step 1: Write the failing test**

Create `src-go/internal/model/plugin_toolchain_test.go`:

```go
package model_test

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
	if step.ToolChain.Steps[1].OutputAs != "github_data" {
		t.Errorf("Steps[1].OutputAs = %q", step.ToolChain.Steps[1].OutputAs)
	}
	if step.ToolChain.OnError != "stop" {
		t.Errorf("OnError = %q", step.ToolChain.OnError)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd src-go && go test ./internal/model/... -run TestToolChainSpec -v
```

Expected: compile error — `WorkflowActionToolChain`, `ToolChainSpec` not defined.

- [ ] **Step 3: Add model definitions to `plugin.go`**

In `src-go/internal/model/plugin.go`, add the constant alongside existing action types:

```go
const (
	WorkflowActionAgent    WorkflowActionType = "agent"
	WorkflowActionReview   WorkflowActionType = "review"
	WorkflowActionTask     WorkflowActionType = "task"
	WorkflowActionWorkflow WorkflowActionType = "workflow"
	WorkflowActionApproval WorkflowActionType = "approval"
	WorkflowActionToolChain WorkflowActionType = "tool_chain" // new
)
```

Add `ToolChainStep` and `ToolChainSpec` structs (before or after `WorkflowStepDefinition`):

```go
// ToolChainStep is one MCP tool call within a ToolChain.
type ToolChainStep struct {
	Tool     string         `yaml:"tool" json:"tool"`
	Input    map[string]any `yaml:"input,omitempty" json:"input,omitempty"`
	OutputAs string         `yaml:"output_as,omitempty" json:"output_as,omitempty"`
}

// ToolChainSpec declares a sequence of MCP tool calls for a Workflow Step.
type ToolChainSpec struct {
	Steps   []ToolChainStep `yaml:"steps" json:"steps"`
	OnError string          `yaml:"on_error,omitempty" json:"on_error,omitempty"` // "stop" | "skip" | "retry(n)"
}
```

Add `ToolChain` field to `WorkflowStepDefinition`:

```go
type WorkflowStepDefinition struct {
	ID        string             `yaml:"id" json:"id"`
	Role      string             `yaml:"role" json:"role"`
	Action    WorkflowActionType `yaml:"action" json:"action"`
	Next      []string           `yaml:"next,omitempty" json:"next,omitempty"`
	Config    map[string]any     `yaml:"config,omitempty" json:"config,omitempty"`
	Metadata  map[string]any     `yaml:"metadata,omitempty" json:"metadata,omitempty"`
	ToolChain *ToolChainSpec     `yaml:"tool_chain,omitempty" json:"tool_chain,omitempty"` // new
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd src-go && go test ./internal/model/... -run TestToolChainSpec -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add src-go/internal/model/plugin.go src-go/internal/model/plugin_toolchain_test.go
git commit -m "feat(model): add WorkflowActionToolChain, ToolChainSpec, ToolChainStep"
```

---

## Task 2: Implement the template resolver

**Files:**
- Create: `src-go/internal/plugin/toolchain_resolver.go`
- Create: `src-go/internal/plugin/toolchain_resolver_test.go`

- [ ] **Step 1: Write the failing test**

Create `src-go/internal/plugin/toolchain_resolver_test.go`:

```go
package plugin_test

import (
	"context"
	"testing"
)

func TestResolver_WorkflowInput(t *testing.T) {
	r := NewToolChainResolver(
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
	r := NewToolChainResolver(
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
	r := NewToolChainResolver(
		map[string]any{},
		nil,
		func(ctx context.Context, name string) (string, error) {
			if name == "API_KEY" {
				return "secret-value", nil
			}
			return "", fmt.Errorf("unknown secret: %s", name)
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

func TestResolver_UnknownVariable(t *testing.T) {
	r := NewToolChainResolver(map[string]any{}, nil, nil)
	_, err := r.Resolve(context.Background(), "{{unknown.path}}")
	if err == nil {
		t.Error("expected error for unknown variable prefix")
	}
}

func TestResolver_NonTemplate(t *testing.T) {
	r := NewToolChainResolver(map[string]any{}, nil, nil)
	result, err := r.Resolve(context.Background(), "plain text value")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if result != "plain text value" {
		t.Errorf("got %q", result)
	}
}

func TestResolver_ResolveMap(t *testing.T) {
	r := NewToolChainResolver(
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
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd src-go && go test ./internal/plugin/... -run TestResolver_ -v
```

Expected: compile error — `NewToolChainResolver` not defined.

- [ ] **Step 3: Implement `toolchain_resolver.go`**

Create `src-go/internal/plugin/toolchain_resolver.go`:

```go
package plugin

import (
	"context"
	"fmt"
	"strings"
)

// SecretLookup is a function that returns the plaintext value of a named secret.
// In production this wraps secrets.Service.GetPlaintext(ctx, projectID, name).
type SecretLookup func(ctx context.Context, name string) (string, error)

// ToolChainResolver resolves {{...}} template variables in tool input values.
// It runs server-side only — WASM plugins never see the resolver logic.
//
// Supported prefixes:
//
//	{{workflow.input.<key>}}  — value from the workflow's initial input
//	{{steps.<id>.<key>}}      — value from a prior ToolChain step's output
//	{{env.<name>}}            — project secret (read-only, via SecretLookup)
type ToolChainResolver struct {
	workflowInput map[string]any
	stepOutputs   map[string]map[string]any // stepID → output map
	secretLookup  SecretLookup
}

// NewToolChainResolver creates a resolver for a single ToolChain execution.
func NewToolChainResolver(
	workflowInput map[string]any,
	stepOutputs map[string]map[string]any,
	secretLookup SecretLookup,
) *ToolChainResolver {
	if stepOutputs == nil {
		stepOutputs = map[string]map[string]any{}
	}
	return &ToolChainResolver{
		workflowInput: workflowInput,
		stepOutputs:   stepOutputs,
		secretLookup:  secretLookup,
	}
}

// SetStepOutput registers the output of a completed step for future resolution.
func (r *ToolChainResolver) SetStepOutput(stepID string, output map[string]any) {
	r.stepOutputs[stepID] = output
}

// Resolve resolves a single value. If the value is not a {{...}} template,
// it is returned as-is (as a string). Returns an error for unknown prefixes.
func (r *ToolChainResolver) Resolve(ctx context.Context, value string) (string, error) {
	if !strings.HasPrefix(value, "{{") || !strings.HasSuffix(value, "}}") {
		return value, nil
	}
	expr := strings.TrimSpace(value[2 : len(value)-2])
	parts := strings.SplitN(expr, ".", 3)

	switch parts[0] {
	case "workflow":
		if len(parts) < 3 || parts[1] != "input" {
			return "", fmt.Errorf("invalid workflow variable %q: expected workflow.input.<key>", expr)
		}
		key := parts[2]
		v, ok := r.workflowInput[key]
		if !ok {
			return "", fmt.Errorf("workflow.input.%s not found", key)
		}
		return fmt.Sprintf("%v", v), nil

	case "steps":
		if len(parts) < 3 {
			return "", fmt.Errorf("invalid steps variable %q: expected steps.<id>.<key>", expr)
		}
		stepID := parts[1]
		key := parts[2]
		output, ok := r.stepOutputs[stepID]
		if !ok {
			return "", fmt.Errorf("step %q output not available (step may not have run yet)", stepID)
		}
		v, ok := output[key]
		if !ok {
			return "", fmt.Errorf("steps.%s.%s not found in step output", stepID, key)
		}
		return fmt.Sprintf("%v", v), nil

	case "env":
		if len(parts) < 2 {
			return "", fmt.Errorf("invalid env variable %q: expected env.<name>", expr)
		}
		name := strings.Join(parts[1:], ".")
		if r.secretLookup == nil {
			return "", fmt.Errorf("env.%s requested but no secret lookup configured", name)
		}
		return r.secretLookup(ctx, name)

	default:
		return "", fmt.Errorf("unknown template variable prefix %q in %q", parts[0], expr)
	}
}

// ResolveMap resolves all string values in a map (including nested maps).
// Non-string values (int, bool, etc.) are passed through unchanged.
func (r *ToolChainResolver) ResolveMap(ctx context.Context, input map[string]any) (map[string]any, error) {
	out := make(map[string]any, len(input))
	for k, v := range input {
		switch val := v.(type) {
		case string:
			resolved, err := r.Resolve(ctx, val)
			if err != nil {
				return nil, fmt.Errorf("key %q: %w", k, err)
			}
			out[k] = resolved
		case map[string]any:
			nested, err := r.ResolveMap(ctx, val)
			if err != nil {
				return nil, fmt.Errorf("key %q: %w", k, err)
			}
			out[k] = nested
		default:
			out[k] = v
		}
	}
	return out, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd src-go && go test ./internal/plugin/... -run TestResolver_ -v
```

Expected: PASS (6 tests).

- [ ] **Step 5: Commit**

```bash
git add src-go/internal/plugin/toolchain_resolver.go src-go/internal/plugin/toolchain_resolver_test.go
git commit -m "feat(plugin): add ToolChainResolver for template variable expansion"
```

---

## Task 3: Implement `ToolChainExecutor`

**Files:**
- Create: `src-go/internal/plugin/toolchain_executor.go`
- Create: `src-go/internal/plugin/toolchain_executor_test.go`

- [ ] **Step 1: Write the failing test**

Create `src-go/internal/plugin/toolchain_executor_test.go`:

```go
package plugin_test

import (
	"context"
	"testing"

	"agentforge/internal/model"
)

// MCPToolCaller is the interface ToolChainExecutor uses — matches PluginService.CallMCPTool signature.
type stubMCPCaller struct {
	calls  []string
	result map[string]any
}

func (s *stubMCPCaller) CallMCPTool(ctx context.Context, pluginID, toolName string, args map[string]any) (*model.PluginMCPToolCallResult, error) {
	s.calls = append(s.calls, toolName)
	content := map[string]any{"result": "value_from_" + toolName}
	for k, v := range s.result {
		content[k] = v
	}
	return &model.PluginMCPToolCallResult{
		Content: []map[string]any{{"type": "text", "text": toolName + "_output"}},
	}, nil
}

func TestToolChainExecutor_ExecutesStepsInOrder(t *testing.T) {
	caller := &stubMCPCaller{}
	exec := NewToolChainExecutor(caller, nil)

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
	if len(caller.calls) != 2 {
		t.Errorf("expected 2 tool calls, got %d: %v", len(caller.calls), caller.calls)
	}
	if caller.calls[0] != "web-search" || caller.calls[1] != "github-tool" {
		t.Errorf("wrong call order: %v", caller.calls)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestToolChainExecutor_TemplateResolution(t *testing.T) {
	caller := &stubMCPCaller{result: map[string]any{"top_result": "github.com/golang"}}
	exec := NewToolChainExecutor(caller, nil)

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
	if caller.calls[0] != "web-search" {
		t.Errorf("calls[0] = %q", caller.calls[0])
	}
}

func TestToolChainExecutor_OnError_Stop(t *testing.T) {
	callCount := 0
	caller := &failingMCPCaller{failAt: "web-search", counter: &callCount}
	exec := NewToolChainExecutor(caller, nil)

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
	if callCount != 1 {
		t.Errorf("expected 1 call (stop after first failure), got %d", callCount)
	}
}

func TestToolChainExecutor_OnError_Skip(t *testing.T) {
	callCount := 0
	caller := &failingMCPCaller{failAt: "web-search", counter: &callCount}
	exec := NewToolChainExecutor(caller, nil)

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
	if callCount != 2 {
		t.Errorf("expected 2 calls (skip and continue), got %d", callCount)
	}
	_ = result
}

type failingMCPCaller struct {
	failAt  string
	counter *int
}

func (f *failingMCPCaller) CallMCPTool(ctx context.Context, pluginID, toolName string, args map[string]any) (*model.PluginMCPToolCallResult, error) {
	*f.counter++
	if toolName == f.failAt {
		return nil, fmt.Errorf("simulated failure for %s", toolName)
	}
	return &model.PluginMCPToolCallResult{
		Content: []map[string]any{{"type": "text", "text": toolName + "_ok"}},
	}, nil
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd src-go && go test ./internal/plugin/... -run TestToolChainExecutor -v
```

Expected: compile error — `NewToolChainExecutor` not defined.

- [ ] **Step 3: Implement `toolchain_executor.go`**

Create `src-go/internal/plugin/toolchain_executor.go`:

```go
package plugin

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"agentforge/internal/model"
)

// MCPToolCaller is satisfied by PluginService, allowing direct Go-function invocation
// without HTTP round-trip. Inject *service.PluginService here.
type MCPToolCaller interface {
	CallMCPTool(ctx context.Context, pluginID, toolName string, args map[string]any) (*model.PluginMCPToolCallResult, error)
}

// ToolChainResult holds the final output and all intermediate step outputs.
type ToolChainResult struct {
	FinalOutput    map[string]any            // output of the last step
	StepOutputs    map[string]map[string]any // keyed by OutputAs
	CompletedSteps int
}

// ToolChainExecutor runs a ToolChainSpec, calling MCP tools in sequence.
// It uses ToolChainResolver to expand {{...}} template variables before each call.
type ToolChainExecutor struct {
	caller       MCPToolCaller
	secretLookup SecretLookup
}

// NewToolChainExecutor creates an executor. secretLookup may be nil if no env.* variables are used.
func NewToolChainExecutor(caller MCPToolCaller, secretLookup SecretLookup) *ToolChainExecutor {
	return &ToolChainExecutor{caller: caller, secretLookup: secretLookup}
}

var retryPattern = regexp.MustCompile(`^retry\((\d+)\)$`)

// Execute runs all steps in the ToolChainSpec and returns the aggregated result.
// workflowInput is used for {{workflow.input.*}} resolution.
func (e *ToolChainExecutor) Execute(
	ctx context.Context,
	pluginID string,
	spec *model.ToolChainSpec,
	workflowInput map[string]any,
) (*ToolChainResult, error) {
	if spec == nil {
		return nil, fmt.Errorf("tool_chain spec is nil")
	}

	onError := spec.OnError
	if onError == "" {
		onError = "stop"
	}

	resolver := NewToolChainResolver(workflowInput, nil, e.secretLookup)
	result := &ToolChainResult{
		StepOutputs: make(map[string]map[string]any),
	}

	for i, step := range spec.Steps {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		resolvedInput, err := resolver.ResolveMap(ctx, step.Input)
		if err != nil {
			return nil, fmt.Errorf("step[%d] %q input resolution: %w", i, step.Tool, err)
		}

		maxRetries := 0
		if m := retryPattern.FindStringSubmatch(onError); m != nil {
			maxRetries, _ = strconv.Atoi(m[1])
		}

		var callResult *model.PluginMCPToolCallResult
		for attempt := 0; attempt <= maxRetries; attempt++ {
			callResult, err = e.caller.CallMCPTool(ctx, pluginID, step.Tool, resolvedInput)
			if err == nil {
				break
			}
			if attempt < maxRetries {
				continue // retry
			}
		}

		if err != nil {
			switch {
			case onError == "stop" || retryPattern.MatchString(onError):
				return result, fmt.Errorf("tool_chain step[%d] %q: %w", i, step.Tool, err)
			case onError == "skip":
				// Record nil output for this step and continue.
				if step.OutputAs != "" {
					result.StepOutputs[step.OutputAs] = nil
					resolver.SetStepOutput(step.OutputAs, nil)
				}
				continue
			default:
				return result, fmt.Errorf("tool_chain step[%d] %q: %w", i, step.Tool, err)
			}
		}

		// Convert tool result content to a flat output map.
		stepOut := mcpResultToMap(callResult)
		if step.OutputAs != "" {
			result.StepOutputs[step.OutputAs] = stepOut
			resolver.SetStepOutput(step.OutputAs, stepOut)
		}
		result.FinalOutput = stepOut
		result.CompletedSteps++
	}

	return result, nil
}

// mcpResultToMap converts MCP tool call content to a flat map for template resolution.
func mcpResultToMap(r *model.PluginMCPToolCallResult) map[string]any {
	if r == nil {
		return map[string]any{}
	}
	out := map[string]any{}
	for i, c := range r.Content {
		if text, ok := c["text"].(string); ok {
			if i == 0 {
				out["text"] = text
				out["top_result"] = text
			}
			out[fmt.Sprintf("content_%d", i)] = text
		}
	}
	return out
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd src-go && go test ./internal/plugin/... -run TestToolChainExecutor -v
```

Expected: PASS (4 tests).

- [ ] **Step 5: Commit**

```bash
git add src-go/internal/plugin/toolchain_executor.go src-go/internal/plugin/toolchain_executor_test.go
git commit -m "feat(plugin): implement ToolChainExecutor with stop/skip/retry error policies"
```

---

## Task 4: Wire `tool_chain` into `WorkflowStepRouterExecutor`

**Files:**
- Modify: `src-go/internal/service/workflow_step_router.go`

- [ ] **Step 1: Write the failing test**

In `src-go/internal/service/workflow_step_router_test.go` (create if not exists), add:

```go
package service_test

import (
	"context"
	"testing"

	"agentforge/internal/model"
)

func TestWorkflowStepRouter_ToolChainAction(t *testing.T) {
	caller := &stubMCPCallerForRouter{}
	router := NewWorkflowStepRouterExecutorWithToolChain(
		/* existing deps ... */
		caller,
		nil, // secretLookup
	)

	req := WorkflowStepExecutionRequest{
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
		Input: map[string]any{},
	}

	result, err := router.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

type stubMCPCallerForRouter struct{}

func (s *stubMCPCallerForRouter) CallMCPTool(ctx context.Context, pluginID, toolName string, args map[string]any) (*model.PluginMCPToolCallResult, error) {
	return &model.PluginMCPToolCallResult{
		Content: []map[string]any{{"type": "text", "text": "result"}},
	}, nil
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd src-go && go test ./internal/service/... -run TestWorkflowStepRouter_ToolChainAction -v
```

Expected: FAIL — `WorkflowActionToolChain` not handled.

- [ ] **Step 3: Add `tool_chain` case to the router's `Execute` switch**

In `src-go/internal/service/workflow_step_router.go`, locate the `switch req.Step.Action` block and add:

```go
case model.WorkflowActionToolChain:
    return e.executeToolChain(ctx, req)
```

Then add the `executeToolChain` method to `WorkflowStepRouterExecutor`:

```go
func (e *WorkflowStepRouterExecutor) executeToolChain(
	ctx context.Context,
	req WorkflowStepExecutionRequest,
) (*WorkflowStepExecutionResult, error) {
	if req.Step.ToolChain == nil {
		return nil, fmt.Errorf("step %q has action tool_chain but no tool_chain spec", req.Step.ID)
	}

	chainResult, err := e.toolChainExecutor.Execute(ctx, req.PluginID, req.Step.ToolChain, req.Input)
	if err != nil {
		return nil, err
	}

	output := map[string]any{
		"completed_steps": chainResult.CompletedSteps,
		"final_output":    chainResult.FinalOutput,
		"step_outputs":    chainResult.StepOutputs,
	}
	return &WorkflowStepExecutionResult{Output: output}, nil
}
```

Add `toolChainExecutor *plugin.ToolChainExecutor` field to `WorkflowStepRouterExecutor`. In the constructor, inject it:

```go
// In the constructor that creates WorkflowStepRouterExecutor:
router.toolChainExecutor = plugin.NewToolChainExecutor(pluginService, secretLookup)
```

(`pluginService` implements `plugin.MCPToolCaller` via its `CallMCPTool` method. `secretLookup` wraps `secretsService.GetPlaintext`.)

- [ ] **Step 4: Run all service tests to confirm no regressions**

```bash
cd src-go && go test ./internal/service/... -v
```

Expected: all PASS.

- [ ] **Step 5: Run all plugin tests**

```bash
cd src-go && go test ./internal/plugin/... -v
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add src-go/internal/service/workflow_step_router.go
git commit -m "feat(service): wire tool_chain action into WorkflowStepRouterExecutor"
```

---

## Final Verification

- [ ] **Run all affected tests**

```bash
cd src-go && go test ./internal/model/... ./internal/plugin/... ./internal/service/... -v
```

Expected: all PASS.

- [ ] **Verify `go build` succeeds**

```bash
cd src-go && go build ./...
```

Expected: no errors.
