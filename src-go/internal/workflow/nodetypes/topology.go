package nodetypes

import "github.com/agentforge/server/internal/model"

// FindNodesBetween returns node IDs reachable from fromID on paths that lead
// toward toID (exclusive of toID). Used by loop handlers to determine which
// nodes to reset on iteration.
//
// The traversal is a BFS from fromID over the provided edge list; visited
// nodes other than fromID itself are collected. toID acts as a sink: the
// traversal does not cross through it, and it is never included in the
// returned slice. If fromID is not reachable in the graph, or if toID is
// unreachable, the result still contains every node reachable from fromID
// (minus fromID itself).
func FindNodesBetween(fromID, toID string, nodes []model.WorkflowNode, edges []model.WorkflowEdge) []string {
	_ = nodes // kept in the signature for symmetry with the caller; BFS uses edges only.

	adjacency := make(map[string][]string)
	for _, e := range edges {
		adjacency[e.Source] = append(adjacency[e.Source], e.Target)
	}

	visited := make(map[string]bool)
	var result []string
	queue := []string{fromID}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if visited[current] || current == toID {
			continue
		}
		visited[current] = true
		if current != fromID {
			result = append(result, current)
		}
		for _, next := range adjacency[current] {
			if !visited[next] {
				queue = append(queue, next)
			}
		}
	}
	return result
}
