import type { Node, Edge } from "@xyflow/react";

// ── findPredecessors ──────────────────────────────────────────────────────────

/**
 * BFS backward traversal from `nodeId`.
 * Returns all predecessor nodes ordered by topological distance
 * (direct predecessors first, then their predecessors, etc.).
 * Cycle-safe via a visited set.
 */
export function findPredecessors(
  nodeId: string,
  nodes: Node[],
  edges: Edge[]
): Node[] {
  const nodeMap = new Map<string, Node>(nodes.map((n) => [n.id, n]));
  const visited = new Set<string>();
  const result: Node[] = [];

  // BFS queue — each entry is a node ID to process at the current level
  let queue: string[] = [nodeId];
  visited.add(nodeId);

  while (queue.length > 0) {
    const nextQueue: string[] = [];

    for (const currentId of queue) {
      // Find all edges whose target is the current node
      const incomingEdges = edges.filter((e) => e.target === currentId);

      for (const edge of incomingEdges) {
        const sourceId = edge.source;
        if (!visited.has(sourceId)) {
          visited.add(sourceId);
          const sourceNode = nodeMap.get(sourceId);
          if (sourceNode) {
            result.push(sourceNode);
            nextQueue.push(sourceId);
          }
        }
      }
    }

    queue = nextQueue;
  }

  return result;
}

// ── getUpstreamOutputFields ───────────────────────────────────────────────────

/**
 * Returns known output-field hints for a given node type.
 * The `copyTemplate` strings use `{{nodeId.path}}` syntax for use in
 * expression fields and config panels.
 */
export function getUpstreamOutputFields(
  nodeId: string,
  nodeType: string
): { path: string; copyTemplate: string }[] {
  switch (nodeType) {
    case "llm_agent":
    case "agent_dispatch":
    case "function":
    case "condition":
    case "gate":
      return [{ path: "output", copyTemplate: `{{${nodeId}.output}}` }];

    default:
      return [{ path: "output.*", copyTemplate: `{{${nodeId}.output}}` }];
  }
}
