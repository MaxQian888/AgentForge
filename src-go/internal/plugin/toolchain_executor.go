package plugin

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/agentforge/server/internal/model"
)

// MCPToolCaller is satisfied by *service.PluginService — its CallMCPTool
// signature matches exactly. The interface lets the executor stay free of
// service-package imports and lets tests inject stubs.
type MCPToolCaller interface {
	CallMCPTool(ctx context.Context, pluginID, toolName string, args map[string]any) (*model.PluginMCPToolCallResult, error)
}

// ToolChainResult holds the aggregated output of a ToolChain execution.
// FinalOutput is the flattened map for the last successfully-completed
// step; StepOutputs is keyed by OutputAs (skipped/failed steps appear with
// nil value when OnError=="skip"). CompletedSteps counts only steps that
// succeeded.
type ToolChainResult struct {
	FinalOutput    map[string]any
	StepOutputs    map[string]map[string]any
	CompletedSteps int
}

// ToolChainExecutor runs a ToolChainSpec, calling MCP tools in sequence.
// Template variables in step inputs ({{workflow.input.*}}, {{steps.*.*}},
// {{env.*}}) are expanded before each call via ToolChainResolver.
type ToolChainExecutor struct {
	caller       MCPToolCaller
	secretLookup SecretLookup
}

// NewToolChainExecutor builds an executor. secretLookup may be nil when no
// {{env.*}} variables are referenced; the resolver returns an error if a
// secret is requested without one.
func NewToolChainExecutor(caller MCPToolCaller, secretLookup SecretLookup) *ToolChainExecutor {
	return &ToolChainExecutor{caller: caller, secretLookup: secretLookup}
}

var retryPattern = regexp.MustCompile(`^retry\((\d+)\)$`)

// Execute runs every step in spec sequentially and returns the aggregated
// result. workflowInput drives {{workflow.input.*}} resolution; prior
// steps' outputs drive {{steps.*.*}} resolution.
//
// OnError policies:
//   - "" or "stop"  → first failure aborts the chain (returns the error).
//   - "skip"        → log nil for the step output and continue.
//   - "retry(n)"    → retry up to n additional times before failing the chain.
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

	maxRetries := 0
	if m := retryPattern.FindStringSubmatch(onError); m != nil {
		maxRetries, _ = strconv.Atoi(m[1])
	}

	resolver := NewToolChainResolver(workflowInput, nil, e.secretLookup)
	result := &ToolChainResult{
		StepOutputs: make(map[string]map[string]any),
	}

	for i, step := range spec.Steps {
		if ctx.Err() != nil {
			return result, ctx.Err()
		}

		resolvedInput, err := resolver.ResolveMap(ctx, step.Input)
		if err != nil {
			return result, fmt.Errorf("step[%d] %q input resolution: %w", i, step.Tool, err)
		}

		var (
			callResult *model.PluginMCPToolCallResult
			callErr    error
		)
		for attempt := 0; attempt <= maxRetries; attempt++ {
			callResult, callErr = e.caller.CallMCPTool(ctx, pluginID, step.Tool, resolvedInput)
			if callErr == nil {
				break
			}
			if attempt < maxRetries {
				backoff := 100 * time.Millisecond * time.Duration(1<<attempt)
				timer := time.NewTimer(backoff)
				select {
				case <-ctx.Done():
					timer.Stop()
					return result, ctx.Err()
				case <-timer.C:
				}
				continue
			}
		}

		if callErr != nil {
			switch {
			case onError == "skip":
				if step.OutputAs != "" {
					result.StepOutputs[step.OutputAs] = nil
					resolver.SetStepOutput(step.OutputAs, nil)
				}
				continue
			default:
				return result, fmt.Errorf("tool_chain step[%d] %q: %w", i, step.Tool, callErr)
			}
		}

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

// mcpResultToMap flattens an MCP tool-call result into the shape the
// resolver expects. StructuredContent fields take precedence (they're the
// rich payload); content text blocks fold into "text" and "top_result"
// for convenience plus indexed "content_<i>" entries.
func mcpResultToMap(r *model.PluginMCPToolCallResult) map[string]any {
	if r == nil {
		return map[string]any{}
	}
	out := map[string]any{}
	for k, v := range r.Result.StructuredContent {
		out[k] = v
	}
	for i, c := range r.Result.Content {
		if c.Text == "" {
			continue
		}
		if i == 0 {
			if _, exists := out["text"]; !exists {
				out["text"] = c.Text
			}
			if _, exists := out["top_result"]; !exists {
				out["top_result"] = c.Text
			}
		}
		out[fmt.Sprintf("content_%d", i)] = c.Text
	}
	return out
}
