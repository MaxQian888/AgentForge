"use client";

import {
  getBezierPath,
  EdgeLabelRenderer,
  MarkerType,
  type EdgeProps,
} from "@xyflow/react";

export function CustomEdge({
  id,
  sourceX,
  sourceY,
  targetX,
  targetY,
  sourcePosition,
  targetPosition,
  selected,
  label,
  data,
}: EdgeProps) {
  const [edgePath, labelX, labelY] = getBezierPath({
    sourceX,
    sourceY,
    sourcePosition,
    targetX,
    targetY,
    targetPosition,
  });

  const strokeColor = selected ? "#3b82f6" : "#6b7280";
  const strokeWidth = selected ? 3 : 2;

  const displayLabel =
    (data as { condition?: string } | undefined)?.condition ??
    (typeof label === "string" ? label : undefined);

  return (
    <>
      {/* Invisible wide path for easier click targeting */}
      <path
        id={`${id}-click`}
        d={edgePath}
        fill="none"
        strokeWidth={20}
        stroke="transparent"
        opacity={0}
        style={{ cursor: "pointer" }}
      />

      {/* Visible edge path */}
      <path
        id={id}
        d={edgePath}
        fill="none"
        stroke={strokeColor}
        strokeWidth={strokeWidth}
        markerEnd={`url(#${MarkerType.ArrowClosed})`}
        style={{ transition: "stroke 0.15s, stroke-width 0.15s" }}
      />

      {/* Edge label */}
      {displayLabel && (
        <EdgeLabelRenderer>
          <div
            style={{
              position: "absolute",
              transform: `translate(-50%, -50%) translate(${labelX}px, ${labelY}px)`,
              pointerEvents: "all",
            }}
            className="nodrag nopan"
          >
            <span className="bg-background border rounded-full px-2 py-0.5 text-xs shadow-sm">
              {displayLabel}
            </span>
          </div>
        </EdgeLabelRenderer>
      )}
    </>
  );
}

export const customEdgeTypes = {
  default: CustomEdge,
};
