package nodetypes

import (
	"sort"
	"testing"

	"github.com/react-go-quick-starter/server/internal/model"
)

func TestFindNodesBetween(t *testing.T) {
	cases := []struct {
		name   string
		from   string
		to     string
		nodes  []model.WorkflowNode
		edges  []model.WorkflowEdge
		expect []string
	}{
		{
			name: "direct edge returns empty",
			from: "a",
			to:   "b",
			nodes: []model.WorkflowNode{
				{ID: "a"}, {ID: "b"},
			},
			edges: []model.WorkflowEdge{
				{Source: "a", Target: "b"},
			},
			expect: nil,
		},
		{
			name: "linear chain a->b->c",
			from: "a",
			to:   "c",
			nodes: []model.WorkflowNode{
				{ID: "a"}, {ID: "b"}, {ID: "c"},
			},
			edges: []model.WorkflowEdge{
				{Source: "a", Target: "b"},
				{Source: "b", Target: "c"},
			},
			expect: []string{"b"},
		},
		{
			name: "diamond a->b->d, a->c->d",
			from: "a",
			to:   "d",
			nodes: []model.WorkflowNode{
				{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"},
			},
			edges: []model.WorkflowEdge{
				{Source: "a", Target: "b"},
				{Source: "a", Target: "c"},
				{Source: "b", Target: "d"},
				{Source: "c", Target: "d"},
			},
			expect: []string{"b", "c"},
		},
		{
			name:   "fromID not in graph returns empty",
			from:   "missing",
			to:     "anything",
			nodes:  []model.WorkflowNode{{ID: "a"}, {ID: "b"}},
			edges:  []model.WorkflowEdge{{Source: "a", Target: "b"}},
			expect: nil,
		},
		{
			name: "toID unreachable returns all visited from fromID",
			from: "a",
			to:   "never",
			nodes: []model.WorkflowNode{
				{ID: "a"}, {ID: "b"}, {ID: "c"},
			},
			edges: []model.WorkflowEdge{
				{Source: "a", Target: "b"},
				{Source: "b", Target: "c"},
			},
			expect: []string{"b", "c"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := FindNodesBetween(tc.from, tc.to, tc.nodes, tc.edges)
			sort.Strings(got)
			sort.Strings(tc.expect)
			if !equalStringSlice(got, tc.expect) {
				t.Errorf("FindNodesBetween(%q, %q) = %v, want %v", tc.from, tc.to, got, tc.expect)
			}
		})
	}
}

func equalStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
