package nodetypes

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

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
