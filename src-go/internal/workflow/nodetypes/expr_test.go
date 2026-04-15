package nodetypes

import (
	"strings"
	"testing"
)

func TestResolveTemplateVars(t *testing.T) {
	dataStore := map[string]any{
		"node1": map[string]any{
			"output": map[string]any{
				"field":  "hello",
				"count":  float64(7),
				"nested": map[string]any{"key": "value"},
			},
		},
		"top": "world",
	}

	tests := []struct {
		name     string
		template string
		want     string
	}{
		{
			name:     "top-level string hit",
			template: "{{top}}",
			want:     "world",
		},
		{
			name:     "nested string hit",
			template: "x={{node1.output.field}}",
			want:     "x=hello",
		},
		{
			name:     "missing path keeps original",
			template: "{{nope.missing}}",
			want:     "{{nope.missing}}",
		},
		{
			name:     "numeric value JSON-serialized",
			template: "n={{node1.output.count}}",
			want:     "n=7",
		},
		{
			name:     "map value JSON-serialized",
			template: "m={{node1.output.nested}}",
			want:     `m={"key":"value"}`,
		},
		{
			name:     "no template vars",
			template: "literal text",
			want:     "literal text",
		},
		{
			name:     "multiple substitutions",
			template: "{{top}}-{{node1.output.field}}",
			want:     "world-hello",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveTemplateVars(tc.template, dataStore)
			if got != tc.want {
				t.Errorf("ResolveTemplateVars(%q) = %q, want %q", tc.template, got, tc.want)
			}
		})
	}
}

func TestLookupPath(t *testing.T) {
	data := map[string]any{
		"a": "alpha",
		"b": map[string]any{
			"c": "gamma",
			"d": map[string]any{
				"e": float64(42),
			},
		},
		"scalar": "value",
	}

	tests := []struct {
		name string
		path string
		want any
	}{
		{"top-level hit", "a", "alpha"},
		{"nested hit", "b.c", "gamma"},
		{"deep nested hit", "b.d.e", float64(42)},
		{"missing top-level", "missing", nil},
		{"missing intermediate segment", "b.missing.x", nil},
		{"traverse through scalar returns nil", "scalar.x", nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := LookupPath(data, tc.path)
			if got != tc.want {
				t.Errorf("LookupPath(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

func TestEvaluateExpression(t *testing.T) {
	dataStore := map[string]any{
		"items":   []any{"a", "b", "c"},
		"name":    "hello",
		"mapping": map[string]any{"k1": 1, "k2": 2},
	}

	tests := []struct {
		name string
		expr string
		want any
	}{
		{"len of slice", "len(items)", 3},
		{"len of string", "len(name)", 5},
		{"len of map", "len(mapping)", 2},
		{"len of missing path", "len(missing)", 0},
		{"numeric literal", "3.14", 3.14},
		{"boolean true", "true", true},
		{"boolean false", "false", false},
		{"unrecognized passes through as string", "some unrecognized text", "some unrecognized text"},
		{"trims whitespace", "  true  ", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := EvaluateExpression(tc.expr, dataStore)
			if got != tc.want {
				t.Errorf("EvaluateExpression(%q) = %v (%T), want %v (%T)", tc.expr, got, got, tc.want, tc.want)
			}
		})
	}
}

// Sanity check that nested ResolveTemplateVars + EvaluateExpression compose.
func TestResolveAndEvaluate_Composes(t *testing.T) {
	ds := map[string]any{
		"items": []any{1, 2, 3, 4},
	}
	resolved := ResolveTemplateVars("len(items)", ds)
	got := EvaluateExpression(resolved, ds)
	if got != 4 {
		t.Errorf("len(items) via Evaluate = %v, want 4", got)
	}
	if !strings.HasPrefix(resolved, "len(") {
		t.Errorf("expected resolved to remain %q-like, got %q", "len(items)", resolved)
	}
}
