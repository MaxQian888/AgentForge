package plugin

import (
	"context"
	"fmt"
	"strings"
)

// SecretLookup returns the plaintext value of a named secret. In production
// this wraps secrets.Service.GetPlaintext(ctx, projectID, name); the
// resolver intentionally takes a closure so the project scope is bound at
// the wiring layer, not leaked into ToolChain code.
type SecretLookup func(ctx context.Context, name string) (string, error)

// ToolChainResolver resolves {{...}} template variables in tool input
// values. It runs server-side only — WASM plugins never see the resolver
// logic; they receive already-resolved input maps.
//
// Supported prefixes:
//
//	{{workflow.input.<key>}}  — value from the workflow's initial input
//	{{steps.<id>.<key>}}      — value from a prior ToolChain step's output
//	{{env.<name>}}            — project secret (read-only, via SecretLookup)
type ToolChainResolver struct {
	workflowInput map[string]any
	stepOutputs   map[string]map[string]any
	secretLookup  SecretLookup
}

func NewToolChainResolver(
	workflowInput map[string]any,
	stepOutputs map[string]map[string]any,
	secretLookup SecretLookup,
) *ToolChainResolver {
	if workflowInput == nil {
		workflowInput = map[string]any{}
	}
	if stepOutputs == nil {
		stepOutputs = map[string]map[string]any{}
	}
	return &ToolChainResolver{
		workflowInput: workflowInput,
		stepOutputs:   stepOutputs,
		secretLookup:  secretLookup,
	}
}

// SetStepOutput registers the output of a completed step so subsequent
// {{steps.<id>.*}} references resolve against it.
func (r *ToolChainResolver) SetStepOutput(stepID string, output map[string]any) {
	r.stepOutputs[stepID] = output
}

// Resolve resolves a single value. Non-template values are returned as-is.
// Returns an error for unknown prefixes or missing keys so silent
// substitution failures can't leak past the executor.
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

// ResolveMap walks a map and resolves all string values (including nested
// maps). Non-string values are passed through unchanged so tool inputs that
// declare numbers, bools, or arrays don't get stringified.
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
