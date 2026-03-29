"use client";

import { useMemo } from "react";
import { useTranslations } from "next-intl";
import type { Task } from "@/lib/stores/task-store";

interface TaskDependencyGraphProps {
  tasks: Task[];
  onTaskClick: (taskId: string) => void;
}

interface GraphNode {
  id: string;
  title: string;
  status: Task["status"];
  layer: number;
  x: number;
  y: number;
}

interface GraphEdge {
  from: string;
  to: string;
  isCritical: boolean;
}

const NODE_WIDTH = 160;
const NODE_HEIGHT = 44;
const LAYER_GAP_X = 220;
const NODE_GAP_Y = 60;
const PADDING = 40;

function statusColor(status: Task["status"]): string {
  switch (status) {
    case "done":
      return "var(--color-emerald-500, #10b981)";
    case "in_progress":
      return "var(--color-blue-500, #3b82f6)";
    case "in_review":
      return "var(--color-violet-500, #8b5cf6)";
    case "blocked":
    case "budget_exceeded":
      return "var(--color-red-500, #ef4444)";
    case "assigned":
    case "triaged":
      return "var(--color-amber-500, #f59e0b)";
    default:
      return "var(--color-gray-400, #9ca3af)";
  }
}

function statusFill(status: Task["status"]): string {
  switch (status) {
    case "done":
      return "rgba(16,185,129,0.12)";
    case "in_progress":
      return "rgba(59,130,246,0.12)";
    case "in_review":
      return "rgba(139,92,246,0.12)";
    case "blocked":
    case "budget_exceeded":
      return "rgba(239,68,68,0.12)";
    case "assigned":
    case "triaged":
      return "rgba(245,158,11,0.12)";
    default:
      return "rgba(156,163,175,0.08)";
  }
}

function buildGraph(tasks: Task[]): { nodes: GraphNode[]; edges: GraphEdge[] } {
  const taskMap = new Map(tasks.map((t) => [t.id, t]));

  // Compute layers via topological depth
  const depth = new Map<string, number>();

  function getDepth(id: string, visited: Set<string>): number {
    if (depth.has(id)) return depth.get(id)!;
    if (visited.has(id)) return 0; // cycle guard
    visited.add(id);

    const task = taskMap.get(id);
    if (!task || task.blockedBy.length === 0) {
      depth.set(id, 0);
      return 0;
    }

    let maxParentDepth = 0;
    for (const blockerId of task.blockedBy) {
      if (taskMap.has(blockerId)) {
        maxParentDepth = Math.max(maxParentDepth, getDepth(blockerId, visited) + 1);
      }
    }
    depth.set(id, maxParentDepth);
    return maxParentDepth;
  }

  for (const task of tasks) {
    getDepth(task.id, new Set());
  }

  // Group by layer
  const layers = new Map<number, Task[]>();
  for (const task of tasks) {
    const layer = depth.get(task.id) ?? 0;
    if (!layers.has(layer)) layers.set(layer, []);
    layers.get(layer)!.push(task);
  }

  // Assign positions
  const nodes: GraphNode[] = [];
  const sortedLayers = [...layers.keys()].sort((a, b) => a - b);

  for (const layer of sortedLayers) {
    const layerTasks = layers.get(layer)!;
    layerTasks.sort((a, b) => a.title.localeCompare(b.title));

    for (let i = 0; i < layerTasks.length; i++) {
      const task = layerTasks[i];
      nodes.push({
        id: task.id,
        title: task.title.length > 22 ? task.title.slice(0, 20) + "..." : task.title,
        status: task.status,
        layer,
        x: PADDING + layer * LAYER_GAP_X,
        y: PADDING + i * NODE_GAP_Y,
      });
    }
  }

  // Build edges
  const edges: GraphEdge[] = [];
  for (const task of tasks) {
    for (const blockerId of task.blockedBy) {
      if (taskMap.has(blockerId)) {
        edges.push({ from: blockerId, to: task.id, isCritical: false });
      }
    }
  }

  // Find critical path (longest path)
  const longestPath = findLongestPath(tasks, taskMap, depth);
  const criticalEdges = new Set<string>();
  for (let i = 0; i < longestPath.length - 1; i++) {
    criticalEdges.add(`${longestPath[i]}->${longestPath[i + 1]}`);
  }
  for (const edge of edges) {
    if (criticalEdges.has(`${edge.from}->${edge.to}`)) {
      edge.isCritical = true;
    }
  }

  return { nodes, edges };
}

