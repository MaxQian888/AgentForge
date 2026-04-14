import type { Node } from "@xyflow/react";
import type { ReactElement } from "react";

export interface SnapResult {
  xLines: number[];
  yLines: number[];
}

/**
 * Calculates alignment snap lines for a node being dragged, based on the
 * positions of neighbouring nodes.
 */
export function calculateSnapLines(
  draggingNodeId: string,
  draggingX: number,
  draggingY: number,
  allNodes: Node[],
  threshold = 8,
  maxNeighbors = 20
): SnapResult {
  // Exclude the node currently being dragged
  const others = allNodes.filter((n) => n.id !== draggingNodeId);

  // Sort by distance to the dragging position and take the nearest N
  const nearest = others
    .map((n) => {
      const dx = n.position.x - draggingX;
      const dy = n.position.y - draggingY;
      return { node: n, dist: Math.sqrt(dx * dx + dy * dy) };
    })
    .sort((a, b) => a.dist - b.dist)
    .slice(0, maxNeighbors)
    .map((r) => r.node);

  const xSet = new Set<number>();
  const ySet = new Set<number>();

  for (const neighbor of nearest) {
    if (Math.abs(neighbor.position.x - draggingX) < threshold) {
      xSet.add(neighbor.position.x);
    }
    if (Math.abs(neighbor.position.y - draggingY) < threshold) {
      ySet.add(neighbor.position.y);
    }
  }

  return {
    xLines: Array.from(xSet),
    yLines: Array.from(ySet),
  };
}

/**
 * Renders SVG snap guide lines inside the ReactFlow viewport.
 * Uses absolute positioning in the same coordinate system as the canvas.
 */
export function SnapLines({ xLines, yLines }: SnapResult): ReactElement {
  return (
    <svg
      style={{
        position: "absolute",
        top: 0,
        left: 0,
        width: "100%",
        height: "100%",
        pointerEvents: "none",
        overflow: "visible",
        zIndex: 10,
      }}
    >
      {xLines.map((x) => (
        <line
          key={`x-${x}`}
          x1={x}
          y1={-10000}
          x2={x}
          y2={10000}
          stroke="#93c5fd"
          strokeWidth={1}
          strokeDasharray="4 4"
          opacity={0.8}
        />
      ))}
      {yLines.map((y) => (
        <line
          key={`y-${y}`}
          x1={-10000}
          y1={y}
          x2={10000}
          y2={y}
          stroke="#93c5fd"
          strokeWidth={1}
          strokeDasharray="4 4"
          opacity={0.8}
        />
      ))}
    </svg>
  );
}
