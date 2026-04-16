package nodetypes

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
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

// fakeTaskRepo satisfies ConditionTaskResolver for testing task.status lookups.
type fakeTaskRepo struct {
	status string
	called bool
}

func (f *fakeTaskRepo) GetByID(_ context.Context, _ uuid.UUID) (*model.Task, error) {
	f.called = true
	return &model.Task{Status: f.status}, nil
}

func TestEvaluateCondition(t *testing.T) {
	ctx := context.Background()
	taskID := uuid.New()
	exec := &model.WorkflowExecution{TaskID: &taskID}
	dsCount := map[string]any{
		"input": map[string]any{"count": float64(10)},
	}

	tests := []struct {
		name       string
		expression string
		ds         map[string]any
		repo       ConditionTaskResolver
		exec       *model.WorkflowExecution
		want       bool
	}{
		{"empty → true", "", nil, nil, exec, true},
		{"literal true", "true", nil, nil, exec, true},
		{"literal false", "false", nil, nil, exec, false},
		{"literal true with whitespace", "  true  ", nil, nil, exec, true},
		{"numeric ==", "5 == 5", nil, nil, exec, true},
		{"numeric !=", "5 != 6", nil, nil, exec, true},
		{"numeric >", "10 > 5", nil, nil, exec, true},
		{"numeric <", "5 < 10", nil, nil, exec, true},
		{"numeric >=", "10 >= 10", nil, nil, exec, true},
		{"numeric <=", "5 <= 10", nil, nil, exec, true},
		// Note: existing implementation strips quotes from RHS only; LHS is
		// looked up via DataStore/task.status. So a literal-vs-literal string
		// comparison only matches if the LHS is unquoted.
		{"string == (unquoted lhs)", `hi == "hi"`, nil, nil, exec, true},
		{"string != (unquoted lhs)", `hi != "bye"`, nil, nil, exec, true},
		{"template var resolution",
			"{{input.count}} > 5",
			dsCount, nil, exec, true},
		{"template var resolution false branch",
			"{{input.count}} < 5",
			dsCount, nil, exec, false},
		{"unrecognized → true (no panic)",
			"some random expression with no operator",
			nil, nil, exec, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := EvaluateCondition(ctx, tc.exec, tc.expression, tc.ds, tc.repo)
			if got != tc.want {
				t.Errorf("EvaluateCondition(%q) = %v, want %v", tc.expression, got, tc.want)
			}
		})
	}
}

func TestEvaluateCondition_TaskStatus(t *testing.T) {
	ctx := context.Background()
	taskID := uuid.New()
	exec := &model.WorkflowExecution{TaskID: &taskID}

	repo := &fakeTaskRepo{status: "done"}
	got := EvaluateCondition(ctx, exec, `task.status == "done"`, nil, repo)
	if !got {
		t.Errorf("EvaluateCondition(task.status == done) = false, want true")
	}
	if !repo.called {
		t.Errorf("expected taskRepo.GetByID to be called")
	}

	repo2 := &fakeTaskRepo{status: "in_progress"}
	got = EvaluateCondition(ctx, exec, `task.status == "done"`, nil, repo2)
	if got {
		t.Errorf("EvaluateCondition(in_progress vs done) = true, want false")
	}
}

func TestEvaluateCondition_NilRepoIsSafe(t *testing.T) {
	ctx := context.Background()
	taskID := uuid.New()
	exec := &model.WorkflowExecution{TaskID: &taskID}

	// With no repo, task.status falls through to LookupPath (which returns nil),
	// so the left side stays the literal "task.status" and compares unequally.
	got := EvaluateCondition(ctx, exec, `task.status == "done"`, nil, nil)
	if got {
		t.Errorf("EvaluateCondition with nil repo should be false (literal vs done), got true")
	}
}

func TestCompareValues(t *testing.T) {
	tests := []struct {
		name           string
		left, op, right string
		want           bool
	}{
		{"numeric ==", "5", "==", "5", true},
		{"numeric != true", "5", "!=", "6", true},
		{"numeric >", "10", ">", "5", true},
		{"numeric <", "5", "<", "10", true},
		{"numeric >= eq", "5", ">=", "5", true},
		{"numeric <= eq", "5", "<=", "5", true},
		{"string ==", "hi", "==", "hi", true},
		{"string !=", "hi", "!=", "bye", true},
		{"unknown op falls through to false", "a", "??", "b", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := compareValues(tc.left, tc.op, tc.right)
			if got != tc.want {
				t.Errorf("compareValues(%q,%q,%q) = %v, want %v", tc.left, tc.op, tc.right, got, tc.want)
			}
		})
	}
}
