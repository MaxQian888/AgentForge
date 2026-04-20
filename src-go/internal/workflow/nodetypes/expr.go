package nodetypes

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

// ConditionTaskResolver resolves task-status references like `task.status`
// in condition expressions. Implementations only need to satisfy the GetByID
// signature; callers may pass nil to skip task lookups entirely.
type ConditionTaskResolver interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Task, error)
}

var templateVarRe = regexp.MustCompile(`\{\{([^}]+)\}\}`)

// ResolveTemplateVars replaces {{node_id.output.field}} patterns in the input
// string with values looked up from dataStore. Patterns whose path does not
// resolve are left unchanged. Non-string values are JSON-marshaled.
func ResolveTemplateVars(template string, dataStore map[string]any) string {
	return templateVarRe.ReplaceAllStringFunc(template, func(match string) string {
		path := strings.TrimSpace(match[2 : len(match)-2])
		val := LookupPath(dataStore, path)
		if val == nil {
			return match // Keep original if not found
		}
		switch v := val.(type) {
		case string:
			return v
		default:
			b, _ := json.Marshal(v)
			return string(b)
		}
	})
}

// LookupPath traverses a nested map using a dot-separated path and returns the
// value at that path, or nil if any segment is missing or traverses through a
// non-map value.
func LookupPath(data map[string]any, path string) any {
	parts := strings.Split(path, ".")
	var current any = data
	for _, part := range parts {
		switch c := current.(type) {
		case map[string]any:
			current = c[part]
		default:
			return nil
		}
	}
	return current
}

// EvaluateExpression evaluates simple expressions against the provided
// dataStore. It supports:
//   - len(path)        — length of a slice/string/map looked up via LookupPath
//   - numeric literals — parsed via strconv.ParseFloat
//   - boolean literals — "true" / "false"
//   - everything else  — returned as the raw string
func EvaluateExpression(expr string, dataStore map[string]any) any {
	expr = strings.TrimSpace(expr)

	// len(path) — returns length of array or string
	if strings.HasPrefix(expr, "len(") && strings.HasSuffix(expr, ")") {
		path := strings.TrimSpace(expr[4 : len(expr)-1])
		val := LookupPath(dataStore, path)
		if val == nil {
			return 0
		}
		switch v := val.(type) {
		case []any:
			return len(v)
		case string:
			return len(v)
		case map[string]any:
			return len(v)
		default:
			return 0
		}
	}

	// Try to parse as number
	if num, err := strconv.ParseFloat(expr, 64); err == nil {
		return num
	}

	// Boolean literals
	if expr == "true" {
		return true
	}
	if expr == "false" {
		return false
	}

	// Return as string
	return expr
}

// EvaluateCondition evaluates a workflow condition against the execution
// context and dataStore. It supports literal "true"/"false", template-var
// substitution, and binary comparisons (==, !=, >=, <=, >, <). When the
// left-hand side is `task.status` and the execution carries a TaskID, the
// status is looked up via taskRepo (which may be nil to skip the lookup).
// Unrecognized expressions log a warning and default to true, preserving the
// historical behavior of the service-layer implementation.
func EvaluateCondition(
	ctx context.Context,
	exec *model.WorkflowExecution,
	expression string,
	dataStore map[string]any,
	taskRepo ConditionTaskResolver,
) bool {
	expression = strings.TrimSpace(expression)
	if expression == "" || expression == "true" {
		return true
	}
	if expression == "false" {
		return false
	}

	// Resolve template variables first
	expression = ResolveTemplateVars(expression, dataStore)

	// Re-check after resolution
	expression = strings.TrimSpace(expression)
	if expression == "true" {
		return true
	}
	if expression == "false" {
		return false
	}

	// Comparison operators: ==, !=, >, <, >=, <=
	for _, op := range []string{"==", "!=", ">=", "<=", ">", "<"} {
		if strings.Contains(expression, op) {
			parts := strings.SplitN(expression, op, 2)
			if len(parts) == 2 {
				left := strings.TrimSpace(parts[0])
				right := strings.Trim(strings.TrimSpace(parts[1]), "\"'")

				// Resolve left side: e.g., "task.status"
				if strings.HasPrefix(left, "task.status") && exec != nil && exec.TaskID != nil && taskRepo != nil {
					task, err := taskRepo.GetByID(ctx, *exec.TaskID)
					if err == nil {
						left = task.Status
					}
				}
				// Try resolving left as DataStore path
				if val := LookupPath(dataStore, left); val != nil {
					left = fmt.Sprintf("%v", val)
				}

				return compareValues(left, op, right)
			}
		}
	}

	log.WithField("expression", expression).Warn("workflow: unrecognized condition, defaulting to true")
	return true
}

// compareValues compares two string values using the given operator. It tries
// numeric comparison first (when both sides parse as float64) and falls back
// to lexicographic string comparison.
func compareValues(left, op, right string) bool {
	// Try numeric comparison
	leftNum, leftErr := strconv.ParseFloat(left, 64)
	rightNum, rightErr := strconv.ParseFloat(right, 64)

	if leftErr == nil && rightErr == nil {
		switch op {
		case "==":
			return leftNum == rightNum
		case "!=":
			return leftNum != rightNum
		case ">":
			return leftNum > rightNum
		case "<":
			return leftNum < rightNum
		case ">=":
			return leftNum >= rightNum
		case "<=":
			return leftNum <= rightNum
		}
	}

	// String comparison
	switch op {
	case "==":
		return left == right
	case "!=":
		return left != right
	case ">":
		return left > right
	case "<":
		return left < right
	case ">=":
		return left >= right
	case "<=":
		return left <= right
	}
	return false
}