function findLongestPath(
  tasks: Task[],
  taskMap: Map<string, Task>,
  depth: Map<string, number>
): string[] {
  if (tasks.length === 0) return [];

  // Find the node with maximum depth
  let maxDepth = 0;
  let endNode = tasks[0].id;
  for (const [id, d] of depth) {
    if (d > maxDepth) {
      maxDepth = d;
      endNode = id;
    }
  }

  // Trace back from endNode
  const path: string[] = [endNode];
  let current = endNode;

  while (true) {
    const task = taskMap.get(current);
    if (!task || task.blockedBy.length === 0) break;

    // Pick the blocker with the highest depth
    let bestBlocker: string | null = null;
    let bestDepth = -1;
    for (const blockerId of task.blockedBy) {
      const d = depth.get(blockerId) ?? -1;
      if (d > bestDepth && taskMap.has(blockerId)) {
        bestDepth = d;
        bestBlocker = blockerId;
      }
    }

    if (!bestBlocker) break;
    path.unshift(bestBlocker);
    current = bestBlocker;
  }

  return path;
}

function bezierPath(
  x1: number,
  y1: number,
  x2: number,
  y2: number
): string {
  const cx = (x1 + x2) / 2;
  return `M ${x1} ${y1} C ${cx} ${y1}, ${cx} ${y2}, ${x2} ${y2}`;
}

export function TaskDependencyGraph({ tasks, onTaskClick }: TaskDependencyGraphProps) {
  const t = useTranslations("tasks");
  const { nodes, edges } = useMemo(() => buildGraph(tasks), [tasks]);

  const nodeMap = useMemo(
    () => new Map(nodes.map((n) => [n.id, n])),
    [nodes]
  );

  if (tasks.length === 0) {
    return (
      <div className="flex h-48 items-center justify-center text-sm text-muted-foreground">
        {t("empty.noTasksToVisualize")}
      </div>
    );
  }

  const maxX = Math.max(...nodes.map((n) => n.x)) + NODE_WIDTH + PADDING;
  const maxY = Math.max(...nodes.map((n) => n.y)) + NODE_HEIGHT + PADDING;

  return (
    <div className="overflow-auto rounded-md border bg-background">
      <svg
        viewBox={`0 0 ${maxX} ${maxY}`}
        width={maxX}
        height={maxY}
        className="min-w-full"
      >
        <defs>
          <marker
            id="arrowhead"
            markerWidth="8"
            markerHeight="6"
            refX="8"
            refY="3"
            orient="auto"
          >
            <polygon
              points="0 0, 8 3, 0 6"
              fill="var(--muted-foreground, #9ca3af)"
            />
          </marker>
          <marker
            id="arrowhead-critical"
            markerWidth="8"
            markerHeight="6"
            refX="8"
            refY="3"
            orient="auto"
          >
            <polygon
              points="0 0, 8 3, 0 6"
              fill="var(--color-blue-500, #3b82f6)"
            />
          </marker>
        </defs>

        {/* Edges */}
        {edges.map((edge) => {
          const fromNode = nodeMap.get(edge.from);
          const toNode = nodeMap.get(edge.to);
          if (!fromNode || !toNode) return null;

          const x1 = fromNode.x + NODE_WIDTH;
          const y1 = fromNode.y + NODE_HEIGHT / 2;
          const x2 = toNode.x;
          const y2 = toNode.y + NODE_HEIGHT / 2;

          return (
            <path
              key={`${edge.from}-${edge.to}`}
              d={bezierPath(x1, y1, x2, y2)}
              fill="none"
              stroke={
                edge.isCritical
                  ? "var(--color-blue-500, #3b82f6)"
                  : "var(--muted-foreground, #9ca3af)"
              }
              strokeWidth={edge.isCritical ? 2.5 : 1.5}
              strokeOpacity={edge.isCritical ? 0.9 : 0.4}
              markerEnd={
                edge.isCritical
                  ? "url(#arrowhead-critical)"
                  : "url(#arrowhead)"
              }
            />
          );
        })}

        {/* Nodes */}
        {nodes.map((node) => (
          <g
            key={node.id}
            transform={`translate(${node.x},${node.y})`}
            onClick={() => onTaskClick(node.id)}
            className="cursor-pointer"
            role="button"
            tabIndex={0}
            onKeyDown={(e) => {
              if (e.key === "Enter" || e.key === " ") onTaskClick(node.id);
            }}
          >
            <rect
              width={NODE_WIDTH}
              height={NODE_HEIGHT}
              rx={8}
              ry={8}
              fill={statusFill(node.status)}
              stroke={statusColor(node.status)}
              strokeWidth={2}
            />
            <text
              x={NODE_WIDTH / 2}
              y={NODE_HEIGHT / 2 - 4}
              textAnchor="middle"
              fontSize={12}
              fontWeight={500}
              fill="currentColor"
              className="fill-foreground"
            >
              {node.title}
            </text>
            <text
              x={NODE_WIDTH / 2}
              y={NODE_HEIGHT / 2 + 12}
              textAnchor="middle"
              fontSize={10}
              fill={statusColor(node.status)}
            >
              {node.status.replace("_", " ")}
            </text>
          </g>
        ))}
      </svg>
    </div>
  );
}
